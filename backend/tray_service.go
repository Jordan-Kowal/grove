package backend

import (
	"context"
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TODO: Replace placeholder icons with proper Grove tray icons (22x22 template PNGs).

//go:embed icons/tray.png
var trayIcon []byte

//go:embed icons/tray-badge.png
var trayBadgeIcon []byte

// TrayService manages the macOS menu bar (system tray) icon.
type TrayService struct {
	tray   *application.SystemTray
	window *application.WebviewWindow
}

// NewTrayService creates a new TrayService.
func NewTrayService() *TrayService {
	return &TrayService{}
}

// Init sets up the system tray after the app and window are created.
// Must be called from main before app.Run().
func (s *TrayService) Init(app *application.App, window *application.WebviewWindow) {
	s.window = window
	s.tray = app.SystemTray.New()
	s.setNormalIcon()
	s.tray.SetTooltip("Grove")

	menu := app.NewMenu()
	menu.Add("Show Grove").OnClick(func(_ *application.Context) {
		window.Show()
		window.Focus()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	s.tray.SetMenu(menu)

	s.tray.OnClick(func() {
		if window.IsVisible() {
			window.Hide()
		} else {
			window.Show()
			window.Focus()
		}
	})

	// Start hidden — frontend setting will call SetEnabled to show
	s.tray.Hide()
}

// ServiceStartup satisfies the Wails service lifecycle.
func (s *TrayService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	return nil
}

// SetEnabled shows or hides the system tray icon.
func (s *TrayService) SetEnabled(enabled bool) {
	if s.tray == nil {
		return
	}
	if enabled {
		s.tray.Show()
	} else {
		s.tray.Hide()
	}
}

// SetBadge switches the tray icon to the badge variant (red dot).
func (s *TrayService) SetBadge() {
	if s.tray == nil {
		return
	}
	s.tray.SetIcon(trayBadgeIcon)
}

// RemoveBadge switches the tray icon back to normal.
func (s *TrayService) RemoveBadge() {
	if s.tray == nil {
		return
	}
	s.setNormalIcon()
}

func (s *TrayService) setNormalIcon() {
	s.tray.SetIcon(trayIcon)
}
