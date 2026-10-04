package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

const testChat = "42"

// fakeTelegram is a minimal Bot API: it records sent/edited messages and
// serves queued updates (button taps, replies) to getUpdates long-polls.
type fakeTelegram struct {
	mu        sync.Mutex
	sent      []tgSent
	edits     map[int64]string
	cbAnswers []string
	updates   []tgUpdate
	nextMsg   int64
	nextUpd   int64
	fail      bool
	newUpdate chan struct{}
	srv       *httptest.Server
}

type tgSent struct {
	ID      int64
	ChatID  string
	Text    string
	Buttons []string // callback_data per button
	Labels  []string
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	recentChatsMu.Lock()
	recentChats = map[string]seenChat{} // each test starts with no remembered chats
	recentChatsMu.Unlock()
	f := &fakeTelegram{edits: map[int64]string{}, nextMsg: 100, newUpdate: make(chan struct{}, 100)}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	t.Setenv("MOMENTUM_TELEGRAM_API", f.srv.URL)
	return f
}

func (f *fakeTelegram) serve(w http.ResponseWriter, r *http.Request) {
	var p map[string]any
	json.NewDecoder(r.Body).Decode(&p)
	ok := func(result any) {
		b, _ := json.Marshal(map[string]any{"ok": true, "result": result})
		w.Write(b)
	}
	if !strings.HasPrefix(r.URL.Path, "/botTOKEN/") {
		fmt.Fprint(w, `{"ok":false,"description":"Unauthorized"}`)
		return
	}
	switch method := strings.TrimPrefix(r.URL.Path, "/botTOKEN/"); method {
	case "getMe":
		ok(map[string]any{"id": 1, "username": "momentum_test_bot"})
	case "deleteWebhook":
		ok(true)
	case "sendMessage":
		f.mu.Lock()
		if f.fail {
			f.mu.Unlock()
			fmt.Fprint(w, `{"ok":false,"description":"Bad Request: chat not found"}`)
			return
		}
		f.nextMsg++
		s := tgSent{ID: f.nextMsg, ChatID: fmt.Sprint(p["chat_id"]), Text: fmt.Sprint(p["text"])}
		if rm, _ := p["reply_markup"].(map[string]any); rm != nil {
			rows, _ := rm["inline_keyboard"].([]any)
			for _, row := range rows {
				for _, b := range row.([]any) {
					bm := b.(map[string]any)
					s.Buttons = append(s.Buttons, fmt.Sprint(bm["callback_data"]))
					s.Labels = append(s.Labels, fmt.Sprint(bm["text"]))
				}
			}
		}
		f.sent = append(f.sent, s)
		f.mu.Unlock()
		ok(map[string]any{"message_id": s.ID, "chat": map[string]any{"id": 42}})
	case "editMessageText":
		f.mu.Lock()
		f.edits[int64(p["message_id"].(float64))] = fmt.Sprint(p["text"])
		f.mu.Unlock()
		ok(true)
	case "answerCallbackQuery":
		f.mu.Lock()
		f.cbAnswers = append(f.cbAnswers, fmt.Sprint(p["text"]))
		f.mu.Unlock()
		ok(true)
	case "getUpdates":
		offset := int64(0)
		if v, okv := p["offset"].(float64); okv {
			offset = int64(v)
		}
		wait := 2 * time.Second // shorter than real long-polling
		if tmo, _ := p["timeout"].(float64); tmo == 0 {
			wait = 0 // timeout 0 returns immediately, like the real API
		}
		deadline := time.After(wait)
		for {
			f.mu.Lock()
			var out []tgUpdate
			for _, u := range f.updates {
				if u.UpdateID >= offset {
					out = append(out, u)
				}
			}
			f.mu.Unlock()
			if len(out) > 0 {
				ok(out)
				return
			}
			select {
			case <-f.newUpdate:
			case <-deadline:
				ok([]tgUpdate{})
				return
			case <-r.Context().Done():
				return
			}
		}
	default:
		ok(true)
	}
}

func (f *fakeTelegram) push(u tgUpdate) {
	f.mu.Lock()
	f.nextUpd++
	u.UpdateID = f.nextUpd
	f.updates = append(f.updates, u)
	f.mu.Unlock()
	f.newUpdate <- struct{}{}
}

// tap simulates pressing button i on message m in chat.
func (f *fakeTelegram) tap(m tgSent, i int, chat int64) {
	f.push(tgUpdate{Callback: &tgCallback{ID: fmt.Sprint("cb", m.ID, i), From: tgUser{ID: chat},
		Message: &tgMessage{MessageID: m.ID, Chat: tgChat{ID: chat}}, Data: m.Buttons[i]}})
}

// reply simulates typing text, optionally as a reply to message replyTo.
func (f *fakeTelegram) reply(text string, replyTo int64, chat int64) {
	f.mu.Lock()
	id := 9000 + f.nextUpd
	f.mu.Unlock()
	m := &tgMessage{MessageID: id, Date: time.Now().Unix(), Chat: tgChat{ID: chat, FirstName: "Harshal"}, Text: text}
	if replyTo != 0 {
		m.ReplyTo = &tgMessage{MessageID: replyTo}
	}
	f.push(tgUpdate{Message: m})
}

// waitSent waits for the n-th sent message.
func (f *fakeTelegram) waitSent(t *testing.T, n int) tgSent {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		if len(f.sent) > n {
			s := f.sent[n]
			f.mu.Unlock()
			return s
		}
		f.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("message %d never sent", n)
	return tgSent{}
}

func (f *fakeTelegram) waitEdit(t *testing.T, id int64) string {
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
	t.Fatalf("message %d was never edited", id)
	return ""
}

func freePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func telegramCfg() BridgeConfig {
	return BridgeConfig{Channel: "telegram", Telegram: TelegramConfig{BotToken: "TOKEN", ChatID: testChat}}
}

func startTestHub(t *testing.T, cfg BridgeConfig) (*Hub, *fakeTelegram) {
	t.Helper()
	t.Setenv("MOMENTUM_HOME", t.TempDir())
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	tg := newFakeTelegram(t)
	h := NewHub()
	h.Logf = func(s string) { t.Log(s) }
	if err := h.Start(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
	return h, tg
}

type askResult struct {
	r   askResponse
	err error
}

func askAsync(req askRequest) chan askResult {
	ch := make(chan askResult, 1)
	go func() {
		r, err := newHubClient().ask(context.Background(), req)
		ch <- askResult{r, err}
	}()
	return ch
}

func await(t *testing.T, ch chan askResult) askResponse {
	t.Helper()
	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatal(res.err)
		}
		return res.r
	case <-time.After(10 * time.Second):
		t.Fatal("ask did not return")
	}
	return askResponse{}
}

func TestTelegramButtonTapAnswers(t *testing.T) {
	h, tg := startTestHub(t, telegramCfg())
	if h.PublicURL() != "" {
		t.Error("Telegram must not need a tunnel / public URL")
	}
	done := askAsync(askRequest{Question: `Delete <b>main.go</b>?`, Options: []string{"Yes", "No"}, Client: "Cursor", Project: "momentum"})

	m := tg.waitSent(t, 0)
	if m.ChatID != testChat || !strings.Contains(m.Text, "Cursor · momentum") || !strings.Contains(m.Text, "&lt;b&gt;main.go&lt;/b&gt;") {
		t.Errorf("message not labelled/escaped: %+v", m)
	}
	if strings.Join(m.Labels, ",") != "Yes,No" || strings.Contains(m.Text, "http") {
		t.Errorf("want Yes/No buttons and no link, got %+v", m)
	}
	tg.tap(m, 1, 42)
	if r := await(t, done); r.Status != stateAnswered || r.Answer != "No" {
		t.Fatalf("got %+v", r)
	}
	// The message is edited to show the outcome and lose its buttons.
	if e := tg.waitEdit(t, m.ID); !strings.Contains(e, "Answered: No") {
		t.Errorf("edit = %s", e)
	}
	// A second tap on the old message is refused politely.
	tg.tap(m, 0, 42)
	time.Sleep(300 * time.Millisecond)
	tg.mu.Lock()
	answers := strings.Join(tg.cbAnswers, "|")
	tg.mu.Unlock()
	if !strings.Contains(answers, "Sent: No") || !strings.Contains(answers, "Already answered") {
		t.Errorf("callback answers = %s", answers)
	}
}

func TestTelegramReplyGivesFreeTextAnswer(t *testing.T) {
	_, tg := startTestHub(t, telegramCfg())
	first := askAsync(askRequest{Question: "Which DB?", Options: []string{"Postgres", "SQLite"}})
	m1 := tg.waitSent(t, 0)
	second := askAsync(askRequest{Question: "Branch name?"})
	m2 := tg.waitSent(t, 1)

	// Two open questions: a plain message is ambiguous, so it is not used.
	tg.reply("hello", 0, 42)
	// Replying to a specific message answers that question.
	tg.reply("use MySQL instead", m1.ID, 42)
	if r := await(t, first); r.Answer != "use MySQL instead" {
		t.Fatalf("first = %+v", r)
	}
	// Now only one question is open, so a plain message answers it.
	tg.reply("feat/login", 0, 42)
	if r := await(t, second); r.Answer != "feat/login" {
		t.Fatalf("second = %+v", r)
	}
	tg.waitEdit(t, m2.ID)
	tg.mu.Lock()
	var replies []string
	for _, s := range tg.sent[2:] {
		replies = append(replies, s.Text)
	}
	tg.mu.Unlock()
	if !strings.Contains(strings.Join(replies, "|"), "Reply directly") {
		t.Errorf("ambiguous message should be explained, replies: %v", replies)
	}
}

func TestTelegramIgnoresOtherChats(t *testing.T) {
	_, tg := startTestHub(t, telegramCfg())
	done := askAsync(askRequest{Question: "Deploy?", WaitSeconds: 2})
	m := tg.waitSent(t, 0)
	tg.tap(m, 0, 666)           // a stranger who somehow got the button
	tg.reply("Approve", 0, 666) // or messages the bot
	if r := await(t, done); r.Status != stateWaiting {
		t.Fatalf("stranger answered the question: %+v", r)
	}
}

func TestHubConcurrentQuestionsFromSeveralIDEs(t *testing.T) {
	_, tg := startTestHub(t, telegramCfg())
	ides := []string{"VS Code", "Cursor", "Windsurf", "Antigravity", "Claude Code"}
	results := map[string]chan askResult{}
	for _, ide := range ides {
		results[ide] = askAsync(askRequest{Question: "Proceed in " + ide + "?", Options: []string{"ok-" + ide, "no"}, Client: ide})
	}
	for i := range ides {
		tg.tap(tg.waitSent(t, i), 0, 42)
	}
	for _, ide := range ides {
		if r := await(t, results[ide]); r.Answer != "ok-"+ide {
			t.Errorf("%s got %+v", ide, r)
		}
	}
}

func TestHubWaitBudgetThenCollect(t *testing.T) {
	_, tg := startTestHub(t, telegramCfg())
	c := newHubClient()
	r, err := c.ask(context.Background(), askRequest{Question: "Slow?", WaitSeconds: 1})
	if err != nil || r.Status != stateWaiting || r.ID == "" {
		t.Fatalf("want pending with id, got %+v %v", r, err)
	}
	tg.tap(tg.waitSent(t, 0), 0, 42)
	r, err = c.answer(context.Background(), r.ID, 5)
	if err != nil || r.Status != stateAnswered || r.Answer != "Approve" {
		t.Fatalf("collect = %+v %v", r, err)
	}
	res, _ := toolResult(r, nil)
	if tc, ok := res.Content[0].(mcp.TextContent); !ok || res.IsError || !strings.Contains(tc.Text, "The user answered: Approve") {
		t.Errorf("tool result should carry the answer: %+v", res)
	}
}

func TestHubFailsFastWhenPhoneUnreachable(t *testing.T) {
	_, tg := startTestHub(t, telegramCfg())
	tg.mu.Lock()
	tg.fail = true
	tg.mu.Unlock()
	start := time.Now()
	r, err := newHubClient().ask(context.Background(), askRequest{Question: "x"})
	if err != nil || r.Status != "error" || !strings.Contains(r.Error, "chat not found") {
		t.Fatalf("want immediate error, got %+v %v", r, err)
	}
	if time.Since(start) > 3*time.Second {
		t.Error("did not fail fast")
	}
}

func TestDetectTelegramChat(t *testing.T) {
	tg := newFakeTelegram(t)
	ctx := context.Background()
	if c := DetectTelegramChat(ctx, "TOKEN"); !strings.Contains(c.Error, "@momentum_test_bot") {
		t.Errorf("no messages yet should explain what to do: %+v", c)
	}
	tg.reply("/start", 0, 4242)
	c := DetectTelegramChat(ctx, "TOKEN")
	if c.ID != "4242" || c.Name != "Harshal" {
		t.Fatalf("detect = %+v", c)
	}
	if last := tg.waitSent(t, 0); last.ChatID != "4242" || !strings.Contains(last.Text, "linked") {
		t.Errorf("confirmation not sent: %+v", last)
	}
	if c := DetectTelegramChat(ctx, "WRONG"); !strings.Contains(c.Error, "rejected") {
		t.Errorf("bad token: %+v", c)
	}
}

// Link-based channels (WhatsApp) still use the answer page.
func TestAnswerPageEscapesAndIsOneTime(t *testing.T) {
	t.Setenv("MOMENTUM_ALLOW_LOCAL_LINKS", "1")
	h, _ := startTestHub(t, BridgeConfig{Channel: "whatsapp", WhatsApp: WhatsAppConfig{APIKey: "k", Phone: "+1"}})
	q := &pendingQuestion{ID: "abcd1234", Token: randomHex(16), Question: `rm -rf <script>alert(1)</script>`,
		Options: []string{"Yes", "No"}, Expires: time.Now().Add(time.Minute), state: stateWaiting, done: make(chan struct{})}
	h.register(q)
	link := h.PublicURL() + "/r/" + q.Token
	if !strings.HasPrefix(link, "http://127.0.0.1:") {
		t.Fatalf("link = %s", link)
	}
	resp, err := http.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(page), "<script>alert") || !strings.Contains(string(page), "&lt;script&gt;") ||
		!strings.Contains(resp.Header.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Error("page not escaped / no CSP")
	}
	post := func(v string) string {
		resp, _ := http.PostForm(link, url.Values{"answer": {v}})
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return string(b)
	}
	if !strings.Contains(post("Yes"), "Response sent") || !strings.Contains(post("No"), "already answered") {
		t.Error("answer page should accept exactly one answer")
	}
	if r := h.wait(context.Background(), q, 1); r.Answer != "Yes" {
		t.Errorf("answer = %+v", r)
	}
	resp, _ = http.Get(h.PublicURL() + "/r/wrongtoken")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Error("unknown token should 404")
	}
}

func TestHubRejectsUnauthenticatedAndDuplicateHubs(t *testing.T) {
	h, _ := startTestHub(t, telegramCfg())
	base := fmt.Sprintf("http://127.0.0.1:%d", hubPort())
	resp, err := http.Post(base+"/api/ask", "application/json", strings.NewReader(`{"question":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated ask got %d", resp.StatusCode)
	}
	resp, _ = http.Get(base + "/api/health")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(b), "version") {
		t.Errorf("unauthenticated health leaks details: %s", b)
	}
	// The public page server must not expose the local API.
	resp, _ = http.Post(h.publicLocal+"/api/ask", "application/json", strings.NewReader(`{}`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("public side exposes /api/ask: %d", resp.StatusCode)
	}
	if err := NewHub().Start(BridgeConfig{}); err != ErrHubBusy {
		t.Errorf("second hub should get ErrHubBusy, got %v", err)
	}
}

func TestHubNotConfigured(t *testing.T) {
	startTestHub(t, BridgeConfig{})
	r, _ := newHubClient().ask(context.Background(), askRequest{Question: "x"})
	if r.Status != "error" || !strings.Contains(r.Error, "not set up") {
		t.Errorf("got %+v", r)
	}
}

func TestReloadSwitchesChannels(t *testing.T) {
	h, tg := startTestHub(t, BridgeConfig{})
	h.Reload(telegramCfg())
	done := askAsync(askRequest{Question: "after reload?"})
	tg.tap(tg.waitSent(t, 0), 0, 42)
	if r := await(t, done); r.Answer != "Approve" {
		t.Fatalf("got %+v", r)
	}
}

func TestStopClosesOpenQuestions(t *testing.T) {
	h, tg := startTestHub(t, telegramCfg())
	done := askAsync(askRequest{Question: "Still there?"})
	m := tg.waitSent(t, 0)
	h.Stop()
	if r := await(t, done); r.Status != stateStopped {
		t.Errorf("got %+v", r)
	}
	if e := tg.waitEdit(t, m.ID); !strings.Contains(e, "stopped") {
		t.Errorf("edit = %s", e)
	}
}

func TestDaemonShutdownOnlyWhenAllowed(t *testing.T) {
	h, _ := startTestHub(t, telegramCfg())
	c := newHubClient()
	if err := c.post(context.Background(), "/api/shutdown"); err == nil {
		t.Error("UI-owned hub must refuse shutdown")
	}
	h.AllowShutdown = true
	if err := c.post(context.Background(), "/api/shutdown"); err != nil {
		t.Fatal(err)
	}
	if !c.waitGone(3 * time.Second) {
		t.Error("daemon hub did not stop")
	}
}

func TestConfigProblems(t *testing.T) {
	t.Setenv("MOMENTUM_ALLOW_LOCAL_LINKS", "")
	if p := configProblem(telegramCfg()); p != "" {
		t.Errorf("Telegram needs no ngrok token, got %q", p)
	}
	wa := BridgeConfig{Channel: "whatsapp", WhatsApp: WhatsAppConfig{APIKey: "k", Phone: "+1"}}
	if p := configProblem(wa); !strings.Contains(p, "ngrok") {
		t.Errorf("WhatsApp without ngrok: %q", p)
	}
	wa.NgrokToken = "x"
	if p := configProblem(wa); p != "" {
		t.Errorf("configProblem = %q", p)
	}
}

// Found in live testing: a message sent before Momentum started was still
// queued at Telegram and got taken as the answer to the next question.
func TestOldTelegramMessagesNeverAnswer(t *testing.T) {
	t.Setenv("MOMENTUM_HOME", t.TempDir())
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	tg := newFakeTelegram(t)
	tg.reply("Hey", 0, 42) // queued before the hub starts
	h := NewHub()
	h.Logf = func(s string) { t.Log(s) }
	if err := h.Start(telegramCfg()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
	time.Sleep(300 * time.Millisecond) // let the poller start

	done := askAsync(askRequest{Question: "Delete temp files?", WaitSeconds: 2})
	tg.waitSent(t, 0)
	// A message stamped before the question (e.g. delivered late) is ignored too.
	tg.mu.Lock()
	tg.nextUpd++
	tg.updates = append(tg.updates, tgUpdate{UpdateID: tg.nextUpd, Message: &tgMessage{MessageID: 1, Date: time.Now().Add(-time.Hour).Unix(), Chat: tgChat{ID: 42}, Text: "yes"}})
	tg.mu.Unlock()
	tg.newUpdate <- struct{}{}
	if r := await(t, done); r.Status != stateWaiting {
		t.Fatalf("an old message answered the question: %+v", r)
	}
}

func TestAtDeskTellsAgentToUseChat(t *testing.T) {
	cfg := telegramCfg()
	cfg.AtDesk = true
	_, tg := startTestHub(t, cfg)
	r, err := newHubClient().ask(context.Background(), askRequest{Question: "x"})
	if err != nil || r.Status != statusAtDesk {
		t.Fatalf("got %+v %v", r, err)
	}
	res, _ := toolResult(r, nil)
	if tc := res.Content[0].(mcp.TextContent); res.IsError || !strings.Contains(tc.Text, "in the chat instead") {
		t.Errorf("tool result = %+v", res)
	}
	tg.mu.Lock()
	defer tg.mu.Unlock()
	if len(tg.sent) != 0 {
		t.Error("nothing should be sent to the phone while at the desk")
	}
}

func TestActivityIsRecorded(t *testing.T) {
	h, tg := startTestHub(t, telegramCfg())
	var mu sync.Mutex
	var events []Activity
	h.OnActivity = func(a Activity) { mu.Lock(); events = append(events, a); mu.Unlock() }
	done := askAsync(askRequest{Question: "Ship it?", Options: []string{"Yes", "No"}, Client: "Cursor", Project: "app"})
	tg.tap(tg.waitSent(t, 0), 0, 42)
	await(t, done)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if items := ReadActivity(); len(items) == 1 && items[0].State == stateAnswered {
			if items[0].Answer != "Yes" || items[0].Client != "Cursor" || items[0].Closed.IsZero() {
				t.Errorf("activity = %+v", items[0])
			}
			mu.Lock()
			n := len(events)
			mu.Unlock()
			if n < 2 {
				t.Errorf("want asked+answered events, got %d", n)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("activity not recorded: %+v", ReadActivity())
}

// waitQuestion waits for the n-th question message, skipping confirmations
// such as "✅ Sent to …" that can interleave when answers arrive quickly.
func (f *fakeTelegram) waitQuestion(t *testing.T, n int) tgSent {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		var qs []tgSent
		for _, s := range f.sent {
			if strings.Contains(s.Text, "Input needed") {
				qs = append(qs, s)
			}
		}
		f.mu.Unlock()
		if len(qs) > n {
			return qs[n]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("question %d never sent", n)
	return tgSent{}
}

// Found while testing: if an IDE quit mid-question, the question stayed open
// on the phone although no one could receive the answer any more.
func TestQuestionClosesWhenAgentStopsWaiting(t *testing.T) {
	_, tg := startTestHub(t, telegramCfg())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan askResponse, 1)
	go func() {
		r, _ := newHubClient().ask(ctx, askRequest{Question: "Still need me?"})
		done <- r
	}()
	m := tg.waitQuestion(t, 0)
	cancel() // the IDE goes away
	if e := tg.waitEdit(t, m.ID); !strings.Contains(e, "stopped waiting") {
		t.Errorf("phone message not updated: %s", e)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a := ReadActivity(); len(a) == 1 && a[0].State == stateCanceled {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("activity = %+v", ReadActivity())
}

// The pager's keys answer from the PC; the phone message is closed too.
func TestAnswerFromPC(t *testing.T) {
	h, tg := startTestHub(t, telegramCfg())
	done := askAsync(askRequest{Question: "Ship it?", Options: []string{"Ship", "Hold"}})
	m := tg.waitQuestion(t, 0)
	var id string
	deadline := time.Now().Add(3 * time.Second)
	for id == "" && time.Now().Before(deadline) {
		for _, a := range ReadActivity() {
			if a.State == stateWaiting {
				id = a.ID
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !h.AnswerFromPC(id, "Hold") {
		t.Fatal("answering from the PC failed")
	}
	if h.AnswerFromPC(id, "Ship") {
		t.Error("a second answer must be refused")
	}
	if r := await(t, done); r.Answer != "Hold" {
		t.Fatalf("agent got %+v", r)
	}
	if e := tg.waitEdit(t, m.ID); !strings.Contains(e, "Answered: Hold") {
		t.Errorf("phone message not closed: %s", e)
	}
	if h.AnswerFromPC("nope", "x") {
		t.Error("unknown question must be refused")
	}
}

func TestIntroShownOncePerMajorVersion(t *testing.T) {
	cfg := telegramCfg()
	if !needsIntro(cfg) {
		t.Error("a set-up user upgrading from 1.x should see the 2.x story once")
	}
	cfg.IntroSeen = majorVersion(Version)
	if needsIntro(cfg) {
		t.Error("the story must not repeat after it was seen")
	}
	cfg.IntroSeen = "1"
	if !needsIntro(cfg) {
		t.Error("seeing the 1.x story doesn't count for 2.x")
	}
	if needsIntro(BridgeConfig{}) {
		t.Error("new users see the welcome screen anyway; no extra intro")
	}
	if majorVersion("2.0.1") != "2" {
		t.Error("majorVersion")
	}
}
