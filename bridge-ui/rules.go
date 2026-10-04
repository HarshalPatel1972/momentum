package main

import (
	"os"
	"path/filepath"
	"strings"
)

// The MCP server already sends agentInstructions to every client. Some agents
// weigh project rules files more heavily, so users can also drop the same
// guidance into a project. AGENTS.md is read by Codex, Cursor, Windsurf,
// Copilot, Zed, Antigravity and others; CLAUDE.md / GEMINI.md are updated
// only if the project already has them.

const (
	rulesBegin = "<!-- momentum:begin -->"
	rulesEnd   = "<!-- momentum:end -->"
)

func rulesBlock() string {
	return rulesBegin + "\n## Remote approvals (Momentum)\n\n" + agentInstructions + "\n" + rulesEnd + "\n"
}

// WriteProjectRules adds or refreshes the Momentum block and returns the files it wrote.
func WriteProjectRules(dir string) ([]string, error) {
	var written []string
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"} {
		p := filepath.Join(dir, name)
		data, err := os.ReadFile(p)
		if err != nil && (name != "AGENTS.md" || !os.IsNotExist(err)) {
			continue
		}
		if err := os.WriteFile(p, []byte(upsertBlock(string(data))), 0644); err != nil {
			return written, err
		}
		written = append(written, p)
	}
	return written, nil
}

func upsertBlock(s string) string {
	if i := strings.Index(s, rulesBegin); i >= 0 {
		if j := strings.Index(s[i:], rulesEnd); j >= 0 {
			end := i + j + len(rulesEnd)
			if end < len(s) && s[end] == '\n' {
				end++
			}
			return s[:i] + rulesBlock() + s[end:]
		}
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	if s != "" {
		s += "\n"
	}
	return s + rulesBlock()
}
