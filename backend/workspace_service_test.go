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

func TestValidateBranchName(t *testing.T) {
	valid := []string{
		"my-branch",
		"feature/my-branch",
		"feature/sub/deep",
		"a",
		"abc-def_ghi",
		"123",
		"origin/main",
	}
	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if err := validateBranchName(name); err != nil {
				t.Errorf("validateBranchName(%q) = %v, want nil", name, err)
			}
		})
	}

	invalid := []struct {
		name    string
		wantMsg string
	}{
		{"", "cannot be empty"},
		{".", "invalid branch name"},
		{"..", "invalid branch name"},
		{"has space", "invalid characters"},
		{"has..dots", "'..'"},
		{"double//slash", "consecutive slashes"},
		{"/leading-slash", "invalid characters"},
		{"trailing-slash/", "invalid characters"},
		{"café", "invalid characters"},
	}
	for _, tt := range invalid {
		label := tt.name
		if label == "" {
			label = "(empty)"
		}
		t.Run("invalid/"+label, func(t *testing.T) {
			err := validateBranchName(tt.name)
			if err == nil {
				t.Fatalf("validateBranchName(%q) = nil, want error containing %q", tt.name, tt.wantMsg)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("validateBranchName(%q) = %q, want error containing %q", tt.name, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestFetchRemoteIfNeeded(t *testing.T) {
	// Create a "remote" repo and clone it
	remoteDir := initTestRepo(t)
	cloneDir := t.TempDir()
	cmd := exec.Command("git", "clone", remoteDir, cloneDir) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %s: %v", out, err)
	}

	// Local branch — should not error and not fetch
	if err := fetchRemoteIfNeeded(cloneDir, "main"); err != nil {
		t.Errorf("local branch: unexpected error: %v", err)
	}

	// Remote ref — should fetch successfully
	if err := fetchRemoteIfNeeded(cloneDir, "origin/main"); err != nil {
		t.Errorf("remote ref: unexpected error: %v", err)
	}

	// Unknown remote — should be a no-op
	if err := fetchRemoteIfNeeded(cloneDir, "nonexistent/branch"); err != nil {
		t.Errorf("unknown remote: unexpected error: %v", err)
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
	if branch != unknownBranch {
		t.Errorf("getGitBranch(invalid) = %q, want %q", branch, unknownBranch)
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

func TestListBranches(t *testing.T) {
	repoDir := initTestRepo(t)

	// Create a second branch
	cmd := exec.Command("git", "branch", "feature/my-thing")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch failed: %s: %v", out, err)
	}

	// Set up a workspace service pointing at a temp grove dir
	groveDir := t.TempDir()
	projectDir := filepath.Join(groveDir, "test-ws")
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		t.Fatal(err)
	}
	configData := `{"repoPath":"` + repoDir + `"}`
	if err := os.WriteFile(filepath.Join(projectDir, "config.json"), []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := &WorkspaceService{groveDir: groveDir, runningCmds: make(map[string]*exec.Cmd)}
	branches, err := svc.ListBranches("test-ws")
	if err != nil {
		t.Fatalf("ListBranches() error: %v", err)
	}

	// Should have at least main and feature/my-thing (local only, no remotes)
	branchMap := make(map[string]BranchInfo)
	for _, b := range branches {
		branchMap[b.Name] = b
	}

	if _, ok := branchMap["main"]; !ok {
		t.Error("expected 'main' branch in results")
	}
	if b, ok := branchMap["feature/my-thing"]; !ok {
		t.Error("expected 'feature/my-thing' branch in results")
	} else if b.IsRemote {
		t.Error("expected 'feature/my-thing' to be local, got remote")
	}

	// All branches should be local (no remotes in a fresh repo)
	for _, b := range branches {
		if b.IsRemote {
			t.Errorf("expected all branches to be local, got remote: %s", b.Name)
		}
	}
}

func TestListBranchesWithRemotes(t *testing.T) {
	// Create a "remote" repo and clone it to get actual remote branches
	remoteDir := initTestRepo(t)
	cloneDir := t.TempDir()

	cmd := exec.Command("git", "clone", remoteDir, cloneDir) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %s: %v", out, err)
	}

	groveDir := t.TempDir()
	projectDir := filepath.Join(groveDir, "test-ws")
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		t.Fatal(err)
	}
	configData := `{"repoPath":"` + cloneDir + `"}`
	if err := os.WriteFile(filepath.Join(projectDir, "config.json"), []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := &WorkspaceService{groveDir: groveDir, runningCmds: make(map[string]*exec.Cmd)}
	branches, err := svc.ListBranches("test-ws")
	if err != nil {
		t.Fatalf("ListBranches() error: %v", err)
	}

	hasLocal := false
	hasRemote := false
	for _, b := range branches {
		if !b.IsRemote && b.Name == "main" {
			hasLocal = true
		}
		if b.IsRemote && b.Name == "origin/main" {
			hasRemote = true
		}
	}

	if !hasLocal {
		t.Error("expected local 'main' branch")
	}
	if !hasRemote {
		t.Error("expected remote 'origin/main' branch")
	}
}

func TestListBranchesInvalidWorkspace(t *testing.T) {
	svc := &WorkspaceService{groveDir: t.TempDir(), runningCmds: make(map[string]*exec.Cmd)}
	_, err := svc.ListBranches("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
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
