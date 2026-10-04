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
// Security: anyone who knows the topic could read the questions, so the
// topic is long and random. Each button also carries the question's own
// secret token, so a forged reply can't answer anything.
//
// Limits: at most 3 buttons, and no typed replies (ntfy has no reply box).

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
	return c.publish(ctx, map[string]any{
		"title":    title,
		"message":  body,
		"priority": 4,
		"tags":     []string{"robot"},
		"actions":  actions,
	})
}

// Close: ntfy can't edit a sent notification. Tapping a button already
// dismisses it ("clear"), so there is nothing to update.
func (c *ntfyChannel) Close(ctx context.Context, q *pendingQuestion, ref, state, answer string) {}

// Run subscribes to the answers topic and turns verified taps into answers.
func (c *ntfyChannel) Run(ctx context.Context, host *ChannelHost, ready func()) {
	since := fmt.Sprint(time.Now().Unix()) // never replay taps from before we started
	backoff := time.Second
	failing := false
	for ctx.Err() == nil {
		err := c.subscribe(ctx, since, ready, func(id, message string) {
			since = id
			c.handle(host, message)
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

func (c *ntfyChannel) subscribe(ctx context.Context, since string, ready func(), onMessage func(id, message string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.server()+"/"+c.cfg.answersTopic()+"/json?since="+since, nil)
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
		var ev struct {
			ID      string `json:"id"`
			Event   string `json:"event"`
			Message string `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		switch ev.Event {
		case "open":
			ready()
		case "message":
			onMessage(ev.ID, ev.Message)
		}
	}
	return sc.Err()
}

func (c *ntfyChannel) handle(host *ChannelHost, message string) {
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

// SendNtfyTest publishes a plain notification so the user can check they're subscribed.
func SendNtfyTest(ctx context.Context, cfg NtfyConfig) error {
	_, err := (&ntfyChannel{cfg: cfg}).publish(ctx, map[string]any{
		"title":   "✅ Momentum is linked",
		"message": "Questions from your AI agents will appear here.",
		"tags":    []string{"white_check_mark"},
	})
	return err
}
