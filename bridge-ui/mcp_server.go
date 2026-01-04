package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Global state for MCP mode
var (
	mcpBridge *BridgeService
	mcpMutex  sync.Mutex
)

// runMCPServer starts the MCP stdio server (no UI)
func runMCPServer() {
	fmt.Fprintln(os.Stderr, "[BRIDGE] 🚀 Remote Bridge Starting (MCP Mode)...")

	// Load configuration
	configPath := getConfigPath()
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[BRIDGE] ❌ Config load error: %v\n", err)
		os.Exit(1)
	}

	// Create bridge service
	mcpBridge = NewBridgeService()
	// Set empty context (no UI in MCP mode)
	mcpBridge.SetContext(context.Background())

	// Start bridge (Ngrok + HTTP + Telegram)
	if err := mcpBridge.Start(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[BRIDGE] ❌ Bridge start error: %v\n", err)
		os.Exit(1)
	}

	// Wait a bit for Ngrok to initialize
	time.Sleep(2 * time.Second)

	// Start MCP server
	startMCPStdioServer()
}

// startMCPStdioServer creates and runs the MCP stdio server
func startMCPStdioServer() {
	s := server.NewMCPServer("Remote Bridge", "2.1.0", server.WithToolCapabilities(true))

	askTool := mcp.NewTool("ask_remote_human",
		mcp.WithDescription("Ask the user a question via Telegram (interactive HTML form)"),
		mcp.WithString("question", mcp.Required()),
		mcp.WithArray("options", mcp.Required()),
	)

	s.AddTool(askTool, handleAskHuman)

	fmt.Fprintln(os.Stderr, "[BRIDGE] 📡 MCP Server listening on Stdio...")
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "[BRIDGE] ❌ MCP Server Error: %v\n", err)
		os.Exit(1)
	}
}

// handleAskHuman implements the ask_remote_human tool
func handleAskHuman(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, _ := request.RequireString("question")
	args := request.GetArguments()
	optionsSlice, _ := args["options"].([]interface{})

	var options []string
	for _, o := range optionsSlice {
		if s, ok := o.(string); ok {
			options = append(options, s)
		}
	}

	fmt.Fprintf(os.Stderr, "[BRIDGE] 🔔 Question: %s\n", question)
	fmt.Fprintf(os.Stderr, "[BRIDGE] 📋 Options: %v\n", options)

	// Generate request ID
	requestID := uuid.New().String()[:8]

	// Use the bridge service to handle the request
	mcpMutex.Lock()
	if mcpBridge == nil {
		mcpMutex.Unlock()
		return mcp.NewToolResultError("Bridge not initialized"), nil
	}
	bridge := mcpBridge
	mcpMutex.Unlock()

	// Create response channel
	responseChan := make(chan string, 1)
	bridge.pendingMu.Lock()
	bridge.pendingRequests[requestID] = responseChan
	bridge.requestData[requestID] = RequestData{Question: question, Options: options}
	bridge.pendingMu.Unlock()

	// Send notifications
	bridge.sendNotification(question, options, requestID)

	// Wait for response (with timeout)
	select {
	case resp := <-responseChan:
		// Cleanup
		bridge.pendingMu.Lock()
		delete(bridge.pendingRequests, requestID)
		delete(bridge.requestData, requestID)
		bridge.pendingMu.Unlock()

		fmt.Fprintf(os.Stderr, "[BRIDGE] ✅ Response: %s\n", resp)
		return mcp.NewToolResultText(resp), nil

	case <-time.After(15 * time.Minute):
		// Cleanup
		bridge.pendingMu.Lock()
		delete(bridge.pendingRequests, requestID)
		delete(bridge.requestData, requestID)
		bridge.pendingMu.Unlock()

		return mcp.NewToolResultError("Request timed out after 15 minutes"), nil

	case <-ctx.Done():
		// Cleanup
		bridge.pendingMu.Lock()
		delete(bridge.pendingRequests, requestID)
		delete(bridge.requestData, requestID)
		bridge.pendingMu.Unlock()

		return mcp.NewToolResultError("Request cancelled"), nil
	}
}

// getConfigPath returns the path to bridge-config.json
func getConfigPath() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "bridge-config.json")
}

// loadConfig loads the bridge configuration from JSON
func loadConfig(path string) (BridgeConfig, error) {
	var cfg BridgeConfig
	
	// Try to read config file
	data, err := os.ReadFile(path)
	if err != nil {
		// If no config file, return empty config (will use .env if available)
		return cfg, nil
	}

	// Parse JSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}
