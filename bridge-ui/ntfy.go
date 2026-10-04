package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ntfy (ntfy.sh, or a self-hosted server) is a free, open-source push app
// with no account. The question is a push notification with up to three
// action buttons; tapping one makes the phone publish the answer to a second,
// secret topic that Momentum subscribes to over an outgoing HTTP stream.
//
// Typed answers: the ntfy app's message box posts into the topic itself, so
// Momentum listens there too. A message (that isn't one of Momentum's own,
// which always have a title) answers the only open question, or the question
// whose short code it starts with when several are open.
//
// Security: the topic is the key. Anyone who knows it can read the questions
// and post into it, so it is long and random and must stay private. Button
// payloads also carry the question's token, so taps can't be replayed or
// guessed by anyone who hasn't seen the question.
//
// Limit: at most 3 buttons per notification.

var ntfyHTTP = &http.Client{Timeout: 20 * time.Second}

const ntfyMaxActions = 3

type ntfyChannel struct{ cfg NtfyConfig }

func (n NtfyConfig) answersTopic() string { return n.Topic + "-answers" }

func (n NtfyConfig) authorize(req *http.Request) {
	if n.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.Token)
	}
}

// NewNtfyTopic returns a fresh unguessable topic name.
func NewNtfyTopic() string { return "momentum-" + randomHex(12) }

func (c *ntfyChannel) publish(ctx context.Context, msg map[string]any) (string, error) {
	msg["topic"] = c.cfg.Topic
	b, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.server(), strings.NewReader(string(b)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.cfg.authorize(req)
	resp, err := ntfyHTTP.Do(req)
	if err != nil {
		return "", errors.New(redactToken(err.Error(), c.cfg.Token))
	}
	defer resp.Body.Close()
	var out struct {
		ID    string `json:"id"`
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 300 {
		if out.Error == "" {
			out.Error = resp.Status
		}
		return "", fmt.Errorf("ntfy: %s", out.Error)
	}
	return out.ID, nil
}

// ntfyCode is the short code a typed answer can start with to pick its question.
func ntfyCode(q *pendingQuestion) string { return strings.ToUpper(q.Token[:4]) }

func (c *ntfyChannel) Send(ctx context.Context, q *pendingQuestion) (string, error) {
	title := "🤖 Input needed"
	if src := sourceLabel(q); src != "" {
		title += " · " + src
	}
	body := q.Question
	opts := q.Options
	if len(opts) > ntfyMaxActions {
		body += "\n\nOptions: " + strings.Join(opts, " / ") + "\n(ntfy shows the first " + fmt.Sprint(ntfyMaxActions) + " as buttons)"
		opts = opts[:ntfyMaxActions]
	}
	var actions []any
	for i, opt := range opts {
		a := map[string]any{
			"action": "http",
			"label":  truncate(opt, 40),
			"url":    c.cfg.server() + "/" + c.cfg.answersTopic(),
			"method": "POST",
			// The question's secret token proves this tap came from the real notification.
			"body":  buttonValue(q, i) + ":" + q.Token,
			"clear": true,
		}
		if c.cfg.Token != "" {
			a["headers"] = map[string]string{"Authorization": "Bearer " + c.cfg.Token}
		}
		actions = append(actions, a)
	}
	body += "\n\n💬 Or type an answer in this topic. If several questions are open, start it with " + ntfyCode(q) + "."
	_, err := c.publish(ctx, map[string]any{
		"title":    title,
		"message":  body,
		"priority": 4,
		"tags":     []string{"robot"},
		"actions":  actions,
	})
	// The ref is the code: typed answers that start with it are matched to this question.
	return ntfyCode(q), err
}

// say posts a short confirmation into the topic. It has a title, so Momentum
// recognises it as its own and never reads it as an answer.
func (c *ntfyChannel) say(text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c.publish(ctx, map[string]any{"title": "Momentum", "message": text, "priority": 2})
}

// Close: ntfy can't edit a sent notification. Tapping a button already
// dismisses it ("clear"), so there is nothing to update.
func (c *ntfyChannel) Close(ctx context.Context, q *pendingQuestion, ref, state, answer string) {}

// Run subscribes to the topic and the answers topic: verified taps and typed
// messages become answers.
func (c *ntfyChannel) Run(ctx context.Context, host *ChannelHost, ready func()) {
	since := fmt.Sprint(time.Now().Unix()) // never replay anything from before we started
	backoff := time.Second
	failing := false
	for ctx.Err() == nil {
		err := c.subscribe(ctx, since, ready, func(ev ntfyEvent) {
			since = ev.ID
			if ev.Topic == c.cfg.answersTopic() {
				c.handleTap(host, ev.Message)
			} else if ev.Topic == c.cfg.Topic && ev.Title == "" {
				c.handleText(host, ev)
			}
		})
		ready() // don't hold questions back while ntfy is unreachable
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			if !failing {
				host.Logf("⚠️ ntfy connection: %v (retrying)", err)
				failing = true
			}
		} else if failing {
			failing = false
		}
		if !backoffWait(ctx, &backoff) {
			return
		}
	}
}

type ntfyEvent struct {
	ID      string `json:"id"`
	Time    int64  `json:"time"`
	Event   string `json:"event"`
	Topic   string `json:"topic"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

func (c *ntfyChannel) subscribe(ctx context.Context, since string, ready func(), onMessage func(ntfyEvent)) error {
	topics := c.cfg.Topic + "," + c.cfg.answersTopic()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.server()+"/"+topics+"/json?since="+since, nil)
	if err != nil {
		return err
	}
	c.cfg.authorize(req)
	resp, err := (&http.Client{}).Do(req) // no timeout: this is a long-lived stream
	if err != nil {
		return errors.New(redactToken(err.Error(), c.cfg.Token))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ntfy: %s", resp.Status)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var ev ntfyEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		switch ev.Event {
		case "open":
			ready()
		case "message":
			onMessage(ev)
		}
	}
	return sc.Err()
}

func (c *ntfyChannel) handleTap(host *ChannelHost, message string) {
	// "a:<question id>:<option>:<question token>"
	v := strings.TrimSpace(message)
	i := strings.LastIndex(v, ":")
	if i < 0 {
		return
	}
	payload, token := v[:i], v[i+1:]
	id, idx, ok := parseButton(payload)
	secret := host.Secret(id)
	if !ok || secret == "" || token != secret {
		return // not from one of our notifications
	}
	host.Tap(id, idx)
}

// handleText turns a message typed in the ntfy app into an answer.
func (c *ntfyChannel) handleText(host *ChannelHost, ev ntfyEvent) {
	text := strings.TrimSpace(ev.Message)
	if text == "" {
		return
	}
	code := ""
	if first, rest, ok := strings.Cut(text, " "); ok && len(first) == 4 && host.HasRef(strings.ToUpper(first)) {
		code, text = strings.ToUpper(first), strings.TrimSpace(rest)
	}
	var sent time.Time
	if ev.Time > 0 {
		sent = time.Unix(ev.Time, 0)
	}
	res := host.Text(code, text, sent)
	msg := res.Reply()
	if res.Answered == nil && res.Open > 1 {
		msg = fmt.Sprintf("%d questions are open. Start your message with the code shown in the question you're answering.", res.Open)
	}
	c.say(msg)
}

// SendNtfyTest publishes a plain notification so the user can check they're subscribed.
func SendNtfyTest(ctx context.Context, cfg NtfyConfig) error {
	_, err := (&ntfyChannel{cfg: cfg}).publish(ctx, map[string]any{
		"title":   "Momentum is linked", // the tag below already adds the ✅
		"message": "Questions from your AI agents will appear here.",
		"tags":    []string{"white_check_mark"},
	})
	return err
}
