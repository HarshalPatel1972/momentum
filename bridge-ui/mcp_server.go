package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// agentInstructions are sent to every MCP client on initialize, so agents in
// any IDE learn when to use Momentum without a per-IDE rules file.
const agentInstructions = `Momentum reaches the user on their phone while they are away from the computer.

Use the ask_remote_human tool whenever you need the user's decision, approval or clarification: before destructive or hard-to-undo actions (deleting files, force-pushing, running migrations, installing or removing packages, changing system settings), when requirements are ambiguous, or any time you would otherwise stop and ask in chat. The user may not be watching the chat window, so a question asked only in chat can stall the task indefinitely.

Keep the question short and self-contained (it is read on a phone) and give 2-4 short options, e.g. ["Approve", "Deny"]. The tool waits until the user answers and returns the answer; follow it. If the result says the question is still pending, call get_remote_answer with the request_id to keep waiting. Never carry out the action you asked about until the user has approved it.`

// mcpOptions are set from the command line by the IDE config Momentum writes.
type mcpOptions struct {
	Client      string // IDE id passed via --client, e.g. "cursor"
	WaitSeconds int    // -1 = choose per client
}

func runMCPServer(opts mcpOptions) {
	s := server.NewMCPServer("Momentum", Version,
		server.WithToolCapabilities(false),
		server.WithInstructions(agentInstructions),
		server.WithRoots(),
	)

	ask := mcp.NewTool("ask_remote_human",
		mcp.WithDescription("Ask the user a question on their phone (Telegram/WhatsApp) and wait for the answer. "+
			"Use this for approvals before destructive or irreversible actions and for any clarification you need; "+
			"the user may be away from the computer and cannot see questions asked in chat."),
		mcp.WithString("question", mcp.Required(), mcp.Description("Short, self-contained question, readable on a phone. Include the file/command involved.")),
		mcp.WithArray("options", mcp.Description(`2-4 short answer choices, e.g. ["Approve", "Deny"]. The user can also type a free-text answer.`), mcp.WithStringItems()),
	)
	s.AddTool(ask, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		question := strings.TrimSpace(req.GetString("question", ""))
		if question == "" {
			return mcp.NewToolResultError("question is required"), nil
		}
		client := clientName(ctx, opts.Client)
		body := askRequest{
			Question:    question,
			Options:     parseOptions(req.GetArguments()["options"]),
			Client:      client,
			Project:     projectName(ctx),
			WaitSeconds: waitBudget(opts, client),
		}
		hc, err := ensureHub(ctx)
		if err != nil {
			return mcp.NewToolResultError("Momentum is not running: " + err.Error()), nil
		}
		defer keepAlive(ctx, req)()
		resp, err := hc.ask(ctx, body)
		return toolResult(resp, err)
	})

	answer := mcp.NewTool("get_remote_answer",
		mcp.WithDescription("Keep waiting for the user's answer to an earlier ask_remote_human question that returned a request_id while still pending."),
		mcp.WithString("request_id", mcp.Required(), mcp.Description("The request_id returned by ask_remote_human.")),
	)
	s.AddTool(answer, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := strings.TrimSpace(req.GetString("request_id", ""))
		hc, err := ensureHub(ctx)
		if err != nil {
			return mcp.NewToolResultError("Momentum is not running: " + err.Error()), nil
		}
		defer keepAlive(ctx, req)()
		resp, err := hc.answer(ctx, id, waitBudget(opts, clientName(ctx, opts.Client)))
		return toolResult(resp, err)
	})

	// Exit as soon as the IDE closes our stdin, even mid-question. The hub sees
	// the dropped request and closes the question on the phone.
	in, pw := io.Pipe()
	go func() {
		io.Copy(pw, os.Stdin)
		pw.Close()
		time.Sleep(300 * time.Millisecond) // let a clean shutdown finish first
		os.Exit(0)
	}()
	if err := server.NewStdioServer(s).Listen(context.Background(), in, os.Stdout); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "[momentum] MCP server error: %v\n", err)
		os.Exit(1)
	}
}

func toolResult(resp askResponse, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError("Momentum: " + err.Error()), nil
	}
	switch resp.Status {
	case stateAnswered:
		return mcp.NewToolResultText("The user answered: " + resp.Answer), nil
	case stateWaiting:
		return mcp.NewToolResultText(fmt.Sprintf("The user has not answered yet (request_id: %s). "+
			"Call get_remote_answer with request_id %q to keep waiting. Do not proceed with the action until the user answers.", resp.ID, resp.ID)), nil
	case stateExpired:
		return mcp.NewToolResultText("The user did not answer before the question expired. Treat this as NOT approved: do not perform the action."), nil
	case statusAtDesk:
		return mcp.NewToolResultText("The user is at their computer right now (Momentum Away mode is off). Ask your question here in the chat instead."), nil
	case stateStopped:
		return mcp.NewToolResultText("Momentum was stopped before the user answered. Treat this as NOT approved: do not perform the action."), nil
	default:
		return mcp.NewToolResultError("Momentum: " + resp.Error), nil
	}
}

// parseOptions accepts an array, a JSON-encoded array, or a comma/slash separated string,
// because different IDE agents serialise array arguments differently.
func parseOptions(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, o := range t {
			out = append(out, fmt.Sprint(o))
		}
		return out
	case []string:
		return t
	case string:
		var arr []string
		if json.Unmarshal([]byte(t), &arr) == nil {
			return arr
		}
		sep := ","
		if !strings.Contains(t, ",") && strings.Contains(t, "/") {
			sep = "/"
		}
		return strings.Split(t, sep)
	}
	return nil
}

// knownClients maps --client ids and substrings of MCP clientInfo.name to display names.
var knownClients = []struct{ match, name string }{
	{"vscode-insiders", "VS Code Insiders"},
	{"vscode", "VS Code"}, {"visual studio code", "VS Code"},
	{"cursor", "Cursor"},
	{"windsurf", "Windsurf"}, {"codeium", "Windsurf"},
	{"antigravity", "Antigravity"},
	{"claude-code", "Claude Code"},
	{"claude-desktop", "Claude Desktop"}, {"claude-ai", "Claude Desktop"},
	{"codex", "Codex"},
	{"gemini", "Gemini CLI"},
	{"zed", "Zed"},
	{"cline", "Cline"},
	{"roo", "Roo Code"},
	{"kiro", "Kiro"},
	{"trae", "Trae"},
}

func clientName(ctx context.Context, flagClient string) string {
	raw := flagClient
	if raw == "" {
		if s, ok := server.ClientSessionFromContext(ctx).(server.SessionWithClientInfo); ok {
			raw = s.GetClientInfo().Name
		}
	}
	lower := strings.ToLower(raw)
	for _, k := range knownClients {
		if strings.Contains(lower, k.match) {
			return k.name
		}
	}
	return raw
}

// waitBudget decides how long one tool call may block. Clients known to wait
// indefinitely (or configured by us with a long timeout) get the whole wait;
// others get under a minute and continue via get_remote_answer, which avoids
// the common 60-second MCP request timeout.
func waitBudget(opts mcpOptions, client string) int {
	if opts.WaitSeconds >= 0 {
		return opts.WaitSeconds
	}
	switch client {
	case "VS Code", "VS Code Insiders", "Claude Code", "Codex", "Gemini CLI", "Cline", "Roo Code":
		return 0
	}
	return 50
}

// projectName finds the workspace the agent is working in: MCP roots if the
// client supports them, otherwise the process working directory.
func projectName(ctx context.Context) string {
	if srv := server.ServerFromContext(ctx); srv != nil {
		if s, ok := server.ClientSessionFromContext(ctx).(server.SessionWithClientInfo); ok && s.GetClientCapabilities().Roots != nil {
			rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if res, err := srv.RequestRoots(rctx, mcp.ListRootsRequest{}); err == nil && len(res.Roots) > 0 {
				if u, err := url.Parse(res.Roots[0].URI); err == nil && u.Path != "" {
					return filepath.Base(strings.TrimSuffix(u.Path, "/"))
				}
			}
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	home, _ := os.UserHomeDir()
	exe, _ := os.Executable()
	lw := strings.ToLower(filepath.Clean(wd))
	// IDEs that launch servers from their own install dir or $HOME say nothing about the project.
	if lw == strings.ToLower(filepath.Clean(home)) || lw == strings.ToLower(filepath.Dir(exe)) ||
		strings.Contains(lw, `\windows\system32`) || strings.Contains(lw, `\program files`) || strings.Contains(lw, `\appdata\local\programs`) {
		return ""
	}
	return filepath.Base(wd)
}

// keepAlive sends MCP progress notifications while a call blocks, which lets
// clients that support it reset their request timeout.
func keepAlive(ctx context.Context, req mcp.CallToolRequest) func() {
	if req.Params.Meta == nil || req.Params.Meta.ProgressToken == nil {
		return func() {}
	}
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		return func() {}
	}
	token := req.Params.Meta.ProgressToken
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for n := 1; ; n++ {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				srv.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
					"progressToken": token,
					"progress":      n,
					"message":       "Waiting for the user to answer on their phone…",
				})
			}
		}
	}()
	return func() { close(stop) }
}

// ---------- hub client ----------

type hubClient struct {
	base string
	key  string
	http *http.Client
}

func newHubClient() *hubClient {
	return &hubClient{
		base: fmt.Sprintf("http://127.0.0.1:%d", hubPort()),
		key:  hubKey(),
		http: &http.Client{}, // no timeout: asks long-poll; contexts bound them
	}
}

func (c *hubClient) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-Momentum-Key", c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("bad response from hub (%s): %v", resp.Status, err)
		}
	}
	if resp.StatusCode >= 400 {
		if r, ok := out.(*askResponse); ok && r.Error != "" {
			return errors.New(r.Error)
		}
		return fmt.Errorf("hub returned %s", resp.Status)
	}
	return nil
}

func (c *hubClient) health(ctx context.Context) (healthResponse, error) {
	var h healthResponse
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := c.do(ctx, http.MethodGet, "/api/health", nil, &h)
	return h, err
}

func (c *hubClient) ask(ctx context.Context, req askRequest) (askResponse, error) {
	var r askResponse
	err := c.do(ctx, http.MethodPost, "/api/ask", req, &r)
	return r, err
}

func (c *hubClient) answer(ctx context.Context, id string, wait int) (askResponse, error) {
	var r askResponse
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/answer?id=%s&wait=%d", url.QueryEscape(id), wait), nil, &r)
	return r, err
}

func (c *hubClient) post(ctx context.Context, path string) error {
	var r askResponse
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.do(ctx, http.MethodPost, path, nil, &r)
}

// waitGone waits until nothing answers on the hub port (after a shutdown request).
func (c *hubClient) waitGone(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := c.health(context.Background()); err != nil {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

// ensureHub returns a client for a running hub, starting a background daemon
// if none is running (or replacing a daemon from an older version).
func ensureHub(ctx context.Context) (*hubClient, error) {
	c := newHubClient()
	if h, err := c.health(ctx); err == nil {
		switch {
		case h.App != "momentum":
			return nil, fmt.Errorf("port %d is used by another program (set MOMENTUM_PORT to change it)", hubPort())
		case h.Version == "":
			return nil, errors.New("the running Momentum hub rejected this client's key; restart the Momentum app")
		case h.Version == Version || !h.Daemon:
			return c, nil
		}
		// An older background daemon: replace it so fixes take effect after an update.
		c.post(ctx, "/api/shutdown")
		c.waitGone(5 * time.Second)
	}
	if err := spawnDaemon(); err != nil {
		return nil, fmt.Errorf("could not start background hub: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if h, err := c.health(ctx); err == nil && h.App == "momentum" {
			return c, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return nil, errors.New("background hub did not start; open the Momentum app and check its logs")
}

func spawnDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--daemon")
	cmd.Dir = filepath.Dir(exe)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
