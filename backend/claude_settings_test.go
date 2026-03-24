package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeGroveHooks_EmptySettings(t *testing.T) {
	settings := make(map[string]any)
	hooks := groveHooks("/home/user/.grove/hook.sh")

	modified := mergeGroveHooks(settings, hooks)

	if !modified {
		t.Fatal("expected settings to be modified")
	}

	hooksObj, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("expected hooks key in settings")
	}

	expectedEvents := []string{"UserPromptSubmit", "PostToolUse", "PermissionRequest", "Notification", "Stop"}
	for _, event := range expectedEvents {
		if _, ok := hooksObj[event]; !ok {
			t.Errorf("expected event %q in hooks", event)
		}
	}
}

func TestMergeGroveHooks_AlreadyPresent(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/home/user/.grove/hook.sh working",
							"async":   true,
						},
					},
				},
			},
			"PostToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/home/user/.grove/hook.sh working",
							"async":   true,
						},
					},
				},
			},
			"PermissionRequest": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/home/user/.grove/hook.sh permission",
							"async":   true,
						},
					},
				},
			},
			"Notification": []any{
				map[string]any{
					"matcher": "elicitation_dialog",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/home/user/.grove/hook.sh question",
							"async":   true,
						},
					},
				},
			},
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/home/user/.grove/hook.sh idle",
							"async":   true,
						},
					},
				},
			},
		},
	}

	modified := mergeGroveHooks(settings, groveHooks("/home/user/.grove/hook.sh"))

	if modified {
		t.Fatal("expected no modification when hooks already present")
	}
}

func TestMergeGroveHooks_PreservesExistingHooks(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/some/other/hook.sh",
						},
					},
				},
			},
			"UserPromptSubmit": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "/some/other/hook.sh",
						},
					},
				},
			},
		},
	}

	modified := mergeGroveHooks(settings, groveHooks("/home/user/.grove/hook.sh"))

	if !modified {
		t.Fatal("expected settings to be modified")
	}

	hooksObj := settings["hooks"].(map[string]any)

	// PreToolUse should be untouched (Grove doesn't use it)
	preToolUse := hooksObj["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Errorf("PreToolUse should still have 1 entry, got %d", len(preToolUse))
	}

	// UserPromptSubmit should have both the existing hook and the new Grove hook
	userPrompt := hooksObj["UserPromptSubmit"].([]any)
	if len(userPrompt) != 2 {
		t.Errorf("UserPromptSubmit should have 2 entries (existing + grove), got %d", len(userPrompt))
	}
}

func TestMergeGroveHooks_NotificationHasMatcher(t *testing.T) {
	settings := make(map[string]any)

	mergeGroveHooks(settings, groveHooks("/home/user/.grove/hook.sh"))

	hooksObj := settings["hooks"].(map[string]any)
	notifHooks := hooksObj["Notification"].([]any)
	group := notifHooks[0].(map[string]any)

	matcher, ok := group["matcher"].(string)
	if !ok || matcher != "elicitation_dialog" {
		t.Errorf("Notification hook should have matcher 'elicitation_dialog', got %q", matcher)
	}
}

func TestMergeGroveHooks_Idempotent(t *testing.T) {
	settings := make(map[string]any)
	hooks := groveHooks("/home/user/.grove/hook.sh")

	mergeGroveHooks(settings, hooks)
	first, _ := json.Marshal(settings)

	modified := mergeGroveHooks(settings, hooks)
	second, _ := json.Marshal(settings)

	if modified {
		t.Fatal("second merge should not modify")
	}
	if string(first) != string(second) {
		t.Error("settings changed after second merge")
	}
}

func TestReadSettingsFile_NonExistent(t *testing.T) {
	settings, err := readSettingsFile(filepath.Join(t.TempDir(), "nonexistent.json"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(settings) != 0 {
		t.Errorf("expected empty map, got %v", settings)
	}
}

func TestWriteSettingsFile_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Write initial file
	initial := map[string]any{"version": "1"}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Write new settings (should create backup)
	updated := map[string]any{"version": "2"}
	if err := writeSettingsFile(path, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check backup exists with original content
	backupData, err := os.ReadFile(path + ".bak") // #nosec G304
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}

	var backup map[string]any
	if err = json.Unmarshal(backupData, &backup); err != nil {
		t.Fatal(err)
	}
	if backup["version"] != "1" {
		t.Errorf("backup should have version 1, got %v", backup["version"])
	}

	// Check new file has updated content
	newData, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(newData, &result); err != nil {
		t.Fatal(err)
	}
	if result["version"] != "2" {
		t.Errorf("new file should have version 2, got %v", result["version"])
	}
}

func TestWriteSettingsFile_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "nested")
	path := filepath.Join(dir, "settings.json")

	settings := map[string]any{"test": true}
	if err := writeSettingsFile(path, settings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(dst) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}
