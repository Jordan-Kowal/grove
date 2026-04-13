package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidMode(t *testing.T) {
	valid := []string{"never", "permission", "all"}
	for _, mode := range valid {
		t.Run("valid/"+mode, func(t *testing.T) {
			if !validMode(mode) {
				t.Errorf("validMode(%q) = false, want true", mode)
			}
		})
	}

	invalid := []string{"", "Never", "ALL", "always", "mute"}
	for _, mode := range invalid {
		label := mode
		if label == "" {
			label = "(empty)"
		}
		t.Run("invalid/"+label, func(t *testing.T) {
			if validMode(mode) {
				t.Errorf("validMode(%q) = true, want false", mode)
			}
		})
	}
}

func TestValidSound(t *testing.T) {
	valid := []string{"Glass", "Ping", "Pop", "Purr", "Tink"}
	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if !validSound(name) {
				t.Errorf("validSound(%q) = false, want true", name)
			}
		})
	}

	invalid := []string{"", "glass", "GLASS", "Beep", "unknown", " Glass"}
	for _, name := range invalid {
		label := name
		if label == "" {
			label = "(empty)"
		}
		t.Run("invalid/"+label, func(t *testing.T) {
			if validSound(name) {
				t.Errorf("validSound(%q) = true, want false", name)
			}
		})
	}
}

func TestGetSounds(t *testing.T) {
	svc := NewSoundService()
	sounds := svc.GetSounds()

	if len(sounds) != 5 {
		t.Fatalf("GetSounds() returned %d sounds, want 5", len(sounds))
	}

	expected := map[string]bool{"Glass": true, "Ping": true, "Pop": true, "Purr": true, "Tink": true}
	for _, s := range sounds {
		if !expected[s] {
			t.Errorf("unexpected sound %q", s)
		}
	}
}

func TestSetPreferences(t *testing.T) {
	svc := NewSoundService()

	if err := svc.SetPreferences("never", "Pop"); err != nil {
		t.Fatalf("SetPreferences(never, Pop) unexpected error: %v", err)
	}
	svc.mu.RLock()
	if svc.mode != SoundModeNever {
		t.Errorf("expected mode=never, got %q", svc.mode)
	}
	if svc.sound != "Pop" {
		t.Errorf("expected sound=Pop, got %q", svc.sound)
	}
	svc.mu.RUnlock()

	if err := svc.SetPreferences("permission", "Tink"); err != nil {
		t.Fatalf("SetPreferences(permission, Tink) unexpected error: %v", err)
	}
	svc.mu.RLock()
	if svc.mode != SoundModePermission {
		t.Errorf("expected mode=permission, got %q", svc.mode)
	}
	if svc.sound != "Tink" {
		t.Errorf("expected sound=Tink, got %q", svc.sound)
	}
	svc.mu.RUnlock()
}

func TestSetPreferencesRejectsInvalid(t *testing.T) {
	svc := NewSoundService()

	if err := svc.SetPreferences("invalid", "Glass"); err == nil {
		t.Error("expected error for invalid mode")
	}
	if err := svc.SetPreferences("all", "NonExistent"); err == nil {
		t.Error("expected error for invalid sound")
	}

	// Verify original defaults unchanged after rejected calls.
	svc.mu.RLock()
	if svc.mode != SoundModeAll {
		t.Errorf("mode changed to %q after rejected call", svc.mode)
	}
	if svc.sound != "Glass" {
		t.Errorf("sound changed to %q after rejected call", svc.sound)
	}
	svc.mu.RUnlock()
}

func TestPlayPreviewRejectsUnknownSound(t *testing.T) {
	svc := NewSoundService()
	err := svc.PlayPreview("NonExistent")
	if err == nil {
		t.Error("expected error for unknown sound")
	}
}

func TestEnsureCached(t *testing.T) {
	cacheDir := t.TempDir()
	svc := &SoundService{
		mode:     SoundModeAll,
		sound:    "Glass",
		cacheDir: cacheDir,
	}

	// First call should extract from embedded FS
	path, err := svc.ensureCached("Glass")
	if err != nil {
		t.Fatalf("ensureCached(Glass) failed: %v", err)
	}
	expectedPath := filepath.Join(cacheDir, "Glass.aiff")
	if path != expectedPath {
		t.Errorf("path = %q, want %q", path, expectedPath)
	}

	// File should exist on disk
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cached file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("cached file is empty")
	}
	firstSize := info.Size()

	// Second call should use cache (same path, no error)
	path2, err := svc.ensureCached("Glass")
	if err != nil {
		t.Fatalf("second ensureCached(Glass) failed: %v", err)
	}
	if path2 != expectedPath {
		t.Errorf("second call path = %q, want %q", path2, expectedPath)
	}
	info2, _ := os.Stat(path2)
	if info2.Size() != firstSize {
		t.Errorf("cached file size changed: %d -> %d", firstSize, info2.Size())
	}
}

func TestEnsureCachedUnknownSound(t *testing.T) {
	cacheDir := t.TempDir()
	svc := &SoundService{
		mode:     SoundModeAll,
		sound:    "Glass",
		cacheDir: cacheDir,
	}

	_, err := svc.ensureCached("DoesNotExist")
	if err == nil {
		t.Error("expected error for unknown embedded sound")
	}
}
