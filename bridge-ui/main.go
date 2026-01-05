package main

import (
	"embed"
	"flag"
	"fmt"

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
	// Parse command-line flags
	mcpMode := flag.Bool("mcp", false, "Run as MCP stdio server (no UI)")
	flag.Parse()

	// If --mcp flag is set, run MCP server instead of UI
	if *mcpMode {
		runMCPServer()
		return
	}

	// Otherwise, run normal Wails UI
	runWailsUI()
}

func runWailsUI() {
	// Create an instance of the app structure
	app = NewApp()

	// Run systray in a goroutine (it has its own event loop)
	go systray.Run(onSystrayReady, onSystrayExit)

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Momentum",
		Width:     1100,
		Height:    700,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			Theme:                             windows.Dark,
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
