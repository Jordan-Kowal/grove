package backend

import "testing"

func TestPositionWindowValidation(t *testing.T) {
	svc := NewEditorService()

	// Empty app name should be a no-op (no error)
	if err := svc.PositionWindow("", 0, 0, 800, 600); err != nil {
		t.Errorf("expected nil error for empty app, got %v", err)
	}

	// Zero width should be a no-op
	if err := svc.PositionWindow("Zed", 0, 0, 0, 600); err != nil {
		t.Errorf("expected nil error for zero width, got %v", err)
	}

	// Zero height should be a no-op
	if err := svc.PositionWindow("Zed", 0, 0, 800, 0); err != nil {
		t.Errorf("expected nil error for zero height, got %v", err)
	}
}

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"double quote", `say "hi"`, `say \"hi\"`},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"both", `"hello\world"`, `\"hello\\world\"`},
		{"empty", "", ""},
		{"single quote", "it's", "it'\"'\"'s"},
		{"path with spaces", "/Users/me/my project", "/Users/me/my project"},
		{"multiple backslashes", `a\\b`, `a\\\\b`},
		{"backslash then quote", `a\"b`, `a\\\"b`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeAppleScript(tt.input)
			if got != tt.want {
				t.Errorf("escapeAppleScript(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
