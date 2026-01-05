package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx         context.Context
	mu          sync.Mutex
	wantsToQuit bool
	bridge      *BridgeService
}

// BridgeConfig represents the full configuration structure
type BridgeConfig struct {
	Channel    string         `json:"channel"`
	Source     string         `json:"source"`
	Telegram   TelegramConfig `json:"telegram"`
	Gmail      GmailConfig    `json:"gmail"`
	WhatsApp   WhatsAppConfig `json:"whatsapp"`
	SMS        SMSConfig      `json:"sms"`
	NgrokToken string         `json:"ngrokToken"`
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type GmailConfig struct {
	Email       string `json:"email"`
	AppPassword string `json:"app_password"`
}

type WhatsAppConfig struct {
	APIKey string `json:"api_key"`
	Phone  string `json:"phone"`
}

type SMSConfig struct {
	TwilioSID   string `json:"twilio_sid"`
	TwilioToken string `json:"twilio_token"`
	From        string `json:"from"`
	To          string `json:"to"`
}

// RecentChannel represents a recently configured channel
type RecentChannel struct {
	Name      string `json:"name"`       // "Telegram", "Discord", etc.
	Icon      string `json:"icon"`       // Emoji or icon identifier
	ConfigKey string `json:"config_key"` // For quick lookup ("telegram", "whatsapp", etc.)
	LastUsed  string `json:"last_used"`  // ISO timestamp
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		bridge: NewBridgeService(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.bridge.SetContext(ctx)
	
	// Auto-kill any existing bridge/ngrok processes on startup
	a.KillExistingBridges()
	
	// Check for updates in background (after 3 seconds)
	a.AutoCheckForUpdates()
}

// getConfigPath returns the path to the config file
func (a *App) getConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "bridge-config.json"
	}
	return filepath.Join(filepath.Dir(exePath), "bridge-config.json")
}

// beforeClose is called when the user clicks the window's X button.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if a.wantsToQuit {
		return false
	}
	runtime.WindowHide(ctx)
	return true
}

// ShowWindow brings the window back from the tray
func (a *App) ShowWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
}

// HideWindow hides the window to system tray
func (a *App) HideWindow() {
	runtime.WindowHide(a.ctx)
}

// QuitApp sets the flag and performs a real quit
func (a *App) QuitApp() {
	// Stop bridge first
	if a.bridge != nil {
		a.bridge.Stop()
	}
	a.wantsToQuit = true
	runtime.Quit(a.ctx)
}

// StartBridge loads config and starts the bridge service
func (a *App) StartBridge() string {
	// Kill any existing bridge/ngrok processes first
	a.KillExistingBridges()
	
	configPath := a.getConfigPath()

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Sprintf("Error loading config: %v", err)
	}

	var cfg BridgeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Sprintf("Error parsing config: %v", err)
	}

	if cfg.NgrokToken == "" {
		return "Error: Ngrok token not configured"
	}

	if err := a.bridge.Start(cfg); err != nil {
		return fmt.Sprintf("Error starting bridge: %v", err)
	}

	return "Bridge started successfully"
}

// KillExistingBridges stops any running bridge or ngrok processes
func (a *App) KillExistingBridges() {
	// This will be implemented differently on Windows vs Unix
	// For Windows, we use PowerShell to kill processes
	// Note: This only kills OTHER processes, not the current app
	exec.Command("powershell", "-Command", "Get-Process | Where-Object {$_.ProcessName -match 'bridge|ngrok' -and $_.Id -ne $PID} | Stop-Process -Force").Run()
}

// StopBridge stops the bridge service
func (a *App) StopBridge() string {
	a.bridge.Stop()
	return "Bridge stopped"
}

// IsBridgeRunning returns the bridge state
func (a *App) IsBridgeRunning() bool {
	return a.bridge.IsRunning()
}

// SaveConfig saves the configuration to disk
func (a *App) SaveConfig(jsonConfig string) string {
	configPath := a.getConfigPath()

	var cfg BridgeConfig
	if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
		return fmt.Sprintf("Error: Invalid JSON - %v", err)
	}

	prettyJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if err := ioutil.WriteFile(configPath, prettyJSON, 0644); err != nil {
		return fmt.Sprintf("Error saving config: %v", err)
	}

	return "Configuration saved successfully!"
}

// LoadConfig loads the configuration from disk
func (a *App) LoadConfig() string {
	configPath := a.getConfigPath()

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		emptyConfig := BridgeConfig{}
		jsonBytes, _ := json.Marshal(emptyConfig)
		return string(jsonBytes)
	}

	return string(data)
}

// ReadLogs returns the content of bridge.log
func (a *App) ReadLogs() []string {
	exePath, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(exePath), "bridge.log")

	data, err := ioutil.ReadFile(logPath)
	if err != nil {
		return []string{"Waiting for logs..."}
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 50 {
		return lines[len(lines)-50:]
	}
	return lines
}

// getRecentsPath returns the path to the recents file
func (a *App) getRecentsPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "recents.json"
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "recents.json")
}

// GetRecentChannels returns recently configured channels
func (a *App) GetRecentChannels() []RecentChannel {
	data, err := ioutil.ReadFile(a.getRecentsPath())
	if err != nil {
		return []RecentChannel{}
	}

	var recents []RecentChannel
	if err := json.Unmarshal(data, &recents); err != nil {
		return []RecentChannel{}
	}

	// Return max 5 most recent
	if len(recents) > 5 {
		return recents[:5]
	}
	return recents
}

// AddRecentChannel adds a channel to recents (or updates if exists)
func (a *App) AddRecentChannel(name, icon, configKey string) error {
	recents := a.GetRecentChannels()
	
	// Check if already exists, remove if so
	var filtered []RecentChannel
	for _, r := range recents {
		if r.ConfigKey != configKey {
			filtered = append(filtered, r)
		}
	}
	
	// Add new recent at the beginning
	newRecent := RecentChannel{
		Name:      name,
		Icon:      icon,
		ConfigKey: configKey,
		LastUsed:  time.Now().Format(time.RFC3339),
	}
	
	recents = append([]RecentChannel{newRecent}, filtered...)
	
	// Keep max 5
	if len(recents) > 5 {
		recents = recents[:5]
	}
	
	// Save to file
	data, err := json.MarshalIndent(recents, "", "  ")
	if err != nil {
		return err
	}
	
	return ioutil.WriteFile(a.getRecentsPath(), data, 0644)
}
