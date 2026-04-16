package cmd

import (
	"fmt"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
)

// handleRebaseConflict prints conflict info and resolution steps.
func handleRebaseConflict(remaining []string) {
	ui.PrintError("Rebase conflict encountered")
	fmt.Println()
	conflicts := git.RunUnchecked("diff", "--name-only", "--diff-filter=U")
	if conflicts != "" {
		fmt.Println("  Conflicting files:")
		for _, f := range strings.Split(conflicts, "\n") {
			if f = strings.TrimSpace(f); f != "" {
				fmt.Printf("    %s\n", f)
			}
		}
		fmt.Println()
	}
	fmt.Println("  To resolve:")
	fmt.Println("    1. Fix the conflicts in the listed files")
	fmt.Println("    2. Run: git add . && git rebase --continue")
	if len(remaining) > 1 {
		fmt.Printf("    3. Run: gx sync %s\n", strings.Join(remaining, " "))
	}
}

// requireBranch checks we're in a repo and returns current branch.
func requireBranch() (string, error) {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return "", err
	}
	branch, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return "", err
	}
	return branch, nil
}

// warnIfDirty prints a warning if the working tree is dirty.
func warnIfDirty() {
	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes. They may conflict with the target branch.")
	}
}

// switchTo checks out a branch and prints the result.
func switchTo(target string) error {
	if _, err := git.Run("checkout", target); err != nil {
		ui.PrintError("Failed to switch: " + err.Error())
		return err
	}
	return nil
}
