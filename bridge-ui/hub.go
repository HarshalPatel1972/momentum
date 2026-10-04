package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

// The hub is the single long-lived owner of the notification channels (the
// Telegram poller, or the ngrok tunnel for link-based channels). Exactly one runs per user (it holds a fixed localhost port), either
// inside the desktop UI or as a headless `--daemon`. Every IDE's `--mcp`
// process is a thin client that forwards questions to it over localhost, so any
// number of IDEs can be open at once without fighting over the tunnel.

var ErrHubBusy = errors.New("another Momentum hub is already running")

const (
	stateWaiting  = "waiting"
	stateAnswered = "answered"
	stateExpired  = "expired"
	stateStopped  = "stopped"
	stateFailed   = "failed"   // the phone could not be reached
	stateCanceled = "canceled" // the agent stopped waiting (IDE closed, request cancelled)
	statusAtDesk  = "at_desk"  // Away mode is off
)

type pendingQuestion struct {
	ID       string
	Token    string // unguessable; the only thing in the public link
	Question string
	Options  []string
	Client   string
	Project  string
	Created  time.Time
	Expires  time.Time

	channel Channel // the app it was sent through (nil for link-based channels)
	ref     string  // the posted message, for matching replies and closing it

	state  string
	answer string
	done   chan struct{} // closed when state leaves "waiting"
}

type Hub struct {
	Logf          func(string)
	OnPublicURL   func(string)
	OnActivity    func(Activity) // called whenever a question is asked or closed
	dir           string         // data folder, fixed at Start so late writes can't go elsewhere
	writes        sync.WaitGroup // in-flight activity writes; Stop waits for them
	AllowShutdown bool           // daemon mode: newer clients / the UI may ask it to exit
	Done          chan struct{}  // closed after Stop

	mu           sync.Mutex
	cfg          BridgeConfig
	key          string
	running      bool
	localSrv     *http.Server
	publicSrv    *http.Server
	publicLocal  string
	tunnel       ngrok.Tunnel
	tunnelCancel context.CancelFunc
	tunnelToken  string
	publicURL    string
	tunnelErr    string
	linkKey      string  // what the tunnel was started for; "" = no tunnel
	ch           Channel // the active button channel (Telegram, Slack…), nil for link channels
	chKind       string  // its config name, e.g. "slack"
	chKey        string  // its settings; it restarts only when they change
	chCancel     context.CancelFunc
	chReady      chan struct{} // closed once the channel can hear answers
	questions    map[string]*pendingQuestion
	byToken      map[string]*pendingQuestion
}

func NewHub() *Hub {
	return &Hub{
		Logf:      func(string) {},
		Done:      make(chan struct{}),
		questions: map[string]*pendingQuestion{},
		byToken:   map[string]*pendingQuestion{},
	}
}

func (h *Hub) log(format string, args ...any) { h.Logf(fmt.Sprintf(format, args...)) }

func (h *Hub) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

func (h *Hub) PublicURL() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.publicURL
}

// Start binds the hub's localhost port and brings up the tunnel. It returns
// ErrHubBusy if another hub already owns the port.
func (h *Hub) Start(cfg BridgeConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return nil
	}
	localLn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", hubPort()))
	if err != nil {
		return ErrHubBusy
	}
	publicLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		localLn.Close()
		return err
	}
	h.cfg = cfg
	h.dir = dataDir()
	h.key = hubKey()
	h.running = true
	h.Done = make(chan struct{})
	h.publicLocal = "http://" + publicLn.Addr().String()

	h.localSrv = &http.Server{Handler: h.localMux()}
	h.publicSrv = &http.Server{Handler: h.publicMux(), ReadHeaderTimeout: 10 * time.Second}
	go h.localSrv.Serve(localLn)
	go h.publicSrv.Serve(publicLn)
	go h.janitor(h.Done)

	h.log("🚀 Momentum hub v%s listening on 127.0.0.1:%d", Version, hubPort())
	h.applyChannelsLocked()
	return nil
}

// needsLink reports whether the channel sends a link to a web page (and so
// needs the ngrok tunnel). Button channels don't: answers come back as taps.
func needsLink(cfg BridgeConfig) bool { return !buttonChannels[cfg.Channel] }

// ChannelKind is the button channel currently listening ("" if none).
func (h *Hub) ChannelKind() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.chKind
}

// applyChannelsLocked starts/stops the channel listener and the tunnel so they
// match the current config, restarting only what changed.
func (h *Hub) applyChannelsLocked() {
	ch, key := newChannel(h.cfg)
	if key != h.chKey {
		h.stopChannelLocked()
		h.ch, h.chKey = ch, key
		if ch != nil {
			h.chKind = h.cfg.Channel
			ctx, cancel := context.WithCancel(context.Background())
			ready := make(chan struct{})
			var once sync.Once
			h.chCancel, h.chReady = cancel, ready
			go ch.Run(ctx, &ChannelHost{h: h, name: h.chKind}, func() { once.Do(func() { close(ready) }) })
			h.log("💬 %s connected: answers arrive as button taps (no tunnel needed)", channelTitle(h.chKind))
		}
	}

	linkKey := ""
	if needsLink(h.cfg) {
		linkKey = "link|" + h.cfg.NgrokToken
	}
	if linkKey != h.linkKey || (linkKey != "" && h.tunnelErr != "") {
		h.linkKey = linkKey
		if linkKey != "" {
			h.startTunnelLocked()
		} else {
			h.stopTunnelLocked()
		}
	}
}

func (h *Hub) stopTunnelLocked() {
	if h.tunnelCancel != nil {
		h.tunnelCancel()
		h.tunnelCancel = nil
	}
	if h.tunnel != nil {
		h.tunnel.Close()
		h.tunnel = nil
	}
	h.publicURL, h.tunnelErr = "", ""
}

// startTunnelLocked (re)starts ngrok in the background. Without a token the
// hub falls back to a localhost link, which is only useful for testing.
func (h *Hub) startTunnelLocked() {
	h.stopTunnelLocked()
	h.tunnelToken = h.cfg.NgrokToken
	h.tunnelErr = ""
	if h.cfg.NgrokToken == "" {
		h.publicURL = h.publicLocal
		h.log("⚠️ No ngrok token: links only work on this PC (%s)", h.publicURL)
		h.emitURL(h.publicURL)
		return
	}
	h.publicURL = ""
	ctx, cancel := context.WithCancel(context.Background())
	h.tunnelCancel = cancel
	token := h.cfg.NgrokToken
	go func() {
		var tun ngrok.Tunnel
		var err error
		// The free ngrok plan allows one session; a hub that just exited may
		// still hold it for a moment, so retry briefly.
		for attempt := 1; attempt <= 5; attempt++ {
			h.log("🔄 Starting ngrok tunnel (attempt %d)...", attempt)
			tun, err = ngrok.Listen(ctx, config.HTTPEndpoint(), ngrok.WithAuthtoken(token))
			if err == nil || ctx.Err() != nil {
				break
			}
			h.log("⚠️ ngrok: %v", err)
			select {
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			case <-ctx.Done():
			}
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if ctx.Err() != nil {
			if tun != nil {
				tun.Close()
			}
			return
		}
		if err != nil {
			h.tunnelErr = err.Error()
			h.log("❌ ngrok tunnel failed: %v", err)
			return
		}
		h.tunnel = tun
		h.publicURL = tun.URL()
		h.log("✅ Tunnel live: %s", h.publicURL)
		h.emitURL(h.publicURL)
		go http.Serve(tun, h.publicSrv.Handler)
	}()
}

func (h *Hub) emitURL(u string) {
	if h.OnPublicURL != nil {
		go h.OnPublicURL(u)
	}
}

// Reload applies a new config, restarting only the channels whose settings changed.
func (h *Hub) Reload(cfg BridgeConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return
	}
	h.cfg = cfg
	h.log("🔁 Configuration reloaded")
	h.applyChannelsLocked()
}

func (h *Hub) Stop() {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return
	}
	h.running = false
	h.stopChannelLocked()
	h.linkKey = ""
	h.stopTunnelLocked()
	for _, q := range h.questions {
		h.finishLocked(q, stateStopped, "")
	}
	localSrv, publicSrv, done := h.localSrv, h.publicSrv, h.Done
	h.mu.Unlock()

	// Let woken long-polls send their "stopped" reply before connections close.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if localSrv.Shutdown(ctx) != nil {
		localSrv.Close()
	}
	publicSrv.Close()
	// Make sure the final state of every question reaches the history file.
	flushed := make(chan struct{})
	go func() { h.writes.Wait(); close(flushed) }()
	select {
	case <-flushed:
	case <-time.After(2 * time.Second):
	}
	close(done)
	h.log("🛑 Hub stopped")
}

func (h *Hub) stopChannelLocked() {
	if h.chCancel != nil {
		h.chCancel()
	}
	h.ch, h.chKind, h.chKey, h.chCancel, h.chReady = nil, "", "", nil, nil
}

func channelTitle(kind string) string {
	switch kind {
	case "ntfy":
		return "ntfy"
	case "whatsapp":
		return "WhatsApp"
	}
	return strings.ToUpper(kind[:1]) + kind[1:]
}

// closeOnChannel updates the posted message with the outcome, in the background.
func closeOnChannel(q *pendingQuestion, state, answer string) {
	if q.channel == nil || q.ref == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		q.channel.Close(ctx, q, q.ref, state, answer)
	}()
}

func (h *Hub) finishLocked(q *pendingQuestion, state, answer string) bool {
	if q.state != stateWaiting {
		return false
	}
	q.state, q.answer = state, answer
	close(q.done)
	snapshot := *q
	h.writes.Add(1)
	go func() {
		defer h.writes.Done()
		h.noteActivity(&snapshot)
	}()
	closeOnChannel(q, state, answer)
	return true
}

func (h *Hub) janitor(done chan struct{}) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-t.C:
			h.mu.Lock()
			for id, q := range h.questions {
				if now.After(q.Expires) {
					if h.finishLocked(q, stateExpired, "") {
						h.log("⌛ Question %s expired unanswered", id)
					}
				}
				// Keep finished questions around for a while so a client that
				// timed out can still collect the answer.
				if q.state != stateWaiting && now.Sub(q.Expires) > time.Hour {
					delete(h.questions, id)
					delete(h.byToken, q.Token)
				}
			}
			h.mu.Unlock()
		}
	}
}

// ---------- localhost API (used by `--mcp` clients) ----------

type askRequest struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Client      string   `json:"client"`
	Project     string   `json:"project"`
	WaitSeconds int      `json:"wait_seconds"` // 0 = until answered or expired
}

type askResponse struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status"` // answered | waiting | expired | stopped | error
	Answer string `json:"answer,omitempty"`
	Error  string `json:"error,omitempty"`
}

type healthResponse struct {
	App       string `json:"app"`
	Version   string `json:"version,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Daemon    bool   `json:"daemon,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
	Channel   string `json:"channel,omitempty"`
	Problem   string `json:"problem,omitempty"`
	Pending   int    `json:"pending,omitempty"`
}

func (h *Hub) authorized(r *http.Request) bool {
	got := r.Header.Get("X-Momentum-Key")
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(h.key)) == 1
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Hub) localMux() http.Handler {
	mux := http.NewServeMux()
	auth := func(fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !h.authorized(r) {
				writeJSON(w, http.StatusUnauthorized, askResponse{Status: "error", Error: "unauthorized"})
				return
			}
			fn(w, r)
		}
	}
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		resp := healthResponse{App: "momentum"}
		if h.authorized(r) {
			h.mu.Lock()
			resp.Version = Version
			resp.PID = os.Getpid()
			resp.Daemon = h.AllowShutdown
			resp.PublicURL = h.publicURL
			resp.Channel = h.cfg.Channel
			resp.Problem = configProblem(h.cfg)
			if resp.Problem == "" && needsLink(h.cfg) && h.tunnelErr != "" {
				resp.Problem = "ngrok tunnel failed: " + h.tunnelErr
			}
			for _, q := range h.questions {
				if q.state == stateWaiting {
					resp.Pending++
				}
			}
			h.mu.Unlock()
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("POST /api/ask", auth(h.handleAsk))
	mux.HandleFunc("GET /api/answer", auth(h.handleAnswer))
	mux.HandleFunc("POST /api/reload", auth(func(w http.ResponseWriter, r *http.Request) {
		cfg, err := loadConfig()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, askResponse{Status: "error", Error: err.Error()})
			return
		}
		h.Reload(cfg)
		writeJSON(w, http.StatusOK, askResponse{Status: "ok"})
	}))
	mux.HandleFunc("POST /api/shutdown", auth(func(w http.ResponseWriter, r *http.Request) {
		if !h.AllowShutdown {
			writeJSON(w, http.StatusForbidden, askResponse{Status: "error", Error: "the desktop app owns this hub"})
			return
		}
		writeJSON(w, http.StatusOK, askResponse{Status: "ok"})
		go func() {
			time.Sleep(100 * time.Millisecond)
			h.Stop()
		}()
	}))
	return mux
}

// waitForLink waits briefly for the tunnel so a question sent right after
// start-up still gets a public link.
func (h *Hub) waitForLink(ctx context.Context) (string, error) {
	deadline := time.Now().Add(25 * time.Second)
	for {
		h.mu.Lock()
		u, terr, running := h.publicURL, h.tunnelErr, h.running
		h.mu.Unlock()
		switch {
		case !running:
			return "", errors.New("hub stopped")
		case u != "":
			return u, nil
		case terr != "":
			return "", fmt.Errorf("ngrok tunnel failed: %s", terr)
		case time.Now().After(deadline):
			return "", errors.New("ngrok tunnel is not ready yet")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (h *Hub) handleAsk(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		writeJSON(w, http.StatusBadRequest, askResponse{Status: "error", Error: "question is required"})
		return
	}
	h.mu.Lock()
	cfg := h.cfg
	h.mu.Unlock()
	if p := configProblem(cfg); p != "" {
		writeJSON(w, http.StatusOK, askResponse{Status: "error", Error: "Momentum is not set up: " + p + ". Open the Momentum app to configure it."})
		return
	}
	if cfg.AtDesk {
		writeJSON(w, http.StatusOK, askResponse{Status: statusAtDesk})
		return
	}
	timeout := time.Duration(cfg.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	q := &pendingQuestion{
		ID:       randomHex(4),
		Token:    randomHex(16),
		Question: strings.TrimSpace(req.Question),
		Options:  cleanOptions(req.Options),
		Client:   req.Client,
		Project:  req.Project,
		Created:  time.Now(),
		Expires:  time.Now().Add(timeout),
		state:    stateWaiting,
		done:     make(chan struct{}),
	}
	h.log("🔔 [%s] %s", orDash(sourceLabel(q)), q.Question)

	var sendErr error
	h.mu.Lock()
	ch, ready := h.ch, h.chReady
	h.mu.Unlock()
	if ch != nil {
		// Don't ask until the channel can hear the answer (e.g. Telegram has
		// skipped its backlog), or a tap could be lost.
		select {
		case <-ready:
		case <-time.After(15 * time.Second):
		case <-r.Context().Done():
			return
		}
		q.channel = ch
		h.register(q) // before sending: a tap may arrive before Send returns
		var ref string
		if ref, sendErr = ch.Send(r.Context(), q); sendErr == nil {
			h.mu.Lock()
			q.ref = ref
			if q.state != stateWaiting { // answered in the meantime
				closeOnChannel(q, q.state, q.answer)
			}
			h.mu.Unlock()
		}
	} else {
		base, err := h.waitForLink(r.Context())
		if err != nil {
			writeJSON(w, http.StatusOK, askResponse{Status: "error", Error: err.Error()})
			return
		}
		h.register(q)
		sendErr = notify(cfg, q, base+"/r/"+q.Token)
	}
	if sendErr != nil {
		h.mu.Lock()
		delete(h.questions, q.ID)
		delete(h.byToken, q.Token)
		q.state = stateFailed
		snapshot := *q
		h.mu.Unlock()
		h.noteActivity(&snapshot)
		h.log("❌ Notification failed: %v", sendErr)
		writeJSON(w, http.StatusOK, askResponse{Status: "error", Error: "could not reach your phone: " + sendErr.Error()})
		return
	}
	h.log("📤 Sent via %s (id %s)", cfg.Channel, q.ID)

	writeJSON(w, http.StatusOK, h.wait(r.Context(), q, req.WaitSeconds))
}

func (h *Hub) register(q *pendingQuestion) {
	h.mu.Lock()
	h.questions[q.ID] = q
	h.byToken[q.Token] = q
	snapshot := *q
	h.mu.Unlock()
	h.noteActivity(&snapshot)
}

func (h *Hub) noteActivity(q *pendingQuestion) {
	a := recordActivity(h.dir, q)
	if h.OnActivity != nil {
		h.OnActivity(a)
	}
}

// AnswerFromPC answers a waiting question from the desktop app itself.
func (h *Hub) AnswerFromPC(id, answer string) bool {
	answer = strings.TrimSpace(answer)
	h.mu.Lock()
	q := h.questions[id]
	ok := q != nil && answer != "" && h.finishLocked(q, stateAnswered, answer)
	h.mu.Unlock()
	if ok {
		h.log("📥 [%s] answered on this PC: %s", orDash(sourceLabel(q)), answer)
	}
	return ok
}

func (h *Hub) handleAnswer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	h.mu.Lock()
	q := h.questions[id]
	h.mu.Unlock()
	if q == nil {
		writeJSON(w, http.StatusOK, askResponse{ID: id, Status: "error", Error: "unknown request id (it may have expired)"})
		return
	}
	var wait int
	fmt.Sscan(r.URL.Query().Get("wait"), &wait)
	writeJSON(w, http.StatusOK, h.wait(r.Context(), q, wait))
}

func (h *Hub) wait(ctx context.Context, q *pendingQuestion, waitSeconds int) askResponse {
	var budget <-chan time.Time
	if waitSeconds > 0 {
		t := time.NewTimer(time.Duration(waitSeconds) * time.Second)
		defer t.Stop()
		budget = t.C
	}
	select {
	case <-q.done:
	case <-budget:
	case <-ctx.Done():
		// The asking agent went away (IDE closed, request cancelled). Nobody can
		// receive an answer now, so close the question instead of leaving it
		// open on the phone.
		h.mu.Lock()
		if h.finishLocked(q, stateCanceled, "") {
			h.log("🚫 [%s] agent stopped waiting", orDash(sourceLabel(q)))
		}
		h.mu.Unlock()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return askResponse{ID: q.ID, Status: q.state, Answer: q.answer}
}

// ---------- public pages (served through the tunnel) ----------

func (h *Hub) publicMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "Momentum is running.")
	})
	mux.HandleFunc("GET /r/{token}", func(w http.ResponseWriter, r *http.Request) {
		q, state, answer := h.lookup(r.PathValue("token"))
		if q == nil {
			renderPage(w, http.StatusNotFound, pageData{Title: "Link expired", Message: "This question no longer exists."})
			return
		}
		if state != stateWaiting {
			renderPage(w, http.StatusOK, pageData{Title: "Already closed", Message: closedMessage(state), Answer: answer, Q: q})
			return
		}
		renderPage(w, http.StatusOK, pageData{Title: "Agent question", Q: q, Form: true})
	})
	mux.HandleFunc("POST /r/{token}", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		answer := strings.TrimSpace(r.FormValue("answer"))
		if answer == "" {
			answer = strings.TrimSpace(r.FormValue("custom"))
		}
		h.mu.Lock()
		q := h.byToken[r.PathValue("token")]
		ok := false
		if q != nil && answer != "" {
			ok = h.finishLocked(q, stateAnswered, answer)
		}
		h.mu.Unlock()
		switch {
		case q == nil:
			renderPage(w, http.StatusNotFound, pageData{Title: "Link expired", Message: "This question no longer exists."})
		case answer == "":
			renderPage(w, http.StatusOK, pageData{Title: "Agent question", Q: q, Form: true, Message: "Please pick an option or type an answer."})
		case !ok:
			_, state, prev := h.lookup(q.Token)
			renderPage(w, http.StatusOK, pageData{Title: "Already closed", Message: closedMessage(state), Answer: prev, Q: q})
		default:
			h.log("📥 [%s] answered: %s", orDash(sourceLabel(q)), answer)
			renderPage(w, http.StatusOK, pageData{Title: "Response sent", Message: "Your agent is continuing. You can close this page.", Answer: answer, Q: q, Success: true})
		}
	})
	return mux
}

func (h *Hub) lookup(token string) (*pendingQuestion, string, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	q := h.byToken[token]
	if q == nil {
		return nil, "", ""
	}
	return q, q.state, q.answer
}

func closedMessage(state string) string {
	switch state {
	case stateAnswered:
		return "This question was already answered."
	case stateExpired:
		return "This question expired before it was answered."
	default:
		return "Momentum was stopped before this question was answered."
	}
}

func cleanOptions(opts []string) []string {
	var out []string
	for _, o := range opts {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		out = []string{"Approve", "Deny"}
	}
	return out
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
