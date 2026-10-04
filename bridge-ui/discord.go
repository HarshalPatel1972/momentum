package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Discord works like Telegram and Slack: the question is a DM with answer
// buttons; replying to it is a free-text answer. Taps and replies arrive over
// the Gateway, a WebSocket Momentum opens to Discord, so nothing is exposed.
// The bot only needs the (non-privileged) Direct Messages intent.

var discordHTTP = &http.Client{Timeout: 20 * time.Second}

const discordIntentDirectMessages = 1 << 12

func discordAPIBase() string {
	if b := os.Getenv("MOMENTUM_DISCORD_API"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return "https://discord.com/api/v10"
}

// discordCall invokes a REST endpoint. Errors never contain the token.
func discordCall(ctx context.Context, token, method, path string, payload any, out any) error {
	var body *strings.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = strings.NewReader(string(b))
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, discordAPIBase()+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DiscordBot (https://github.com/HarshalPatel1972/momentum, "+Version+")")
	resp, err := discordHTTP.Do(req)
	if err != nil {
		return errors.New(redactToken(err.Error(), token))
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return errors.New("discord: the bot token was rejected. Copy it again (Bot → Reset Token)")
		case http.StatusForbidden:
			return fmt.Errorf("discord: not allowed (%s). Make sure you share a server with the bot", e.Message)
		}
		if e.Message == "" {
			e.Message = resp.Status
		}
		return fmt.Errorf("discord: %s", e.Message)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

type discordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Bot        bool   `json:"bot"`
}

func (u discordUser) name() string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

type discordMessage struct {
	ID        string      `json:"id"`
	ChannelID string      `json:"channel_id"`
	GuildID   string      `json:"guild_id"`
	Author    discordUser `json:"author"`
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
	Reference *struct {
		MessageID string `json:"message_id"`
	} `json:"message_reference"`
}

type discordInteraction struct {
	ID        string       `json:"id"`
	Token     string       `json:"token"`
	Type      int          `json:"type"` // 3 = message component (button)
	ChannelID string       `json:"channel_id"`
	User      *discordUser `json:"user"`
	Member    *struct {
		User discordUser `json:"user"`
	} `json:"member"`
	Data struct {
		CustomID string `json:"custom_id"`
	} `json:"data"`
}

func (i discordInteraction) userID() string {
	if i.User != nil {
		return i.User.ID
	}
	if i.Member != nil {
		return i.Member.User.ID
	}
	return ""
}

// ---------- gateway ----------

type discordGateway struct {
	conn *websocket.Conn
	wmu  sync.Mutex
	seq  *int64
	smu  sync.Mutex
}

func (g *discordGateway) send(v any) error {
	g.wmu.Lock()
	defer g.wmu.Unlock()
	return g.conn.WriteJSON(v)
}

type discordPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int64          `json:"s"`
	T  string          `json:"t"`
}

// discordConnect opens the Gateway, identifies with the DM intent and keeps
// the heartbeat going. Dispatch events are passed to handle until the socket
// closes or Discord asks us to reconnect.
func discordConnect(ctx context.Context, token string, onReady func(), handle func(t string, d json.RawMessage)) error {
	var gw struct {
		URL string `json:"url"`
	}
	if err := discordCall(ctx, token, http.MethodGet, "/gateway/bot", nil, &gw); err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, gw.URL+"?v=10&encoding=json", nil)
	if err != nil {
		return fmt.Errorf("discord gateway: %v", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	g := &discordGateway{conn: conn}

	var hello discordPayload
	if err := conn.ReadJSON(&hello); err != nil || hello.Op != 10 {
		return fmt.Errorf("discord gateway: no hello (%v)", err)
	}
	var hb struct {
		Interval float64 `json:"heartbeat_interval"`
	}
	json.Unmarshal(hello.D, &hb)
	g.send(map[string]any{"op": 2, "d": map[string]any{
		"token":      token,
		"intents":    discordIntentDirectMessages,
		"properties": map[string]string{"os": "windows", "browser": "momentum", "device": "momentum"},
	}})

	hctx, hcancel := context.WithCancel(ctx)
	defer hcancel()
	go func() {
		t := time.NewTicker(time.Duration(hb.Interval) * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-hctx.Done():
				return
			case <-t.C:
				g.smu.Lock()
				seq := g.seq
				g.smu.Unlock()
				if g.send(map[string]any{"op": 1, "d": seq}) != nil {
					conn.Close()
					return
				}
			}
		}
	}()

	for {
		var p discordPayload
		if err := conn.ReadJSON(&p); err != nil {
			return err
		}
		if p.S != nil {
			g.smu.Lock()
			g.seq = p.S
			g.smu.Unlock()
		}
		switch p.Op {
		case 0:
			if p.T == "READY" && onReady != nil {
				onReady()
			}
			handle(p.T, p.D)
		case 1: // Discord wants a heartbeat now
			g.smu.Lock()
			seq := g.seq
			g.smu.Unlock()
			g.send(map[string]any{"op": 1, "d": seq})
		case 7:
			return errors.New("reconnect requested")
		case 9:
			return errors.New("invalid session")
		}
	}
}

// ---------- channel ----------

type discordChannel struct{ cfg DiscordConfig }

func discordRef(channel, msg string) string { return channel + "|" + msg }

func discordButtons(q *pendingQuestion) []any {
	var rows []any
	var row []any
	for i, opt := range q.Options {
		style := 2 // secondary
		if i == 0 {
			style = 1 // primary
		}
		row = append(row, map[string]any{"type": 2, "style": style, "label": truncate(opt, 80), "custom_id": buttonValue(q, i)})
		if len(row) == 5 {
			rows = append(rows, map[string]any{"type": 1, "components": row})
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, map[string]any{"type": 1, "components": row})
	}
	return rows
}

func discordMessageBody(q *pendingQuestion, status string) map[string]any {
	embed := map[string]any{
		"title":       "🤖 Input needed",
		"description": q.Question,
		"color":       0x6a6dff,
	}
	if src := sourceLabel(q); src != "" {
		embed["footer"] = map[string]any{"text": src}
	}
	body := map[string]any{"embeds": []any{embed}}
	if status == "" {
		body["content"] = "**Input needed** · " + orDash(sourceLabel(q))
		body["components"] = discordButtons(q)
		embed["description"] = q.Question + "\n\n💬 *Tap a button, or reply to this message to type an answer.*"
	} else {
		body["components"] = []any{}
		embed["description"] = q.Question + "\n\n" + status
	}
	return body
}

func (c *discordChannel) Send(ctx context.Context, q *pendingQuestion) (string, error) {
	var m discordMessage
	err := discordCall(ctx, c.cfg.BotToken, http.MethodPost, "/channels/"+c.cfg.ChannelID+"/messages", discordMessageBody(q, ""), &m)
	return discordRef(c.cfg.ChannelID, m.ID), err
}

func (c *discordChannel) Close(ctx context.Context, q *pendingQuestion, ref, state, answer string) {
	channel, msg, _ := strings.Cut(ref, "|")
	status := statusEmoji[state] + " **" + statusText(state, answer) + "**"
	discordCall(ctx, c.cfg.BotToken, http.MethodPatch, "/channels/"+channel+"/messages/"+msg, discordMessageBody(q, status), nil)
}

func (c *discordChannel) say(ctx context.Context, channel, replyTo, text string) {
	body := map[string]any{"content": text, "allowed_mentions": map[string]any{"parse": []string{}}}
	if replyTo != "" {
		body["message_reference"] = map[string]any{"message_id": replyTo, "fail_if_not_exists": false}
	}
	discordCall(ctx, c.cfg.BotToken, http.MethodPost, "/channels/"+channel+"/messages", body, nil)
}

func (c *discordChannel) Run(ctx context.Context, host *ChannelHost, ready func()) {
	backoff := time.Second
	failing := false
	for ctx.Err() == nil {
		err := discordConnect(ctx, c.cfg.BotToken, func() {
			ready()
			if failing {
				host.Logf("✅ Discord connection recovered")
				failing = false
			}
			backoff = time.Second
		}, func(t string, d json.RawMessage) { c.handle(ctx, host, t, d) })
		ready() // don't hold questions back while Discord is unreachable
		if ctx.Err() != nil {
			return
		}
		if err != nil && !strings.Contains(err.Error(), "reconnect requested") && !failing {
			host.Logf("⚠️ Discord connection: %v (retrying)", err)
			failing = true
		}
		if !backoffWait(ctx, &backoff) {
			return
		}
	}
}

func (c *discordChannel) handle(ctx context.Context, host *ChannelHost, t string, d json.RawMessage) {
	switch t {
	case "INTERACTION_CREATE":
		var in discordInteraction
		if json.Unmarshal(d, &in) != nil || in.Type != 3 {
			return
		}
		respond := func(body map[string]any) {
			discordCall(ctx, c.cfg.BotToken, http.MethodPost, "/interactions/"+in.ID+"/"+in.Token+"/callback", body, nil)
		}
		ephemeral := func(text string) {
			respond(map[string]any{"type": 4, "data": map[string]any{"content": text, "flags": 64}})
		}
		if in.userID() != c.cfg.UserID {
			ephemeral("Only the person who set up Momentum can answer.")
			return
		}
		switch res, _ := host.TapButton(in.Data.CustomID); res {
		case TapAnswered:
			respond(map[string]any{"type": 6}) // acknowledge; Close edits the message
		case TapClosed:
			ephemeral("This question was already answered.")
		default:
			ephemeral("This question has expired.")
		}

	case "MESSAGE_CREATE":
		var m discordMessage
		if json.Unmarshal(d, &m) != nil || m.GuildID != "" || m.Author.Bot || m.Author.ID != c.cfg.UserID {
			return
		}
		replyTo := ""
		if m.Reference != nil && m.Reference.MessageID != "" {
			replyTo = discordRef(m.ChannelID, m.Reference.MessageID)
		}
		res := host.Text(replyTo, m.Content, m.Timestamp)
		c.say(ctx, m.ChannelID, m.ID, res.Reply())
	}
}

// ---------- setup ----------

type DiscordLink struct {
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	ChannelID string `json:"channelId"`
	Bot       string `json:"bot"`
	Error     string `json:"error,omitempty"`
}

// DiscordInviteURL is the link that adds the bot to one of the user's servers,
// which Discord requires before a user can DM a bot. No permissions are requested.
func DiscordInviteURL(ctx context.Context, token string) (string, error) {
	var app struct {
		ID string `json:"id"`
	}
	if err := discordCall(ctx, strings.TrimSpace(token), http.MethodGet, "/oauth2/applications/@me", nil, &app); err != nil {
		return "", err
	}
	return "https://discord.com/oauth2/authorize?client_id=" + app.ID + "&scope=bot&permissions=0", nil
}

// DetectDiscordUser checks the token, then waits for the user to DM the bot.
func DetectDiscordUser(ctx context.Context, token string, wait time.Duration) DiscordLink {
	token = strings.TrimSpace(token)
	var me discordUser
	if err := discordCall(ctx, token, http.MethodGet, "/users/@me", nil, &me); err != nil {
		return DiscordLink{Error: strings.TrimPrefix(err.Error(), "discord: ")}
	}
	wctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	found := make(chan discordMessage, 1)
	go discordConnect(wctx, token, nil, func(t string, d json.RawMessage) {
		var m discordMessage
		if t == "MESSAGE_CREATE" && json.Unmarshal(d, &m) == nil && m.GuildID == "" && !m.Author.Bot && m.Author.ID != me.ID {
			select {
			case found <- m:
			default:
			}
		}
	})
	select {
	case m := <-found:
		link := DiscordLink{UserID: m.Author.ID, UserName: m.Author.name(), ChannelID: m.ChannelID, Bot: me.Username}
		(&discordChannel{cfg: DiscordConfig{BotToken: token}}).say(ctx, m.ChannelID, "",
			"✅ Momentum is linked to you. Questions from your AI agents will appear here.")
		return link
	case <-wctx.Done():
		return DiscordLink{Error: fmt.Sprintf("No message yet. In Discord, open a DM with %s and send it any message (click Detect first, then send).", me.Username)}
	}
}
