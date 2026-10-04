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

// ---------- fake Discord: REST + Gateway ----------

const (
	discordToken  = "discord-test-token"
	discordUserID = "111"
	discordDM     = "999"
)

type discordSent struct {
	Channel, ID, Content, Embed, ReplyTo string
	Buttons, Labels                      []string
}

type fakeDiscord struct {
	mu        sync.Mutex
	sent      []discordSent
	edits     map[string]string // message id -> embed description after edit
	callbacks []int             // interaction response types
	ephemeral []string
	gw        *websocket.Conn
	gwMu      sync.Mutex
	seq       int64
	identify  map[string]any
	connCh    chan struct{}
	nextID    int
	srv       *httptest.Server
}

func newFakeDiscord(t *testing.T) *fakeDiscord {
	f := &fakeDiscord{edits: map[string]string{}, connCh: make(chan struct{}, 10), nextID: 5000}
	up := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/gw", func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c.WriteJSON(map[string]any{"op": 10, "d": map[string]any{"heartbeat_interval": 40000}})
		var id map[string]any
		if c.ReadJSON(&id) != nil {
			return
		}
		f.mu.Lock()
		f.identify = id
		f.mu.Unlock()
		f.gwMu.Lock()
		f.gw = c
		f.gwMu.Unlock()
		f.dispatch("READY", map[string]any{"user": map[string]any{"id": "BOT", "username": "Momentum"}})
		f.connCh <- struct{}{}
		go func() {
			for {
				var p map[string]any
				if c.ReadJSON(&p) != nil {
					return
				}
				if p["op"] == float64(1) {
					f.gwMu.Lock()
					c.WriteJSON(map[string]any{"op": 11})
					f.gwMu.Unlock()
				}
			}
		}()
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot "+discordToken {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"401: Unauthorized"}`)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api")
		var p map[string]any
		json.NewDecoder(r.Body).Decode(&p)
		reply := func(v any) { json.NewEncoder(w).Encode(v) }
		switch {
		case path == "/gateway/bot":
			reply(map[string]any{"url": "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/gw"})
		case path == "/users/@me":
			reply(map[string]any{"id": "BOT", "username": "Momentum"})
		case path == "/oauth2/applications/@me":
			reply(map[string]any{"id": "APP123"})
		case strings.HasPrefix(path, "/interactions/"):
			f.mu.Lock()
			typ := int(p["type"].(float64))
			f.callbacks = append(f.callbacks, typ)
			if typ == 4 {
				f.ephemeral = append(f.ephemeral, fmt.Sprint(p["data"].(map[string]any)["content"]))
			}
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch:
			parts := strings.Split(path, "/")
			f.mu.Lock()
			f.edits[parts[len(parts)-1]] = rawJSON(p)
			f.mu.Unlock()
			reply(map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/messages"):
			f.mu.Lock()
			f.nextID++
			s := discordSent{Channel: strings.Split(path, "/")[2], ID: fmt.Sprint(f.nextID), Content: fmt.Sprint(p["content"]), Embed: rawJSON(p["embeds"])}
			if ref, ok := p["message_reference"].(map[string]any); ok {
				s.ReplyTo = fmt.Sprint(ref["message_id"])
			}
			if rows, ok := p["components"].([]any); ok {
				for _, row := range rows {
					for _, b := range row.(map[string]any)["components"].([]any) {
						bm := b.(map[string]any)
						s.Buttons = append(s.Buttons, fmt.Sprint(bm["custom_id"]))
						s.Labels = append(s.Labels, fmt.Sprint(bm["label"]))
					}
				}
			}
			f.sent = append(f.sent, s)
			f.mu.Unlock()
			reply(map[string]any{"id": s.ID, "channel_id": s.Channel})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(func() {
		f.gwMu.Lock()
		if f.gw != nil {
			f.gw.Close()
		}
		f.gwMu.Unlock()
		f.srv.Close()
	})
	t.Setenv("MOMENTUM_DISCORD_API", f.srv.URL+"/api")
	return f
}

func (f *fakeDiscord) dispatch(event string, d any) {
	f.gwMu.Lock()
	defer f.gwMu.Unlock()
	f.seq++
	f.gw.WriteJSON(map[string]any{"op": 0, "t": event, "s": f.seq, "d": d})
}

func (f *fakeDiscord) waitConn(t *testing.T) {
	t.Helper()
	select {
	case <-f.connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Momentum never connected to the Discord gateway")
	}
}

func (f *fakeDiscord) waitQuestion(t *testing.T, n int) discordSent {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		var qs []discordSent
		for _, s := range f.sent {
			if len(s.Buttons) > 0 {
				qs = append(qs, s)
			}
		}
		f.mu.Unlock()
		if len(qs) > n {
			return qs[n]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Discord question %d never sent", n)
	return discordSent{}
}

func (f *fakeDiscord) tap(s discordSent, i int, user string) {
	f.dispatch("INTERACTION_CREATE", map[string]any{"id": fmt.Sprint("int", time.Now().UnixNano()), "token": "itok", "type": 3,
		"channel_id": s.Channel, "user": map[string]any{"id": user}, "data": map[string]any{"custom_id": s.Buttons[i]}})
}

func (f *fakeDiscord) dm(text, replyTo, user string) {
	m := map[string]any{"id": fmt.Sprint(time.Now().UnixNano()), "channel_id": discordDM, "content": text,
		"timestamp": time.Now().Format(time.RFC3339), "author": map[string]any{"id": user, "username": "harshal", "global_name": "Harshal"}}
	if replyTo != "" {
		m["message_reference"] = map[string]any{"message_id": replyTo}
	}
	f.dispatch("MESSAGE_CREATE", m)
}

func (f *fakeDiscord) waitEdit(t *testing.T, id string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		e, ok := f.edits[id]
		f.mu.Unlock()
		if ok {
			return e
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Discord message %s never edited", id)
	return ""
}

func startDiscordHub(t *testing.T) (*Hub, *fakeDiscord) {
	t.Helper()
	t.Setenv("MOMENTUM_HOME", t.TempDir())
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	dc := newFakeDiscord(t)
	h := NewHub()
	h.Logf = func(s string) { t.Log(s) }
	if err := h.Start(BridgeConfig{Channel: "discord", Discord: DiscordConfig{BotToken: discordToken, UserID: discordUserID, ChannelID: discordDM}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
	dc.waitConn(t)
	return h, dc
}

func TestDiscordButtonTapAnswers(t *testing.T) {
	_, dc := startDiscordHub(t)
	dc.mu.Lock()
	intents := dc.identify["d"].(map[string]any)["intents"]
	dc.mu.Unlock()
	if intents != float64(discordIntentDirectMessages) {
		t.Errorf("must only ask for the DM intent, got %v", intents)
	}
	done := askAsync(askRequest{Question: "Merge PR #42?", Options: []string{"Merge", "Wait"}, Client: "Codex", Project: "web"})
	s := dc.waitQuestion(t, 0)
	if s.Channel != discordDM || strings.Join(s.Labels, ",") != "Merge,Wait" || !strings.Contains(s.Embed, "Codex · web") {
		t.Errorf("message = %+v", s)
	}
	dc.tap(s, 0, discordUserID)
	if r := await(t, done); r.Answer != "Merge" {
		t.Fatalf("got %+v", r)
	}
	if e := dc.waitEdit(t, s.ID); !strings.Contains(e, "Answered: Merge") || !strings.Contains(e, `"components":[]`) {
		t.Errorf("edit = %s", e)
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if len(dc.callbacks) == 0 || dc.callbacks[0] != 6 {
		t.Errorf("interaction should be acknowledged with a deferred update, got %v", dc.callbacks)
	}
}

func TestDiscordReplyAndStrangers(t *testing.T) {
	_, dc := startDiscordHub(t)
	done := askAsync(askRequest{Question: "Commit message?"})
	s := dc.waitQuestion(t, 0)
	dc.tap(s, 0, "STRANGER")
	dc.dm("lol", s.ID, "STRANGER")
	time.Sleep(200 * time.Millisecond)
	dc.dm("fix: handle empty input", s.ID, discordUserID)
	if r := await(t, done); r.Answer != "fix: handle empty input" {
		t.Fatalf("got %+v", r)
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if len(dc.ephemeral) == 0 || !strings.Contains(dc.ephemeral[0], "Only the person") {
		t.Errorf("stranger's tap should be refused privately: %v", dc.ephemeral)
	}
}

func TestDiscordReconnectsWhenAsked(t *testing.T) {
	_, dc := startDiscordHub(t)
	dc.gwMu.Lock()
	dc.gw.WriteJSON(map[string]any{"op": 7})
	dc.gwMu.Unlock()
	dc.waitConn(t)
	done := askAsync(askRequest{Question: "Still there?"})
	dc.tap(dc.waitQuestion(t, 0), 0, discordUserID)
	if r := await(t, done); r.Answer != "Approve" {
		t.Fatalf("got %+v", r)
	}
}

func TestDetectDiscordUser(t *testing.T) {
	dc := newFakeDiscord(t)
	ctx := context.Background()
	if l := DetectDiscordUser(ctx, "wrong", time.Second); !strings.Contains(l.Error, "rejected") {
		t.Errorf("bad token: %+v", l)
	}
	go func() {
		dc.waitConn(t)
		dc.dm("hi", "", discordUserID)
	}()
	l := DetectDiscordUser(ctx, discordToken, 5*time.Second)
	if l.Error != "" || l.UserID != discordUserID || l.UserName != "Harshal" || l.ChannelID != discordDM {
		t.Fatalf("detect = %+v", l)
	}
	u, err := DiscordInviteURL(ctx, discordToken)
	if err != nil || !strings.Contains(u, "client_id=APP123") || !strings.Contains(u, "permissions=0") {
		t.Errorf("invite = %q %v", u, err)
	}
}

// ---------- fake ntfy ----------

type fakeNtfy struct {
	mu        sync.Mutex
	published []map[string]any
	streams   map[string]chan string // topic -> lines for subscribers
	since     []string
	srv       *httptest.Server
	subCh     chan struct{}
}

func newFakeNtfy(t *testing.T) *fakeNtfy {
	f := &fakeNtfy{streams: map[string]chan string{}, subCh: make(chan struct{}, 10)}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/" {
			var m map[string]any
			json.NewDecoder(r.Body).Decode(&m)
			f.mu.Lock()
			f.published = append(f.published, m)
			f.mu.Unlock()
			// Like the real server, everything published reaches the topic's subscribers.
			f.deliver(fmt.Sprint(m["topic"]), fmt.Sprint(m["title"]), fmt.Sprint(m["message"]))
			fmt.Fprintf(w, `{"id":"msg%d"}`, time.Now().UnixNano())
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/json") {
			topics := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/json")
			ch := make(chan string, 20)
			f.mu.Lock()
			for _, topic := range strings.Split(topics, ",") { // one subscription, several topics
				f.streams[topic] = ch
			}
			f.since = append(f.since, r.URL.Query().Get("since"))
			f.mu.Unlock()
			fl := w.(http.Flusher)
			fmt.Fprintln(w, `{"event":"open"}`)
			fl.Flush()
			f.subCh <- struct{}{}
			for {
				select {
				case line := <-ch:
					fmt.Fprintln(w, line)
					fl.Flush()
				case <-r.Context().Done():
					return
				}
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// tapAction does what the phone does when a notification button is tapped.
func (f *fakeNtfy) tapAction(t *testing.T, msg map[string]any, i int) {
	t.Helper()
	a := msg["actions"].([]any)[i].(map[string]any)
	topic := strings.TrimPrefix(fmt.Sprint(a["url"]), f.srv.URL+"/")
	f.publishTo(topic, fmt.Sprint(a["body"]))
}

// publishTo posts like the phone does: no title (a tap, or text typed in the app).
func (f *fakeNtfy) publishTo(topic, body string) { f.deliver(topic, "", body) }

func (f *fakeNtfy) deliver(topic, title, body string) {
	if title == "<nil>" {
		title = ""
	}
	b, _ := json.Marshal(map[string]any{"id": fmt.Sprint("m", time.Now().UnixNano()), "time": time.Now().Unix(),
		"event": "message", "topic": topic, "title": title, "message": body})
	f.mu.Lock()
	ch := f.streams[topic]
	f.mu.Unlock()
	if ch != nil {
		ch <- string(b)
	}
}

// said returns the messages Momentum posted without buttons (its confirmations).
func (f *fakeNtfy) said() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, m := range f.published {
		if m["actions"] == nil {
			out = append(out, fmt.Sprint(m["message"]))
		}
	}
	return out
}

func (f *fakeNtfy) waitPublished(t *testing.T, n int) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		if len(f.published) > n {
			m := f.published[n]
			f.mu.Unlock()
			return m
		}
		f.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ntfy message %d never published", n)
	return nil
}

func startNtfyHub(t *testing.T) (*Hub, *fakeNtfy, NtfyConfig) {
	t.Helper()
	t.Setenv("MOMENTUM_HOME", t.TempDir())
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	nf := newFakeNtfy(t)
	cfg := NtfyConfig{Server: nf.srv.URL, Topic: NewNtfyTopic()}
	h := NewHub()
	h.Logf = func(s string) { t.Log(s) }
	if err := h.Start(BridgeConfig{Channel: "ntfy", Ntfy: cfg}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
	select {
	case <-nf.subCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Momentum never subscribed to the answers topic")
	}
	return h, nf, cfg
}

func TestNtfyTapAnswers(t *testing.T) {
	_, nf, cfg := startNtfyHub(t)
	done := askAsync(askRequest{Question: "Rotate the API keys?", Options: []string{"Rotate", "Skip", "Later", "Never"}, Client: "Kiro", Project: "infra"})
	m := nf.waitPublished(t, 0)
	if m["topic"] != cfg.Topic || !strings.Contains(fmt.Sprint(m["title"]), "Kiro · infra") {
		t.Errorf("message = %v", m)
	}
	acts := m["actions"].([]any)
	if len(acts) != 3 || !strings.Contains(fmt.Sprint(m["message"]), "Never") {
		t.Errorf("ntfy allows 3 buttons; the rest must be listed in the text: %v", m)
	}
	if u := fmt.Sprint(acts[0].(map[string]any)["url"]); u != nf.srv.URL+"/"+cfg.Topic+"-answers" {
		t.Errorf("answers go to the wrong topic: %s", u)
	}
	nf.tapAction(t, m, 1)
	if r := await(t, done); r.Answer != "Skip" {
		t.Fatalf("got %+v", r)
	}
	nf.mu.Lock()
	defer nf.mu.Unlock()
	if nf.since[0] == "" || nf.since[0] == "all" {
		t.Errorf("subscription must not replay old answers, since=%q", nf.since[0])
	}
}

func TestNtfyRejectsForgedAnswers(t *testing.T) {
	_, nf, cfg := startNtfyHub(t)
	done := askAsync(askRequest{Question: "Delete prod DB?", WaitSeconds: 2})
	m := nf.waitPublished(t, 0)
	body := fmt.Sprint(m["actions"].([]any)[0].(map[string]any)["body"])
	forged := body[:strings.LastIndex(body, ":")] + ":not-the-secret"
	nf.publishTo(cfg.Topic+"-answers", forged)
	nf.publishTo(cfg.Topic+"-answers", "Approve")
	if r := await(t, done); r.Status != stateWaiting {
		t.Fatalf("a forged answer was accepted: %+v", r)
	}
}

func TestNtfyTopicsAreUnguessable(t *testing.T) {
	a, b := NewNtfyTopic(), NewNtfyTopic()
	if a == b || len(a) < 30 || !strings.HasPrefix(a, "momentum-") {
		t.Errorf("topics %q %q", a, b)
	}
	if p := configProblem(BridgeConfig{Channel: "ntfy", Ntfy: NtfyConfig{Topic: "short"}}); p == "" {
		t.Error("a short topic must be rejected")
	}
}

func TestRecentTelegramChatExpires(t *testing.T) {
	recentChatsMu.Lock()
	recentChats = map[string]seenChat{"T": {TelegramChat{ID: "1"}, time.Now().Add(-time.Hour)}}
	recentChatsMu.Unlock()
	if _, ok := recentTelegramChat("T"); ok {
		t.Error("a chat seen an hour ago must not be used to link")
	}
}

func TestNtfyTypedReply(t *testing.T) {
	_, nf, cfg := startNtfyHub(t)
	done := askAsync(askRequest{Question: "Which DB?", Options: []string{"Postgres", "SQLite"}})
	m := nf.waitPublished(t, 0)
	if !strings.Contains(fmt.Sprint(m["message"]), "type an answer in this topic") {
		t.Errorf("the notification should say typing works: %v", m["message"])
	}
	nf.publishTo(cfg.Topic, "use MySQL instead") // typed in the ntfy app's message box
	if r := await(t, done); r.Answer != "use MySQL instead" {
		t.Fatalf("got %+v", r)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(strings.Join(nf.said(), "|"), "Sent to") {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(strings.Join(nf.said(), "|"), "Sent to") {
		t.Errorf("no confirmation posted: %v", nf.said())
	}
}

func TestNtfyTypedReplyPicksQuestionByCode(t *testing.T) {
	_, nf, cfg := startNtfyHub(t)
	first := askAsync(askRequest{Question: "Region?", WaitSeconds: 3})
	nf.waitPublished(t, 0)
	second := askAsync(askRequest{Question: "Release name?"})
	m2 := nf.waitPublished(t, 1)
	msg := fmt.Sprint(m2["message"])
	code := msg[strings.LastIndex(msg, "start it with ")+len("start it with "):]
	code = strings.TrimSuffix(code, ".")
	if len(code) != 4 {
		t.Fatalf("no code in %q", msg)
	}
	nf.publishTo(cfg.Topic, "hello") // ambiguous: two questions open
	nf.publishTo(cfg.Topic, strings.ToLower(code)+" v2.4.0")
	if r := await(t, second); r.Answer != "v2.4.0" {
		t.Fatalf("second = %+v", r)
	}
	if r := await(t, first); r.Status != stateWaiting {
		t.Errorf("the ambiguous message answered the first question: %+v", r)
	}
	if !strings.Contains(strings.Join(nf.said(), "|"), "questions are open") {
		t.Errorf("the ambiguous message should be explained: %v", nf.said())
	}
}
