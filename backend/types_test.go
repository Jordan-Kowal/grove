package backend

import "testing"

func TestClaudeStatusPriority(t *testing.T) {
	tests := []struct {
		status ClaudeStatus
		want   int
	}{
		{ClaudeStatusPermission, 3},
		{ClaudeStatusQuestion, 3},
		{ClaudeStatusWorking, 2},
		{ClaudeStatusIdle, 1},
		{ClaudeStatusDone, 1},
		{ClaudeStatus("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := claudeStatusPriority(tt.status); got != tt.want {
				t.Errorf("claudeStatusPriority(%q) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}

	// Verify ordering: permission > working > idle
	if claudeStatusPriority(ClaudeStatusPermission) <= claudeStatusPriority(ClaudeStatusWorking) {
		t.Error("permission should have higher priority than working")
	}
	if claudeStatusPriority(ClaudeStatusWorking) <= claudeStatusPriority(ClaudeStatusIdle) {
		t.Error("working should have higher priority than idle")
	}
}
