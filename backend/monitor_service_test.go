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
