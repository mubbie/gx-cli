// Package git provides helpers for executing git commands.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Git state file paths (relative to .git/).
const (
	MergeHeadPath      = "MERGE_HEAD"
	RebaseMergePath    = "rebase-merge"
	RebaseApplyPath    = "rebase-apply"
	CherryPickHeadPath = "CHERRY_PICK_HEAD"
)

// Error is returned when a git command fails.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	if e.Stderr != "" {
		return e.Stderr
	}
	return fmt.Sprintf("git %s failed: %v", strings.Join(e.Args, " "), e.Err)
}

// Run executes a git command and returns trimmed stdout.
func Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunCombined executes a git command and returns combined stdout+stderr.
// Used for rebase operations that need to capture conflict output.
func RunCombined(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, &Error{Args: args, Stderr: output, Err: err}
	}
	return output, nil
}

// RunUnchecked executes a git command and returns stdout even on failure.
func RunUnchecked(args ...string) string {
	out, _ := Run(args...)
	return out
}

// Lines executes a git command and returns stdout split into lines.
func Lines(args ...string) ([]string, error) {
	output, err := Run(args...)
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}

// EnsureRepo checks that we are inside a git repository.
func EnsureRepo() error {
	_, err := Run("rev-parse", "--is-inside-work-tree")
	if err != nil {
		return fmt.Errorf("not a git repository. Run this from inside a git project")
	}
	return nil
}

// CurrentBranch returns the current branch name.
func CurrentBranch() (string, error) {
	branch, err := Run("symbolic-ref", "--short", "HEAD")
	if err != nil || branch == "" {
		return "", fmt.Errorf("HEAD is detached. Not on any branch")
	}
	return branch, nil
}

var (
	headBranchOnce  sync.Once
	headBranchCache string
)

// HeadBranch detects the repo's main/master/develop branch.
// The result is cached for the lifetime of the process.
func HeadBranch() string {
	headBranchOnce.Do(func() {
		headBranchCache = detectHeadBranch()
	})
	return headBranchCache
}

func detectHeadBranch() string {
	// Try local symbolic ref for origin HEAD (no network call)
	out := RunUnchecked("symbolic-ref", "refs/remotes/origin/HEAD")
	if out != "" {
		// Output is e.g. "refs/remotes/origin/main"
		name := strings.TrimPrefix(out, "refs/remotes/origin/")
		if name != "" && BranchExists(name) {
			return name
		}
	}

	for _, name := range []string{"main", "master", "develop"} {
		if BranchExists(name) {
			return name
		}
	}

	// Last resort: use whatever branch HEAD is on
	if branch, err := CurrentBranch(); err == nil && branch != "" {
		return branch
	}
	return "main"
}

// FileExists checks if a file (not directory) exists at path.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// DirExists checks if a directory exists at path.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// RepoRoot returns the root directory of the git repo.
func RepoRoot() (string, error) {
	return Run("rev-parse", "--show-toplevel")
}

// BranchExists checks if a local branch exists.
func BranchExists(name string) bool {
	_, err := Run("rev-parse", "--verify", name)
	return err == nil
}

// RemoteBranchExists checks if a remote branch exists.
func RemoteBranchExists(name string) bool {
	out := RunUnchecked("branch", "-r", "--list", "origin/"+name)
	return strings.TrimSpace(out) != ""
}

// IsClean returns true if the working tree has no uncommitted changes.
func IsClean() bool {
	out := RunUnchecked("status", "--porcelain")
	return out == ""
}

// LastCommit returns info about the HEAD commit.
func LastCommit() (hash, shortHash, message, author, date string) {
	out := RunUnchecked("log", "-1", "--format=%H%n%h%n%s%n%an%n%aI")
	lines := strings.Split(out, "\n")
	if len(lines) >= 5 {
		return lines[0], lines[1], lines[2], lines[3], lines[4]
	}
	return "", "", "", "", ""
}

// StashCount returns the number of stash entries.
func StashCount() int {
	out := RunUnchecked("stash", "list")
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

// MergeBase returns the merge base commit of two refs.
func MergeBase(a, b string) (string, error) {
	return Run("merge-base", a, b)
}

// AheadBehind returns the ahead/behind counts between two refs.
func AheadBehind(a, b string) (ahead, behind int) {
	out := RunUnchecked("rev-list", "--left-right", "--count", a+"..."+b)
	parts := strings.Fields(out)
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &ahead)
		fmt.Sscanf(parts[1], "%d", &behind)
	}
	return
}

// IsCommitPushed checks if HEAD exists on the remote tracking branch.
func IsCommitPushed() bool {
	branch, err := CurrentBranch()
	if err != nil {
		return false
	}
	tracking := RunUnchecked("rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if tracking == "" {
		return false
	}
	out := RunUnchecked("branch", "-r", "--contains", "HEAD")
	return strings.TrimSpace(out) != ""
}

// TimeAgo converts an ISO date string to a human-readable relative time.
func TimeAgo(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// Try alternate format
		t, err = time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err != nil {
			return dateStr
		}
	}
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		m := int(diff.Minutes())
		return fmt.Sprintf("%d min ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case diff < 30*24*time.Hour:
		d := int(diff.Hours() / 24)
		if d == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", d)
	default:
		m := int(diff.Hours() / 24 / 30)
		if m <= 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", m)
	}
}
