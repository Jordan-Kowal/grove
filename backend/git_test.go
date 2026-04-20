package backend

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

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

func TestCountFileLines(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name      string
		content   []byte
		wantLines int
		wantBin   bool
	}{
		{"empty", []byte{}, 0, false},
		{"one line no newline", []byte("hello"), 1, false},
		{"one line with newline", []byte("hello\n"), 1, false},
		{"three lines trailing newline", []byte("a\nb\nc\n"), 3, false},
		{"three lines no trailing", []byte("a\nb\nc"), 3, false},
		{"binary nul byte", []byte("abc\x00def"), 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, tc.name)
			if err := os.WriteFile(p, tc.content, 0o600); err != nil {
				t.Fatal(err)
			}
			lines, bin, err := countFileLines(p)
			if err != nil {
				t.Fatal(err)
			}
			if bin != tc.wantBin {
				t.Errorf("isBinary=%v want %v", bin, tc.wantBin)
			}
			if !tc.wantBin && lines != tc.wantLines {
				t.Errorf("lines=%d want %d", lines, tc.wantLines)
			}
		})
	}
}

func TestGetUntrackedStatsAndCache(t *testing.T) {
	dir := initTestRepo(t)
	ctx := context.Background()

	// Clean repo: no untracked files.
	if f, ins := getUntrackedStats(ctx, dir, nil, nil); f != 0 || ins != 0 {
		t.Errorf("clean repo: got files=%d ins=%d, want 0 0", f, ins)
	}

	// Add an untracked 3-line text file and a binary file.
	txt := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(txt, []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(bin, []byte("abc\x00def\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cache := newUntrackedCache()
	files, ins := getUntrackedStats(ctx, dir, cache, nil)
	if files != 2 {
		t.Errorf("files=%d want 2", files)
	}
	if ins != 3 {
		t.Errorf("insertions=%d want 3 (binary contributes 0)", ins)
	}

	// Cache hit: mutate the cached entry to a sentinel and confirm the cache is used
	// (size/mtime unchanged → value comes from cache, not a re-read).
	cache.mu.Lock()
	cache.entries[txt] = untrackedCacheEntry{
		modTime: cache.entries[txt].modTime,
		size:    cache.entries[txt].size,
		lines:   999,
	}
	cache.mu.Unlock()
	_, ins2 := getUntrackedStats(ctx, dir, cache, nil)
	if ins2 != 999 {
		t.Errorf("cache not used: insertions=%d want 999", ins2)
	}

	// Modify the file: mtime + size change invalidate the cache.
	if err := os.WriteFile(txt, []byte("x\ny\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, ins3 := getUntrackedStats(ctx, dir, cache, nil)
	if ins3 != 2 {
		t.Errorf("after modify insertions=%d want 2", ins3)
	}
}

func TestGetUntrackedStatsRespectsGitignore(t *testing.T) {
	dir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.log"), []byte("1\n2\n3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tracked-later.txt"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, ins := getUntrackedStats(context.Background(), dir, nil, nil)
	// .gitignore itself + tracked-later.txt = 2 untracked; ignored.log excluded.
	if files != 2 {
		t.Errorf("files=%d want 2", files)
	}
	if ins != 2 {
		t.Errorf("ins=%d want 2 (1 from .gitignore + 1 from tracked-later)", ins)
	}
}

func TestGetUntrackedStatsFileLimit(t *testing.T) {
	dir := initTestRepo(t)

	// Create more files than the cap. Each is 1 line, so uncapped insertions would equal file count.
	total := untrackedFileLimit + 50
	for i := 0; i < total; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%04d.txt", i))
		if err := os.WriteFile(p, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files, ins := getUntrackedStats(context.Background(), dir, nil, nil)
	if files != untrackedFileLimit {
		t.Errorf("files=%d want cap %d", files, untrackedFileLimit)
	}
	if ins != untrackedFileLimit {
		t.Errorf("ins=%d want %d (1 line per file up to cap)", ins, untrackedFileLimit)
	}
}

func TestGetUntrackedStatsLargeFileSkipped(t *testing.T) {
	dir := initTestRepo(t)

	// A file bigger than untrackedMaxFileSize must count as a file but contribute 0 lines.
	big := filepath.Join(dir, "big.log")
	payload := bytes.Repeat([]byte("x\n"), (untrackedMaxFileSize/2)+1) // > 1 MiB
	if err := os.WriteFile(big, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	// Plus a small file so we know the counter actually iterated past big.
	small := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(small, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, ins := getUntrackedStats(context.Background(), dir, nil, nil)
	if files != 2 {
		t.Errorf("files=%d want 2", files)
	}
	if ins != 2 {
		t.Errorf("ins=%d want 2 (big file skipped, small file 2 lines)", ins)
	}
}

func TestGetUntrackedStatsTimeoutCancelsGit(t *testing.T) {
	dir := initTestRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel: ls-files should fail fast.

	files, ins := getUntrackedStats(ctx, dir, nil, nil)
	if files != 0 || ins != 0 {
		t.Errorf("cancelled ctx: got files=%d ins=%d, want 0 0", files, ins)
	}
}

func TestGetUntrackedStatsPopulatesSeen(t *testing.T) {
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]struct{})
	getUntrackedStats(context.Background(), dir, nil, seen)
	if len(seen) != 2 {
		t.Errorf("seen size=%d want 2", len(seen))
	}
	if _, ok := seen[filepath.Join(dir, "a.txt")]; !ok {
		t.Error("a.txt missing from seen")
	}
}

func TestUntrackedCacheSweepUnseen(t *testing.T) {
	c := newUntrackedCache()
	c.entries["/wt/a/file.txt"] = untrackedCacheEntry{lines: 1}
	c.entries["/wt/a/old.txt"] = untrackedCacheEntry{lines: 2}
	c.entries["/wt/b/file.txt"] = untrackedCacheEntry{lines: 3}

	// Only file.txt entries were seen this pass; old.txt was deleted.
	seen := map[string]struct{}{
		"/wt/a/file.txt": {},
		"/wt/b/file.txt": {},
	}
	c.sweepUnseen(seen)

	if _, ok := c.entries["/wt/a/old.txt"]; ok {
		t.Error("old.txt should have been swept")
	}
	if _, ok := c.entries["/wt/a/file.txt"]; !ok {
		t.Error("a/file.txt should remain")
	}
	if _, ok := c.entries["/wt/b/file.txt"]; !ok {
		t.Error("b/file.txt should remain")
	}
}
