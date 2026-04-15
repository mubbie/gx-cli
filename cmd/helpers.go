package cmd

import (
	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
)

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
