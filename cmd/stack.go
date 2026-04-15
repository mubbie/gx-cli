package cmd

import (
	"fmt"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "stack [new-branch] [parent-branch]",
		Short: "Create a new branch on top of a parent branch",
		Args:  cobra.MaximumNArgs(2),
		RunE:  runStack,
	})
}

func runStack(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	if len(args) < 2 {
		ui.PrintError("Usage: gx stack <new-branch> <parent-branch>")
		return nil
	}

	newBranch := args[0]
	parentBranch := args[1]

	if git.BranchExists(newBranch) {
		ui.PrintError(fmt.Sprintf("Branch '%s' already exists.", newBranch))
		return nil
	}
	if !git.BranchExists(parentBranch) {
		ui.PrintError(fmt.Sprintf("Parent branch '%s' does not exist.", parentBranch))
		return nil
	}
	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes. They will carry over to the new branch.")
	}

	if _, err := git.Run("checkout", "-b", newBranch, parentBranch); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create branch: %s", err))
		return nil
	}

	if err := stack.RecordRelationship(newBranch, parentBranch); err != nil {
		ui.PrintWarning(fmt.Sprintf("Branch created but failed to save relationship: %s", err))
		return nil
	}

	fmt.Println()
	ui.PrintSuccess(fmt.Sprintf("Created %s on top of %s", newBranch, parentBranch))
	fmt.Println("  Relationship saved to stack config.")
	return nil
}
