package backend

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed sounds/*.aiff
var soundFiles embed.FS

// Available bundled sounds.
var bundledSounds = []string{"Glass", "Ping", "Pop", "Purr", "Tink"}

// SoundMode controls when notification sounds play.
type SoundMode string

const (
	SoundModeNever      SoundMode = "never"
	SoundModePermission SoundMode = "permission"
	SoundModeAll        SoundMode = "all"
)

// SoundService handles sound playback and sound preferences.
type SoundService struct {
	mu       sync.RWMutex
	mode     SoundMode
	sound    string
	cacheDir string
}

// NewSoundService creates a new SoundService with defaults.
func NewSoundService() *SoundService {
	cacheDir := filepath.Join(os.TempDir(), "grove-sounds")
	_ = os.MkdirAll(cacheDir, 0o700) // #nosec G301
	return &SoundService{
		mode:     SoundModeAll,
		sound:    "Glass",
		cacheDir: cacheDir,
	}
}

// GetSounds returns the list of available sounds.
func (s *SoundService) GetSounds() []string {
	return bundledSounds
}

// SetPreferences updates sound preferences from the frontend.
func (s *SoundService) SetPreferences(mode string, sound string) error {
	if !validMode(mode) {
		return fmt.Errorf("unknown sound mode %q", mode)
	}
	if !validSound(sound) {
		return fmt.Errorf("unknown sound %q", sound)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = SoundMode(mode)
	s.sound = sound
	return nil
}

// PlayPreview plays a sound for the settings preview button.
func (s *SoundService) PlayPreview(name string) error {
	if !validSound(name) {
		return fmt.Errorf("unknown sound %q", name)
	}
	return s.play(name)
}

// PlayIfNeeded plays the configured sound based on the current mode.
// isPermission indicates whether the trigger is a permission/question event.
func (s *SoundService) PlayIfNeeded(isPermission bool) {
	s.mu.RLock()
	mode := s.mode
	sound := s.sound
	s.mu.RUnlock()

	if sound == "" {
		return
	}

	switch mode {
	case SoundModeAll:
		if err := s.play(sound); err != nil {
			log.Printf("grove: sound playback failed: %v", err)
		}
	case SoundModePermission:
		if isPermission {
			if err := s.play(sound); err != nil {
				log.Printf("grove: sound playback failed: %v", err)
			}
		}
	case SoundModeNever:
		// no-op
	}
}

func (s *SoundService) play(name string) error {
	path, err := s.ensureCached(name)
	if err != nil {
		return err
	}
	cmd := exec.Command("afplay", path) // #nosec G204 -- name validated
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // reap the process to avoid zombies
	return nil
}

func (s *SoundService) ensureCached(name string) (string, error) {
	cached := filepath.Join(s.cacheDir, name+".aiff")
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	data, err := soundFiles.ReadFile("sounds/" + name + ".aiff")
	if err != nil {
		return "", fmt.Errorf("sound %q not found: %w", name, err)
	}
	if err := os.WriteFile(cached, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to cache sound: %w", err)
	}
	return cached, nil
}

func validMode(mode string) bool {
	switch SoundMode(mode) {
	case SoundModeNever, SoundModePermission, SoundModeAll:
		return true
	}
	return false
}

func validSound(name string) bool {
	for _, s := range bundledSounds {
		if s == name {
			return true
		}
	}
	return false
}
