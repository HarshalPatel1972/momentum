# README

## About

This is the official Wails React-TS template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

## Development

```bash
wails build                      # -> build/bin/Momentum.exe
go test ./...                    # unit + hub tests (no network needed)
MOMENTUM_EXE=build/bin/Momentum.exe go test -tags e2e -run E2E -v .   # drives the real exe over MCP stdio
```

Source map:

| File | What it does |
|---|---|
| `hub.go` | The single per-user hub: localhost API for IDEs; starts the Telegram poller (or ngrok tunnel for WhatsApp) |
| `telegram.go` | Telegram: questions with answer buttons, long-polling for taps/replies, chat-ID detection |
| `mcp_server.go` | `--mcp` mode: the thin MCP stdio server each IDE launches; starts the hub on demand |
| `ide.go` | Detects IDEs and edits their MCP config files (`--ide-*` flags, "Connect your IDEs" screen) |
| `notify.go`, `pages.go` | WhatsApp message and the mobile answer page it links to |
| `rules.go` | Optional AGENTS.md / CLAUDE.md instructions block |
| `paths.go` | Config/log locations (`%APPDATA%\Momentum`), version |

Test-only environment variables: `MOMENTUM_HOME` (config dir), `MOMENTUM_PORT` (hub port, default 47821), `MOMENTUM_IDE_HOME` (fake home for IDE configs), `MOMENTUM_TELEGRAM_API` (fake Telegram), `MOMENTUM_NO_TRAY`, `MOMENTUM_ALLOW_LOCAL_LINKS=1` (allow 127.0.0.1 answer links without ngrok).
