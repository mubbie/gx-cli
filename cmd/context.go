package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Short:   "Repo status at a glance",
		RunE:    runContext,
	}
	rootCmd.AddCommand(cmd)
}

func runContext(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	fmt.Println()

	// Branch
	branch, err := git.CurrentBranch()
	if err != nil {
		shortHash := git.RunUnchecked("rev-parse", "--short", "HEAD")
		fmt.Printf("Branch:       (detached HEAD at %s)\n", shortHash)
		ui.PrintWarning("You are in detached HEAD state.")
	} else {
		fmt.Printf("Branch:       %s\n", branch)

		// Tracking
		tracking := git.RunUnchecked("rev-parse", "--abbrev-ref", branch+"@{upstream}")
		if tracking != "" {
			ahead, behind := git.AheadBehind(branch, tracking)
			if ahead == 0 && behind == 0 {
				fmt.Printf("Tracking:     %s (up to date)\n", tracking)
			} else {
				parts := ""
				if ahead > 0 {
					parts += fmt.Sprintf("%d ahead", ahead)
				}
				if behind > 0 {
					if parts != "" {
						parts += ", "
					}
					parts += fmt.Sprintf("%d behind", behind)
				}
				fmt.Printf("Tracking:     %s (%s)\n", tracking, parts)
			}
		}

		// vs HEAD branch
		headBranch := git.HeadBranch()
		if branch != headBranch {
			ahead, behind := git.AheadBehind(branch, headBranch)
			fmt.Printf("vs %s:      %d ahead, %d behind\n", headBranch, ahead, behind)
		}
	}

	fmt.Println()

	// Last commit
	_, shortHash, message, _, date := git.LastCommit()
	if shortHash != "" {
		fmt.Printf("Last commit:  %s \"%s\" (%s)\n", shortHash, message, git.TimeAgo(date))
	} else {
		fmt.Println("Last commit:  No commits yet")
	}

	fmt.Println()

	// Working tree
	status := git.RunUnchecked("status", "--porcelain")
	if status == "" {
		fmt.Println("Working tree: clean")
	} else {
		modified, staged, untracked := 0, 0, 0
		for _, line := range splitLines(status) {
			if len(line) < 2 {
				continue
			}
			if line[:2] == "??" {
				untracked++
			} else {
				if line[0] != ' ' && line[0] != '?' {
					staged++
				}
				if line[1] != ' ' && line[1] != '?' {
					modified++
				}
			}
		}
		fmt.Println("Working tree:")
		if modified > 0 {
			fmt.Printf("  Modified:   %d file%s\n", modified, plural(modified))
		}
		if staged > 0 {
			fmt.Printf("  Staged:     %d file%s\n", staged, plural(staged))
		}
		if untracked > 0 {
			fmt.Printf("  Untracked:  %d file%s\n", untracked, plural(untracked))
		}
	}

	fmt.Println()

	// Stash
	stashCount := git.StashCount()
	if stashCount > 0 {
		fmt.Printf("Stash:        %d entr%s\n", stashCount, pluralIES(stashCount))
	} else {
		fmt.Println("Stash:        empty")
	}

	// Active operations
	root, _ := git.RepoRoot()
	if root != "" {
		if fileExists(filepath.Join(root, ".git", "MERGE_HEAD")) {
			fmt.Println()
			ui.PrintWarning("Merge in progress")
		} else if dirExists(filepath.Join(root, ".git", "rebase-merge")) || dirExists(filepath.Join(root, ".git", "rebase-apply")) {
			fmt.Println()
			ui.PrintWarning("Rebase in progress")
		} else if fileExists(filepath.Join(root, ".git", "CHERRY_PICK_HEAD")) {
			fmt.Println()
			ui.PrintWarning("Cherry-pick in progress")
		}
	}

	fmt.Println()
	return nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func pluralIES(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := []string{}
	for _, line := range splitBy(s, '\n') {
		lines = append(lines, line)
	}
	return lines
}

func splitBy(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
