package backend

import "testing"

// TrayService methods guard against a nil system tray so callers don't have to
// check Init status. These tests exercise the guard paths — behavior with a
// live tray requires a running Wails app and is not covered here.

func TestNewTrayService(t *testing.T) {
	svc := NewTrayService()
	if svc == nil {
		t.Fatal("NewTrayService returned nil")
	}
	if svc.tray != nil {
		t.Error("expected tray to be nil before Init")
	}
	if svc.window != nil {
		t.Error("expected window to be nil before Init")
	}
}

func TestTrayService_SetEnabled_NilGuard(t *testing.T) {
	svc := NewTrayService()
	// Must not panic when tray is nil.
	svc.SetEnabled(true)
	svc.SetEnabled(false)
}

func TestTrayService_SetBadge_NilGuard(t *testing.T) {
	svc := NewTrayService()
	// Must not panic when tray is nil.
	svc.SetBadge()
}

func TestTrayService_RemoveBadge_NilGuard(t *testing.T) {
	svc := NewTrayService()
	// Must not panic when tray is nil.
	svc.RemoveBadge()
}

func TestTrayService_ImplementsTrayBadger(t *testing.T) {
	// Compile-time check: TrayService must satisfy the trayBadger interface
	// consumed by MonitorService.
	var _ trayBadger = (*TrayService)(nil)
}

func TestTrayService_EmbeddedIconsPresent(t *testing.T) {
	if len(trayIcon) == 0 {
		t.Error("trayIcon embed is empty")
	}
	if len(trayBadgeIcon) == 0 {
		t.Error("trayBadgeIcon embed is empty")
	}
}
