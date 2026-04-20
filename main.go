package main

import (
	"embed"
	"log"
	"os"

	"github.com/Jordan-Kowal/grove/backend"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:dist
var assets embed.FS

//go:embed public/app-icon.png
var appIcon []byte

// Keep in sync with package.json and build/config.yml
const appVersion = "0.3.2"

func main() {
	backend.FixPath()

	workspaceSvc := backend.NewWorkspaceService()
	editorSvc := backend.NewEditorService()
	soundSvc := backend.NewSoundService()
	snapSvc := backend.NewSnapService()
	traySvc := backend.NewTrayService()
	monitorSvc := backend.NewMonitorService(workspaceSvc, editorSvc, soundSvc, traySvc)
	appSvc := backend.NewAppService(appVersion)

	app := application.New(application.Options{
		Name:        "Grove",
		Description: "Lightweight worktree dashboard",
		Icon:        appIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(appSvc),
			application.NewService(workspaceSvc),
			application.NewService(monitorSvc),
			application.NewService(soundSvc),
			application.NewService(snapSvc),
			application.NewService(traySvc),
			application.NewService(editorSvc),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
			ActivationPolicy: application.ActivationPolicyRegular,
		},
	})

	// Application menu
	menu := application.NewMenu()
	menu.AddRole(application.AppMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	helpMenu := menu.AddSubmenu("Help")
	helpMenu.Add("Version: " + appVersion).SetEnabled(false)
	app.Menu.Set(menu)

	// Narrow sidebar window, always on top
	isDevMode := os.Getenv("FRONTEND_DEVSERVER_URL") != ""
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:       "Grove",
		Width:       250,
		Height:      800,
		URL:         "/",
		AlwaysOnTop: true,
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHiddenInset,
		},
	})

	// System tray (menu bar icon)
	traySvc.Init(app, window)
	snapSvc.SetWindow(window)

	// Open devtools in dev mode
	if isDevMode {
		devToolsOpened := false
		window.RegisterHook(events.Common.WindowShow, func(_ *application.WindowEvent) {
			if !devToolsOpened {
				devToolsOpened = true
				window.OpenDevTools()
			}
		})
	}

	// Window edge snapping
	window.RegisterHook(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		snapSvc.HandleMove(window)
	})

	// macOS: hide window on close instead of quitting
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		window.Hide()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
