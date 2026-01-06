package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"

	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

// BridgeService manages the MCP server and Ngrok tunnel
type BridgeService struct {
	ctx       context.Context // Wails app context for events
	cancel    context.CancelFunc
	tunnel    ngrok.Tunnel
	running   bool
	mu        sync.Mutex
	publicURL string
	cfg       BridgeConfig

	// Pending requests waiting for user response
	pendingRequests map[string]chan string
	requestData     map[string]RequestData
	pendingMu       sync.Mutex
}

type RequestData struct {
	Question string
	Options  []string
}

// NewBridgeService creates a new bridge service instance
func NewBridgeService() *BridgeService {
	return &BridgeService{
		pendingRequests: make(map[string]chan string),
		requestData:     make(map[string]RequestData),
	}
}

// SetContext sets the Wails context for emitting events
func (b *BridgeService) SetContext(ctx context.Context) {
	b.ctx = ctx
}

// log emits a log message to the frontend
func (b *BridgeService) log(message string) {
	// Only emit to UI if we have a valid Wails context
	if b.ctx != nil {
		// Try to emit, but don't fail if context is invalid
		defer func() {
			if recover() != nil {
				// Context not available, just log to stderr
			}
		}()
		// Only call EventsEmit if we're in UI mode
		if b.ctx != context.Background() {
			runtime.EventsEmit(b.ctx, "log", message)
		}
	}
	fmt.Println("[BRIDGE]", message)
}

// IsRunning returns the current state
func (b *BridgeService) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// Start initializes the Ngrok tunnel and MCP server
func (b *BridgeService) Start(cfg BridgeConfig) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		b.log("⚠️ Bridge is already running!")
		return fmt.Errorf("bridge is already running")
	}
	b.cfg = cfg
	b.running = true
	b.mu.Unlock()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel

	// Log that we're ATTEMPTING to start (not success yet)
	b.log("🔄 Attempting to start ngrok tunnel...")
	b.log(fmt.Sprintf("📝 Using auth token: %s...", cfg.NgrokToken[:10]))

	// Start Ngrok tunnel (DON'T log success yet - ngrok might fail!)
	tunnel, err := ngrok.Listen(ctx,
		config.HTTPEndpoint(),
		ngrok.WithAuthtoken(cfg.NgrokToken),
	)
	if err != nil {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		errMsg := fmt.Sprintf("Failed to start ngrok: %v", err)
		b.log("❌ " + errMsg)
		return fmt.Errorf(errMsg)
	}

	// SUCCESS - ngrok started! Now we can log
	b.tunnel = tunnel
	b.publicURL = tunnel.URL()
	
	b.log("🚀 Starting Remote Bridge...")
	b.log(fmt.Sprintf("✅ Tunnel Live: %s", b.publicURL))
	b.log("🌐 HTTP Server listening on tunnel...")

	// Emit public URL event (only in UI mode)
	if b.ctx != nil && b.ctx != context.Background() {
		runtime.EventsEmit(b.ctx, "publicURL", b.publicURL)
	}

	// Start HTTP handler as goroutine
	// Tunnel stays alive because it's stored in b.tunnel
	go b.runHTTPServer(ctx, tunnel)

	return nil
}

// Stop shuts down the bridge
func (b *BridgeService) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return
	}

	b.log("🛑 Stopping Bridge...")

	if b.cancel != nil {
		b.cancel()
	}

	if b.tunnel != nil {
		b.tunnel.CloseWithContext(context.Background())
	}

	// Kill any lingering ngrok processes
	exec.Command("powershell", "-Command",
		"Get-Process | Where-Object {$_.ProcessName -eq 'ngrok'} | Stop-Process -Force").Run()

	b.running = false
	b.log("✅ Bridge Stopped")
	if b.ctx != nil && b.ctx != context.Background() {
		runtime.EventsEmit(b.ctx, "bridgeStopped", true)
	}
}

// runMCPServer starts the MCP stdio server
func (b *BridgeService) runMCPServer(ctx context.Context, tunnel ngrok.Tunnel) {
	b.log("🔧 Starting MCP Server...")

	s := server.NewMCPServer("Remote Bridge", "1.0.0")

	// Register the ask_remote_human tool
	askTool := mcp.NewTool("ask_remote_human",
		mcp.WithDescription("Ask the user a question via configured notification channels (Telegram/WhatsApp/SMS)"),
		mcp.WithString("question", mcp.Required(), mcp.Description("The question to ask the user")),
		mcp.WithArray("options", mcp.Required(), mcp.Description("Available response options")),
	)

	s.AddTool(askTool, func(c context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return b.handleAskHuman(request)
	})

	b.log("✅ MCP Server Ready - Waiting for requests...")

	// Run the stdio server (blocks until context cancelled)
	if err := server.ServeStdio(s); err != nil {
		if ctx.Err() == nil {
			b.log(fmt.Sprintf("❌ MCP Server Error: %v", err))
		}
	}
}

// handleAskHuman processes the ask_remote_human tool call
func (b *BridgeService) handleAskHuman(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	b.log("🔔 Received ask_remote_human request")

	var question string
	var options []string

	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		if q, ok := args["question"].(string); ok {
			question = q
		}
		if opts, ok := args["options"].([]interface{}); ok {
			for _, o := range opts {
				if str, ok := o.(string); ok {
					options = append(options, str)
				}
			}
		}
	}

	b.log(fmt.Sprintf("📨 Question: %s", question))
	b.log(fmt.Sprintf("📋 Options: %v", options))

	// Generate request ID
	requestID := uuid.New().String()[:8]

	// Create response channel
	responseChan := make(chan string, 1)
	b.pendingMu.Lock()
	b.pendingRequests[requestID] = responseChan
	b.requestData[requestID] = RequestData{Question: question, Options: options}
	b.pendingMu.Unlock()

	// Send notification
	b.sendNotification(question, options, requestID)

	// Wait for response (with timeout handled by MCP)
	response := <-responseChan

	// Cleanup
	b.pendingMu.Lock()
	delete(b.pendingRequests, requestID)
	delete(b.requestData, requestID)
	b.pendingMu.Unlock()

	b.log(fmt.Sprintf("✅ User Response: %s", response))

	return mcp.NewToolResultText(response), nil
}

// getTunnelFilePath returns the path to tunnel-url.txt, checking both dev and production locations
func getTunnelFilePath() string {
	// Try dev mode path first (relative to project root)
	devPath := "tunnel-url.txt"
	if _, err := os.Stat(devPath); err == nil {
		return devPath
	}
	
	// Try production path (relative to executable)
	exePath, err := os.Executable()
	if err == nil {
		prodPath := filepath.Join(filepath.Dir(exePath), "tunnel-url.txt")
		return prodPath
	}
	
	// Fallback
	return "tunnel-url.txt"
}

// getPublicURL returns the public tunnel URL from memory or file
func (b *BridgeService) getPublicURL() string {
	// A. If we are the Main UI, we have it in memory
	b.mu.Lock()
	if b.publicURL != "" {
		b.mu.Unlock()
		return b.publicURL
	}
	b.mu.Unlock()

	// B. If we are the MCP Process (VS Code), read it from the file
	tunnelPath := getTunnelFilePath()
	data, err := ioutil.ReadFile(tunnelPath)
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	return "" // Failed to find URL
}

// sendNotification sends the question to configured channels
func (b *BridgeService) sendNotification(question string, options []string, requestID string) {
	switch b.cfg.Channel {
	case "telegram":
		b.sendTelegram(question, options, requestID)
	case "whatsapp":
		b.sendWhatsApp(question, options, requestID)
	default:
		b.log(fmt.Sprintf("⚠️ Channel '%s' not implemented yet", b.cfg.Channel))
	}
}

// sendTelegram sends notification via Telegram
func (b *BridgeService) sendTelegram(question string, options []string, requestID string) {
	if b.cfg.Telegram.BotToken == "" || b.cfg.Telegram.ChatID == "" {
		b.log("⚠️ Telegram not configured")
		return
	}

	// [FIX] Get URL from memory OR file
	publicURL := b.getPublicURL()
	if publicURL == "" {
		b.log("❌ Error: Could not find Bridge Public URL. Is the UI running?")
		return
	}

	b.log("📤 Sending Telegram notification...")

	bot, err := tgbotapi.NewBotAPI(b.cfg.Telegram.BotToken)
	if err != nil {
		b.log(fmt.Sprintf("❌ Telegram Error: %v", err))
		return
	}

	var chatID int64
	fmt.Sscanf(b.cfg.Telegram.ChatID, "%d", &chatID)

	responseURL := fmt.Sprintf("%s/respond?id=%s", publicURL, requestID)

	// Use exact template user provided
	msgText := fmt.Sprintf(
		"<b>🤖 Input Needed</b>\n\n"+
			"%s\n\n"+
			"I've hit a decision point and need your guidance to continue.\n\n"+
			"<a href='%s'>📲 Launch Interface</a>",
		question,
		responseURL,
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "HTML"

	// Add inline keyboard button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Tap to Respond", responseURL),
		),
	)
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		b.log(fmt.Sprintf("❌ Telegram Send Error: %v", err))
	} else {
		b.log("✅ Telegram notification sent!")
	}
}

// sendWhatsApp sends notification via CallMeBot
func (b *BridgeService) sendWhatsApp(question string, options []string, requestID string) {
	if b.cfg.WhatsApp.APIKey == "" || b.cfg.WhatsApp.Phone == "" {
		b.log("⚠️ WhatsApp not configured")
		return
	}

	b.log("📤 Sending WhatsApp notification...")

	// Build message with response links
	message := fmt.Sprintf("🤖 AI Agent Question:\n\n%s\n\n", question)
	for _, opt := range options {
		message += fmt.Sprintf("➡️ %s/respond?id=%s&answer=%s\n", b.publicURL, requestID, opt)
	}

	// CallMeBot API
	url := fmt.Sprintf("https://api.callmebot.com/whatsapp.php?phone=%s&text=%s&apikey=%s",
		b.cfg.WhatsApp.Phone, message, b.cfg.WhatsApp.APIKey)

	resp, err := http.Get(url)
	if err != nil {
		b.log(fmt.Sprintf("❌ WhatsApp Error: %v", err))
		return
	}
	resp.Body.Close()
	b.log("✅ WhatsApp notification sent!")
}

// runHTTPServer handles response callbacks
func (b *BridgeService) runHTTPServer(ctx context.Context, tunnel ngrok.Tunnel) {
	mux := http.NewServeMux()

	// Save tunnel URL to file for MCP adapter
	tunnelPath := getTunnelFilePath()
	ioutil.WriteFile(tunnelPath, []byte(b.publicURL), 0644)

	mux.HandleFunc("/respond", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.URL.Query().Get("id")
		answer := r.URL.Query().Get("answer")

		b.pendingMu.Lock()
		ch, exists := b.pendingRequests[requestID]
		data := b.requestData[requestID]
		b.pendingMu.Unlock()

		if !exists {
			http.Error(w, "Request not found or expired", 404)
			return
		}

		// If no answer provided, show the interactive form
		if answer == "" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			optionsHTML := ""
			for _, opt := range data.Options {
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
			window.location.href = '/respond?id=%s&answer=' + encodeURIComponent(answer);
		}
		function submitCustom() {
			const custom = document.getElementById('customAnswer').value;
			if (custom.trim()) {
				window.location.href = '/respond?id=%s&answer=' + encodeURIComponent(custom);
			} else {
				alert('Please enter an answer');
			}
		}
	</script>
</body>
</html>`, data.Question, optionsHTML, requestID, requestID)
			return
		}

		// Answer provided - process it
		if answer != "" {
			b.log(fmt.Sprintf("📥 Response received: %s -> %s", requestID, answer))
			ch <- answer
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
</html>`, answer)
		}
	})

	// New /ask endpoint for MCP adapter
	mux.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}

		var req struct {
			Question string   `json:"question"`
			Options  []string `json:"options"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}

		b.log(fmt.Sprintf("🔔 HTTP Request: %s", req.Question))

		// Generate request ID
		requestID := uuid.New().String()[:8]

		// Create response channel
		responseChan := make(chan string, 1)
		b.pendingMu.Lock()
		b.pendingRequests[requestID] = responseChan
	b.requestData[requestID] = RequestData{Question: req.Question, Options: req.Options}
		b.pendingMu.Unlock()

		// Send notification
		b.sendNotification(req.Question, req.Options, requestID)

		// Wait for response
		answer := <-responseChan

		// Cleanup
		b.pendingMu.Lock()
		delete(b.pendingRequests, requestID)
		b.pendingMu.Unlock()

		b.log(fmt.Sprintf("✅ Response: %s", answer))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"answer": answer})
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	b.log("🌐 HTTP Server listening on tunnel...")
	http.Serve(tunnel, mux)
}
