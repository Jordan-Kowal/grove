package backend

import "testing"

func TestParseGitDiffStat(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantFiles int
		wantIns   int
		wantDel   int
	}{
		{
			name:      "empty output",
			output:    "",
			wantFiles: 0, wantIns: 0, wantDel: 0,
		},
		{
			name:      "no changes",
			output:    "nothing to commit",
			wantFiles: 0, wantIns: 0, wantDel: 0,
		},
		{
			name:      "single file",
			output:    " main.go | 5 +++--\n 1 file changed, 3 insertions(+), 2 deletions(-)",
			wantFiles: 1, wantIns: 3, wantDel: 2,
		},
		{
			name:      "multiple files",
			output:    " a.go | 10 ++++\n b.go | 3 ---\n 2 files changed, 10 insertions(+), 3 deletions(-)",
			wantFiles: 2, wantIns: 10, wantDel: 3,
		},
		{
			name:      "insertions only",
			output:    " new.go | 47 +++++\n 1 file changed, 47 insertions(+)",
			wantFiles: 1, wantIns: 47, wantDel: 0,
		},
		{
			name:      "deletions only",
			output:    " old.go | 12 ----\n 1 file changed, 12 deletions(-)",
			wantFiles: 1, wantIns: 0, wantDel: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, ins, del := parseGitDiffStat(tt.output)
			if files != tt.wantFiles || ins != tt.wantIns || del != tt.wantDel {
				t.Errorf("got (%d, %d, %d), want (%d, %d, %d)", files, ins, del, tt.wantFiles, tt.wantIns, tt.wantDel)
			}
		})
	}
}
