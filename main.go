// Package main implements the Remote Bridge MCP Server (Phase 2).
// Remote Bridge is a "Blocking Interceptor" that forces an AI Agent to pause
// and wait for external human input before proceeding.
//
// Phase 2 Features:
// - Ngrok tunnel for public HTTPS access
// - Telegram notifications when agent pauses
// - Request ID tracking for concurrent requests
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

// Global state
var (
	pendingRequests sync.Map
	publicURL       string
	telegramBot     *tgbotapi.BotAPI
	telegramChatID  int64
	httpServer      *http.Server
	httpPort        int
	
	// Dynamic Config Control
	ngrokCancel     context.CancelFunc
	ngrokMutex      sync.Mutex
	configPath      string
	logFile         *os.File
	mu              sync.Mutex // General mutex
)

// BridgeUIConfig struct (Shared with Wails)
type BridgeUIConfig struct {
	NgrokToken      string `json:"ngrokToken"`
	TelegramToken   string `json:"telegramToken"`
	TelegramChatID  string `json:"telegramChatId"`
	DiscordWebhook  string `json:"discordWebhook"`
	WhatsappEnabled bool   `json:"whatsappEnabled"`
	WhatsappKey     string `json:"whatsappKey"`
	UserPhone       string `json:"userPhone"`
}

func main() {
	setupLogging()
	defer logFile.Close()

	logInfo("🚀 Remote Bridge Starting...")

	// Locate config file
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	configPath = filepath.Join(exeDir, "bridge-config.json")
	
	// Initial Config Load
	loadAndApplyConfig()

	// Start Config Watcher (Hot Reload)
	go watchConfig()
	
	// Start Services
	startServices()

	// MCP Server Loop
	startMCPServer()
}

func setupLogging() {
	exePath, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(exePath), "bridge.log")
	
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	logFile = f
}

func logInfo(msg string) {
	// 1. To Agent (Stderr)
	fmt.Fprintln(os.Stderr, "[BRIDGE] "+msg)
	
	// 2. To UI (Log File)
	if logFile != nil {
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(logFile, "[%s] %s\n", ts, msg)
	}
}

func watchConfig() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logInfo("❌ Config watcher failed: " + err.Error())
		return
	}
	defer watcher.Close()

	// Watch the directory (more robust for file atomic writes)
	watcher.Add(filepath.Dir(configPath))

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Name == configPath && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
				logInfo("🔄 Config change detected! Reloading...")
				// Add slight delay to ensure write complete
				time.Sleep(100 * time.Millisecond)
				loadAndApplyConfig()
				restartServices()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logInfo("Watcher error: " + err.Error())
		}
	}
}

func loadAndApplyConfig() {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Fallback to .env
		if envErr := godotenv.Load(); envErr == nil {
			logInfo("Loaded .env (No JSON config found)")
		} else {
			logInfo("⚠️ No config found. Waiting for UI...")
		}
		return
	}

	var cfg BridgeUIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		logInfo("❌ Config parse error: " + err.Error())
		return
	}

	// update env vars
	if cfg.NgrokToken != "" { os.Setenv("NGROK_AUTHTOKEN", cfg.NgrokToken) }
	if cfg.TelegramToken != "" { os.Setenv("TELEGRAM_BOT_TOKEN", cfg.TelegramToken) }
	if cfg.TelegramChatID != "" { os.Setenv("TELEGRAM_CHAT_ID", cfg.TelegramChatID) }
	if cfg.DiscordWebhook != "" { os.Setenv("DISCORD_WEBHOOK_URL", cfg.DiscordWebhook) }
	if cfg.WhatsappKey != "" { os.Setenv("WHATSAPP_API_KEY", cfg.WhatsappKey) }
	if cfg.UserPhone != "" { os.Setenv("USER_PHONE", cfg.UserPhone) }
	
	initNotifications() // Reload notification settings
	logInfo("✅ Configuration Applied")
}

func startServices() {
	// Telegram
	if err := initTelegram(); err != nil {
		logInfo("⚠️ Telegram skipped: " + err.Error())
	}
	
	// HTTP Server
	if httpServer == nil {
		startHTTPServer()
	}

	// Ngrok
	restartNgrok()
}

func restartServices() {
	// Re-init Telegram
	if err := initTelegram(); err != nil {
		logInfo("⚠️ Telegram re-init failed: " + err.Error())
	}
	// Restart Ngrok
	restartNgrok()
}

func restartNgrok() {
	ngrokMutex.Lock()
	defer ngrokMutex.Unlock()

	// Stop existing
	if ngrokCancel != nil {
		ngrokCancel()
		logInfo("Stopped previous tunnel")
	}

	// Start new
	ctx, cancel := context.WithCancel(context.Background())
	ngrokCancel = cancel

	go func() {
		token := os.Getenv("NGROK_AUTHTOKEN")
		if token == "" {
			logInfo("⚠️ No Ngrok Token. Public access disabled.")
			publicURL = fmt.Sprintf("http://127.0.0.1:%d", httpPort)
			return
		}

		tun, err := ngrok.Listen(ctx,
			config.HTTPEndpoint(),
			ngrok.WithAuthtoken(token),
		)
		if err != nil {
			if ctx.Err() == nil { // Only log if not cancelled intentionally
				logInfo("❌ Ngrok failed: " + err.Error())
			}
			return
		}

		publicURL = tun.URL()
		logInfo("🌍 Tunnel Active: " + publicURL)
		
		// Simple forwarder
		for {
			conn, err := tun.Accept()
			if err != nil { return }
			go handleTunnelConn(conn)
		}
	}()
}

func handleTunnelConn(remote net.Conn) {
	defer remote.Close()
	local, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", httpPort))
	if err != nil { return }
	defer local.Close()
	
	go io.Copy(local, remote)
	io.Copy(remote, local)
}

func startHTTPServer() {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	httpPort = listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHTTPRequest)
	mux.HandleFunc("/submit", handleHTTPSubmit)

	httpServer = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", httpPort), Handler: mux}
	
	go func() {
		httpServer.ListenAndServe()
	}()
	logInfo(fmt.Sprintf("🌐 Local Server: http://127.0.0.1:%d", httpPort))
}

func startMCPServer() {
	s := server.NewMCPServer("Remote Bridge", "2.1.0", server.WithToolCapabilities(false))

	askTool := mcp.NewTool("ask_remote_human",
		mcp.WithDescription("Ask the user a question via configured channels (Telegram/Discord/WhatsApp)"),
		mcp.WithString("question", mcp.Required()),
		mcp.WithArray("options", mcp.Required()),
	)

	s.AddTool(askTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		question, _ := request.RequireString("question")
		args := request.GetArguments()
		optionsSlice, _ := args["options"].([]interface{})
		
		var options []string
		for _, o := range optionsSlice {
			if s, ok := o.(string); ok { options = append(options, s) }
		}

		reqID := uuid.New().String()[:8]
		logInfo(fmt.Sprintf("🔔 Question: %s", question))
		
		// Wait for URL if Ngrok is restarting
		if publicURL == "" {
			time.Sleep(2 * time.Second)
		}
		remoteURL := fmt.Sprintf("%s/?id=%s", publicURL, reqID)
		
		broadcastNotification(question, options, remoteURL, reqID)

		// Create response channel
		respChan := make(chan string, 1)
		pendingRequests.Store(reqID, respChan)
		defer pendingRequests.Delete(reqID)
		
		// Details for HTTP handler
		requestDetails.Store(reqID, RequestDetails{Question: question, Options: options})
		defer requestDetails.Delete(reqID)

		select {
		case resp := <-respChan:
			logInfo("✅ Response: " + resp)
			return mcp.NewToolResultText("User Response: " + resp), nil
		case <-time.After(15 * time.Minute):
			return mcp.NewToolResultError("Timeout"), nil
		case <-ctx.Done():
			return mcp.NewToolResultError("Cancelled"), nil
		}
	})

	logInfo("📡 MCP Server listening on Stdio")
	if err := server.ServeStdio(s); err != nil {
		logInfo("Fatal Server Error: " + err.Error())
		os.Exit(1)
	}
}

// ----- Helper Functions (Telegram, etc. reuse previous logic) -----
// (Assuming initTelegram, initNotifications, broadcastNotification, handleHTTPRequest 
//  are mostly unchanged but using logInfo instead of fmt.Fprintf)

// Re-implementing critical helpers for completeness in this overwrite
func initTelegram() error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" { return fmt.Errorf("No token") }
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)
	telegramChatID = chatID
	
	var err error
	telegramBot, err = tgbotapi.NewBotAPI(token)
	if err != nil { return err }
	logInfo("🤖 Telegram Active: " + telegramBot.Self.UserName)
	return nil
}

// ... (Rest of HTTP handlers would be here, effectively same as before but cleaner)
// For brevity in this tool call, I will include the HTTP handlers to ensure compilation.

type RequestDetails struct { Question string; Options []string }
var requestDetails sync.Map

func handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if val, ok := requestDetails.Load(id); ok {
		details := val.(RequestDetails)
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		
		// Build option buttons HTML
		optionsHTML := ""
		for _, opt := range details.Options {
			optionsHTML += fmt.Sprintf(`<button class="option-btn" onclick="submitAnswer('%s')">%s</button>`, opt, opt)
		}
		
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Remote Bridge - Respond</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
			margin: 0;
			padding: 20px;
		}
		.container {
			background: white;
			border-radius: 20px;
			box-shadow: 0 20px 60px rgba(0,0,0,0.3);
			max-width: 500px;
			width: 100%%;
			padding: 40px;
		}
		h1 {
			color: #333;
			margin: 0 0 10px 0;
			font-size: 24px;
		}
		.subtitle {
			color: #666;
			margin: 0 0 30px 0;
			font-size: 14px;
		}
		.question {
			background: #f8f9fa;
			padding: 20px;
			border-radius: 10px;
			margin-bottom: 30px;
			color: #333;
			font-size: 16px;
			line-height: 1.5;
		}
		.options {
			display: flex;
			flex-direction: column;
			gap: 10px;
			margin-bottom: 20px;
		}
		.option-btn {
			background: #667eea;
			color: white;
			border: none;
			padding: 15px 20px;
			border-radius: 10px;
			font-size: 16px;
			cursor: pointer;
			transition: all 0.3s;
		}
		.option-btn:hover {
			background: #5568d3;
			transform: translateY(-2px);
			box-shadow: 0 5px 15px rgba(102, 126, 234, 0.4);
		}
		.divider {
			text-align: center;
			margin: 20px 0;
			color: #999;
			font-size: 14px;
		}
		.custom-input {
			width: 100%%;
			padding: 15px;
			border: 2px solid #e0e0e0;
			border-radius: 10px;
			font-size: 16px;
			box-sizing: border-box;
			margin-bottom: 10px;
		}
		.custom-input:focus {
			outline: none;
			border-color: #667eea;
		}
		.submit-btn {
			width: 100%%;
			background: #764ba2;
			color: white;
			border: none;
			padding: 15px;
			border-radius: 10px;
			font-size: 16px;
			cursor: pointer;
			font-weight: 600;
		}
		.submit-btn:hover {
			background: #653a8a;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>🤖 Agent Question</h1>
		<p class="subtitle">Please provide your response</p>
		<div class="question">%s</div>
		<div class="options">%s</div>
		<div class="divider">OR</div>
		<input type="text" id="customAnswer" class="custom-input" placeholder="Type your custom answer...">
		<button class="submit-btn" onclick="submitCustom()">Send Custom Answer</button>
	</div>
	<script>
		function submitAnswer(answer) {
			window.location.href = '/submit?id=%s&response=' + encodeURIComponent(answer);
		}
		function submitCustom() {
			const custom = document.getElementById('customAnswer').value;
			if (custom.trim()) {
				window.location.href = '/submit?id=%s&response=' + encodeURIComponent(custom);
			} else {
				alert('Please enter an answer');
			}
		}
	</script>
</body>
</html>`, details.Question, optionsHTML, id, id)
	} else {
		http.NotFound(w, r)
	}
}

func handleHTTPSubmit(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	resp := r.FormValue("response")
	if resp == "" {
		resp = r.URL.Query().Get("response") // Also check URL params for JS submissions
	}
	if ch, ok := pendingRequests.Load(id); ok {
		ch.(chan string) <- resp
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<style>
		body { font-family: sans-serif; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); min-height: 100vh; display: flex; align-items: center; justify-content: center; margin: 0; }
		.container { background: white; border-radius: 20px; padding: 40px; text-align: center; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
		h1 { color: #4CAF50; margin: 0 0 20px 0; }
		p { color: #666; margin: 10px 0; }
		.answer { background: #f8f9fa; padding: 15px; border-radius: 10px; margin: 20px 0; color: #333; font-weight: 600; }
	</style>
</head>
<body>
	<div class="container">
		<h1>✅ Response Sent!</h1>
		<p>You answered:</p>
		<div class="answer">%s</div>
		<p>You can close this window.</p>
	</div>
</body>
</html>`, resp)
	} else {
		http.Error(w, "Request not found or expired", 404)
	}
}
