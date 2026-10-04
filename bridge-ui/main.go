package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var icon []byte

var app *App

func main() {
	mcpMode := flag.Bool("mcp", false, "Run as an MCP stdio server (launched by your IDE)")
	client := flag.String("client", "", "IDE id, set by the config Momentum writes (e.g. cursor)")
	wait := flag.Int("wait", -1, "Seconds one tool call may block before returning a request_id (-1 = auto)")
	daemon := flag.Bool("daemon", false, "Run the hub in the background without a window")
	ideList := flag.Bool("ide-list", false, "List supported IDEs and whether Momentum is connected")
	ideConnect := flag.String("ide-connect", "", "Connect Momentum to an IDE: <id>, 'detected' or 'all'")
	ideDisconnect := flag.String("ide-disconnect", "", "Remove Momentum from an IDE: <id> or 'all'")
	ideSnippet := flag.String("ide-snippet", "", "Print the config snippet for an IDE")
	writeRules := flag.String("write-rules", "", "Add Momentum instructions to AGENTS.md (and CLAUDE.md/GEMINI.md if present) in this folder")
	status := flag.Bool("status", false, "Show whether the hub is running")
	startMini := flag.Bool("mini", false, "Start as the mini pager in the corner of the screen")
	flag.Parse()

	switch {
	case *mcpMode:
		runMCPServer(mcpOptions{Client: *client, WaitSeconds: *wait})
	case *daemon:
		runDaemon()
	case *ideList, *ideConnect != "", *ideDisconnect != "", *ideSnippet != "", *writeRules != "", *status:
		attachParentConsole()
		os.Exit(runCLI(*ideList, *ideConnect, *ideDisconnect, *ideSnippet, *writeRules, *status))
	default:
		runWailsUI(*startMini)
	}
}

func runCLI(list bool, connect, disconnect, snippet, rules string, status bool) int {
	code := 0
	if status {
		c := newHubClient()
		h, err := c.health(context.Background())
		if err != nil {
			fmt.Println("Hub: not running (it starts automatically on the first question)")
		} else {
			b, _ := json.MarshalIndent(h, "", "  ")
			fmt.Println(string(b))
		}
	}
	targets := func(arg string) []ideDef {
		var out []ideDef
		for _, d := range ideDefs {
			if d.Format == "manual" {
				continue
			}
			if arg == "all" || d.ID == arg || (arg == "detected" && d.Detect()) {
				out = append(out, d)
			}
		}
		if len(out) == 0 && arg != "detected" {
			fmt.Printf("Unknown IDE %q. Run --ide-list to see ids.\n", arg)
			code = 2
		}
		return out
	}
	if connect != "" {
		for _, d := range targets(connect) {
			if err := ConnectIDE(d.ID); err != nil {
				fmt.Printf("✗ %-20s %v\n", d.Name, err)
				if err == errHasComments {
					fmt.Println(ManualSnippet(d.ID))
				}
				code = 1
			} else {
				fmt.Printf("✓ %-20s %s\n", d.Name, d.Path())
			}
		}
	}
	if disconnect != "" {
		for _, d := range targets(disconnect) {
			if err := DisconnectIDE(d.ID); err != nil {
				fmt.Printf("✗ %-20s %v\n", d.Name, err)
				code = 1
			} else {
				fmt.Printf("✓ %-20s removed\n", d.Name)
			}
		}
	}
	if snippet != "" {
		fmt.Println(ManualSnippet(snippet))
	}
	if rules != "" {
		files, err := WriteProjectRules(rules)
		for _, f := range files {
			fmt.Println("✓ " + f)
		}
		if err != nil {
			fmt.Println("✗ " + err.Error())
			code = 1
		}
	}
	if list {
		for _, s := range ListIDEs() {
			state := "not connected"
			switch {
			case s.Manual:
				state = "manual setup (--ide-snippet " + s.ID + ")"
			case s.Stale:
				state = "connected to a different Momentum.exe — reconnect"
			case s.Connected:
				state = "connected"
			case !s.Installed:
				state = "not detected"
			}
			if s.Error != "" {
				state += " (" + s.Error + ")"
			}
			fmt.Printf("%-16s %-20s %s\n", s.ID, s.Name, state)
		}
	}
	return code
}

// runDaemon hosts the hub without a window. MCP clients start it on demand;
// it shows a tray icon so the user can see it and quit it.
func runDaemon() {
	logf := newLogger("daemon")
	cfg, err := loadConfig()
	if err != nil {
		logf("❌ " + err.Error())
	}
	hub := NewHub()
	hub.AllowShutdown = true
	hub.Logf = logf
	if err := hub.Start(cfg); err != nil {
		logf("ℹ️ " + err.Error() + "; exiting")
		return
	}
	done := hub.Done
	if os.Getenv("MOMENTUM_NO_TRAY") != "" {
		<-done
		return
	}
	go func() { <-done; systray.Quit() }()
	systray.Run(func() {
		systray.SetIcon(icon)
		systray.SetTooltip("Momentum (background) - forwarding agent questions to your phone")
		mOpen := systray.AddMenuItem("Open Momentum", "Open the Momentum window")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit background hub", "Stop forwarding questions until an IDE needs it again")
		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					exe, _ := os.Executable()
					exec.Command(exe).Start()
				case <-mQuit.ClickedCh:
					hub.Stop()
					return
				}
			}
		}()
	}, nil)
	hub.Stop()
}

func runWailsUI(startMini bool) {
	// Create an instance of the app structure
	app = NewApp()
	app.startMini = startMini

	// Run systray in a goroutine (it has its own event loop)
	go systray.Run(onSystrayReady, onSystrayExit)

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Momentum",
		Width:     1020,
		Height:    680,
		MinWidth:  860,
		MinHeight: 580,
		// The pager draws its own title bar; Windows still gives it a shadow,
		// rounded corners and resize borders.
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 231, G: 228, B: 220, A: 1},
		OnStartup:        app.startup,
		// Opening Momentum again brings the running window forward instead of
		// starting a second copy that can't own the hub.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: instanceID(),
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if app != nil && app.ctx != nil {
					runtime.WindowUnminimise(app.ctx)
					app.ShowWindow()
				}
			},
		},
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			Theme:                             windows.Light,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func onSystrayReady() {
	systray.SetIcon(icon)
	systray.SetTitle("Momentum")
	systray.SetTooltip("Momentum - Keep your AI Agent moving")

	// Menu Items
	mShow := systray.AddMenuItem("Show Momentum", "Show the main window")
	mCheckUpdate := systray.AddMenuItem("Check for Updates", "Check if a new version is available")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				if app != nil {
					app.ShowWindow()
				}
			case <-mCheckUpdate.ClickedCh:
				if app != nil {
					go func() {
						version, err := app.CheckForUpdates()
						if err != nil {
							runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
								Type:    runtime.ErrorDialog,
								Title:   "Update Check Failed",
								Message: fmt.Sprintf("Could not check for updates: %v", err),
							})
						} else if version != "" {
							result, _ := runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
								Type:    runtime.InfoDialog,
								Title:   "Update Available",
								Message: fmt.Sprintf("Momentum v%s is available!\n\nWould you like to download and install it now?", version),
								Buttons: []string{"Update Now", "Later"},
							})
							if result == "Update Now" {
								if err := app.DownloadUpdate(version); err != nil {
									runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
										Type:    runtime.ErrorDialog,
										Title:   "Update Failed",
										Message: fmt.Sprintf("Failed to update: %v", err),
									})
								}
							}
						} else {
							runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
								Type:    runtime.InfoDialog,
								Title:   "No Updates",
								Message: "You're running the latest version of Momentum!",
							})
						}
					}()
				}
			case <-mQuit.ClickedCh:
				if app != nil {
					app.QuitApp()
				}
				systray.Quit()
				return
			}
		}
	}()
}

func onSystrayExit() {
	// Cleanup
}

// newLogger appends timestamped lines to momentum.log (shared by UI and daemon) and stderr.
func newLogger(role string) func(string) {
	return func(msg string) {
		line := fmt.Sprintf("[%s %s] %s", nowStamp(), role, strings.TrimRight(msg, "\n"))
		fmt.Fprintln(os.Stderr, line)
		appendLog(line)
	}
}

// instanceID is unique per data folder, so a test copy with its own
// MOMENTUM_HOME doesn't hand over to the user's real Momentum.
func instanceID() string {
	sum := sha256.Sum256([]byte(strings.ToLower(dataDir())))
	return "momentum-" + hex.EncodeToString(sum[:6])
}
