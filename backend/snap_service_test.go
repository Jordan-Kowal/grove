package backend

import "testing"

func TestSnapServiceSetEnabled(t *testing.T) {
	svc := NewSnapService()

	// Default is enabled
	svc.mu.RLock()
	if !svc.enabled {
		t.Error("expected default enabled=true")
	}
	svc.mu.RUnlock()

	// Disable
	svc.SetEnabled(false)
	svc.mu.RLock()
	if svc.enabled {
		t.Error("expected enabled=false after SetEnabled(false)")
	}
	svc.mu.RUnlock()

	// Re-enable
	svc.SetEnabled(true)
	svc.mu.RLock()
	if !svc.enabled {
		t.Error("expected enabled=true after SetEnabled(true)")
	}
	svc.mu.RUnlock()
}

func TestSnapServiceSetEnabledClearsSnapSide(t *testing.T) {
	svc := NewSnapService()

	// Simulate a snap side being set
	svc.mu.Lock()
	svc.snapSide = snapLeft
	svc.mu.Unlock()

	// Disabling should clear snap side
	svc.SetEnabled(false)
	if got := svc.GetSnapSide(); got != "" {
		t.Errorf("expected snapSide=\"\" after disable, got %q", got)
	}
}

func TestSnapServiceGetSnapSide(t *testing.T) {
	svc := NewSnapService()

	if got := svc.GetSnapSide(); got != "" {
		t.Errorf("expected initial snapSide=\"\", got %q", got)
	}

	svc.mu.Lock()
	svc.snapSide = snapRight
	svc.mu.Unlock()

	if got := svc.GetSnapSide(); got != snapRight {
		t.Errorf("expected snapSide=\"right\", got %q", got)
	}
}

func TestSnapServiceGetEditorBoundsNotSnapped(t *testing.T) {
	svc := NewSnapService()
	bounds := svc.GetEditorBounds()
	if bounds.Width != 0 || bounds.Height != 0 {
		t.Errorf("expected zero bounds when not snapped, got %+v", bounds)
	}
}

func TestSnapServiceGetEditorBoundsNoWindow(t *testing.T) {
	svc := NewSnapService()
	svc.mu.Lock()
	svc.snapSide = snapLeft
	svc.mu.Unlock()

	bounds := svc.GetEditorBounds()
	if bounds.Width != 0 || bounds.Height != 0 {
		t.Errorf("expected zero bounds when no window, got %+v", bounds)
	}
}

func TestSnapServiceHandleMoveDisabled(t *testing.T) {
	svc := NewSnapService()
	svc.SetEnabled(false)

	// Should not panic with nil window when disabled
	svc.HandleMove(nil)
}
