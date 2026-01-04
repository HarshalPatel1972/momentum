# 🌉 Remote Bridge: Setup Guide

Because the Bridge needs to "talk" to the Agent continuously (via Stdio), we use a **Split Architecture**:

1.  **The UI (Bridge Control)**: A beautiful app to set your keys and preferences.
2.  **The Engine (CLI)**: A silent background process that actually connects to the Agent.

## Step 1: Configure with UI
1.  Open the terminal in `bridge-ui`: `cd bridge-ui`
2.  Run `wails dev`
3.  Enter your **Ngrok Token**, **Telegram**, etc.
4.  Click **💾 Save Config**.
5.  *Optional*: You can click "Start" to test if your keys work (logs will show "Tunnel Active"), but **Stop it** before Step 2.

## Step 2: "Install" the Bridge
The Agent uses `bridge.exe` in the root folder. We need to make sure it's the latest version that reads your new config.

1.  Open a new terminal in the **root** folder (`test-19`).
2.  Run: `go build -o bridge.exe .`

## Step 3: Activate
1.  **Restart VS Code** (or reload the window).
2.  The Agent will automatically start `bridge.exe`.
3.  It will read your saved `bridge-config.json`.
4.  **Done!** 🎉

## Troubleshooting
-   **"Connection Closed"**: You might have `wails dev` running while the Agent is trying to connect. Close the UI.
-   **"Ngrok Error 334"**: The tunnel is already open. Kill old processes: `taskkill /IM bridge.exe /F`
