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

func TestMatchOpenPaths(t *testing.T) {
	svc := NewEditorService()

	tests := []struct {
		name         string
		windowTitles []string
		paths        []string
		wantOpen     map[string]bool
	}{
		{
			"exact folder name in title",
			[]string{"grove — ~/Projects"},
			[]string{"/Users/me/Projects/grove"},
			map[string]bool{"/Users/me/Projects/grove": true},
		},
		{
			"no match",
			[]string{"other-project — ~/Work"},
			[]string{"/Users/me/Projects/grove"},
			map[string]bool{},
		},
		{
			"empty titles",
			nil,
			[]string{"/Users/me/Projects/grove"},
			map[string]bool{},
		},
		{
			"worktree wins over root when both base names appear",
			[]string{"my-feature — ~/Projects/grove/.worktrees/my-feature"},
			[]string{"/Users/me/Projects/grove", "/Users/me/Projects/grove/.worktrees/my-feature"},
			map[string]bool{"/Users/me/Projects/grove/.worktrees/my-feature": true},
		},
		{
			"root matches only when worktree does not",
			[]string{"grove — ~/Projects/grove"},
			[]string{"/Users/me/Projects/grove", "/Users/me/Projects/grove/.worktrees/my-feature"},
			map[string]bool{"/Users/me/Projects/grove": true},
		},
		{
			"multiple windows match different paths",
			[]string{"grove — editor", "my-feature — editor"},
			[]string{"/Users/me/Projects/grove", "/Users/me/Projects/grove/.worktrees/my-feature"},
			map[string]bool{"/Users/me/Projects/grove": true, "/Users/me/Projects/grove/.worktrees/my-feature": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.MatchOpenPaths(tt.windowTitles, tt.paths)
			for _, p := range tt.paths {
				if got[p] != tt.wantOpen[p] {
					t.Errorf("path %q: got open=%v, want open=%v", p, got[p], tt.wantOpen[p])
				}
			}
		})
	}
}
