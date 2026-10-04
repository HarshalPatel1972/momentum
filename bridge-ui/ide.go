package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Every IDE speaks MCP over stdio, but each keeps its server list in a
// different file and under a different key. This file knows those locations
// and edits them in place, leaving the user's other servers untouched.

const serverName = "momentum"

type ideDef struct {
	ID     string
	Name   string
	Format string // "servers" (VS Code), "mcpServers", "context_servers" (Zed), "toml" (Codex), "manual"
	Path   func() string
	Detect func() bool
	Extra  map[string]any // extra fields for this IDE's entry (e.g. tool timeouts)
	Note   string
}

type IDEStatus struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Installed  bool   `json:"installed"`
	Connected  bool   `json:"connected"`
	Stale      bool   `json:"stale"` // connected, but to a different Momentum.exe path
	ConfigPath string `json:"configPath"`
	Manual     bool   `json:"manual"`
	Note       string `json:"note"`
	Error      string `json:"error,omitempty"`
}

// ideHome / ideConfigDir honour MOMENTUM_IDE_HOME so tests never touch real IDE configs.
func ideHome() string {
	if h := os.Getenv("MOMENTUM_IDE_HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func ideConfigDir() string {
	if h := os.Getenv("MOMENTUM_IDE_HOME"); h != "" {
		return filepath.Join(h, "AppData", "Roaming")
	}
	d, _ := os.UserConfigDir()
	return d
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func homePath(parts ...string) func() string {
	return func() string { return filepath.Join(append([]string{ideHome()}, parts...)...) }
}

func cfgPath(parts ...string) func() string {
	return func() string { return filepath.Join(append([]string{ideConfigDir()}, parts...)...) }
}

func existsFn(f func() string) func() bool { return func() bool { return exists(f()) } }

func vscodeUserDir(flavor string) func() string { return cfgPath(flavor, "User") }

func zedSettings() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(ideConfigDir(), "Zed", "settings.json")
	}
	return filepath.Join(ideHome(), ".config", "zed", "settings.json")
}

var ideDefs = []ideDef{
	{ID: "vscode", Name: "VS Code (Copilot)", Format: "servers",
		Path: cfgPath("Code", "User", "mcp.json"), Detect: existsFn(vscodeUserDir("Code"))},
	{ID: "vscode-insiders", Name: "VS Code Insiders", Format: "servers",
		Path: cfgPath("Code - Insiders", "User", "mcp.json"), Detect: existsFn(vscodeUserDir("Code - Insiders"))},
	{ID: "cursor", Name: "Cursor", Format: "mcpServers",
		Path: homePath(".cursor", "mcp.json"), Detect: existsFn(homePath(".cursor"))},
	{ID: "windsurf", Name: "Windsurf", Format: "mcpServers",
		Path: homePath(".codeium", "windsurf", "mcp_config.json"), Detect: existsFn(homePath(".codeium", "windsurf"))},
	{ID: "antigravity", Name: "Antigravity", Format: "mcpServers",
		Path:   homePath(".gemini", "antigravity", "mcp_config.json"),
		Detect: func() bool { return exists(homePath(".gemini", "antigravity")()) || exists(cfgPath("Antigravity")()) }},
	{ID: "claude-code", Name: "Claude Code", Format: "mcpServers",
		Path: homePath(".claude.json"), Detect: existsFn(homePath(".claude.json"))},
	{ID: "claude-desktop", Name: "Claude Desktop", Format: "mcpServers",
		Path: cfgPath("Claude", "claude_desktop_config.json"), Detect: existsFn(cfgPath("Claude"))},
	{ID: "gemini-cli", Name: "Gemini CLI", Format: "mcpServers",
		Path: homePath(".gemini", "settings.json"), Detect: existsFn(homePath(".gemini", "settings.json")),
		Extra: map[string]any{"timeout": 3600000}},
	{ID: "codex", Name: "Codex", Format: "toml",
		Path: homePath(".codex", "config.toml"), Detect: existsFn(homePath(".codex"))},
	{ID: "cline", Name: "Cline (VS Code)", Format: "mcpServers",
		Path:   cfgPath("Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "cline_mcp_settings.json"),
		Detect: existsFn(cfgPath("Code", "User", "globalStorage", "saoudrizwan.claude-dev")),
		Extra:  map[string]any{"timeout": 3600}},
	{ID: "roo", Name: "Roo Code (VS Code)", Format: "mcpServers",
		Path:   cfgPath("Code", "User", "globalStorage", "rooveterinaryinc.roo-cline", "settings", "mcp_settings.json"),
		Detect: existsFn(cfgPath("Code", "User", "globalStorage", "rooveterinaryinc.roo-cline")),
		Extra:  map[string]any{"timeout": 3600}},
	{ID: "kiro", Name: "Kiro", Format: "mcpServers",
		Path: homePath(".kiro", "settings", "mcp.json"), Detect: existsFn(homePath(".kiro"))},
	{ID: "zed", Name: "Zed", Format: "context_servers",
		Path: zedSettings, Detect: func() bool { return exists(filepath.Dir(zedSettings())) }},
	{ID: "jetbrains", Name: "JetBrains IDEs", Format: "manual",
		Path: func() string { return "" }, Detect: func() bool { return false },
		Note: "Settings → Tools → AI Assistant → Model Context Protocol → Add → paste this JSON."},
	{ID: "other", Name: "Any other MCP client", Format: "manual",
		Path: func() string { return "" }, Detect: func() bool { return false },
		Note: "Most clients accept this \"mcpServers\" JSON. Use stdio transport."},
}

func findIDE(id string) (ideDef, bool) {
	for _, d := range ideDefs {
		if d.ID == id {
			return d, true
		}
	}
	return ideDef{}, false
}

func momentumExe() string {
	exe, err := os.Executable()
	if err != nil {
		return "Momentum.exe"
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return exe
}

func serverArgs(id string) []string { return []string{"--mcp", "--client", id} }

// entryFor builds this IDE's server entry, as an ordered JSON object.
func entryFor(d ideDef) json.RawMessage {
	fields := []kv{}
	add := func(k string, v any) {
		b, _ := json.Marshal(v)
		fields = append(fields, kv{k, b})
	}
	if d.Format == "servers" {
		add("type", "stdio")
	}
	if d.Format == "context_servers" {
		add("source", "custom")
	}
	add("command", momentumExe())
	add("args", serverArgs(d.ID))
	for k, v := range d.Extra {
		add(k, v)
	}
	return renderCompact(fields)
}

// ManualSnippet is what the user pastes when we can't (or shouldn't) edit the file.
func ManualSnippet(id string) string {
	d, ok := findIDE(id)
	if !ok {
		d, _ = findIDE("other")
	}
	if d.Format == "toml" {
		return codexBlock(d)
	}
	key := d.Format
	if key == "manual" {
		key = "mcpServers"
	}
	inner := renderCompact([]kv{{serverName, entryFor(d)}})
	return string(indentJSON(renderCompact([]kv{{key, inner}})))
}

func ListIDEs() []IDEStatus {
	var out []IDEStatus
	for _, d := range ideDefs {
		out = append(out, ideStatus(d))
	}
	return out
}

func ideStatus(d ideDef) IDEStatus {
	st := IDEStatus{ID: d.ID, Name: d.Name, ConfigPath: d.Path(), Manual: d.Format == "manual", Note: d.Note}
	if st.Manual {
		return st
	}
	st.Installed = d.Detect() || exists(st.ConfigPath)
	cmd, err := currentEntryCommand(d)
	if err != nil {
		st.Error = err.Error()
	}
	if cmd != "" {
		st.Connected = true
		st.Stale = !strings.EqualFold(filepath.Clean(cmd), filepath.Clean(momentumExe()))
	}
	return st
}

// currentEntryCommand returns the command of our entry in the IDE's config, or "".
func currentEntryCommand(d ideDef) (string, error) {
	data, err := os.ReadFile(d.Path())
	if err != nil {
		return "", nil
	}
	if d.Format == "toml" {
		block := tomlBlock(string(data))
		for _, line := range strings.Split(block, "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && strings.TrimSpace(k) == "command" {
				v = strings.TrimSpace(v)
				if len(v) >= 2 && v[0] == '\'' {
					return v[1 : len(v)-1], nil
				}
				var s string
				json.Unmarshal([]byte(v), &s)
				return s, nil
			}
		}
		return "", nil
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", nil // IDEs such as Antigravity create the file empty
	}
	if !json.Valid(data) {
		data = stripJSONC(data)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("could not read %s: %v", d.Path(), err)
	}
	var servers map[string]struct {
		Command string `json:"command"`
	}
	json.Unmarshal(doc[d.Format], &servers)
	return servers[serverName].Command, nil
}

var errHasComments = errors.New("this config file contains comments, so Momentum won't rewrite it automatically; paste the snippet manually")

// ConnectIDE adds (or refreshes) Momentum in the IDE's MCP config.
func ConnectIDE(id string) error {
	d, ok := findIDE(id)
	if !ok {
		return fmt.Errorf("unknown IDE %q", id)
	}
	switch d.Format {
	case "manual":
		return errors.New("this client is configured manually; copy the snippet")
	case "toml":
		return editTOML(d, true)
	}
	if d.ID == "claude-code" {
		if done, err := claudeCLI(true); done {
			return err
		}
	}
	return editJSON(d, true)
}

func DisconnectIDE(id string) error {
	d, ok := findIDE(id)
	if !ok {
		return fmt.Errorf("unknown IDE %q", id)
	}
	switch d.Format {
	case "manual":
		return nil
	case "toml":
		return editTOML(d, false)
	}
	if d.ID == "claude-code" {
		if done, err := claudeCLI(false); done {
			return err
		}
	}
	return editJSON(d, false)
}

// claudeCLI uses `claude mcp` when a native claude.exe is on PATH, because
// Claude Code rewrites ~/.claude.json itself while it runs.
func claudeCLI(add bool) (bool, error) {
	if os.Getenv("MOMENTUM_IDE_HOME") != "" {
		return false, nil
	}
	bin, err := exec.LookPath("claude")
	if err != nil || (runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(bin), ".exe")) {
		return false, nil
	}
	exec.Command(bin, "mcp", "remove", "--scope", "user", serverName).Run()
	if !add {
		return true, nil
	}
	d, _ := findIDE("claude-code")
	out, err := exec.Command(bin, "mcp", "add-json", "--scope", "user", serverName, string(entryFor(d))).CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("claude mcp add-json failed: %s", strings.TrimSpace(string(out)))
	}
	return true, nil
}

func editJSON(d ideDef, add bool) error {
	path := d.Path()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existed := err == nil
	if len(bytes.TrimSpace(data)) == 0 {
		if !add {
			return nil
		}
		data = []byte("{}")
	}
	if !json.Valid(data) {
		if json.Valid(stripJSONC(data)) {
			return errHasComments
		}
		return fmt.Errorf("%s is not valid JSON; fix or remove it first", path)
	}
	top, err := parseObject(data)
	if err != nil {
		return fmt.Errorf("%s: top level must be a JSON object", path)
	}
	inner := []kv{}
	if v, ok := getKV(top, d.Format); ok && string(bytes.TrimSpace(v)) != "null" {
		if inner, err = parseObject(v); err != nil {
			return fmt.Errorf("%s: %q must be an object", path, d.Format)
		}
	}
	before := len(inner)
	// Older READMEs told users to add Momentum as "remote-bridge"; drop that duplicate.
	inner = removeWhere(inner, func(k string, v json.RawMessage) bool {
		if k == serverName {
			return true
		}
		var e struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		json.Unmarshal(v, &e)
		return strings.Contains(strings.ToLower(filepath.Base(e.Command)), "momentum") && len(e.Args) > 0 && e.Args[0] == "--mcp"
	})
	if add {
		inner = append(inner, kv{serverName, entryFor(d)})
	} else if len(inner) == before {
		return nil // nothing of ours in the file; don't touch it
	}
	top = setKV(top, d.Format, renderCompact(inner))
	if existed {
		os.WriteFile(path+".momentum.bak", data, 0600)
	}
	return writeFileAtomic(path, append(indentJSON(renderCompact(top)), '\n'))
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".momentum.tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------- Codex (TOML) ----------

func codexBlock(d ideDef) string {
	exe := momentumExe()
	cmd := "'" + exe + "'" // TOML literal string: no backslash escaping needed
	if strings.Contains(exe, "'") {
		b, _ := json.Marshal(exe)
		cmd = string(b)
	}
	args, _ := json.Marshal(serverArgs(d.ID))
	return fmt.Sprintf("[mcp_servers.%s]\ncommand = %s\nargs = %s\n# Codex stops MCP tool calls after 60s by default; the user may take longer to answer.\ntool_timeout_sec = 3600\n",
		serverName, cmd, string(args))
}

// tomlBlock returns our [mcp_servers.momentum] section (including subtables), or "".
func tomlBlock(s string) string {
	start, end := tomlBlockRange(s)
	if start < 0 {
		return ""
	}
	return s[start:end]
}

func tomlBlockRange(s string) (int, int) {
	header := "[mcp_servers." + serverName
	lines := strings.SplitAfter(s, "\n")
	pos, start := 0, -1
	for _, line := range lines {
		t := strings.TrimSpace(line)
		isHeader := strings.HasPrefix(t, "[")
		if start < 0 && (t == header+"]" || strings.HasPrefix(t, header+".")) {
			start = pos
		} else if start >= 0 && isHeader && !(t == header+"]" || strings.HasPrefix(t, header+".")) {
			return start, pos
		}
		pos += len(line)
	}
	if start < 0 {
		return -1, -1
	}
	return start, len(s)
}

func editTOML(d ideDef, add bool) error {
	path := d.Path()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	s := string(data)
	if start, end := tomlBlockRange(s); start >= 0 {
		s = s[:start] + s[end:]
	}
	if add {
		if s != "" && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		if s != "" && !strings.HasSuffix(s, "\n\n") {
			s += "\n"
		}
		s += codexBlock(d)
	} else if err != nil {
		return nil
	}
	if len(data) > 0 {
		os.WriteFile(path+".momentum.bak", data, 0600)
	}
	return writeFileAtomic(path, []byte(s))
}

// ---------- ordered JSON objects ----------

type kv struct {
	K string
	V json.RawMessage
}

// parseObject decodes a JSON object keeping key order, so we don't reshuffle the user's file.
func parseObject(data []byte) ([]kv, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New("not an object")
	}
	var out []kv
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, kv{key, v})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return out, nil
}

func renderCompact(kvs []kv) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, p := range kvs {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(p.K)
		b.Write(k)
		b.WriteByte(':')
		b.Write(p.V)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func indentJSON(raw []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return raw
	}
	return out.Bytes()
}

func getKV(kvs []kv, k string) (json.RawMessage, bool) {
	for _, p := range kvs {
		if p.K == k {
			return p.V, true
		}
	}
	return nil, false
}

func setKV(kvs []kv, k string, v json.RawMessage) []kv {
	for i, p := range kvs {
		if p.K == k {
			kvs[i].V = v
			return kvs
		}
	}
	return append(kvs, kv{k, v})
}

func removeWhere(kvs []kv, drop func(string, json.RawMessage) bool) []kv {
	out := kvs[:0]
	for _, p := range kvs {
		if !drop(p.K, p.V) {
			out = append(out, p)
		}
	}
	return out
}

// stripJSONC removes // and /* */ comments and trailing commas (outside strings).
func stripJSONC(data []byte) []byte {
	var out []byte
	inStr, esc := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inStr {
			out = append(out, c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++
		case c == ']' || c == '}':
			// drop a trailing comma before the closer
			j := len(out) - 1
			for j >= 0 && (out[j] == ' ' || out[j] == '\t' || out[j] == '\n' || out[j] == '\r') {
				j--
			}
			if j >= 0 && out[j] == ',' {
				out = append(out[:j], out[j+1:]...)
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}
