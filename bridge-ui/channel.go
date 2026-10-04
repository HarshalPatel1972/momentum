package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A Channel is a messaging app that shows a question with answer buttons and
// reports the user's taps and replies back to the hub. Every channel connects
// *out* to its service (long-polling or a WebSocket), so nothing on the PC is
// exposed. Adapters only translate; the answering rules live in ChannelHost.
type Channel interface {
	// Run listens for answers until ctx ends. It calls ready once answers can
	// be received (or the first attempt failed), so the hub never sends a
	// question it couldn't hear the answer to.
	Run(ctx context.Context, host *ChannelHost, ready func())
	// Send posts the question with its options as buttons and returns a
	// reference to the posted message.
	Send(ctx context.Context, q *pendingQuestion) (ref string, err error)
	// Close updates the posted message with the outcome and removes the buttons.
	Close(ctx context.Context, q *pendingQuestion, ref, state, answer string)
}

// buttonChannels are answered inside the app; the rest (WhatsApp) send a link.
var buttonChannels = map[string]bool{"telegram": true, "slack": true, "discord": true, "ntfy": true}

// newChannel builds the configured channel; key identifies its settings so the
// hub restarts it only when they change. nil means a link-based channel.
func newChannel(cfg BridgeConfig) (Channel, string) {
	if configProblem(cfg) != "" {
		return nil, ""
	}
	switch cfg.Channel {
	case "telegram":
		return &telegramChannel{cfg: cfg.Telegram}, "telegram|" + cfg.Telegram.BotToken + "|" + cfg.Telegram.ChatID
	case "slack":
		s := cfg.Slack
		return &slackChannel{cfg: s}, "slack|" + s.BotToken + "|" + s.AppToken + "|" + s.UserID
	case "discord":
		d := cfg.Discord
		return &discordChannel{cfg: d}, "discord|" + d.BotToken + "|" + d.UserID + "|" + d.ChannelID
	case "ntfy":
		n := cfg.Ntfy
		return &ntfyChannel{cfg: n}, "ntfy|" + n.server() + "|" + n.Topic + "|" + n.Token
	}
	return nil, ""
}

// statusLine is the outcome shown on a closed question, in plain words.
var statusLine = map[string]string{
	stateAnswered: "Answered: %s",
	stateExpired:  "Expired without an answer (treated as not approved)",
	stateStopped:  "Momentum stopped before this was answered",
	stateCanceled: "The agent stopped waiting (its IDE was closed or the request was cancelled)",
}

var statusEmoji = map[string]string{stateAnswered: "✅", stateExpired: "⌛", stateStopped: "⏹", stateCanceled: "🚫"}

// statusText is the closed-question line, e.g. "Answered: Approve".
func statusText(state, answer string) string {
	if state == stateAnswered {
		return fmt.Sprintf(statusLine[state], answer)
	}
	return statusLine[state]
}

// buttonValue is the payload of option i's button; parseButton reverses it.
func buttonValue(q *pendingQuestion, i int) string { return fmt.Sprintf("a:%s:%d", q.ID, i) }

func parseButton(v string) (id string, idx int, ok bool) {
	parts := strings.Split(v, ":")
	if len(parts) < 3 || parts[0] != "a" {
		return "", 0, false
	}
	idx, err := strconv.Atoi(parts[2])
	return parts[1], idx, err == nil
}

// ChannelHost applies the answering rules shared by every channel.
type ChannelHost struct {
	h    *Hub
	name string
}

func (c *ChannelHost) Logf(format string, args ...any) { c.h.log(format, args...) }

type TapResult int

const (
	TapAnswered TapResult = iota
	TapClosed             // already answered, expired or stopped
	TapUnknown            // no such question (e.g. from before a restart)
)

// Tap records a button press for option idx of question id.
func (c *ChannelHost) Tap(id string, idx int) (TapResult, string) {
	h := c.h
	h.mu.Lock()
	q := h.questions[id]
	if q == nil || idx < 0 || idx >= len(q.Options) {
		h.mu.Unlock()
		return TapUnknown, ""
	}
	answer := q.Options[idx]
	ok := h.finishLocked(q, stateAnswered, answer)
	h.mu.Unlock()
	if !ok {
		return TapClosed, ""
	}
	h.log("📥 [%s] answered via %s: %s", orDash(sourceLabel(q)), c.name, answer)
	return TapAnswered, answer
}

// TapButton handles a raw button payload from buttonValue.
func (c *ChannelHost) TapButton(value string) (TapResult, string) {
	id, idx, ok := parseButton(value)
	if !ok {
		return TapUnknown, ""
	}
	return c.Tap(id, idx)
}

// Secret returns a question's unguessable token, for channels whose button
// payloads can be seen by others and so must prove they came from the question.
func (c *ChannelHost) Secret(id string) string {
	c.h.mu.Lock()
	defer c.h.mu.Unlock()
	if q := c.h.questions[id]; q != nil {
		return q.Token
	}
	return ""
}

// HasRef reports whether a waiting question has this message ref (ntfy uses
// short codes as refs, so typed answers can say which question they answer).
func (c *ChannelHost) HasRef(ref string) bool {
	c.h.mu.Lock()
	defer c.h.mu.Unlock()
	for _, q := range c.h.questions {
		if q.ref == ref && q.state == stateWaiting {
			return true
		}
	}
	return false
}

// TextResult says what a typed message did, so the channel can tell the user.
type TextResult struct {
	Answered *pendingQuestion
	Closed   bool // it replied to a question that is already closed
	Open     int  // open questions when nothing was answered
}

// Reply is a short, plain confirmation for the user (channels escape it as needed).
func (r TextResult) Reply() string {
	switch {
	case r.Answered != nil:
		return "✅ Sent to " + orDash(sourceLabel(r.Answered))
	case r.Closed:
		return "That question is already closed."
	case r.Open == 0:
		return "There are no open questions right now."
	default:
		return fmt.Sprintf("%d questions are open. Reply directly to the one you're answering.", r.Open)
	}
}

// Text handles a typed message. replyTo is the ref of the message it replies to
// ("" if none). A reply answers that question; a plain message answers the only
// open question if there is exactly one. Messages sent before a question was
// asked never answer it.
func (c *ChannelHost) Text(replyTo, text string, sent time.Time) TextResult {
	h := c.h
	text = strings.TrimSpace(text)
	h.mu.Lock()
	var target *pendingQuestion
	open := 0
	for _, q := range h.questions {
		if q.state != stateWaiting || (!sent.IsZero() && sent.Unix() < q.Created.Unix()) {
			continue
		}
		open++
		if replyTo != "" && q.ref == replyTo {
			target = q
		}
	}
	if target == nil && replyTo == "" && open == 1 {
		for _, q := range h.questions {
			if q.state == stateWaiting && (sent.IsZero() || sent.Unix() >= q.Created.Unix()) {
				target = q
			}
		}
	}
	ok := target != nil && text != "" && h.finishLocked(target, stateAnswered, text)
	h.mu.Unlock()
	if ok {
		h.log("📥 [%s] answered via %s: %s", orDash(sourceLabel(target)), c.name, text)
		return TextResult{Answered: target}
	}
	// A reply that didn't land means its question is closed (answered, expired, or from before a restart).
	return TextResult{Closed: replyTo != "", Open: open}
}

// backoffWait sleeps for the current backoff (or until ctx ends) and doubles it.
func backoffWait(ctx context.Context, d *time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(*d):
	}
	*d = min(*d*2, 30*time.Second)
	return true
}
