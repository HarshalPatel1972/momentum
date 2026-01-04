# 🚀 Momentum

**Keep your AI Agent moving.**

Never return to a stuck agent again. Momentum connects your local AI agents to your mobile device, letting you approve critical decisions from anywhere in the world.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey)

## The Problem

You're using advanced AI tools like VS Code Copilot, Cursor, or Windsurf to build software. You give the agent a complex task and step away. 

The Agent hits a roadblock:
> "I need to delete main.go to proceed. Confirm?"

The Agent pauses. It waits. You return hours later to find it stuck on step 1, waiting for a simple "Yes."

## The Solution

Momentum is a desktop bridge that intercepts agent requests and pushes them to your phone. You tap "Approve" or "Deny" from anywhere, and the Agent immediately resumes work.

### How It Works

1. **Intercept** - Acts as a local MCP (Model Context Protocol) Server
2. **Tunnel** - Creates secure tunnel via Ngrok (zero router config)
3. **Notify** - Pushes to Telegram, Ntfy, Discord, or Email
4. **Relay** - Your tap unblocks the Agent instantly

## Features

✅ **Universal** - Works with any MCP-compatible agent (Claude Desktop, Cursor, Copilot)  
✅ **Secure** - Data never leaves your machine; only permission requests tunneled  
✅ **Zero Config** - No complex servers. Just run and scan QR code  
✅ **Lightweight** - System Tray app using <15MB RAM  
✅ **Beautiful UI** - Interactive HTML forms for quick decisions  
✅ **Multi-Channel** - Telegram, WhatsApp, Discord, Email support  

## Installation

### Download Release

Download the latest release for your platform from [Releases](https://github.com/yourusername/momentum/releases).

### From Source

**Prerequisites:**
- Go 1.21+
- Node.js 18+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

**Build:**
```bash
git clone https://github.com/yourusername/momentum.git
cd momentum/bridge-ui
wails build
```

## Quick Start

1. **Launch Momentum** - Run the app (it minimizes to System Tray)
2. **Configure** - Click the System Tray icon → "Configure"
3. **Add Channel** - Select Telegram/Discord/Email and enter credentials
4. **Test** - Use the built-in test to verify notifications work
5. **Connect Agent** - Configure your AI agent to use Momentum's MCP server

### MCP Configuration

Add to your AI agent's MCP config (`.vscode/mcp.json` for VS Code):

```json
{
  "servers": {
    "momentum": {
      "type": "stdio",
      "command": "C:/path/to/momentum.exe",
      "args": ["--mcp"]
    }
  }
}
```

## Usage

When your AI agent needs permission:

1. **Agent Pauses** - Calls `ask_remote_human` tool
2. **You Get Notified** - Telegram/Discord/Email on your phone
3. **Tap to Decide** - Opens beautiful form with options
4. **Agent Resumes** - Gets your answer immediately

## Architecture

```
AI Agent (VS Code)
    ↓
MCP stdio protocol
    ↓
Momentum (--mcp mode)
    ↓
Ngrok Tunnel (HTTPS)
    ↓
Notification Channel (Telegram/Discord/etc)
    ↓
Your Phone (Interactive HTML form)
    ↓
Response back to Agent
```

## Tech Stack

- **Backend**: Go, Wails v2
- **Frontend**: React, TypeScript, Vite
- **Networking**: Ngrok
- **Protocols**: MCP (Model Context Protocol)
- **Notifications**: Telegram Bot API, CallMeBot, Discord Webhooks

## Development

**Run in Dev Mode:**
```bash
cd bridge-ui
wails dev
```

**Run MCP Server:**
```bash
./momentum.exe --mcp
```

**Project Structure:**
```
momentum/
├── bridge-ui/           # Wails desktop app
│   ├── frontend/        # React UI
│   ├── bridge.go        # Bridge service
│   ├── mcp_server.go    # MCP stdio server
│   └── app.go           # Wails app logic
├── main.go              # Standalone CLI version
├── notifications.go     # Notification channels
└── .vscode/mcp.json     # MCP configuration
```

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- 📧 Email: support@momentum.dev
- 💬 Discord: [Join Server](https://discord.gg/momentum)
- 🐛 Issues: [GitHub Issues](https://github.com/yourusername/momentum/issues)

## Roadmap

- [ ] macOS and Linux builds
- [ ] Additional notification channels (Slack, SMS)
- [ ] QR code quick setup
- [ ] Request history and analytics
- [ ] Multi-device support
- [ ] Custom approval workflows

---

**Built with ❤️ to keep your agents moving**
