//go:build e2e

// End-to-end test against a built Momentum.exe, driven exactly like an IDE
// drives it: over MCP stdio. Run with:
//
//	MOMENTUM_EXE=build/bin/Momentum.exe go test -tags e2e -run E2E -v
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type fakeIDE struct {
	name       string // clientInfo.name the IDE reports
	args       []string
	answerWith string
}

func startIDE(t *testing.T, exe string, env []string, ide fakeIDE) (*client.Client, *mcp.InitializeResult) {
	t.Helper()
	c, err := client.NewStdioMCPClient(exe, env, ide.args...)
	if err != nil {
		t.Fatalf("%s: start: %v", ide.name, err)
	}
	t.Cleanup(func() { c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: ide.name, Version: "1.0"}
	res, err := c.Initialize(ctx, init)
	if err != nil {
		t.Fatalf("%s: initialize: %v", ide.name, err)
	}
	return c, res
}

func callTool(ctx context.Context, c *client.Client, name string, args map[string]any) (string, bool, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := c.CallTool(ctx, req)
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	for _, ct := range res.Content {
		if tc, ok := ct.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError, nil
}

func TestE2EMultipleIDEsShareOneHub(t *testing.T) {
	exe, _ := filepath.Abs(os.Getenv("MOMENTUM_EXE"))
	if os.Getenv("MOMENTUM_EXE") == "" {
		t.Skip("MOMENTUM_EXE not set")
	}
	home := t.TempDir()
	t.Setenv("MOMENTUM_HOME", home)
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	tg := newFakeTelegram(t)
	cfg, _ := json.Marshal(BridgeConfig{Channel: "telegram", Telegram: TelegramConfig{BotToken: "TOKEN", ChatID: "42"}})
	os.WriteFile(filepath.Join(home, "bridge-config.json"), cfg, 0600)
	env := []string{
		"MOMENTUM_HOME=" + home,
		"MOMENTUM_PORT=" + os.Getenv("MOMENTUM_PORT"),
		"MOMENTUM_TELEGRAM_API=" + tg.srv.URL,
		"MOMENTUM_NO_TRAY=1",
	}
	hc := newHubClient()
	if _, err := hc.health(context.Background()); err == nil {
		t.Fatal("a hub is already running on the test port")
	}
	t.Cleanup(func() {
		if t.Failed() {
			b, _ := os.ReadFile(filepath.Join(home, "momentum.log"))
			t.Logf("hub log:\n%s", b)
		}
	})
	t.Cleanup(func() {
		// Stop the background daemon the MCP clients spawned.
		hc.post(context.Background(), "/api/shutdown")
		hc.waitGone(5 * time.Second)
	})

	ides := []fakeIDE{
		{name: "Visual Studio Code", args: []string{"--mcp", "--client", "vscode"}, answerWith: "Approve"},
		{name: "cursor-vscode", args: []string{"--mcp", "--client", "cursor"}, answerWith: "Deny"},
		{name: "windsurf-client", args: []string{"--mcp", "--client", "windsurf"}, answerWith: "Yes"},
		{name: "antigravity-client", args: []string{"--mcp", "--client", "antigravity"}, answerWith: "Ship it"},
		// No --client flag: the name must come from MCP clientInfo.
		{name: "claude-code", args: []string{"--mcp"}, answerWith: "Go ahead"},
	}

	clients := make([]*client.Client, len(ides))
	for i, ide := range ides {
		c, res := startIDE(t, exe, env, ide)
		clients[i] = c
		if res.ServerInfo.Name != "Momentum" || res.ServerInfo.Version != Version {
			t.Errorf("serverInfo = %+v", res.ServerInfo)
		}
		if !strings.Contains(res.Instructions, "ask_remote_human") {
			t.Errorf("%s: server instructions missing", ide.name)
		}
		tools, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tl := range tools.Tools {
			names = append(names, tl.Name)
		}
		if strings.Join(names, ",") != "ask_remote_human,get_remote_answer" && strings.Join(names, ",") != "get_remote_answer,ask_remote_human" {
			t.Errorf("tools = %v", names)
		}
	}

	// All five IDEs ask at the same time.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	results := make([]string, len(ides))
	for i := range ides {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			text, isErr, err := callTool(ctx, clients[i], "ask_remote_human", map[string]any{
				"question": fmt.Sprintf("Q%d: run migration?", i),
				"options":  []string{"Approve", "Deny"},
			})
			if err != nil || isErr {
				t.Errorf("%s: %v %s", ides[i].name, err, text)
			}
			results[i] = text
		}(i)
	}

	wantLabel := []string{"VS Code", "Cursor", "Windsurf", "Antigravity", "Claude Code"}
	for n := range ides {
		m := tg.waitQuestion(t, n)
		var i int
		fmt.Sscanf(m.Text[strings.Index(m.Text, "Q"):], "Q%d", &i)
		if !strings.Contains(m.Text, wantLabel[i]) {
			t.Errorf("message for %s not labelled with IDE: %s", ides[i].name, m.Text)
		}
		// Tap the matching button if there is one, otherwise reply with free text.
		tapped := false
		for b, label := range m.Labels {
			if label == ides[i].answerWith {
				tg.tap(m, b, 42)
				tapped = true
			}
		}
		if !tapped {
			tg.reply(ides[i].answerWith, m.ID, 42)
		}
	}
	wg.Wait()
	for i, ide := range ides {
		if results[i] != "The user answered: "+ide.answerWith {
			t.Errorf("%s got %q, want answer %q", ide.name, results[i], ide.answerWith)
		}
	}

	// Exactly one hub served everyone, and it is a background daemon.
	h, err := hc.health(context.Background())
	if err != nil || !h.Daemon || h.Version != Version {
		t.Fatalf("health = %+v %v", h, err)
	}
	t.Logf("shared daemon pid %d", h.PID)
}

// A client with a short request timeout (e.g. Claude Desktop) gets a request_id
// back quickly and collects the answer with get_remote_answer.
func TestE2EShortTimeoutClientPolls(t *testing.T) {
	exe, _ := filepath.Abs(os.Getenv("MOMENTUM_EXE"))
	if os.Getenv("MOMENTUM_EXE") == "" {
		t.Skip("MOMENTUM_EXE not set")
	}
	home := t.TempDir()
	t.Setenv("MOMENTUM_HOME", home)
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	tg := newFakeTelegram(t)
	cfg, _ := json.Marshal(BridgeConfig{Channel: "telegram", Telegram: TelegramConfig{BotToken: "TOKEN", ChatID: "42"}})
	os.WriteFile(filepath.Join(home, "bridge-config.json"), cfg, 0600)
	env := []string{"MOMENTUM_HOME=" + home, "MOMENTUM_PORT=" + os.Getenv("MOMENTUM_PORT"),
		"MOMENTUM_TELEGRAM_API=" + tg.srv.URL, "MOMENTUM_NO_TRAY=1"}
	hc := newHubClient()
	t.Cleanup(func() { hc.post(context.Background(), "/api/shutdown"); hc.waitGone(5 * time.Second) })

	c, _ := startIDE(t, exe, env, fakeIDE{name: "claude-ai", args: []string{"--mcp", "--wait", "2"}})
	ctx := context.Background()
	start := time.Now()
	text, isErr, err := callTool(ctx, c, "ask_remote_human", map[string]any{"question": "Delete build/?", "options": `["Yes","No"]`})
	if err != nil || isErr || !strings.Contains(text, "has not answered yet") {
		t.Fatalf("want pending, got %q %v %v", text, isErr, err)
	}
	if time.Since(start) > 10*time.Second {
		t.Error("pending result took too long")
	}
	id := text[strings.Index(text, "request_id: ")+len("request_id: "):]
	id = id[:strings.Index(id, ")")]

	m := tg.waitQuestion(t, 0)
	if !strings.Contains(m.Text, "Claude Desktop") || strings.Join(m.Labels, "/") != "Yes/No" {
		t.Errorf("message = %+v", m)
	}
	tg.tap(m, 1, 42)
	text, isErr, err = callTool(ctx, c, "get_remote_answer", map[string]any{"request_id": id})
	if err != nil || isErr || text != "The user answered: No" {
		t.Fatalf("collect = %q %v %v", text, isErr, err)
	}
}

// When Momentum isn't configured the agent gets a clear, immediate error.
func TestE2ENotConfigured(t *testing.T) {
	exe, _ := filepath.Abs(os.Getenv("MOMENTUM_EXE"))
	if os.Getenv("MOMENTUM_EXE") == "" {
		t.Skip("MOMENTUM_EXE not set")
	}
	home := t.TempDir()
	t.Setenv("MOMENTUM_HOME", home)
	t.Setenv("MOMENTUM_PORT", fmt.Sprint(freePort(t)))
	env := []string{"MOMENTUM_HOME=" + home, "MOMENTUM_PORT=" + os.Getenv("MOMENTUM_PORT"), "MOMENTUM_NO_TRAY=1"}
	hc := newHubClient()
	t.Cleanup(func() { hc.post(context.Background(), "/api/shutdown"); hc.waitGone(5 * time.Second) })
	c, _ := startIDE(t, exe, env, fakeIDE{name: "cursor", args: []string{"--mcp"}})
	text, isErr, err := callTool(context.Background(), c, "ask_remote_human", map[string]any{"question": "x"})
	if err != nil || !isErr || !strings.Contains(text, "not set up") {
		t.Fatalf("got %q %v %v", text, isErr, err)
	}
}
