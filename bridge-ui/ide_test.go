package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupIDEHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MOMENTUM_IDE_HOME", home)
	return home
}

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0700)
	if err := os.WriteFile(p, []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConnectCursorPreservesOtherServersAndOrder(t *testing.T) {
	home := setupIDEHome(t)
	p := filepath.Join(home, ".cursor", "mcp.json")
	writeFile(t, p, `{"zeta": 1, "mcpServers": {"github": {"command": "npx", "args": ["gh"]},
		"remote-bridge": {"command": "C:\\Tools\\Momentum.exe", "args": ["--mcp"]}}, "alpha": true}`)

	if err := ConnectIDE("cursor"); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, p)
	if !(strings.Index(out, `"zeta"`) < strings.Index(out, `"mcpServers"`) && strings.Index(out, `"mcpServers"`) < strings.Index(out, `"alpha"`)) {
		t.Errorf("top-level key order changed:\n%s", out)
	}
	var doc struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.MCPServers["github"]; !ok {
		t.Error("existing server was removed")
	}
	if _, ok := doc.MCPServers["remote-bridge"]; ok {
		t.Error("legacy remote-bridge duplicate should be replaced")
	}
	m := doc.MCPServers["momentum"]
	if m.Command != momentumExe() || strings.Join(m.Args, " ") != "--mcp --client cursor" {
		t.Errorf("bad entry: %+v", m)
	}
	if !exists(p + ".momentum.bak") {
		t.Error("expected a backup of the original file")
	}

	// Reconnecting is idempotent.
	if err := ConnectIDE("cursor"); err != nil {
		t.Fatal(err)
	}
	if strings.Count(readFile(t, p), `"momentum"`) != 1 {
		t.Error("duplicate entry after reconnect")
	}
	st := ideStatus(mustIDE(t, "cursor"))
	if !st.Connected || st.Stale || !st.Installed {
		t.Errorf("status = %+v", st)
	}

	if err := DisconnectIDE("cursor"); err != nil {
		t.Fatal(err)
	}
	out = readFile(t, p)
	if strings.Contains(out, "momentum") || !strings.Contains(out, "github") {
		t.Errorf("disconnect removed the wrong things:\n%s", out)
	}
}

func mustIDE(t *testing.T, id string) ideDef {
	d, ok := findIDE(id)
	if !ok {
		t.Fatalf("no ide %s", id)
	}
	return d
}

func TestConnectVSCodeUsesServersKeyAndStdioType(t *testing.T) {
	home := setupIDEHome(t)
	p := filepath.Join(home, "AppData", "Roaming", "Code", "User", "mcp.json")
	writeFile(t, p, `{"inputs": [{"id": "x"}], "servers": {}}`)
	if err := ConnectIDE("vscode"); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Inputs  []any                     `json:"inputs"`
		Servers map[string]map[string]any `json:"servers"`
	}
	json.Unmarshal([]byte(readFile(t, p)), &doc)
	if len(doc.Inputs) != 1 {
		t.Error("inputs lost")
	}
	if doc.Servers["momentum"]["type"] != "stdio" {
		t.Errorf("vscode entry needs type stdio: %v", doc.Servers)
	}
}

func TestConnectCreatesMissingFiles(t *testing.T) {
	home := setupIDEHome(t)
	for _, id := range []string{"windsurf", "antigravity", "claude-desktop", "kiro"} {
		if err := ConnectIDE(id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if st := ideStatus(mustIDE(t, id)); !st.Connected {
			t.Errorf("%s not connected after ConnectIDE: %+v", id, st)
		}
	}
	if !exists(filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")) {
		t.Error("windsurf config not created")
	}
}

func TestExtraFieldsForShortTimeoutClients(t *testing.T) {
	home := setupIDEHome(t)
	if err := ConnectIDE("gemini-cli"); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, filepath.Join(home, ".gemini", "settings.json"))
	if !strings.Contains(out, `"timeout": 3600000`) {
		t.Errorf("gemini entry missing timeout:\n%s", out)
	}
}

func TestJSONCFileIsNotRewritten(t *testing.T) {
	home := setupIDEHome(t)
	p := filepath.Join(home, ".cursor", "mcp.json")
	orig := "{\n  // my servers\n  \"mcpServers\": {},\n}\n"
	writeFile(t, p, orig)
	if err := ConnectIDE("cursor"); err != errHasComments {
		t.Fatalf("want errHasComments, got %v", err)
	}
	if readFile(t, p) != orig {
		t.Error("file with comments was modified")
	}
	if !strings.Contains(ManualSnippet("cursor"), `"mcpServers"`) {
		t.Error("manual snippet should use mcpServers")
	}
}

func TestInvalidJSONIsNotRewritten(t *testing.T) {
	home := setupIDEHome(t)
	p := filepath.Join(home, ".cursor", "mcp.json")
	writeFile(t, p, "{not json")
	if err := ConnectIDE("cursor"); err == nil {
		t.Fatal("expected error")
	}
	if readFile(t, p) != "{not json" {
		t.Error("invalid file was modified")
	}
}

func TestDisconnectWithoutEntryLeavesFileAlone(t *testing.T) {
	home := setupIDEHome(t)
	p := filepath.Join(home, ".cursor", "mcp.json")
	orig := `{"mcpServers":{"github":{"command":"npx"}}}`
	writeFile(t, p, orig)
	if err := DisconnectIDE("cursor"); err != nil {
		t.Fatal(err)
	}
	if readFile(t, p) != orig {
		t.Error("file was rewritten although nothing was removed")
	}
}

func TestCodexTOML(t *testing.T) {
	home := setupIDEHome(t)
	p := filepath.Join(home, ".codex", "config.toml")
	writeFile(t, p, "model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"x\"\n")
	if err := ConnectIDE("codex"); err != nil {
		t.Fatal(err)
	}
	if err := ConnectIDE("codex"); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, p)
	if strings.Count(out, "[mcp_servers.momentum]") != 1 || !strings.Contains(out, "tool_timeout_sec = 3600") ||
		!strings.Contains(out, "'"+momentumExe()+"'") || !strings.Contains(out, "[mcp_servers.other]") {
		t.Errorf("bad toml:\n%s", out)
	}
	if st := ideStatus(mustIDE(t, "codex")); !st.Connected || st.Stale {
		t.Errorf("codex status %+v", st)
	}
	// A user-added env subtable is removed with our block, other tables stay.
	writeFile(t, p, out+"\n[mcp_servers.momentum.env]\nA = \"1\"\n\n[profiles.x]\nmodel = \"y\"\n")
	if err := DisconnectIDE("codex"); err != nil {
		t.Fatal(err)
	}
	out = readFile(t, p)
	if strings.Contains(out, "momentum") || !strings.Contains(out, "[profiles.x]") || !strings.Contains(out, "model = \"o3\"") {
		t.Errorf("bad toml after disconnect:\n%s", out)
	}
}

func TestStaleEntryDetected(t *testing.T) {
	home := setupIDEHome(t)
	writeFile(t, filepath.Join(home, ".cursor", "mcp.json"),
		`{"mcpServers":{"momentum":{"command":"D:\\old\\Momentum.exe","args":["--mcp"]}}}`)
	st := ideStatus(mustIDE(t, "cursor"))
	if !st.Connected || !st.Stale {
		t.Errorf("want stale, got %+v", st)
	}
}

func TestStripJSONC(t *testing.T) {
	in := `{"a": "http://x//y", /* c */ "b": [1,2,], // tail
	}`
	if !json.Valid(stripJSONC([]byte(in))) {
		t.Errorf("stripJSONC failed: %s", stripJSONC([]byte(in)))
	}
}

func TestWriteProjectRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# Project\nexisting\n")
	files, err := WriteProjectRules(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	WriteProjectRules(dir) // idempotent
	claude := readFile(t, filepath.Join(dir, "CLAUDE.md"))
	if strings.Count(claude, rulesBegin) != 1 || !strings.HasPrefix(claude, "# Project\nexisting\n") {
		t.Errorf("bad CLAUDE.md:\n%s", claude)
	}
	if exists(filepath.Join(dir, "GEMINI.md")) {
		t.Error("GEMINI.md should only be updated if it exists")
	}
}

func TestParseOptions(t *testing.T) {
	cases := map[string]struct {
		in   any
		want string
	}{
		"array":   {[]any{"Yes", "No"}, "Yes|No"},
		"json":    {`["Approve","Deny"]`, "Approve|Deny"},
		"comma":   {"Yes, No", "Yes| No"},
		"slash":   {"Yes/No", "Yes|No"},
		"missing": {nil, ""},
	}
	for name, c := range cases {
		if got := strings.Join(parseOptions(c.in), "|"); got != c.want {
			t.Errorf("%s: got %q want %q", name, got, c.want)
		}
	}
	if got := cleanOptions(parseOptions(nil)); strings.Join(got, "|") != "Approve|Deny" {
		t.Errorf("default options = %v", got)
	}
}

func TestClientNames(t *testing.T) {
	for raw, want := range map[string]string{
		"cursor": "Cursor", "Visual Studio Code": "VS Code", "claude-ai": "Claude Desktop",
		"antigravity": "Antigravity", "windsurf-client": "Windsurf", "my-tool": "my-tool",
	} {
		if got := clientName(context.Background(), raw); got != want {
			t.Errorf("%q -> %q, want %q", raw, got, want)
		}
	}
	if waitBudget(mcpOptions{WaitSeconds: -1}, "Claude Desktop") == 0 || waitBudget(mcpOptions{WaitSeconds: -1}, "VS Code") != 0 {
		t.Error("unexpected wait budgets")
	}
}

func TestEmptyConfigFile(t *testing.T) {
	home := setupIDEHome(t)
	p := filepath.Join(home, ".gemini", "antigravity", "mcp_config.json")
	writeFile(t, p, "")
	if st := ideStatus(mustIDE(t, "antigravity")); st.Error != "" || st.Connected || !st.Installed {
		t.Errorf("empty file status = %+v", st)
	}
	if err := ConnectIDE("antigravity"); err != nil {
		t.Fatal(err)
	}
	if st := ideStatus(mustIDE(t, "antigravity")); !st.Connected {
		t.Errorf("not connected after ConnectIDE on empty file: %+v", st)
	}
}
