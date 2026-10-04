package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the frontend; its exported methods are callable from JS.
type App struct {
	ctx         context.Context
	wantsToQuit bool
	hub         *Hub
}

func NewApp() *App {
	return &App{hub: NewHub()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	uiLog := newLogger("app")
	a.hub.Logf = func(msg string) {
		uiLog(msg)
		runtime.EventsEmit(a.ctx, "log", msg)
	}
	a.hub.OnPublicURL = func(u string) { runtime.EventsEmit(a.ctx, "publicURL", u) }
	a.hub.OnActivity = func(act Activity) { runtime.EventsEmit(a.ctx, "activity", act) }

	// Once set up, Momentum just runs: no "start" button to forget.
	if cfg, err := loadConfig(); err == nil && configProblem(cfg) == "" {
		go func() {
			if msg := a.StartBridge(); strings.HasPrefix(msg, "Error") {
				a.hub.Logf("❌ " + msg)
			}
			runtime.EventsEmit(a.ctx, "state")
		}()
	}

	a.AutoCheckForUpdates()
}

// beforeClose is called when the user clicks the window's X button.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if a.wantsToQuit {
		a.hub.Stop()
		return false
	}
	// Closing the window keeps Momentum running in the tray.
	runtime.WindowHide(ctx)
	return true
}

func (a *App) ShowWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
}

func (a *App) HideWindow() { runtime.WindowHide(a.ctx) }

func (a *App) QuitApp() {
	a.hub.Stop()
	a.wantsToQuit = true
	runtime.Quit(a.ctx)
}

// ---------- state ----------

type AppState struct {
	Version     string `json:"version"`
	Configured  bool   `json:"configured"`
	Problem     string `json:"problem"`
	Channel     string `json:"channel"`
	ChatName    string `json:"chatName"`
	BotUsername string `json:"botUsername"`
	SlackUser   string `json:"slackUser"`
	SlackTeam   string `json:"slackTeam"`
	DiscordUser string `json:"discordUser"`
	Running     bool   `json:"running"`
	AtDesk      bool   `json:"atDesk"`
	IDEsLinked  int    `json:"idesLinked"`
	IDEsFound   int    `json:"idesFound"`
	DataDir     string `json:"dataDir"`
}

// GetState summarises everything the dashboard shows.
func (a *App) GetState() AppState {
	cfg, _ := loadConfig()
	st := AppState{
		Version:     Version,
		Problem:     configProblem(cfg),
		Channel:     cfg.Channel,
		ChatName:    cfg.Telegram.ChatName,
		BotUsername: cfg.Telegram.BotUsername,
		SlackUser:   cfg.Slack.UserName,
		SlackTeam:   cfg.Slack.Team,
		DiscordUser: cfg.Discord.UserName,
		Running:     a.IsBridgeRunning(),
		AtDesk:      cfg.AtDesk,
		DataDir:     dataDir(),
	}
	st.Configured = st.Problem == ""
	for _, s := range ListIDEs() {
		if s.Manual {
			continue
		}
		if s.Connected && !s.Stale {
			st.IDEsLinked++
		}
		if s.Installed || s.Connected {
			st.IDEsFound++
		}
	}
	return st
}

// ---------- hub control ----------

// StartBridge starts the hub inside the app. If a background hub (started
// by an IDE) already holds the hub port, it is asked to hand over.
func (a *App) StartBridge() string {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Sprintf("Error loading config: %v", err)
	}
	if p := configProblem(cfg); p != "" {
		return "Error: " + p
	}
	if a.hub.IsRunning() {
		return "Bridge started successfully"
	}

	err = a.hub.Start(cfg)
	if errors.Is(err, ErrHubBusy) {
		c := newHubClient()
		h, herr := c.health(context.Background())
		if herr != nil || !h.Daemon {
			return "Error: Momentum is already running in another window (or another program uses port " + fmt.Sprint(hubPort()) + ")"
		}
		a.hub.Logf("🔁 Taking over from the background hub...")
		c.post(context.Background(), "/api/shutdown")
		c.waitGone(5 * time.Second)
		err = a.hub.Start(cfg)
	}
	if err != nil {
		return fmt.Sprintf("Error starting bridge: %v", err)
	}
	return "Bridge started successfully"
}

// StopBridge stops the hub, whether it runs in this window or in the background.
func (a *App) StopBridge() string {
	if a.hub.IsRunning() {
		a.hub.Stop()
	} else {
		newHubClient().post(context.Background(), "/api/shutdown")
	}
	return "Bridge stopped"
}

// IsBridgeRunning reports whether any hub (this window's or a background one) is up.
func (a *App) IsBridgeRunning() bool {
	if a.hub.IsRunning() {
		return true
	}
	h, err := newHubClient().health(context.Background())
	return err == nil && h.App == "momentum"
}

// GetPublicURL returns the tunnel URL of the running hub, if any (WhatsApp only).
func (a *App) GetPublicURL() string {
	if a.hub.IsRunning() {
		return a.hub.PublicURL()
	}
	h, _ := newHubClient().health(context.Background())
	return h.PublicURL
}

// applyConfig saves cfg and makes the running hub (or a new one) use it.
func (a *App) applyConfig(cfg BridgeConfig) string {
	if err := saveConfig(cfg); err != nil {
		return "Error saving config: " + err.Error()
	}
	switch {
	case a.hub.IsRunning():
		a.hub.Reload(cfg)
	case configProblem(cfg) == "":
		if msg := a.StartBridge(); strings.HasPrefix(msg, "Error") {
			return msg
		}
	}
	runtime.EventsEmit(a.ctx, "state")
	return ""
}

// SaveTelegram stores the Telegram channel and makes it active. Returns "" on success.
func (a *App) SaveTelegram(token, chatID, chatName, botUsername string) string {
	cfg, _ := loadConfig()
	cfg.Channel = "telegram"
	cfg.Telegram = TelegramConfig{
		BotToken:    strings.TrimSpace(token),
		ChatID:      strings.TrimSpace(chatID),
		ChatName:    chatName,
		BotUsername: botUsername,
	}
	if p := configProblem(cfg); p != "" {
		return p
	}
	return a.applyConfig(cfg)
}

// SaveSlack stores the Slack channel and makes it active. Returns "" on success.
func (a *App) SaveSlack(botToken, appToken string, link SlackLink) string {
	cfg, _ := loadConfig()
	cfg.Channel = "slack"
	cfg.Slack = SlackConfig{
		BotToken: strings.TrimSpace(botToken), AppToken: strings.TrimSpace(appToken),
		UserID: link.UserID, UserName: link.UserName, ChannelID: link.ChannelID, Team: link.Team,
	}
	if p := configProblem(cfg); p != "" {
		return p
	}
	return a.applyConfig(cfg)
}

// GetSlackSettings returns the saved Slack fields for editing.
func (a *App) GetSlackSettings() SlackConfig {
	cfg, _ := loadConfig()
	return cfg.Slack
}

// DetectSlackUser waits (up to 60s) for the user to DM the Momentum Slack app.
func (a *App) DetectSlackUser(botToken, appToken string) SlackLink {
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	// Slack hands each event to just one open socket, so make sure ours is the only one.
	a.releaseChannel(ctx, "slack")
	return DetectSlackUser(ctx, botToken, appToken, 60*time.Second)
}

// OpenSlackAppSetup opens Slack's "create app" page with Momentum's manifest pre-filled.
func (a *App) OpenSlackAppSetup() { runtime.BrowserOpenURL(a.ctx, SlackCreateAppURL()) }

// GetSlackManifest returns the app manifest to paste manually.
func (a *App) GetSlackManifest() string { return slackManifest }

// SaveDiscord stores the Discord channel and makes it active. Returns "" on success.
func (a *App) SaveDiscord(token string, link DiscordLink) string {
	cfg, _ := loadConfig()
	cfg.Channel = "discord"
	cfg.Discord = DiscordConfig{BotToken: strings.TrimSpace(token), UserID: link.UserID, UserName: link.UserName, ChannelID: link.ChannelID}
	if p := configProblem(cfg); p != "" {
		return p
	}
	return a.applyConfig(cfg)
}

func (a *App) GetDiscordSettings() DiscordConfig {
	cfg, _ := loadConfig()
	return cfg.Discord
}

// OpenDiscordInvite opens the page that adds the bot to one of the user's servers.
func (a *App) OpenDiscordInvite(token string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	u, err := DiscordInviteURL(ctx, token)
	if err != nil {
		return strings.TrimPrefix(err.Error(), "discord: ")
	}
	runtime.BrowserOpenURL(a.ctx, u)
	return ""
}

// DetectDiscordUser waits (up to 60s) for the user to DM the bot.
func (a *App) DetectDiscordUser(token string) DiscordLink {
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	a.releaseChannel(ctx, "discord") // one gateway session at a time keeps things simple
	return DetectDiscordUser(ctx, token, 60*time.Second)
}

// SaveNtfy stores the ntfy channel, sends a "linked" notification and makes it active.
func (a *App) SaveNtfy(server, topic, token string) string {
	cfg, _ := loadConfig()
	cfg.Channel = "ntfy"
	cfg.Ntfy = NtfyConfig{Server: strings.TrimSpace(server), Topic: strings.TrimSpace(topic), Token: strings.TrimSpace(token)}
	if p := configProblem(cfg); p != "" {
		return p
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := SendNtfyTest(ctx, cfg.Ntfy); err != nil {
		return "Couldn't reach the ntfy server: " + strings.TrimPrefix(err.Error(), "ntfy: ")
	}
	return a.applyConfig(cfg)
}

// NewNtfyTopic makes a fresh random topic name.
func (a *App) NewNtfyTopic() string { return NewNtfyTopic() }

// GetNtfySettings returns the saved ntfy settings, with a fresh topic if none is set yet.
func (a *App) GetNtfySettings() NtfyConfig {
	cfg, _ := loadConfig()
	n := cfg.Ntfy
	if n.Topic == "" {
		n.Topic = NewNtfyTopic()
	}
	if n.Server == "" {
		n.Server = "https://ntfy.sh"
	}
	return n
}

// SaveWhatsApp stores the WhatsApp (CallMeBot) channel and makes it active.
func (a *App) SaveWhatsApp(apiKey, phone, ngrokToken string) string {
	cfg, _ := loadConfig()
	cfg.Channel = "whatsapp"
	cfg.WhatsApp = WhatsAppConfig{APIKey: strings.TrimSpace(apiKey), Phone: strings.TrimSpace(phone)}
	cfg.NgrokToken = strings.TrimSpace(ngrokToken)
	if p := configProblem(cfg); p != "" {
		return p
	}
	return a.applyConfig(cfg)
}

// SetAtDesk switches Away mode off (true) or on (false).
func (a *App) SetAtDesk(atDesk bool) string {
	cfg, _ := loadConfig()
	cfg.AtDesk = atDesk
	return a.applyConfig(cfg)
}

// GetTelegramSettings returns the saved Telegram fields for editing.
func (a *App) GetTelegramSettings() TelegramConfig {
	cfg, _ := loadConfig()
	return cfg.Telegram
}

// GetWhatsAppSettings returns the saved WhatsApp fields and ngrok token for editing.
func (a *App) GetWhatsAppSettings() map[string]string {
	cfg, _ := loadConfig()
	return map[string]string{"apiKey": cfg.WhatsApp.APIKey, "phone": cfg.WhatsApp.Phone, "ngrokToken": cfg.NgrokToken}
}

// DetectTelegramChat finds the chat that just messaged the bot (setup helper).
func (a *App) DetectTelegramChat(token string) TelegramChat {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Only one process may poll a bot at a time, so stop ours (it is restarted
	// when the settings are saved). Messages it already received are remembered.
	a.releaseChannel(ctx, "telegram")
	return DetectTelegramChat(ctx, token)
}

// releaseChannel stops whatever is listening on this kind of channel (our hub,
// or a background one) so setup detection can listen instead.
func (a *App) releaseChannel(ctx context.Context, kind string) {
	if a.hub.IsRunning() {
		if a.hub.ChannelKind() == kind {
			a.hub.Stop()
		}
		return
	}
	if h, err := newHubClient().health(ctx); err == nil && h.Daemon {
		c := newHubClient()
		c.post(ctx, "/api/shutdown")
		c.waitGone(5 * time.Second)
	}
}

// SendTestQuestion sends a real question to the phone and waits for the answer.
func (a *App) SendTestQuestion() string {
	if msg := a.StartBridge(); strings.HasPrefix(msg, "Error") {
		return msg
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	r, err := newHubClient().ask(ctx, askRequest{
		Question: "This is a test from the Momentum app. Tap a button to confirm everything works.",
		Options:  []string{"It works! 🎉", "Something's off"},
		Client:   "Momentum",
		Project:  "test",
	})
	switch {
	case err != nil:
		return "Error: " + err.Error()
	case r.Status == statusAtDesk:
		return "Error: Away mode is off, so questions stay on this PC. Turn Away mode on to test."
	case r.Status == stateAnswered:
		return r.Answer
	case r.Error != "":
		return "Error: " + r.Error
	default:
		return "Error: no answer (" + r.Status + ")"
	}
}

// ---------- activity ----------

func (a *App) GetActivity() []Activity { return ReadActivity() }

func (a *App) ClearActivity() {
	ClearActivity()
	runtime.EventsEmit(a.ctx, "activity", nil)
}

// ---------- misc ----------

func (a *App) OpenURL(url string) { runtime.BrowserOpenURL(a.ctx, url) }

func (a *App) OpenDataFolder() { exec.Command("explorer", dataDir()).Start() }

// LoadConfig returns the raw config (kept for compatibility with older screens).
func (a *App) LoadConfig() string {
	cfg, _ := loadConfig()
	b, _ := json.Marshal(cfg)
	return string(b)
}

// ReadLogs returns the last lines of momentum.log
func (a *App) ReadLogs() []string {
	data, err := os.ReadFile(logPath())
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > 200 {
		return lines[len(lines)-200:]
	}
	return lines
}

// ---------- IDE integration ----------

// ListIDEs returns every supported IDE with detection / connection status.
func (a *App) ListIDEs() []IDEStatus { return ListIDEs() }

// ConnectIDE writes Momentum into the IDE's MCP config. Returns "" on success.
func (a *App) ConnectIDE(id string) string {
	if err := ConnectIDE(id); err != nil {
		return err.Error()
	}
	return ""
}

// ConnectDetectedIDEs connects every IDE found on this PC; returns failures by name.
func (a *App) ConnectDetectedIDEs() map[string]string {
	failed := map[string]string{}
	for _, s := range ListIDEs() {
		if s.Manual || !(s.Installed || s.Connected) || (s.Connected && !s.Stale) {
			continue
		}
		if err := ConnectIDE(s.ID); err != nil {
			failed[s.Name] = err.Error()
		}
	}
	return failed
}

// DisconnectIDE removes Momentum from the IDE's MCP config. Returns "" on success.
func (a *App) DisconnectIDE(id string) string {
	if err := DisconnectIDE(id); err != nil {
		return err.Error()
	}
	return ""
}

// GetIDESnippet returns the config to paste manually.
func (a *App) GetIDESnippet(id string) string { return ManualSnippet(id) }

// AddRulesToProject asks for a project folder and adds the Momentum block to its AGENTS.md.
func (a *App) AddRulesToProject() string {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a project folder"})
	if err != nil || dir == "" {
		return ""
	}
	files, err := WriteProjectRules(dir)
	if err != nil {
		return "Error: " + err.Error()
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = filepath.Base(f)
	}
	return "Updated " + strings.Join(names, ", ") + " in " + filepath.Base(dir)
}
