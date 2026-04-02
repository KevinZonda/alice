package campaignrepo

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitTaskWorktree_RejectsSharedOccupiedBranch(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	initGitRepo(t, sourceRoot)
	runGitOrFail(t, sourceRoot, "checkout", "-b", "rCM")
	baseCommit := gitHeadCommit(t, sourceRoot)

	worktreePath := filepath.Join(root, ".worktrees", "repo-a", "t001")
	err := ensureGitTaskWorktree(sourceRoot, worktreePath, "rCM", baseCommit)
	if err == nil {
		t.Fatal("expected shared occupied branch to be rejected")
	}
	if !strings.Contains(err.Error(), "task-private branch") {
		t.Fatalf("expected task-private branch guidance, got %v", err)
	}
	if !strings.Contains(err.Error(), sourceRoot) {
		t.Fatalf("expected error to mention occupied worktree path, got %v", err)
	}
}

func TestEnsureGitTaskWorktree_AutoHealsDetachedTaskBranch(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	initGitRepo(t, sourceRoot)
	branchName := "codearmy/camp_demo/t001/repo-a"
	runGitOrFail(t, sourceRoot, "branch", branchName)
	baseCommit := gitHeadCommit(t, sourceRoot)

	worktreePath := filepath.Join(root, ".worktrees", "repo-a", "t001")
	if err := ensureGitTaskWorktree(sourceRoot, worktreePath, branchName, baseCommit); err != nil {
		t.Fatalf("create task worktree failed: %v", err)
	}
	runGitOrFail(t, worktreePath, "checkout", "--detach", baseCommit)
	if _, err := gitCurrentBranch(worktreePath); err == nil {
		t.Fatal("expected detached task worktree to have no current branch")
	}

	if err := ensureGitTaskWorktree(sourceRoot, worktreePath, branchName, baseCommit); err != nil {
		t.Fatalf("expected detached task worktree to auto-heal, got %v", err)
	}
	currentBranch, err := gitCurrentBranch(worktreePath)
	if err != nil {
		t.Fatalf("read repaired branch failed: %v", err)
	}
	if currentBranch != branchName {
		t.Fatalf("expected repaired branch %s, got %s", branchName, currentBranch)
	}
}
