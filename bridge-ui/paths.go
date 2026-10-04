package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Version is the single source of truth for the app version (updater, MCP server, hub health).
const Version = "1.1.0"

// defaultHubPort is the fixed localhost port the hub listens on, so every IDE's
// MCP process can find it. Override with MOMENTUM_PORT.
const defaultHubPort = 47821

// BridgeConfig represents the full configuration structure
type BridgeConfig struct {
	Channel        string         `json:"channel"`
	Source         string         `json:"source,omitempty"`
	Telegram       TelegramConfig `json:"telegram"`
	Slack          SlackConfig    `json:"slack"`
	Discord        DiscordConfig  `json:"discord"`
	Ntfy           NtfyConfig     `json:"ntfy"`
	Gmail          GmailConfig    `json:"gmail"`
	WhatsApp       WhatsAppConfig `json:"whatsapp"`
	SMS            SMSConfig      `json:"sms"`
	NgrokToken     string         `json:"ngrokToken"`
	TimeoutMinutes int            `json:"timeoutMinutes,omitempty"`
	// AtDesk turns Away mode off: agents are told to ask in chat instead of on the phone.
	AtDesk bool `json:"atDesk,omitempty"`
}

type TelegramConfig struct {
	BotToken    string `json:"bot_token"`
	ChatID      string `json:"chat_id"`
	ChatName    string `json:"chat_name,omitempty"`    // for display only
	BotUsername string `json:"bot_username,omitempty"` // for display only
}

type SlackConfig struct {
	BotToken  string `json:"bot_token"`            // xoxb-…
	AppToken  string `json:"app_token"`            // xapp-… (Socket Mode)
	UserID    string `json:"user_id"`              // the only person who may answer
	UserName  string `json:"user_name,omitempty"`  // for display only
	ChannelID string `json:"channel_id,omitempty"` // DM with that person
	Team      string `json:"team,omitempty"`       // for display only
}

type DiscordConfig struct {
	BotToken  string `json:"bot_token"`
	UserID    string `json:"user_id"`              // the only person who may answer
	UserName  string `json:"user_name,omitempty"`  // for display only
	ChannelID string `json:"channel_id,omitempty"` // DM with that person
}

type NtfyConfig struct {
	Server string `json:"server,omitempty"` // default https://ntfy.sh
	Topic  string `json:"topic"`            // long and random: knowing it is what lets you read the questions
	Token  string `json:"token,omitempty"`  // access token, for protected or self-hosted servers
}

func (n NtfyConfig) server() string {
	if n.Server == "" {
		return "https://ntfy.sh"
	}
	return strings.TrimRight(n.Server, "/")
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

// dataDir returns the per-user Momentum directory (%APPDATA%\Momentum on Windows).
// Every copy of the exe and every IDE shares it, so config lives in one place.
// MOMENTUM_HOME overrides it (used by tests).
func dataDir() string {
	if d := os.Getenv("MOMENTUM_HOME"); d != "" {
		os.MkdirAll(d, 0700)
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	d := filepath.Join(base, "Momentum")
	os.MkdirAll(d, 0700)
	return d
}

func configPath() string { return filepath.Join(dataDir(), "bridge-config.json") }
func logPath() string    { return filepath.Join(dataDir(), "momentum.log") }

func hubPort() int {
	if p, err := strconv.Atoi(os.Getenv("MOMENTUM_PORT")); err == nil && p > 0 {
		return p
	}
	return defaultHubPort
}

// hubKey returns the shared secret that local MCP clients must present to the hub.
// It stops web pages in the user's browser from driving the localhost API.
func hubKey() string {
	p := filepath.Join(dataDir(), "hub.key")
	for attempt := 0; attempt < 20; attempt++ {
		if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) >= 32 {
			return strings.TrimSpace(string(b))
		}
		// O_EXCL so two processes starting at once can't write different keys.
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			key := randomHex(32)
			f.WriteString(key)
			f.Close()
			return key
		}
		time.Sleep(50 * time.Millisecond) // another process is writing it
	}
	os.Remove(p) // unreadable/corrupt; the next call recreates it
	return hubKey()
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// legacyConfigPath is where versions <= 1.0 stored config (next to the exe).
func legacyConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "bridge-config.json")
}

// loadConfig reads the config, migrating the legacy exe-adjacent file on first run.
func loadConfig() (BridgeConfig, error) {
	var cfg BridgeConfig
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		if legacy := legacyConfigPath(); legacy != "" {
			if old, lerr := os.ReadFile(legacy); lerr == nil {
				os.WriteFile(configPath(), old, 0600)
				data, err = old, nil
			}
		}
	}
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

func saveConfig(cfg BridgeConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

// configProblem explains what is missing for the hub to reach the user's phone, or "" if usable.
func configProblem(cfg BridgeConfig) string {
	switch cfg.Channel {
	case "telegram":
		if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
			return "Telegram bot token / chat ID not set"
		}
	case "slack":
		if cfg.Slack.BotToken == "" || cfg.Slack.AppToken == "" || cfg.Slack.UserID == "" {
			return "Slack tokens / user not set"
		}
	case "discord":
		if cfg.Discord.BotToken == "" || cfg.Discord.UserID == "" || cfg.Discord.ChannelID == "" {
			return "Discord bot token / user not set"
		}
	case "ntfy":
		if len(cfg.Ntfy.Topic) < 12 {
			return "ntfy topic not set (it must be long and random)"
		}
	case "whatsapp":
		if cfg.WhatsApp.APIKey == "" || cfg.WhatsApp.Phone == "" {
			return "WhatsApp API key / phone not set"
		}
		// WhatsApp can only send a link; without a tunnel it would point at
		// 127.0.0.1, which a phone can't open.
		if cfg.NgrokToken == "" && os.Getenv("MOMENTUM_ALLOW_LOCAL_LINKS") != "1" {
			return "ngrok token not set (WhatsApp needs it for the answer link)"
		}
	case "":
		return "no notification channel configured"
	default:
		return fmt.Sprintf("channel %q is not supported", cfg.Channel)
	}
	return ""
}

var logMu sync.Mutex

func nowStamp() string { return time.Now().Format("2006-01-02 15:04:05") }

// appendLog writes one line to momentum.log, trimming the file when it grows past 1 MB.
func appendLog(line string) {
	logMu.Lock()
	defer logMu.Unlock()
	p := logPath()
	if fi, err := os.Stat(p); err == nil && fi.Size() > 1<<20 {
		if data, err := os.ReadFile(p); err == nil {
			os.WriteFile(p, data[len(data)/2:], 0600)
		}
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
}
