package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "oops",
		Short: "Quick-fix the last commit. Amend message, add forgotten files, or both.",
		RunE:  runOops,
	}
	cmd.Flags().StringP("message", "m", "", "New commit message")
	cmd.Flags().StringSlice("add", nil, "File(s) to add to the last commit")
	cmd.Flags().Bool("dry-run", false, "Show what would change")
	cmd.Flags().Bool("force", false, "Allow amending even if already pushed")
	rootCmd.AddCommand(cmd)
}

func runOops(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	_, shortHash, lastMsg, _, _ := git.LastCommit()
	if shortHash == "" {
		ui.PrintError("No commits yet.")
		return nil
	}

	force, _ := cmd.Flags().GetBool("force")
	if git.IsCommitPushed() && !force {
		ui.PrintError("The last commit has already been pushed to remote.\n  Amending it would rewrite shared history.\n  Use --force to override (dangerous).")
		return nil
	}

	message, _ := cmd.Flags().GetString("message")
	addFiles, _ := cmd.Flags().GetStringSlice("add")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// No flags: open editor
	if message == "" && len(addFiles) == 0 {
		if dryRun {
			ui.PrintDryRun([]string{"Would open editor to amend commit message."})
			return nil
		}
		fmt.Printf("Last commit: \"%s\" (%s)\n", lastMsg, shortHash)
		if !ui.Confirm("Open editor to amend commit message?") {
			ui.PrintInfo("Cancelled.")
			return nil
		}
		c := exec.Command("git", "commit", "--amend")
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to amend: %s", err))
			return nil
		}
		ui.PrintSuccess("Commit message amended.")
		return nil
	}

	fmt.Printf("\nLast commit: \"%s\" (%s)\n\n", lastMsg, shortHash)

	var dryActions []string

	if len(addFiles) > 0 {
		fmt.Println("  Adding to last commit:")
		for _, f := range addFiles {
			if f != "." {
				if _, err := os.Stat(f); os.IsNotExist(err) {
					ui.PrintError(fmt.Sprintf("File not found: %s", f))
					return nil
				}
			}
			fmt.Printf("    + %s\n", f)
			dryActions = append(dryActions, fmt.Sprintf("Would add %s to last commit", f))
		}
		fmt.Println()
	}

	if message != "" {
		fmt.Printf("  Amending message:\n    Before: \"%s\"\n    After:  \"%s\"\n\n", lastMsg, message)
		dryActions = append(dryActions, fmt.Sprintf("Would change message to \"%s\"", message))
	}

	if dryRun {
		ui.PrintDryRun(dryActions)
		return nil
	}

	if !ui.Confirm("Proceed?") {
		ui.PrintInfo("Cancelled.")
		return nil
	}

	for _, f := range addFiles {
		if _, err := git.Run("add", f); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to add %s: %s", f, err))
			return nil
		}
	}

	amendArgs := []string{"commit", "--amend"}
	if message != "" {
		amendArgs = append(amendArgs, "-m", message)
	} else {
		amendArgs = append(amendArgs, "--no-edit")
	}
	if _, err := git.Run(amendArgs...); err != nil {
		ui.PrintError(fmt.Sprintf("Failed: %s", err))
		return nil
	}

	fmt.Println()
	if message != "" && len(addFiles) > 0 {
		ui.PrintSuccess("File added and commit message amended.")
	} else if message != "" {
		ui.PrintSuccess("Commit message amended.")
	} else {
		ui.PrintSuccess("File added to last commit.")
	}
	return nil
}
