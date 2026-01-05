# Momentum 🚀
**Keep your AI Agent moving.**

Momentum acts as a bridge between your AI Agent (Cursor, Windsurf, Copilot) and your phone. When the AI needs permission to delete a file or execute a command, it pings your phone. You tap "Approve," and it continues instantly.

### 📥 Download
[**Download Latest Version for Windows**](https://github.com/HarshalPatel1972/momentum/releases/latest)

---

### ⚡ Quick Start Guide (5 Minutes)

#### Step 1: Install
1. Download `Momentum.exe`.
2. Move it to a folder (e.g., `Documents/Momentum`).
3. Double-click to run.
   * *Note: If Windows Defender says "Windows protected your PC", click **More info** → **Run anyway**. (This is normal for new open-source apps).*

#### Step 2: Get Your Free Telegram Keys
You need a bot to talk to you. It's free and takes 1 minute.
1. Open Telegram and search for **@BotFather**.
2. Send the message: `/newbot`
3. Name it (e.g., `MyAgentBridge_Bot`).
4. **Copy the API Token** it gives you (looks like `123456:ABC-DEF...`).
5. Now, search for **@userinfobot** and click Start.
6. **Copy your Id** (looks like `123456789`).

#### Step 3: Connect
1. Open **Momentum**.
2. Click **Add New Channel**.
3. Select **Telegram**.
4. Paste your **Bot Token** and **Chat ID**.
5. Click **Start Bridge**.

#### Step 4: Hook up your Agent
**For VS Code / Cursor / Windsurf:**
1. Open your project settings (`.vscode/mcp.json`).
2. Add this config:
```json
{
  "servers": {
    "remote-bridge": {
      "type": "stdio",
      "command": "C:\\Path\\To\\Momentum.exe",
      "args": ["--mcp"]
    }
  }
}
```
3. **Replace** `C:\\Path\\To\\Momentum.exe` with your actual path.
4. Restart your AI Agent.

**You're done!** Next time the AI asks a question, your phone will buzz.

---

## How It Works

```
AI Agent (VS Code)
    ↓
MCP stdio protocol
    ↓
Momentum (--mcp mode)
    ↓
Ngrok Tunnel (HTTPS)
    ↓
Telegram Bot
    ↓
Your Phone (Interactive form)
    ↓
Response back to Agent
```

---

## Features

✅ **Universal** - Works with any MCP-compatible agent  
✅ **Secure** - Data never leaves your machine  
✅ **Zero Config** - No complex servers  
✅ **Lightweight** - Uses <15MB RAM  
✅ **Beautiful UI** - Interactive HTML forms  
✅ **Multi-Channel** - Telegram, WhatsApp, Discord support  

---

## Troubleshooting

**Windows says "Windows protected your PC"**
- This is normal for new apps. Click "More info" → "Run anyway"

**Not getting Telegram notifications?**
- Make sure you've started a chat with your bot
- Send any message to your bot first (e.g., `/start`)
- Verify your Chat ID is correct

**Agent doesn't connect via MCP?**
- Check the path in mcp.json is correct
- Use double backslashes in Windows paths (`C:\\Path\\To\\Momentum.exe`)
- Restart your IDE after adding the MCP config

---

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- 📧 Email: harshalpatel6828+momentum@gmail.com
- 🐛 Issues: [GitHub Issues](https://github.com/HarshalPatel1972/momentum/issues)

---

**Built with ❤️ to keep your agents moving**
