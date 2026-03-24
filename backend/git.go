package backend

import (
	"strconv"
	"strings"
)

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
