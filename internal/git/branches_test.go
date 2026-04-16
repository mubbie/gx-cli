package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveBranches(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)

	// Create some branches
	exec.Command("git", "-C", dir, "checkout", "-b", "feature/auth").Run()
	exec.Command("git", "-C", dir, "checkout", "-b", "feature/search").Run()
	exec.Command("git", "-C", dir, "checkout", "main").Run()

	// Exact match
	result := ResolveBranches("feature/auth")
	if len(result) != 1 || result[0] != "feature/auth" {
		t.Errorf("exact match failed: %v", result)
	}

	// Glob match
	result = ResolveBranches("feature/*")
	if len(result) != 2 {
		t.Errorf("glob match expected 2, got %d: %v", len(result), result)
	}

	// No match
	result = ResolveBranches("nonexistent")
	if len(result) != 0 {
		t.Errorf("expected no match, got %v", result)
	}
}

func TestMergedBranches(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)

	// Create and merge a branch
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Run()
	}
	run("checkout", "-b", "feature/done")
	os.WriteFile(filepath.Join(dir, "done.txt"), []byte("done"), 0644)
	run("add", "done.txt")
	run("commit", "-m", "done")
	run("checkout", "main")
	run("merge", "feature/done")

	merged := MergedBranches("main")
	if !merged["feature/done"] {
		t.Error("feature/done should be merged")
	}
}
