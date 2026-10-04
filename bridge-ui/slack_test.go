package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	slackBot  = "xoxb-test"
	slackApp  = "xapp-test"
	slackUser = "UHARSHAL"
	slackDM   = "D0MOMENTUM"
)

// fakeSlack implements the Web API methods Momentum uses plus a Socket Mode
// WebSocket, so tests can tap buttons and send DMs like a real user.
type fakeSlack struct {
	mu      sync.Mutex
	posts   []slackPost
	updates map[string]string // ts -> rendered text of the updated message
	acks    map[string]bool
	conns   []*websocket.Conn
	opens   int
	nextTS  int
	srv     *httptest.Server
	connCh  chan struct{}
}

type slackPost struct {
	Channel, Text, ThreadTS, TS string
	Values, Labels              []string
	Blocks                      string
}

func newFakeSlack(t *testing.T) *fakeSlack {
	f := &fakeSlack{updates: map[string]string{}, acks: map[string]bool{}, connCh: make(chan struct{}, 10)}
	mux := http.NewServeMux()
	up := websocket.Upgrader{}
	mux.HandleFunc("/socket", func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c.WriteJSON(map[string]any{"type": "hello"})
		f.mu.Lock()
		f.conns = append(f.conns, c)
		f.mu.Unlock()
		f.connCh <- struct{}{}
		go func() {
			for {
				var ack map[string]string
				if c.ReadJSON(&ack) != nil {
					return
				}
				f.mu.Lock()
				f.acks[ack["envelope_id"]] = true
				f.mu.Unlock()
			}
		}()
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/api/")
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		var p map[string]any
		json.NewDecoder(r.Body).Decode(&p)
		reply := func(v map[string]any) { v["ok"] = true; json.NewEncoder(w).Encode(v) }
		fail := func(code string) { json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": code}) }
		if method == "apps.connections.open" {
			if tok != slackApp {
				fail("invalid_auth")
				return
			}
			f.mu.Lock()
			f.opens++
			f.mu.Unlock()
			reply(map[string]any{"url": "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/socket"})
			return
		}
		if tok != slackBot {
			fail("invalid_auth")
			return
		}
		switch method {
		case "auth.test":
			reply(map[string]any{"team": "Acme", "user_id": "UBOT"})
		case "users.info":
			reply(map[string]any{"user": map[string]any{"real_name": "Harshal Patel", "profile": map[string]any{"display_name": "harshal"}}})
		case "chat.postMessage":
			f.mu.Lock()
			f.nextTS++
			ts := fmt.Sprintf("%d.%06d", time.Now().Unix(), f.nextTS)
			b := rawJSON(p["blocks"])
			post := slackPost{Channel: fmt.Sprint(p["channel"]), Text: fmt.Sprint(p["text"]), TS: ts, Blocks: b}
			if th, ok := p["thread_ts"].(string); ok {
				post.ThreadTS = th
			}
			if blocks, ok := p["blocks"].([]any); ok {
				for _, bl := range blocks {
					if els, ok := bl.(map[string]any)["elements"].([]any); ok {
						for _, e := range els {
							em := e.(map[string]any)
							if em["type"] == "button" {
								post.Values = append(post.Values, fmt.Sprint(em["value"]))
								post.Labels = append(post.Labels, fmt.Sprint(em["text"].(map[string]any)["text"]))
							}
						}
					}
				}
			}
			f.posts = append(f.posts, post)
			f.mu.Unlock()
			reply(map[string]any{"channel": slackDM, "ts": ts})
		case "chat.update":
			b := rawJSON(p["blocks"])
			f.mu.Lock()
			f.updates[fmt.Sprint(p["ts"])] = b
			f.mu.Unlock()
			reply(map[string]any{})
		default:
			reply(map[string]any{})
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(func() {
		f.mu.Lock()
		for _, c := range f.conns {
			c.Close()
		}
		f.mu.Unlock()
		f.srv.Close()
	})
	t.Setenv("MOMENTUM_SLACK_API", f.srv.URL+"/api")
	return f
}

// rawJSON encodes without HTML escaping, so tests see text as Slack would.
func rawJSON(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
	return b.String()
}

func (f *fakeSlack) waitConn(t *testing.T) {
	t.Helper()
	select {
	case <-f.connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Momentum never opened a Socket Mode connection")
	}
}

func (f *fakeSlack) send(env map[string]any) {
	f.mu.Lock()
	c := f.conns[len(f.conns)-1]
	f.mu.Unlock()
	c.WriteJSON(env)
}

var envSeq int

func (f *fakeSlack) tap(p slackPost, i int, user string) string {
	envSeq++
	id := fmt.Sprint("env-tap-", envSeq)
	f.send(map[string]any{"type": "interactive", "envelope_id": id, "payload": map[string]any{
		"type": "block_actions", "user": map[string]any{"id": user}, "channel": map[string]any{"id": slackDM},
		"actions": []any{map[string]any{"value": p.Values[i]}},
	}})
	return id
}

func (f *fakeSlack) dm(text, threadTS, user string) {
	envSeq++
	ev := map[string]any{"type": "message", "channel_type": "im", "channel": slackDM, "user": user, "text": text,
		"ts": fmt.Sprintf("%d.%06d", time.Now().Unix(), 900000+envSeq)}
	if threadTS != "" {
		ev["thread_ts"] = threadTS
	}
	f.send(map[string]any{"type": "events_api", "envelope_id": fmt.Sprint("env-dm-", envSeq), "payload": map[string]any{"event": ev}})
}

func (f *fakeSlack) waitQuestion(t *testing.T, n int) slackPost {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		var qs []slackPost
		for _, p := range f.posts {
			if len(p.Values) > 0 {
				qs = append(qs, p)
			}
		}
		f.mu.Unlock()
		if len(qs) > n {
			return qs[n]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Slack question %d never posted", n)
	return slackPost{}
}

func (f *fakeSlack) waitUpdate(t *testing.T, ts string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		u, ok := f.updates[ts]
		f.mu.Unlock()
		if ok {
			return u
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Slack message %s was never updated", ts)
	return ""
}

func slackCfg() BridgeConfig {
	return BridgeConfig{Channel: "slack", Slack: SlackConfig{BotToken: slackBot, AppToken: slackApp, UserID: slackUser, ChannelID: slackDM}}
}

func startSlackHub(t *testing.T) (*Hub, *fakeSlack) {
	t.Helper()
	t.Setenv("MOMENTUM_HOME", t.TempDir())
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	sl := newFakeSlack(t)
	h := NewHub()
	h.Logf = func(s string) { t.Log(s) }
	if err := h.Start(slackCfg()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
	sl.waitConn(t)
	return h, sl
}

func TestSlackButtonTapAnswers(t *testing.T) {
	h, sl := startSlackHub(t)
	if h.PublicURL() != "" {
		t.Error("Slack must not need a tunnel")
	}
	done := askAsync(askRequest{Question: "Drop table <users> & reseed?", Options: []string{"Approve", "Deny"}, Client: "Cursor", Project: "api"})
	p := sl.waitQuestion(t, 0)
	if p.Channel != slackDM || strings.Join(p.Labels, ",") != "Approve,Deny" {
		t.Errorf("post = %+v", p)
	}
	if !strings.Contains(p.Blocks, "Cursor · api") || !strings.Contains(p.Blocks, "&lt;users&gt; &amp; reseed") {
		t.Errorf("question not labelled/escaped: %s", p.Blocks)
	}
	env := sl.tap(p, 1, slackUser)
	if r := await(t, done); r.Status != stateAnswered || r.Answer != "Deny" {
		t.Fatalf("got %+v", r)
	}
	if u := sl.waitUpdate(t, p.TS); !strings.Contains(u, "Answered: Deny") || strings.Contains(u, `"button"`) {
		t.Errorf("message not closed: %s", u)
	}
	sl.mu.Lock()
	acked := sl.acks[env]
	sl.mu.Unlock()
	if !acked {
		t.Error("interactive envelope was not acknowledged")
	}
}

func TestSlackThreadReplyAndPlainDM(t *testing.T) {
	_, sl := startSlackHub(t)
	first := askAsync(askRequest{Question: "Which region?", Options: []string{"us-east-1", "eu-west-1"}})
	p1 := sl.waitQuestion(t, 0)
	second := askAsync(askRequest{Question: "Release name?"})
	sl.waitQuestion(t, 1)

	sl.dm("hello", "", slackUser)                // ambiguous with two open: ignored
	sl.dm("ap-south-1 please", p1.TS, slackUser) // thread reply answers that question
	if r := await(t, first); r.Answer != "ap-south-1 please" {
		t.Fatalf("first = %+v", r)
	}
	sl.dm("v2.4.0", "", slackUser) // only one open now: plain DM answers it
	if r := await(t, second); r.Answer != "v2.4.0" {
		t.Fatalf("second = %+v", r)
	}
	time.Sleep(200 * time.Millisecond)
	sl.mu.Lock()
	var said []string
	for _, p := range sl.posts {
		if len(p.Values) == 0 {
			said = append(said, p.Text)
		}
	}
	sl.mu.Unlock()
	if s := strings.Join(said, "|"); !strings.Contains(s, "Reply in the thread") || !strings.Contains(s, "Sent to") {
		t.Errorf("replies to the user = %v", said)
	}
}

func TestSlackIgnoresOtherPeople(t *testing.T) {
	_, sl := startSlackHub(t)
	done := askAsync(askRequest{Question: "Deploy?", WaitSeconds: 2})
	p := sl.waitQuestion(t, 0)
	sl.tap(p, 0, "USTRANGER")
	sl.dm("Approve", p.TS, "USTRANGER")
	if r := await(t, done); r.Status != stateWaiting {
		t.Fatalf("a stranger answered: %+v", r)
	}
}

func TestSlackReconnectsWhenAsked(t *testing.T) {
	_, sl := startSlackHub(t)
	sl.send(map[string]any{"type": "disconnect", "reason": "refresh_requested"})
	sl.waitConn(t) // a fresh socket
	done := askAsync(askRequest{Question: "Still connected?"})
	sl.tap(sl.waitQuestion(t, 0), 0, slackUser)
	if r := await(t, done); r.Answer != "Approve" {
		t.Fatalf("got %+v", r)
	}
}

func TestSlackQuestionClosesWhenAgentStopsWaiting(t *testing.T) {
	_, sl := startSlackHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	go newHubClient().ask(ctx, askRequest{Question: "Need me?"})
	p := sl.waitQuestion(t, 0)
	cancel()
	if u := sl.waitUpdate(t, p.TS); !strings.Contains(u, "stopped waiting") {
		t.Errorf("update = %s", u)
	}
}

func TestDetectSlackUser(t *testing.T) {
	sl := newFakeSlack(t)
	ctx := context.Background()
	if l := DetectSlackUser(ctx, "xapp-wrong-way-round", slackApp, time.Second); !strings.Contains(l.Error, "xoxb-") {
		t.Errorf("token kind check: %+v", l)
	}
	if l := DetectSlackUser(ctx, "xoxb-bad", slackApp, time.Second); !strings.Contains(l.Error, "rejected") {
		t.Errorf("bad bot token: %+v", l)
	}
	if l := DetectSlackUser(ctx, slackBot, slackApp, 300*time.Millisecond); !strings.Contains(l.Error, "send it any message") {
		t.Errorf("no DM yet: %+v", l)
	}
	go func() {
		sl.waitConn(t) // drain the connection from the attempt above
		sl.waitConn(t)
		sl.dm("hi", "", slackUser)
	}()
	l := DetectSlackUser(ctx, slackBot, slackApp, 5*time.Second)
	if l.Error != "" || l.UserID != slackUser || l.UserName != "harshal" || l.ChannelID != slackDM || l.Team != "Acme" {
		t.Fatalf("detect = %+v", l)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sl.mu.Lock()
		n := len(sl.posts)
		sl.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("no 'linked' confirmation was sent")
}

func TestSlackCreateAppURL(t *testing.T) {
	u := SlackCreateAppURL()
	if !strings.HasPrefix(u, "https://api.slack.com/apps?new_app=1&manifest_json=") {
		t.Fatal(u)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(slackManifest), &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if !strings.Contains(slackManifest, `"socket_mode_enabled": true`) || !strings.Contains(slackManifest, "message.im") {
		t.Error("manifest must enable Socket Mode and DM events")
	}
}
