package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// chdir changes to dir and registers a cleanup to restore the original directory.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")
	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("# Test\n"), 0644)
	run("add", "README.md")
	run("commit", "-m", "Initial commit")
	return dir
}

func TestEnsureRepo(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	if err := EnsureRepo(); err != nil {
		t.Errorf("EnsureRepo failed in git repo: %v", err)
	}
}

func TestEnsureRepoNotGit(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := EnsureRepo(); err == nil {
		t.Error("EnsureRepo should fail outside git repo")
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	branch, err := CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}
	if branch != "main" {
		t.Errorf("expected 'main', got '%s'", branch)
	}
}

func TestBranchExists(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	if !BranchExists("main") {
		t.Error("main should exist")
	}
	if BranchExists("nonexistent") {
		t.Error("nonexistent should not exist")
	}
}

func TestIsClean(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	if !IsClean() {
		t.Error("fresh repo should be clean")
	}
	os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644)
	if IsClean() {
		t.Error("repo with untracked file should not be clean")
	}
}

func TestRun(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	out, err := Run("rev-parse", "--is-inside-work-tree")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if out != "true" {
		t.Errorf("expected 'true', got '%s'", out)
	}
}

func TestRunError(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	_, err := Run("checkout", "nonexistent-branch")
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
	gitErr, ok := err.(*Error)
	if !ok {
		t.Error("expected *Error type")
	}
	if gitErr.Stderr == "" {
		t.Error("expected non-empty stderr")
	}
}

func TestLastCommit(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	hash, short, msg, author, date := LastCommit()
	if hash == "" || short == "" {
		t.Error("hash and short should not be empty")
	}
	if msg != "Initial commit" {
		t.Errorf("expected 'Initial commit', got '%s'", msg)
	}
	if author != "Test User" {
		t.Errorf("expected 'Test User', got '%s'", author)
	}
	if date == "" {
		t.Error("date should not be empty")
	}
}

func TestTimeAgo(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"not-a-date", "not-a-date"},
	}
	for _, tt := range tests {
		got := TimeAgo(tt.input)
		if got != tt.want {
			t.Errorf("TimeAgo(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
	// Valid date should not return the input unchanged
	got := TimeAgo("2020-01-01T00:00:00Z")
	if got == "2020-01-01T00:00:00Z" {
		t.Error("TimeAgo should return relative time, not the input")
	}
}

func TestFileExistsDirExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	os.WriteFile(file, []byte("hi"), 0644)

	if !FileExists(file) {
		t.Error("file should exist")
	}
	if FileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("nonexistent file should not exist")
	}
	if !DirExists(dir) {
		t.Error("dir should exist")
	}
	if DirExists(file) {
		t.Error("file should not be detected as dir")
	}
}

func TestStashCount(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)
	count := StashCount()
	if count != 0 {
		t.Errorf("expected 0 stashes, got %d", count)
	}
}
