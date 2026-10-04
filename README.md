<p align="center"><img src="docs/logo.svg" width="96" alt="Momentum logo"></p>

<h1 align="center">Momentum</h1>
<p align="center"><b>Your agent keeps going. Even when you're away.</b></p>

Momentum acts as a bridge between your AI agent (VS Code, Cursor, Windsurf, Antigravity, Claude, Codex and any other MCP client) and your phone. When the AI needs permission to delete a file or execute a command, it pings your phone. You tap "Approve," and it continues instantly.

### 📥 Download

**🎯 Recommended: Installer (Easy Setup)**
- [Download MomentumSetup.exe](https://github.com/HarshalPatel1972/momentum/releases/latest/download/MomentumSetup.exe)
- Automatic installation with shortcuts
- Built-in auto-updates

**💼 Portable Version (Advanced Users)**
- [Download Momentum.exe](https://github.com/HarshalPatel1972/momentum/releases/latest/download/Momentum.exe)
- No installation required
- Manual updates

---

### ⚡ Quick Start Guide (5 Minutes)

#### Step 1: Install

**If you downloaded MomentumSetup.exe (Installer):**
1. Double-click `MomentumSetup.exe`
2. Follow the installation wizard
3. Launch from Start Menu or Desktop shortcut
   * *Note: If Windows Defender says "Windows protected your PC", click **More info** → **Run anyway**.*

**If you downloaded Momentum.exe (Portable):**
1. Move `Momentum.exe` to a folder (e.g., `Documents/Momentum`)
2. Double-click to run
   * *Note: If Windows Defender says "Windows protected your PC", click **More info** → **Run anyway**.*

#### Step 2: Pick where questions reach you

**Telegram (recommended, 1 minute, free):**
1. In Telegram, open **@BotFather** and send `/newbot`.
2. Pick a name (e.g. `MyAgentBridge_Bot`) and **copy the token** it gives you (looks like `123456:ABC-DEF...`).
3. Open your new bot and press **Start**.

That's the only account you need. No ngrok, no port forwarding.

**Slack (great for work, about 3 minutes):** in Momentum choose **Slack** → **Create Slack app**. Slack opens with Momentum's app already filled in; pick your workspace, **Create**, then **Install to Workspace**. Copy the **Bot User OAuth Token** (`xoxb-…`) and an **App-Level Token** with `connections:write` (`xapp-…`) into Momentum, send the Momentum app any DM in Slack, and click **Detect**. The app only asks for `chat:write`, `im:history` and `users:read`.

**Discord (popular with developers, about 3 minutes):** create an application in the [Discord Developer Portal](https://discord.com/developers/applications), open **Bot → Reset Token** and paste the token into Momentum. Click **Add bot to a server** (Discord only lets you DM bots you share a server with; it asks for no permissions), then **Detect** and send the bot any DM.

**ntfy (no account, works in any country):** install the free [ntfy](https://ntfy.sh) app, subscribe to the private topic Momentum shows you, and click **Save & send test**. You can also point it at your own ntfy server. ntfy shows up to 3 answer buttons; to type an answer, use the message box in the ntfy app.

**WhatsApp** also works (via CallMeBot), but it can only send a link, so it needs a free ngrok token too.

#### Step 3: Run the 3-step setup in Momentum
1. **Link your phone:** pick Telegram, Slack, Discord, ntfy or WhatsApp, paste the token(s) and click **Detect**. Momentum finds you and sends a "linked" message to confirm.
2. **Connect your IDEs:** click **Connect all**. Momentum finds the AI IDEs on your PC and adds itself to each one's MCP settings, keeping your other servers as they are. Restart (or reload) each IDE afterwards.
3. **Try it for real:** send yourself a test question and tap the answer on your phone.

From then on Momentum starts by itself and lives in the tray.

#### The pager on your desktop
- **Answer from your PC too:** a waiting page shows on the pager's screen. Press <kbd>Enter</kbd> for the first option, <kbd>Esc</kbd> for the second, or <kbd>R</kbd> to type a reply. The message on your phone updates to match.
- **Mini pager** (<kbd>Ctrl</kbd>+<kbd>M</kbd>): shrinks Momentum into an always-on-top pager in the corner of your screen. If a page arrives while the window is closed, the mini pager pops up so you can answer right there (turn this off in Settings). Start straight into it with `Momentum.exe --mini`.
- **Shortcuts:** <kbd>Ctrl</kbd>+<kbd>1</kbd>–<kbd>4</kbd> switch between Home, Log, IDEs and Link; <kbd>Ctrl</kbd>+<kbd>,</kbd> opens Settings.

#### Away mode
The switch on Momentum's home screen decides where questions go:
- **Away mode on:** questions go to your phone.
- **Away mode off (at your desk):** agents are told to ask in the IDE chat as usual.

The home screen also shows every question your agents asked and what you answered.

#### IDE support

The installer can also do this for you (the *"Connect Momentum to the AI IDEs found on this PC"* option).

| IDE / agent | Setup | Config file Momentum edits |
|---|---|---|
| VS Code (Copilot) / Insiders | Automatic | `%APPDATA%\Code\User\mcp.json` |
| Cursor | Automatic | `%USERPROFILE%\.cursor\mcp.json` |
| Windsurf | Automatic | `%USERPROFILE%\.codeium\windsurf\mcp_config.json` |
| Antigravity | Automatic | `%USERPROFILE%\.gemini\antigravity\mcp_config.json` |
| Claude Code | Automatic | via `claude mcp add-json` (or `~\.claude.json`) |
| Claude Desktop | Automatic | `%APPDATA%\Claude\claude_desktop_config.json` |
| Codex | Automatic | `%USERPROFILE%\.codex\config.toml` |
| Gemini CLI | Automatic | `%USERPROFILE%\.gemini\settings.json` |
| Cline / Roo Code | Automatic | VS Code `globalStorage\…\settings\*.json` |
| Kiro | Automatic | `%USERPROFILE%\.kiro\settings\mcp.json` |
| Zed | Automatic* | `%APPDATA%\Zed\settings.json` |
| JetBrains, anything else | Copy-paste | Momentum shows the JSON to paste |

\* If a config file contains comments (common for Zed), Momentum won't rewrite it. It shows you the snippet to paste instead.

Prefer the command line? These work from any terminal:

```powershell
Momentum.exe --ide-list                 # what's installed / connected
Momentum.exe --ide-connect detected     # connect every IDE found (or: cursor, vscode, all ...)
Momentum.exe --ide-disconnect all       # remove Momentum from every IDE
Momentum.exe --ide-snippet jetbrains    # print the config to paste manually
Momentum.exe --write-rules C:\path\to\project   # optional: add instructions to AGENTS.md
Momentum.exe --status                   # is the bridge running?
```

**You're done!** Next time the AI needs your OK, your phone will buzz.

---

### 🧪 Test It (30 Seconds)

Momentum tells every agent when to use it, so normally you don't have to mention it. To try it right away, ask your agent:

```
"Ask me on my phone whether you should create test.txt, then do what I say"
```

Your phone will buzz. Tap an answer button right in your messaging app (or reply to type your own answer) and watch the agent continue.

---

## How It Works

```
 VS Code ─┐
 Cursor ──┤  MCP (stdio)   ┌───────────────────────┐  question + buttons  ┌──────────┐
 Windsurf ┼──────────────▶ │ Momentum hub (1 per   │ ───────────────────▶ │ Telegram │
 Claude ──┤  each IDE runs │ PC: tray app or       │                      │ on your  │
 Codex …──┘  Momentum --mcp│ background process)   │ ◀─────────────────── │ phone    │
                           └───────────────────────┘   your tap / reply   └──────────┘
                                  (outbound HTTPS only: nothing listens on the internet)
```

- Each IDE starts its own small `Momentum.exe --mcp` process. These all hand their questions to **one shared hub** on `127.0.0.1`, so several IDEs can be open at the same time.
- The hub sends each question to your Telegram bot, Slack or Discord DM, or ntfy topic, with the options as **buttons**. It collects your tap over an outgoing connection: Telegram long-polling, Slack's Socket Mode, the Discord Gateway, or an ntfy subscription. So it needs no tunnel, public URL or firewall change. After you answer, the message updates to show the answer and its buttons disappear.
- Want to type instead? **Reply** to the question (in Slack, reply in its thread). If only one question is open, any message you send answers it. (In ntfy, type in the app's message box; if several questions are open, start with the code shown in the question.)
- If the Momentum window isn't open, the hub starts by itself in the background (tray icon: *Momentum (background)*). Opening the app takes over from it.
- Messages say which IDE and project is asking, e.g. *Cursor · my-app*.
- The agent gets two tools: `ask_remote_human` (asks and waits) and `get_remote_answer`. Some clients (e.g. Claude Desktop) cancel tool calls after about 60 seconds. For those, Momentum returns a `request_id` before that limit and the agent keeps waiting with `get_remote_answer`, so your answer is never lost.
- A question expires after 15 minutes. The agent is told to treat an unanswered question as **not approved**.

### Privacy & security
- Your question text goes through the messaging service you chose (Telegram, Slack, Discord, ntfy, or CallMeBot + an ngrok answer page for WhatsApp). Nothing is stored on any Momentum server; there isn't one.
- Only you can answer: taps and messages from anyone else are ignored. With ntfy there are no user accounts, so the private topic is the key: keep it secret. Each question can be answered once.
- With Telegram, Slack, Discord or ntfy, nothing on your PC is reachable from the internet. The local API that IDEs talk to listens on `127.0.0.1` only and requires a per-user key.
- Config lives in `%APPDATA%\Momentum\` and is shared by every IDE and every copy of `Momentum.exe`.

---

## Features

✅ **Every MCP IDE** - VS Code, Cursor, Windsurf, Antigravity, Claude, Codex, Gemini CLI, Zed, Kiro, Cline, Roo, JetBrains…  
✅ **One-click setup** - Detects your IDEs and edits their config safely (backups kept as `*.momentum.bak`)  
✅ **Many IDEs at once** - One shared hub  
✅ **Lightweight** - Single small exe  
✅ **One-tap answers** - Buttons right in your messaging app, or reply to type your own answer  
✅ **Channels** - Telegram (recommended), Slack, Discord, ntfy (no account, any country), WhatsApp (basic, via CallMeBot + ngrok)  

---

## Troubleshooting

**Windows says "Windows protected your PC"**
- This is normal for new apps. Click "More info" → "Run anyway"

**Not getting Telegram notifications?**
- Open your bot in Telegram and press **Start**, then click **Detect** again in Momentum
- Send `/start` to your bot: if Momentum is running it replies *"Momentum is connected"*
- Check the log: `%APPDATA%\Momentum\momentum.log`

**The IDE doesn't see the Momentum tools**
- Open Momentum → **Connect your IDEs** and check the IDE shows *Connected*. If it says *Points to an old Momentum.exe* (you moved the exe), click **Repair**.
- Restart or reload the IDE after connecting.
- In VS Code, make sure the agent is in **Agent mode** and the `momentum` tools are enabled in the tools picker.

**The agent asks in chat instead of on my phone**
- Run `Momentum.exe --write-rules <your project folder>` (or use the button in the app) to add instructions to the project's `AGENTS.md`.

**Slack: Detect says "No message yet"**
- In Slack, find **Momentum** under *Apps* in the sidebar and send it any message, then click **Detect** again (it waits up to a minute).
- If you can't type in the app's Messages tab, recreate the app from Momentum's manifest (it enables messaging).

**Discord: "not allowed" or Detect never sees my DM**
- Add the bot to a server you're in (Momentum's **Add bot to a server** button), then open the bot from that server's member list and send it a DM *after* clicking Detect.

**ntfy: no notification arrives**
- Check the app is subscribed to exactly the topic Momentum shows (copy it), and on the same server.

**Buttons do nothing / "Conflict: terminated by other getUpdates request" in the log**
- Only one program can read a Telegram bot's updates at a time. Use a bot dedicated to Momentum, not one another tool also uses.

**WhatsApp: "ngrok tunnel failed" / ERR_NGROK_108**
- The free ngrok plan allows one tunnel at a time. Close other ngrok programs. Momentum itself only ever opens one.

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
