package git

import (
	"path/filepath"
	"strings"
)

// ResolveBranches resolves a branch name or glob pattern to matching branch names.
func ResolveBranches(pattern string) []string {
	out := RunUnchecked("branch", "--format=%(refname:short)")
	seen := make(map[string]bool)
	var all []string
	for _, b := range strings.Split(out, "\n") {
		b = strings.TrimSpace(b)
		if b != "" {
			all = append(all, b)
			seen[b] = true
		}
	}

	// Also check remote-only branches
	remoteOut := RunUnchecked("branch", "-r", "--format=%(refname:short)")
	for _, b := range strings.Split(remoteOut, "\n") {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		// Strip "origin/" prefix
		name := strings.TrimPrefix(b, "origin/")
		if name == "HEAD" || seen[name] {
			continue
		}
		all = append(all, name)
		seen[name] = true
	}

	// Exact match
	for _, b := range all {
		if b == pattern {
			return []string{b}
		}
	}

	// Glob match
	var matches []string
	for _, b := range all {
		if matched, _ := filepath.Match(pattern, b); matched {
			matches = append(matches, b)
		}
	}
	return matches
}

// MergedBranches returns a set of branches merged into the given branch.
func MergedBranches(into string) map[string]bool {
	out := RunUnchecked("branch", "--merged", into)
	set := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(strings.TrimLeft(line, "* "))
		if name != "" {
			set[name] = true
		}
	}
	return set
}
