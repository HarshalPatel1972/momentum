package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Telegram is answered entirely inside the chat: options are inline buttons,
// and a reply to the message is a free-text answer. Momentum long-polls the
// Bot API for those taps, so no tunnel, public URL or inbound connection is needed.

var tgClient = &http.Client{Timeout: 60 * time.Second}

const tgPollSeconds = 25

type tgUser struct {
	ID int64 `json:"id"`
}

type tgChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type tgMessage struct {
	MessageID int64      `json:"message_id"`
	Date      int64      `json:"date"` // unix seconds
	From      *tgUser    `json:"from"`
	Chat      tgChat     `json:"chat"`
	Text      string     `json:"text"`
	ReplyTo   *tgMessage `json:"reply_to_message"`
}

type tgCallback struct {
	ID      string     `json:"id"`
	From    tgUser     `json:"from"`
	Message *tgMessage `json:"message"`
	Data    string     `json:"data"`
}

type tgUpdate struct {
	UpdateID int64       `json:"update_id"`
	Message  *tgMessage  `json:"message"`
	Callback *tgCallback `json:"callback_query"`
}

// tgCall invokes a Bot API method. Errors never contain the bot token.
func tgCall(ctx context.Context, token, method string, payload any, result any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/%s", telegramAPIBase(), token, method), strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := tgClient.Do(req)
	if err != nil {
		return errors.New(redactToken(err.Error(), token))
	}
	defer resp.Body.Close()
	var out struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("telegram %s: %s", method, resp.Status)
	}
	if !out.OK {
		if out.Description == "" {
			out.Description = resp.Status
		}
		return fmt.Errorf("telegram: %s", out.Description)
	}
	if result != nil {
		return json.Unmarshal(out.Result, result)
	}
	return nil
}

func chatName(c tgChat) string {
	switch {
	case c.Title != "":
		return c.Title
	case c.Username != "":
		return "@" + c.Username
	default:
		return c.FirstName
	}
}

// questionText renders the message body; status is appended once the question closes.
func questionText(q *pendingQuestion, status string) string {
	var b strings.Builder
	b.WriteString("<b>🤖 Input needed</b>")
	if src := sourceLabel(q); src != "" {
		b.WriteString("\n<i>" + html.EscapeString(src) + "</i>")
	}
	b.WriteString("\n\n" + html.EscapeString(q.Question))
	if status == "" {
		b.WriteString("\n\n💬 <i>Tap a button, or reply to this message to type an answer.</i>")
	} else {
		b.WriteString("\n\n" + status)
	}
	return b.String()
}

func answerKeyboard(q *pendingQuestion) map[string]any {
	var rows [][]map[string]string
	var row []map[string]string
	for i, opt := range q.Options {
		row = append(row, map[string]string{"text": opt, "callback_data": buttonValue(q, i)})
		// Short options sit two per row; long ones get a row each.
		if len(row) == 2 || len(opt) > 18 || (i+1 < len(q.Options) && len(q.Options[i+1]) > 18) {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return map[string]any{"inline_keyboard": rows}
}

type telegramChannel struct{ cfg TelegramConfig }

func (t *telegramChannel) Send(ctx context.Context, q *pendingQuestion) (string, error) {
	var msg tgMessage
	err := tgCall(ctx, t.cfg.BotToken, "sendMessage", map[string]any{
		"chat_id":                  t.cfg.ChatID,
		"text":                     questionText(q, ""),
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
		"reply_markup":             answerKeyboard(q),
	}, &msg)
	return strconv.FormatInt(msg.MessageID, 10), err
}

// Close replaces the buttons with the outcome so old messages can't be tapped.
func (t *telegramChannel) Close(ctx context.Context, q *pendingQuestion, ref, state, answer string) {
	status := statusEmoji[state] + " <b>" + html.EscapeString(statusText(state, answer)) + "</b>"
	id, _ := strconv.ParseInt(ref, 10, 64)
	tgCall(ctx, t.cfg.BotToken, "editMessageText", map[string]any{
		"chat_id":      t.cfg.ChatID,
		"message_id":   id,
		"text":         questionText(q, status),
		"parse_mode":   "HTML",
		"reply_markup": map[string]any{"inline_keyboard": [][]any{}},
	}, nil)
}

func tgSendText(ctx context.Context, tg TelegramConfig, text string, replyTo int64) error {
	p := map[string]any{"chat_id": tg.ChatID, "text": text, "parse_mode": "HTML"}
	if replyTo != 0 {
		p["reply_parameters"] = map[string]any{"message_id": replyTo}
	}
	return tgCall(ctx, tg.BotToken, "sendMessage", p, nil)
}

// Run long-polls for button taps and replies until ctx is cancelled.
func (t *telegramChannel) Run(ctx context.Context, host *ChannelHost, ready func()) {
	tg := t.cfg
	tgCall(ctx, tg.BotToken, "deleteWebhook", map[string]any{}, nil) // getUpdates is refused while a webhook is set
	// Telegram keeps undelivered updates for 24h. Anything sent before we
	// started listening can't be an answer to a question we're about to ask,
	// so skip it rather than let an old "ok" approve something new.
	var offset int64
	var backlog []tgUpdate
	if tgCall(ctx, tg.BotToken, "getUpdates", map[string]any{"timeout": 0}, &backlog) == nil && len(backlog) > 0 {
		offset = backlog[len(backlog)-1].UpdateID + 1
		host.Logf("ℹ️ Skipped %d Telegram message(s) sent before Momentum started", len(backlog))
	}
	ready()
	backoff := time.Second
	failing := false
	for ctx.Err() == nil {
		var updates []tgUpdate
		pctx, cancel := context.WithTimeout(ctx, (tgPollSeconds+15)*time.Second)
		err := tgCall(pctx, tg.BotToken, "getUpdates", map[string]any{
			"offset":          offset,
			"timeout":         tgPollSeconds,
			"allowed_updates": []string{"message", "callback_query"},
		}, &updates)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if !failing {
				host.Logf("⚠️ Telegram polling: %v (retrying)", err)
				failing = true
			}
			if !backoffWait(ctx, &backoff) {
				return
			}
			continue
		}
		if failing {
			host.Logf("✅ Telegram polling recovered")
			failing = false
		}
		backoff = time.Second
		for _, u := range updates {
			offset = u.UpdateID + 1
			t.handle(ctx, host, u)
		}
	}
}

func (t *telegramChannel) handle(ctx context.Context, host *ChannelHost, u tgUpdate) {
	tg := t.cfg
	switch {
	case u.Callback != nil:
		cb := u.Callback
		reply := func(text string) {
			tgCall(ctx, tg.BotToken, "answerCallbackQuery", map[string]any{"callback_query_id": cb.ID, "text": text}, nil)
		}
		// Only the configured chat may answer (in a group, any member of it).
		if cb.Message == nil || strconv.FormatInt(cb.Message.Chat.ID, 10) != tg.ChatID {
			reply("Not allowed")
			return
		}
		switch res, answer := host.TapButton(cb.Data); res {
		case TapAnswered:
			reply("✅ Sent: " + answer)
		case TapClosed:
			reply("Already answered")
		default:
			reply("This question has expired")
		}

	case u.Message != nil:
		m := u.Message
		rememberTelegramChat(tg.BotToken, TelegramChat{ID: strconv.FormatInt(m.Chat.ID, 10), Name: chatName(m.Chat)})
		if strconv.FormatInt(m.Chat.ID, 10) != tg.ChatID {
			return // ignore other chats entirely
		}
		text := strings.TrimSpace(m.Text)
		if text == "" {
			return
		}
		if strings.HasPrefix(text, "/") {
			tgSendText(ctx, tg, "✅ Momentum is connected. Questions from your AI agents will appear here.", 0)
			return
		}
		replyTo := ""
		if m.ReplyTo != nil {
			replyTo = strconv.FormatInt(m.ReplyTo.MessageID, 10)
		}
		var sent time.Time
		if m.Date != 0 {
			sent = time.Unix(m.Date, 0)
		}
		res := host.Text(replyTo, text, sent)
		tgSendText(ctx, tg, html.EscapeString(res.Reply()), m.MessageID)
	}
}

// The last chat that messaged each bot, so Detect still works when a running
// listener consumed the message. Only recent messages count: Detect is meant
// for "I just pressed Start", not someone who wrote to the bot hours ago.
const recentChatWindow = 10 * time.Minute

type seenChat struct {
	chat TelegramChat
	at   time.Time
}

var (
	recentChatsMu sync.Mutex
	recentChats   = map[string]seenChat{} // bot token -> last chat that messaged it
)

func rememberTelegramChat(token string, c TelegramChat) {
	recentChatsMu.Lock()
	recentChats[token] = seenChat{c, time.Now()}
	recentChatsMu.Unlock()
}

func recentTelegramChat(token string) (TelegramChat, bool) {
	recentChatsMu.Lock()
	defer recentChatsMu.Unlock()
	s, ok := recentChats[token]
	return s.chat, ok && time.Since(s.at) < recentChatWindow
}

type TelegramChat struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Bot   string `json:"bot"` // bot @username
	Error string `json:"error,omitempty"`
}

// DetectTelegramChat finds the chat that most recently messaged the bot, so
// setup only needs the bot token: the user sends /start, then clicks Detect.
func DetectTelegramChat(ctx context.Context, token string) TelegramChat {
	token = strings.TrimSpace(token)
	if token == "" {
		return TelegramChat{Error: "Enter the bot token first"}
	}
	var me struct {
		Username string `json:"username"`
	}
	if err := tgCall(ctx, token, "getMe", map[string]any{}, &me); err != nil {
		return TelegramChat{Error: "Bot token rejected by Telegram: " + err.Error()}
	}
	// A running listener may already have consumed the message; it remembers the sender.
	if chat, ok := recentTelegramChat(token); ok {
		chat.Bot = me.Username
		tgSendText(ctx, TelegramConfig{BotToken: token, ChatID: chat.ID},
			"✅ Momentum is linked to this chat. Questions from your AI agents will appear here.", 0)
		return chat
	}
	tgCall(ctx, token, "deleteWebhook", map[string]any{}, nil)
	var updates []tgUpdate
	if err := tgCall(ctx, token, "getUpdates", map[string]any{"timeout": 0, "allowed_updates": []string{"message"}}, &updates); err != nil {
		return TelegramChat{Error: err.Error()}
	}
	for i := len(updates) - 1; i >= 0; i-- {
		if m := updates[i].Message; m != nil {
			chat := TelegramChat{ID: strconv.FormatInt(m.Chat.ID, 10), Name: chatName(m.Chat), Bot: me.Username}
			tgSendText(ctx, TelegramConfig{BotToken: token, ChatID: chat.ID},
				"✅ Momentum is linked to this chat. Questions from your AI agents will appear here.", 0)
			return chat
		}
	}
	return TelegramChat{Error: fmt.Sprintf("No messages yet. Open @%s in Telegram, press Start (or send any message), then try again.", me.Username)}
}
