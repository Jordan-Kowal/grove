package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// groveHookEntry defines a single hook that Grove needs registered in Claude Code settings.
type groveHookEntry struct {
	Event   string // e.g. "UserPromptSubmit"
	Matcher string // optional matcher (e.g. "elicitation_dialog")
	Command string // e.g. "~/.grove/hook.sh working"
}

// groveHooks returns the hook entries Grove needs in settings.json.
func groveHooks(hookPath string) []groveHookEntry {
	return []groveHookEntry{
		{Event: "UserPromptSubmit", Command: hookPath + " working"},
		{Event: "PostToolUse", Command: hookPath + " working"},
		{Event: "PermissionRequest", Command: hookPath + " permission"},
		{Event: "Notification", Matcher: "elicitation_dialog", Command: hookPath + " question"},
		{Event: "Stop", Command: hookPath + " done"},
	}
}

// ensureClaudeSettings reads ~/.claude/settings.json, merges Grove hooks if missing,
// and writes back. Returns true if settings were modified.
func ensureClaudeSettings(hookPath string) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("grove: failed to get home directory: %v", err)
		return false
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	settings, err := readSettingsFile(settingsPath)
	if err != nil {
		log.Printf("grove: failed to read claude settings: %v", err)
		return false
	}

	modified := mergeGroveHooks(settings, groveHooks(hookPath))
	if !modified {
		return false
	}

	if err := writeSettingsFile(settingsPath, settings); err != nil {
		log.Printf("grove: failed to write claude settings: %v", err)
		return false
	}

	log.Printf("grove: claude code hooks installed in %s", settingsPath)
	return true
}

// readSettingsFile reads and parses a JSON settings file.
// Returns an empty map if the file doesn't exist.
func readSettingsFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return settings, nil
}

// writeSettingsFile writes settings to a JSON file with a backup.
func writeSettingsFile(path string, settings map[string]any) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	// Backup existing file
	if _, err := os.Stat(path); err == nil {
		backupPath := path + ".bak"
		if err := copyFile(path, backupPath); err != nil {
			return fmt.Errorf("backup %s: %w", path, err)
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { // #nosec G301
		return fmt.Errorf("create dir: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

// mergeGroveHooks ensures all Grove hook entries in settings are present and up to date.
// Adds missing hooks and updates stale ones (e.g. command changed). Returns true if modified.
func mergeGroveHooks(settings map[string]any, hooks []groveHookEntry) bool {
	hooksObj, _ := settings["hooks"].(map[string]any)
	if hooksObj == nil {
		hooksObj = make(map[string]any)
		settings["hooks"] = hooksObj
	}

	modified := false
	for _, h := range hooks {
		switch groveHookState(hooksObj, h) {
		case hookMissing:
			appendHook(hooksObj, h)
			modified = true
		case hookStale:
			removeGroveHook(hooksObj, h.Event)
			appendHook(hooksObj, h)
			modified = true
		case hookCurrent:
			// already up to date
		}
	}
	return modified
}

type hookStatus int

const (
	hookMissing hookStatus = iota
	hookStale
	hookCurrent
)

// groveHookState checks whether a Grove hook entry exists under the given event key
// and whether its command matches. Returns hookMissing, hookStale, or hookCurrent.
func groveHookState(hooksObj map[string]any, entry groveHookEntry) hookStatus {
	eventHooks, _ := hooksObj[entry.Event].([]any)
	for _, group := range eventHooks {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, _ := groupMap["hooks"].([]any)
		for _, hook := range innerHooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hookMap["command"].(string)
			if !strings.Contains(cmd, "grove/hook.sh") {
				continue
			}
			if cmd == entry.Command {
				return hookCurrent
			}
			return hookStale
		}
	}
	return hookMissing
}

// removeGroveHook removes all Grove hook groups from the given event key.
func removeGroveHook(hooksObj map[string]any, event string) {
	eventHooks, _ := hooksObj[event].([]any)
	var kept []any
	for _, group := range eventHooks {
		groupMap, ok := group.(map[string]any)
		if !ok {
			kept = append(kept, group)
			continue
		}
		innerHooks, _ := groupMap["hooks"].([]any)
		isGrove := false
		for _, hook := range innerHooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hookMap["command"].(string)
			if strings.Contains(cmd, "grove/hook.sh") {
				isGrove = true
				break
			}
		}
		if !isGrove {
			kept = append(kept, group)
		}
	}
	if len(kept) == 0 {
		delete(hooksObj, event)
	} else {
		hooksObj[event] = kept
	}
}

// appendHook adds a Grove hook entry to the event's hook array.
func appendHook(hooksObj map[string]any, entry groveHookEntry) {
	hookDef := map[string]any{
		"type":    "command",
		"command": entry.Command,
		"async":   true,
	}

	group := map[string]any{
		"hooks": []any{hookDef},
	}
	if entry.Matcher != "" {
		group["matcher"] = entry.Matcher
	}

	existing, _ := hooksObj[entry.Event].([]any)
	hooksObj[entry.Event] = append(existing, group)
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) // #nosec G304
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode()) // #nosec G703
}
