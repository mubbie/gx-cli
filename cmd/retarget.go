package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "retarget [branch] <new-target>",
		Short: "Rebase a branch onto a new base and update stack config",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runRetarget,
	}
	cmd.Flags().Bool("dry-run", false, "Show what would happen")
	rootCmd.AddCommand(cmd)
}

func runRetarget(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	var branch, newTarget string
	if len(args) == 2 {
		branch = args[0]
		newTarget = args[1]
	} else if len(args) == 1 {
		var err error
		branch, err = git.CurrentBranch()
		if err != nil {
			ui.PrintError(err.Error())
			return nil
		}
		newTarget = args[0]
	}

	if !git.BranchExists(branch) {
		ui.PrintError(fmt.Sprintf("Branch '%s' does not exist.", branch))
		return nil
	}
	if !git.BranchExists(newTarget) {
		ui.PrintError(fmt.Sprintf("Target branch '%s' does not exist.", newTarget))
		return nil
	}

	// Find old parent
	oldParent := stack.Parent(branch)
	oldParentRef := stack.ParentHead(branch)
	if oldParentRef == "" {
		oldParentRef = oldParent
	}
	if oldParentRef == "" {
		mb, err := git.MergeBase(branch, newTarget)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Cannot determine old parent for %s.", branch))
			return nil
		}
		oldParentRef = mb
		oldParent = mb
		ui.PrintWarning(fmt.Sprintf("No saved parent for %s. Using merge-base.", branch))
	}

	if oldParent == newTarget {
		ui.PrintInfo(fmt.Sprintf("%s is already based on %s. Nothing to do.", branch, newTarget))
		return nil
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

	fmt.Println()
	fmt.Printf("%s\n\n", ui.BoldStyle.Render(fmt.Sprintf("Retargeting %s onto %s...", branch, newTarget)))
	fmt.Printf("  Old parent: %s\n", oldParent)
	fmt.Printf("  New parent: %s\n\n", newTarget)

	remoteTarget := newTarget
	if git.RemoteBranchExists(newTarget) {
		remoteTarget = "origin/" + newTarget
	}

	if dryRun {
		ui.PrintDryRun([]string{
			fmt.Sprintf("Would retarget %s:", branch),
			fmt.Sprintf("  Current parent: %s", oldParent),
			fmt.Sprintf("  New parent:     %s", newTarget),
			"",
			"Steps:",
			"  1. Fetch remote",
			fmt.Sprintf("  2. Run: git rebase --onto %s %s %s", remoteTarget, oldParentRef, branch),
			"  3. Push with --force-with-lease",
			"  4. Update stack config",
			"  5. Attempt PR retarget via gh CLI",
		})
		return nil
	}

	if !ui.Confirm(fmt.Sprintf("Retarget %s onto %s?", branch, newTarget)) {
		ui.PrintInfo("Cancelled.")
		return nil
	}

	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes. Stash or commit them before retargeting.")
		return nil
	}

	// Fetch
	fmt.Println("  Fetching latest from remote...")
	git.Run("fetch", "origin")

	// Rebase
	fmt.Printf("  Rebasing %s onto %s (using --onto)...\n", branch, remoteTarget)
	c := exec.Command("git", "rebase", "--onto", remoteTarget, oldParentRef, branch)
	_, err := c.CombinedOutput()
	if err != nil {
		ui.PrintError("Rebase conflict encountered")
		fmt.Println()
		conflicts := git.RunUnchecked("diff", "--name-only", "--diff-filter=U")
		if conflicts != "" {
			fmt.Println("  Conflicting files:")
			for _, f := range strings.Split(conflicts, "\n") {
				if f != "" {
					fmt.Printf("    %s\n", f)
				}
			}
			fmt.Println()
		}
		fmt.Println("  To resolve:")
		fmt.Println("    1. Fix the conflicts")
		fmt.Println("    2. Run: git add . && git rebase --continue")
		fmt.Printf("    3. Run: gx retarget %s %s\n", branch, newTarget)
		return nil
	}
	// Push
	if _, err := git.Run("push", "--force-with-lease", "origin", branch); err != nil {
		ui.PrintWarning(fmt.Sprintf("Rebased but failed to push: %s", err))
	} else {
		ui.PrintSuccess(fmt.Sprintf("Rebased and pushed %s", branch))
	}

	// Update config
	stack.RecordRelationship(branch, newTarget)
	fmt.Printf("  Stack config updated: %s -> %s (was: %s)\n", branch, newTarget, oldParent)

	// Try auto-retarget PR via gh
	ghCmd := exec.Command("gh", "pr", "edit", branch, "--base", newTarget)
	if err := ghCmd.Run(); err == nil {
		ui.PrintSuccess(fmt.Sprintf("PR for %s automatically retargeted to %s", branch, newTarget))
	} else {
		ui.PrintWarning(fmt.Sprintf("Remember to retarget the PR for %s to %s.", branch, newTarget))
	}
	return nil
}
