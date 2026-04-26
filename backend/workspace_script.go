package backend

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const scriptTimeout = 10 * time.Minute

// WorktreeLogEvent is emitted during script execution with batched log lines.
type WorktreeLogEvent struct {
	WorkspaceName string   `json:"workspaceName"`
	WorktreeName  string   `json:"worktreeName"`
	Lines         []string `json:"lines"`
	Timestamp     int64    `json:"timestamp"` // Unix milliseconds
}

// emitLogLines emits a worktree-log event with the given lines. Skipped when
// either workspace or worktree name is empty (callers that don't have a target).
func (s *WorkspaceService) emitLogLines(workspaceName, worktreeName string, lines ...string) {
	if workspaceName == "" || worktreeName == "" || len(lines) == 0 {
		return
	}
	application.Get().Event.Emit("worktree-log", WorktreeLogEvent{
		WorkspaceName: workspaceName,
		WorktreeName:  worktreeName,
		Lines:         lines,
		Timestamp:     time.Now().UnixMilli(),
	})
}

// runGitCmdTracked runs a git command in repoDir, captures combined output,
// and on failure emits the output as worktree-log so the user can open it
// from the dashboard. label is a short human description of the command
// (e.g. "rebase origin/main") used for the header line.
func (s *WorkspaceService) runGitCmdTracked(workspaceName, worktreeName, repoDir, label string, args ...string) error {
	s.emitLogLines(workspaceName, worktreeName, "$ git "+label)
	gitArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", gitArgs...) // #nosec G204
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimRight(string(out), "\n")
	if trimmed != "" {
		s.emitLogLines(workspaceName, worktreeName, strings.Split(trimmed, "\n")...)
	}
	if err != nil {
		s.emitLogLines(workspaceName, worktreeName, fmt.Sprintf("git %s failed: %s", label, err.Error()))
		if trimmed != "" {
			return fmt.Errorf("git %s failed: %s", label, trimmed)
		}
		return fmt.Errorf("git %s failed: %w", label, err)
	}
	return nil
}

func (s *WorkspaceService) runScriptTracked(workspaceName, worktreeName, script, workDir string) error {
	key := workspaceName + "/" + worktreeName

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script) // #nosec G204
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"GROVE_WORKTREE_PATH="+workDir,
		"GROVE_WORKTREE_NAME="+filepath.Base(workDir),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	s.mu.Lock()
	s.runningCmds[key] = cmd
	s.mu.Unlock()

	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		delete(s.runningCmds, key)
		s.mu.Unlock()
		return err
	}

	// Emit a synthetic first log line immediately so the button appears instantly
	application.Get().Event.Emit("worktree-log", WorktreeLogEvent{
		WorkspaceName: workspaceName,
		WorktreeName:  worktreeName,
		Lines:         []string{"Running script..."},
		Timestamp:     time.Now().UnixMilli(),
	})

	// Stream output lines via batched events
	var pending []string
	var pendingMu sync.Mutex

	flush := func() {
		pendingMu.Lock()
		if len(pending) == 0 {
			pendingMu.Unlock()
			return
		}
		batch := pending
		pending = nil
		pendingMu.Unlock()
		application.Get().Event.Emit("worktree-log", WorktreeLogEvent{
			WorkspaceName: workspaceName,
			WorktreeName:  worktreeName,
			Lines:         batch,
			Timestamp:     time.Now().UnixMilli(),
		})
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	stopTicker := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopTicker:
				return
			case <-ticker.C:
				flush()
			}
		}
	}()

	var wg sync.WaitGroup
	scanPipe := func(pipe io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			pendingMu.Lock()
			pending = append(pending, line)
			pendingMu.Unlock()
		}
	}

	wg.Add(2)
	go scanPipe(stdout)
	go scanPipe(stderr)
	wg.Wait()

	cmdErr := cmd.Wait()

	ticker.Stop()
	close(stopTicker)
	flush() // final flush

	s.mu.Lock()
	delete(s.runningCmds, key)
	s.mu.Unlock()

	if cmdErr != nil {
		log.Printf("[grove] script failed in %s: %s", workDir, cmdErr.Error())
	}

	return cmdErr
}
