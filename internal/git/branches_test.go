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
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("checkout", "-b", "feature/auth")
	run("checkout", "-b", "feature/search")
	run("checkout", "main")

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
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("checkout", "-b", "feature/done")
	if err := os.WriteFile(filepath.Join(dir, "done.txt"), []byte("done"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "done.txt")
	run("commit", "-m", "done")
	run("checkout", "main")
	run("merge", "feature/done")

	merged := MergedBranches("main")
	if !merged["feature/done"] {
		t.Error("feature/done should be merged")
	}
}
