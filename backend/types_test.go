package backend

import "testing"

func TestClaudeStatusPriority(t *testing.T) {
	tests := []struct {
		status ClaudeStatus
		want   int
	}{
		{ClaudeStatusPermission, 4},
		{ClaudeStatusQuestion, 4},
		{ClaudeStatusDone, 3},
		{ClaudeStatusWorking, 2},
		{ClaudeStatusIdle, 1},
		{ClaudeStatus("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := claudeStatusPriority(tt.status); got != tt.want {
				t.Errorf("claudeStatusPriority(%q) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}

	// Verify ordering: permission > done > working > idle
	if claudeStatusPriority(ClaudeStatusPermission) <= claudeStatusPriority(ClaudeStatusDone) {
		t.Error("permission should have higher priority than done")
	}
	if claudeStatusPriority(ClaudeStatusDone) <= claudeStatusPriority(ClaudeStatusWorking) {
		t.Error("done should have higher priority than working")
	}
	if claudeStatusPriority(ClaudeStatusWorking) <= claudeStatusPriority(ClaudeStatusIdle) {
		t.Error("working should have higher priority than idle")
	}
}
