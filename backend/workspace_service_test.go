package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{
		"my-worktree",
		"feature_123",
		"a",
		"A",
		"abc-def_ghi",
		"123",
		"a1",
	}
	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if err := validateName(name); err != nil {
				t.Errorf("validateName(%q) = %v, want nil", name, err)
			}
		})
	}

	invalid := []struct {
		name    string
		wantMsg string
	}{
		{"", "cannot be empty"},
		{".", "invalid name"},
		{"..", "invalid name"},
		{".hidden", "invalid characters"},
		{"-leading-dash", "invalid characters"},
		{"_leading-underscore", "invalid characters"},
		{"has space", "invalid characters"},
		{"has/slash", "invalid characters"},
		{"../traversal", "invalid characters"},
		{"name\x00null", "invalid characters"},
		{"café", "invalid characters"},
	}
	for _, tt := range invalid {
		label := tt.name
		if label == "" {
			label = "(empty)"
		}
		t.Run("invalid/"+label, func(t *testing.T) {
			err := validateName(tt.name)
			if err == nil {
				t.Fatalf("validateName(%q) = nil, want error containing %q", tt.name, tt.wantMsg)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("validateName(%q) = %q, want error containing %q", tt.name, err.Error(), tt.wantMsg)
			}
		})
	}
}

// TestMain clears git env vars that leak from pre-commit hooks, preventing
// test git commands from accidentally operating on the real repo.
func TestMain(m *testing.M) {
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		_ = os.Unsetenv(key)
	}
	os.Exit(m.Run())
}

// initTestRepo creates a temporary git repo with one commit and returns its path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...) // #nosec G204
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v failed: %s: %v", args, out, err)
		}
	}
	return dir
}

func TestGetGitBranch(t *testing.T) {
	dir := initTestRepo(t)

	branch := getGitBranch(dir)
	// Default branch is usually "main" or "master"
	if branch != "main" && branch != "master" {
		t.Errorf("getGitBranch() = %q, want main or master", branch)
	}
}

func TestGetGitBranchInvalidDir(t *testing.T) {
	branch := getGitBranch("/nonexistent/path")
	if branch != "unknown" {
		t.Errorf("getGitBranch(invalid) = %q, want %q", branch, "unknown")
	}
}

func TestGetGitDiffStats(t *testing.T) {
	dir := initTestRepo(t)

	// No diff in clean repo
	files, ins, dels := getGitDiffStats(dir)
	if files != 0 || ins != 0 || dels != 0 {
		t.Errorf("clean repo: got files=%d ins=%d dels=%d, want all 0", files, ins, dels)
	}

	// Create and stage a tracked file, then modify it (diff only shows tracked changes)
	filePath := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(filePath, []byte("line1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "tracked.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s: %v", out, err)
	}
	cmd = exec.Command("git", "commit", "-m", "add file")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s: %v", out, err)
	}

	// Modify the tracked file
	if err := os.WriteFile(filePath, []byte("line1\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, ins, _ = getGitDiffStats(dir)
	if files != 1 {
		t.Errorf("expected 1 changed file, got %d", files)
	}
	if ins != 1 {
		t.Errorf("expected 1 insertion, got %d", ins)
	}
}

func TestReadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(projectDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"repoPath":"/tmp/repo","baseBranch":"origin/main","deleteBranch":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := &WorkspaceService{groveDir: tmpDir}
	config := svc.readConfig("test-project")

	if config.RepoPath != "/tmp/repo" {
		t.Errorf("expected repoPath=/tmp/repo, got %s", config.RepoPath)
	}
	if !config.DeleteBranch {
		t.Error("expected DeleteBranch=true")
	}
}
