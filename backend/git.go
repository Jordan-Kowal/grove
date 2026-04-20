package backend

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pollGitTimeout bounds any `git` subprocess spawned from the 10s polling loop.
// If git hangs (index lock, slow FS, hung lfs/hook), we abort the call so
// gitBusy is released promptly and the next tick isn't skipped.
const pollGitTimeout = 5 * time.Second

// untrackedFileLimit caps how many untracked files we will line-count in a
// single worktree. Accidentally unignored `node_modules` or build output can
// contain tens of thousands of files; beyond this cap we stop reading and
// return what we have, so the 10s poll never turns into a GB-scale scan.
const untrackedFileLimit = 500

// untrackedMaxFileSize skips line counting for any untracked file larger than
// this threshold. Large files (lockfiles, dumps, logs) would require a full
// read per mtime change; we'd rather undercount than thrash the disk.
const untrackedMaxFileSize = 1 << 20 // 1 MiB

// parseGitDiffStat parses the output of `git diff --shortstat` and returns file count, insertions, deletions.
func parseGitDiffStat(output string) (files, insertions, deletions int) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return 0, 0, 0
	}

	summary := lines[len(lines)-1]
	if !strings.Contains(summary, "changed") {
		return 0, 0, 0
	}

	parts := strings.Split(summary, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		switch {
		case strings.Contains(part, "file"):
			files = n
		case strings.Contains(part, "insertion"):
			insertions = n
		case strings.Contains(part, "deletion"):
			deletions = n
		}
	}
	return files, insertions, deletions
}

// untrackedCacheEntry is one cached line-count result for an untracked file.
// Keyed by absolute path in untrackedCache.entries. Invalidated when mtime or size changes.
type untrackedCacheEntry struct {
	modTime int64 // UnixNano
	size    int64
	lines   int
}

// untrackedCache memoizes line counts for untracked files across refreshGit passes.
// Key is the absolute file path. Safe for concurrent use.
type untrackedCache struct {
	mu      sync.Mutex
	entries map[string]untrackedCacheEntry
}

func newUntrackedCache() *untrackedCache {
	return &untrackedCache{entries: make(map[string]untrackedCacheEntry)}
}

// sweepUnseen drops any cache entry whose path was not recorded in seen during the
// most recent refreshGit pass. Files deleted inside a worktree disappear from
// `git ls-files --others` output and thus from seen, so this reclaims their slot.
// Called once per refreshGit after all worktrees have been scanned.
func (c *untrackedCache) sweepUnseen(seen map[string]struct{}) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for p := range c.entries {
		if _, ok := seen[p]; !ok {
			delete(c.entries, p)
		}
	}
}

// untrackedSniffBytes matches git's heuristic: if the first 8KB contains a NUL byte, treat as binary.
const untrackedSniffBytes = 8000

// countFileLines returns the number of newline-terminated lines, or a trailing partial line.
// Mirrors `wc -l`-style counting used by git diff's insertion stat: a file ending without
// a newline still counts its final line (git reports "no newline at end of file" but counts it).
func countFileLines(path string) (lines int, isBinary bool, err error) {
	f, err := os.Open(path) // #nosec G304 — path comes from `git ls-files --others` under a worktree we own.
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 32*1024)
	var total int
	var lastByte byte
	var sniffed bool
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if !sniffed {
				head := chunk
				if len(head) > untrackedSniffBytes {
					head = head[:untrackedSniffBytes]
				}
				if bytes.IndexByte(head, 0) >= 0 {
					return 0, true, nil
				}
				sniffed = true
			}
			total += bytes.Count(chunk, []byte{'\n'})
			if len(chunk) > 0 {
				lastByte = chunk[len(chunk)-1]
			}
		}
		if readErr != nil {
			break
		}
	}
	if total == 0 && lastByte == 0 {
		return 0, false, nil
	}
	if lastByte != '\n' {
		total++
	}
	return total, false, nil
}

// getUntrackedStats counts untracked (but non-ignored) files and their line totals in dir.
// Uses `git ls-files --others --exclude-standard -z` to enumerate, then line-counts each
// file via the mtime+size cache. Behavior:
//   - Binary files count as a file but contribute 0 insertions (matches `git diff`).
//   - Files larger than untrackedMaxFileSize also contribute 0 insertions; we refuse to
//     re-read multi-MB files on every mtime bump.
//   - After untrackedFileLimit files, we stop listing entirely (protects against
//     accidentally unignored node_modules / build output).
//
// If seen is non-nil, each visited absolute path is recorded in it so the caller can
// later sweepUnseen() on the cache.
func getUntrackedStats(ctx context.Context, dir string, cache *untrackedCache, seen map[string]struct{}) (files, insertions int) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "--no-optional-locks", "ls-files", "--others", "--exclude-standard", "-z") // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	if len(out) == 0 {
		return 0, 0
	}

	paths := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		if files >= untrackedFileLimit {
			break
		}
		abs := filepath.Join(dir, rel)
		info, statErr := os.Stat(abs)
		if statErr != nil || info.IsDir() {
			continue
		}
		files++
		if seen != nil {
			seen[abs] = struct{}{}
		}
		insertions += lookupOrCountLines(cache, abs, info)
	}
	return files, insertions
}

// lookupOrCountLines returns the cached line count for abs if mtime+size match; otherwise
// it counts the file, stores the result, and returns it. Files larger than
// untrackedMaxFileSize cache lines=0 without being read. Binary files likewise cache 0.
func lookupOrCountLines(cache *untrackedCache, abs string, info os.FileInfo) int {
	modNano := info.ModTime().UnixNano()
	size := info.Size()

	if cache != nil {
		cache.mu.Lock()
		if entry, ok := cache.entries[abs]; ok && entry.modTime == modNano && entry.size == size {
			cache.mu.Unlock()
			return entry.lines
		}
		cache.mu.Unlock()
	}

	lines := 0
	if size <= untrackedMaxFileSize {
		counted, isBinary, err := countFileLines(abs)
		if err != nil {
			return 0
		}
		if !isBinary {
			lines = counted
		}
	}
	if cache != nil {
		cache.mu.Lock()
		cache.entries[abs] = untrackedCacheEntry{modTime: modNano, size: size, lines: lines}
		cache.mu.Unlock()
	}
	return lines
}
