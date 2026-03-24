package backend

import (
	"os"
	"testing"
)

func TestIsProcessAlive(t *testing.T) {
	if !isProcessAlive(os.Getpid()) {
		t.Error("isProcessAlive(own pid) = false, want true")
	}
	if isProcessAlive(0) {
		t.Error("isProcessAlive(0) = true, want false")
	}
	if isProcessAlive(4999999) {
		t.Error("isProcessAlive(4999999) = true, want false")
	}
}

func TestGroveStateToClaudeStatus(t *testing.T) {
	tests := []struct {
		state string
		want  ClaudeStatus
	}{
		{"working", ClaudeStatusWorking},
		{"permission", ClaudeStatusPermission},
		{"question", ClaudeStatusQuestion},
		{"idle", ClaudeStatusIdle},
		{"", ClaudeStatusIdle},
		{"unknown", ClaudeStatusIdle},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := groveStateToClaudeStatus(tt.state); got != tt.want {
				t.Errorf("groveStateToClaudeStatus(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{2684354560, "2.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatBytes(tt.bytes); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
