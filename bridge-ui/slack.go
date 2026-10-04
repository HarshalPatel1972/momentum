package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Slack works like Telegram: the question is a DM with answer buttons, and a
// thread reply is a free-text answer. Taps and replies arrive over Socket
// Mode, a WebSocket that Momentum opens to Slack, so nothing is exposed.
//
// The user creates a small Slack app from slackManifest (bot token xoxb-…,
// app-level token xapp-… with connections:write), DMs it once, and Momentum
// detects who they are.

var slackHTTP = &http.Client{Timeout: 20 * time.Second}

func slackAPIBase() string {
	if b := os.Getenv("MOMENTUM_SLACK_API"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return "https://slack.com/api"
}

// slackCall invokes a Web API method. Errors never contain the token.
func slackCall(ctx context.Context, token, method string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackAPIBase()+"/"+method, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := slackHTTP.Do(req)
	if err != nil {
		return errors.New(redactToken(err.Error(), token))
	}
	defer resp.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("slack %s: %s", method, resp.Status)
	}
	var status struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	json.Unmarshal(raw, &status)
	if !status.OK {
		if status.Error == "" {
			status.Error = resp.Status
		}
		return fmt.Errorf("slack: %s", slackErrorText(status.Error))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// slackErrorText turns Slack's error codes into something a person can act on.
func slackErrorText(code string) string {
	switch code {
	case "invalid_auth", "not_authed", "token_revoked", "account_inactive":
		return "the token was rejected (" + code + "). Copy it again from your Slack app settings"
	case "not_allowed_token_type":
		return "wrong kind of token. The bot token starts with xoxb-, the app token with xapp-"
	case "missing_scope":
		return "the Slack app is missing a permission. Recreate it from Momentum's app manifest"
	case "channel_not_found":
		return "can't reach your DM. Send the Momentum app a message in Slack, then detect again"
	}
	return code
}

// slackEscape escapes text for Slack mrkdwn.
func slackEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func slackQuestionBlocks(q *pendingQuestion, status string) []any {
	head := "*🤖 Input needed*"
	if src := sourceLabel(q); src != "" {
		head += "\n_" + slackEscape(src) + "_"
	}
	blocks := []any{
		map[string]any{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": head + "\n\n" + slackEscape(q.Question)}},
	}
	if status != "" {
		return append(blocks, map[string]any{"type": "context", "elements": []any{map[string]any{"type": "mrkdwn", "text": status}}})
	}
	var buttons []any
	for i, opt := range q.Options {
		b := map[string]any{
			"type":      "button",
			"text":      map[string]any{"type": "plain_text", "text": truncate(opt, 75), "emoji": true},
			"value":     fmt.Sprintf("a:%s:%d", q.ID, i),
			"action_id": fmt.Sprintf("momentum_%d", i),
		}
		if i == 0 {
			b["style"] = "primary"
		}
		buttons = append(buttons, b)
	}
	return append(blocks,
		map[string]any{"type": "actions", "block_id": "momentum_" + q.ID, "elements": buttons},
		map[string]any{"type": "context", "elements": []any{map[string]any{"type": "mrkdwn", "text": "💬 Tap a button, or reply in thread to type an answer."}}},
	)
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func slackTarget(sl SlackConfig) string {
	if sl.ChannelID != "" {
		return sl.ChannelID
	}
	return sl.UserID // posting to a user ID lands in the app's DM
}

// sendSlackQuestion posts the question with answer buttons; returns the DM channel and message ts.
func sendSlackQuestion(ctx context.Context, sl SlackConfig, q *pendingQuestion) (string, string, error) {
	var out struct {
		Channel string `json:"channel"`
		TS      string `json:"ts"`
	}
	err := slackCall(ctx, sl.BotToken, "chat.postMessage", map[string]any{
		"channel": slackTarget(sl),
		"text":    "Input needed: " + q.Question, // notification / fallback text
		"blocks":  slackQuestionBlocks(q, ""),
	}, &out)
	return out.Channel, out.TS, err
}

// closeSlackQuestion replaces the buttons with the outcome.
func closeSlackQuestion(sl SlackConfig, q *pendingQuestion, state, answer string) {
	status := map[string]string{
		stateAnswered: "✅ *Answered:* " + slackEscape(answer),
		stateExpired:  "⌛ *Expired* without an answer (treated as not approved)",
		stateStopped:  "⏹ *Momentum stopped* before this was answered",
		stateCanceled: "🚫 *The agent stopped waiting* (its IDE was closed or the request was cancelled)",
	}[state]
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	slackCall(ctx, sl.BotToken, "chat.update", map[string]any{
		"channel": q.slackChannel,
		"ts":      q.slackTS,
		"text":    "Input needed: " + q.Question,
		"blocks":  slackQuestionBlocks(q, status),
	}, nil)
}

func slackSay(ctx context.Context, sl SlackConfig, channel, threadTS, text string) {
	p := map[string]any{"channel": channel, "text": text}
	if threadTS != "" {
		p["thread_ts"] = threadTS
	}
	slackCall(ctx, sl.BotToken, "chat.postMessage", p, nil)
}

// ---------- Socket Mode ----------

type slackEnvelope struct {
	Type       string          `json:"type"`
	EnvelopeID string          `json:"envelope_id"`
	Payload    json.RawMessage `json:"payload"`
	Reason     string          `json:"reason"`
}

type slackMessageEvent struct {
	Type        string `json:"type"`
	Subtype     string `json:"subtype"`
	ChannelType string `json:"channel_type"`
	Channel     string `json:"channel"`
	User        string `json:"user"`
	BotID       string `json:"bot_id"`
	Text        string `json:"text"`
	TS          string `json:"ts"`
	ThreadTS    string `json:"thread_ts"`
}

type slackBlockActions struct {
	Type string `json:"type"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
	Actions []struct {
		Value string `json:"value"`
	} `json:"actions"`
}

// slackConnect opens a Socket Mode WebSocket using the app-level token.
func slackConnect(ctx context.Context, appToken string) (*websocket.Conn, error) {
	var open struct {
		URL string `json:"url"`
	}
	if err := slackCall(ctx, appToken, "apps.connections.open", map[string]any{}, &open); err != nil {
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, open.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("slack socket: %v", err)
	}
	return conn, nil
}

// slackRead reads envelopes until the socket closes, acking each one, and
// passes events to handle. It returns when Slack asks us to reconnect.
func slackRead(ctx context.Context, conn *websocket.Conn, handle func(env slackEnvelope)) error {
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	for {
		var env slackEnvelope
		if err := conn.ReadJSON(&env); err != nil {
			return err
		}
		if env.EnvelopeID != "" {
			// Ack first: Slack retries anything not acked within 3 seconds.
			conn.WriteJSON(map[string]string{"envelope_id": env.EnvelopeID})
		}
		if env.Type == "disconnect" {
			return errors.New("slack asked to reconnect (" + env.Reason + ")")
		}
		handle(env)
	}
}

// slackLoop keeps a Socket Mode connection open and routes taps and replies to questions.
func (h *Hub) slackLoop(ctx context.Context, sl SlackConfig, ready chan struct{}) {
	readyOnce := func() {
		select {
		case <-ready:
		default:
			close(ready)
		}
	}
	defer readyOnce()
	backoff := time.Second
	failing := false
	for ctx.Err() == nil {
		conn, err := slackConnect(ctx, sl.AppToken)
		readyOnce() // don't hold questions back while Slack is unreachable
		if err != nil {
			if !failing {
				h.log("⚠️ Slack connection: %v (retrying)", err)
				failing = true
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		if failing {
			h.log("✅ Slack connection recovered")
			failing = false
		}
		backoff = time.Second
		err = slackRead(ctx, conn, func(env slackEnvelope) { h.handleSlackEnvelope(ctx, sl, env) })
		conn.Close()
		if ctx.Err() == nil && err != nil && !strings.Contains(err.Error(), "reconnect") {
			h.log("⚠️ Slack connection dropped: %v (reconnecting)", err)
		}
	}
}

func (h *Hub) handleSlackEnvelope(ctx context.Context, sl SlackConfig, env slackEnvelope) {
	switch env.Type {
	case "interactive":
		var p slackBlockActions
		if json.Unmarshal(env.Payload, &p) != nil || p.Type != "block_actions" || len(p.Actions) == 0 {
			return
		}
		if p.User.ID != sl.UserID {
			return // only the linked person may answer
		}
		parts := strings.Split(p.Actions[0].Value, ":")
		if len(parts) != 3 || parts[0] != "a" {
			return
		}
		idx, _ := strconv.Atoi(parts[2])
		h.mu.Lock()
		q := h.questions[parts[1]]
		ok := q != nil && idx >= 0 && idx < len(q.Options) && h.finishLocked(q, stateAnswered, q.Options[idx])
		h.mu.Unlock()
		if ok {
			h.log("📥 [%s] answered: %s", orDash(sourceLabel(q)), q.Options[idx])
		}

	case "events_api":
		var p struct {
			Event slackMessageEvent `json:"event"`
		}
		if json.Unmarshal(env.Payload, &p) != nil {
			return
		}
		m := p.Event
		if m.Type != "message" || m.ChannelType != "im" || m.BotID != "" || m.Subtype != "" || m.User != sl.UserID {
			return
		}
		text := strings.TrimSpace(m.Text)
		if text == "" {
			return
		}
		sent, _ := strconv.ParseFloat(m.TS, 64)
		h.mu.Lock()
		var target *pendingQuestion
		waiting := 0
		for _, q := range h.questions {
			// A message older than the question can't be answering it.
			if q.state != stateWaiting || (sent > 0 && int64(sent) < q.Created.Unix()) {
				continue
			}
			waiting++
			if m.ThreadTS != "" && q.slackTS == m.ThreadTS {
				target = q
			}
		}
		if target == nil && m.ThreadTS == "" && waiting == 1 {
			for _, q := range h.questions {
				if q.state == stateWaiting && (sent == 0 || int64(sent) >= q.Created.Unix()) {
					target = q
				}
			}
		}
		ok := target != nil && h.finishLocked(target, stateAnswered, text)
		h.mu.Unlock()
		thread := m.ThreadTS
		switch {
		case ok:
			h.log("📥 [%s] answered: %s", orDash(sourceLabel(target)), text)
			slackSay(ctx, sl, m.Channel, target.slackTS, "✅ Sent to "+slackEscape(orDash(sourceLabel(target))))
		case thread != "":
			slackSay(ctx, sl, m.Channel, thread, "That question is already closed.")
		case waiting == 0:
			slackSay(ctx, sl, m.Channel, "", "✅ Momentum is connected. Questions from your AI agents will appear here.")
		default:
			slackSay(ctx, sl, m.Channel, "", fmt.Sprintf("%d questions are open. Reply in the thread of the one you're answering.", waiting))
		}
	}
}

// ---------- setup ----------

type SlackLink struct {
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	ChannelID string `json:"channelId"`
	Team      string `json:"team"`
	Error     string `json:"error,omitempty"`
}

// DetectSlackUser checks both tokens, then waits for the user to DM the app
// and returns who they are. The user is asked to send any message first.
func DetectSlackUser(ctx context.Context, botToken, appToken string, wait time.Duration) SlackLink {
	botToken, appToken = strings.TrimSpace(botToken), strings.TrimSpace(appToken)
	switch {
	case !strings.HasPrefix(botToken, "xoxb-"):
		return SlackLink{Error: "The bot token should start with xoxb- (OAuth & Permissions → Bot User OAuth Token)"}
	case !strings.HasPrefix(appToken, "xapp-"):
		return SlackLink{Error: "The app token should start with xapp- (Basic Information → App-Level Tokens)"}
	}
	var auth struct {
		Team   string `json:"team"`
		UserID string `json:"user_id"`
	}
	if err := slackCall(ctx, botToken, "auth.test", map[string]any{}, &auth); err != nil {
		return SlackLink{Error: "Bot token: " + strings.TrimPrefix(err.Error(), "slack: ")}
	}
	conn, err := slackConnect(ctx, appToken)
	if err != nil {
		return SlackLink{Error: "App token: " + strings.TrimPrefix(err.Error(), "slack: ")}
	}
	defer conn.Close()

	wctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	found := make(chan slackMessageEvent, 1)
	go slackRead(wctx, conn, func(env slackEnvelope) {
		if env.Type != "events_api" {
			return
		}
		var p struct {
			Event slackMessageEvent `json:"event"`
		}
		json.Unmarshal(env.Payload, &p)
		m := p.Event
		if m.Type == "message" && m.ChannelType == "im" && m.BotID == "" && m.Subtype == "" && m.User != "" && m.User != auth.UserID {
			select {
			case found <- m:
			default:
			}
		}
	})
	select {
	case m := <-found:
		link := SlackLink{UserID: m.User, ChannelID: m.Channel, Team: auth.Team, UserName: m.User}
		var info struct {
			User struct {
				RealName string `json:"real_name"`
				Profile  struct {
					DisplayName string `json:"display_name"`
				} `json:"profile"`
			} `json:"user"`
		}
		if slackCall(ctx, botToken, "users.info", map[string]any{"user": m.User}, &info) == nil {
			if n := info.User.Profile.DisplayName; n != "" {
				link.UserName = n
			} else if info.User.RealName != "" {
				link.UserName = info.User.RealName
			}
		}
		slackSay(ctx, SlackConfig{BotToken: botToken}, m.Channel, "", "✅ Momentum is linked to you. Questions from your AI agents will appear here.")
		return link
	case <-wctx.Done():
		return SlackLink{Error: "No message yet. In Slack, open the Momentum app (under Apps), send it any message, then click Detect again."}
	}
}

// slackManifest is the Slack app definition users create the app from.
// Scopes are the minimum Momentum needs.
const slackManifest = `{
  "display_information": {
    "name": "Momentum",
    "description": "Answer your AI coding agents' questions from Slack.",
    "background_color": "#4a3fd1"
  },
  "features": {
    "app_home": { "messages_tab_enabled": true, "messages_tab_read_only_enabled": false },
    "bot_user": { "display_name": "Momentum", "always_online": true }
  },
  "oauth_config": { "scopes": { "bot": ["chat:write", "im:history", "users:read"] } },
  "settings": {
    "event_subscriptions": { "bot_events": ["message.im"] },
    "interactivity": { "is_enabled": true },
    "org_deploy_enabled": false,
    "socket_mode_enabled": true,
    "token_rotation_enabled": false
  }
}`

// SlackCreateAppURL opens Slack's "create app" page with the manifest pre-filled.
func SlackCreateAppURL() string {
	var compact map[string]any
	json.Unmarshal([]byte(slackManifest), &compact)
	b, _ := json.Marshal(compact)
	return "https://api.slack.com/apps?new_app=1&manifest_json=" + url.QueryEscape(string(b))
}
