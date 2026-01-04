package main

import (
	"embed"
	"flag"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
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
		Title:     "Remote Bridge",
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
	systray.SetTitle("Remote Bridge")
	systray.SetTooltip("Remote Bridge - Running in background")

	// Menu Items
	mShow := systray.AddMenuItem("Show Bridge", "Show the main window")
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
