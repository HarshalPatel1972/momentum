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
		row = append(row, map[string]string{"text": opt, "callback_data": fmt.Sprintf("a:%s:%d", q.ID, i)})
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

// sendTelegramQuestion posts the question with answer buttons and returns the message id.
func sendTelegramQuestion(ctx context.Context, tg TelegramConfig, q *pendingQuestion) (int64, error) {
	var msg tgMessage
	err := tgCall(ctx, tg.BotToken, "sendMessage", map[string]any{
		"chat_id":                  tg.ChatID,
		"text":                     questionText(q, ""),
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
		"reply_markup":             answerKeyboard(q),
	}, &msg)
	return msg.MessageID, err
}

// closeTelegramQuestion replaces the buttons with the outcome so old messages can't be tapped.
func closeTelegramQuestion(tg TelegramConfig, q *pendingQuestion, state, answer string) {
	status := map[string]string{
		stateAnswered: "✅ <b>Answered:</b> " + html.EscapeString(answer),
		stateExpired:  "⌛ <b>Expired</b> without an answer (treated as not approved)",
		stateStopped:  "⏹ <b>Momentum stopped</b> before this was answered",
		stateCanceled: "🚫 <b>The agent stopped waiting</b> (its IDE was closed or the request was cancelled)",
	}[state]
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tgCall(ctx, tg.BotToken, "editMessageText", map[string]any{
		"chat_id":      tg.ChatID,
		"message_id":   q.tgMessageID,
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

// telegramLoop long-polls for button taps and replies until ctx is cancelled.
func (h *Hub) telegramLoop(ctx context.Context, tg TelegramConfig, ready chan struct{}) {
	tgCall(ctx, tg.BotToken, "deleteWebhook", map[string]any{}, nil) // getUpdates is refused while a webhook is set
	// Telegram keeps undelivered updates for 24h. Anything sent before we
	// started listening can't be an answer to a question we're about to ask,
	// so skip it rather than let an old "ok" approve something new.
	var offset int64
	var backlog []tgUpdate
	if tgCall(ctx, tg.BotToken, "getUpdates", map[string]any{"timeout": 0}, &backlog) == nil && len(backlog) > 0 {
		offset = backlog[len(backlog)-1].UpdateID + 1
		h.log("ℹ️ Skipped %d Telegram message(s) sent before Momentum started", len(backlog))
	}
	close(ready)
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
				h.log("⚠️ Telegram polling: %v (retrying)", err)
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
			h.log("✅ Telegram polling recovered")
			failing = false
		}
		backoff = time.Second
		for _, u := range updates {
			offset = u.UpdateID + 1
			h.handleTelegramUpdate(ctx, tg, u)
		}
	}
}

func (h *Hub) handleTelegramUpdate(ctx context.Context, tg TelegramConfig, u tgUpdate) {
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
		var id string
		var idx int
		parts := strings.Split(cb.Data, ":")
		if len(parts) == 3 && parts[0] == "a" {
			id = parts[1]
			idx, _ = strconv.Atoi(parts[2])
		}
		h.mu.Lock()
		q := h.questions[id]
		ok := q != nil && idx >= 0 && idx < len(q.Options) && h.finishLocked(q, stateAnswered, q.Options[idx])
		h.mu.Unlock()
		switch {
		case ok:
			h.log("📥 [%s] answered: %s", orDash(sourceLabel(q)), q.Options[idx])
			reply("✅ Sent: " + q.Options[idx])
		case q == nil:
			reply("This question has expired")
		default:
			reply("Already answered")
		}

	case u.Message != nil:
		m := u.Message
		h.mu.Lock()
		h.lastTgChat = TelegramChat{ID: strconv.FormatInt(m.Chat.ID, 10), Name: chatName(m.Chat)}
		h.mu.Unlock()
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
		// A reply to a question answers that question; a plain message answers
		// the only open question, if there is exactly one.
		h.mu.Lock()
		var target *pendingQuestion
		waiting := 0
		for _, q := range h.questions {
			// A message older than the question can't be answering it.
			if q.state != stateWaiting || (m.Date != 0 && m.Date < q.Created.Unix()) {
				continue
			}
			waiting++
			if m.ReplyTo != nil && q.tgMessageID == m.ReplyTo.MessageID {
				target = q
			}
		}
		if target == nil && m.ReplyTo == nil && waiting == 1 {
			for _, q := range h.questions {
				if q.state == stateWaiting && (m.Date == 0 || m.Date >= q.Created.Unix()) {
					target = q
				}
			}
		}
		ok := target != nil && h.finishLocked(target, stateAnswered, text)
		h.mu.Unlock()
		switch {
		case ok:
			h.log("📥 [%s] answered: %s", orDash(sourceLabel(target)), text)
			tgSendText(ctx, tg, "✅ Sent to "+html.EscapeString(orDash(sourceLabel(target))), m.MessageID)
		case m.ReplyTo != nil:
			tgSendText(ctx, tg, "That question is already closed.", m.MessageID)
		case waiting == 0:
			tgSendText(ctx, tg, "There are no open questions right now.", m.MessageID)
		default:
			tgSendText(ctx, tg, fmt.Sprintf("%d questions are open. Reply directly to the one you're answering.", waiting), m.MessageID)
		}
	}
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
