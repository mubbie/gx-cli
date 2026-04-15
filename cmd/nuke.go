package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "nuke <branch-or-pattern>",
		Short: "Delete branches with confidence. Removes local, remote, and tracking refs.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runNuke,
	}
	cmd.Flags().Bool("local", false, "Only delete local branch")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation")
	cmd.Flags().Bool("orphans", false, "Delete all orphaned branches from the stack")
	rootCmd.AddCommand(cmd)
}

func runNuke(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	orphans, _ := cmd.Flags().GetBool("orphans")
	if orphans {
		return nukeOrphans(cmd)
	}

	if len(args) == 0 {
		ui.PrintError("Branch name or pattern required. Usage: gx nuke <branch>")
		return nil
	}

	pattern := args[0]
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	localOnly, _ := cmd.Flags().GetBool("local")
	yes, _ := cmd.Flags().GetBool("yes")

	// Resolve branches
	branches := resolveBranches(pattern)
	if len(branches) == 0 {
		ui.PrintError(fmt.Sprintf("No branches matching '%s' found.", pattern))
		return nil
	}

	current, _ := git.CurrentBranch()
	head := git.HeadBranch()

	for _, b := range branches {
		if b == current {
			ui.PrintError(fmt.Sprintf("Cannot nuke '%s': it's the current branch. Switch first.", b))
			return nil
		}
		if b == head {
			ui.PrintError(fmt.Sprintf("Cannot nuke '%s': it's the HEAD branch. Blocked for safety.", b))
			return nil
		}
	}

	mergedSet := mergedBranches(head)

	if dryRun {
		var actions []string
		for _, b := range branches {
			local := git.BranchExists(b)
			remote := git.RemoteBranchExists(b)
			actions = append(actions, fmt.Sprintf("  %s  Local: %v  Remote: %v  Merged: %v", ui.BranchStyle.Render(b), local, remote, mergedSet[b]))
		}
		actions = append(actions, "", fmt.Sprintf("Would delete: %s", ui.WarningStyle.Render(fmt.Sprintf("%d branches", len(branches)))))
		ui.PrintDryRun(actions)
		return nil
	}

	// Show info and warn about stack children
	hasUnmerged := false
	for _, b := range branches {
		if !mergedSet[b] {
			hasUnmerged = true
			fmt.Printf("\n  %s is %s into %s.\n", ui.BranchStyle.Render(b), ui.ErrorStyle.Bold(true).Render("NOT merged"), ui.BranchStyle.Render(head))
		}
		children := stack.Children(b)
		if len(children) > 0 {
			ui.PrintWarning(fmt.Sprintf("%s has %d dependent branch(es): %s. They will become orphaned.", b, len(children), strings.Join(children, ", ")))
		}
	}

	if !yes || hasUnmerged {
		if !ui.Confirm("Proceed with deletion?") {
			ui.PrintInfo("Cancelled.")
			return nil
		}
	}

	for _, b := range branches {
		flag := "-d"
		if !mergedSet[b] {
			flag = "-D"
		}
		if git.BranchExists(b) {
			if _, err := git.Run("branch", flag, b); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to delete local branch %s: %s", b, err))
			} else {
				ui.PrintSuccess(fmt.Sprintf("Deleted local branch %s", b))
			}
		}
		if !localOnly && git.RemoteBranchExists(b) {
			git.Run("branch", "-dr", "origin/"+b)
			ui.PrintSuccess(fmt.Sprintf("Deleted remote tracking ref origin/%s", b))
			if _, err := git.Run("push", "origin", "--delete", b); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to delete remote branch %s: %s", b, err))
			} else {
				ui.PrintSuccess(fmt.Sprintf("Deleted remote branch origin/%s", b))
			}
		}
		stack.RemoveBranch(b)
	}
	return nil
}

func nukeOrphans(cmd *cobra.Command) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")

	tree := stack.BuildTree()
	if len(tree.Orphans) == 0 {
		ui.PrintInfo("No orphaned branches found.")
		return nil
	}

	fmt.Println()
	fmt.Println(ui.BoldStyle.Render("Orphaned branches:"))
	for _, o := range tree.Orphans {
		fmt.Println("  " + ui.BranchStyle.Render(o.Name))
	}
	fmt.Println()

	if dryRun {
		fmt.Printf("%s would delete %d orphaned branches.\n", ui.WarningStyle.Render("DRY RUN"), len(tree.Orphans))
		return nil
	}

	if !yes {
		if !ui.Confirm(fmt.Sprintf("Delete %d orphaned branches?", len(tree.Orphans))) {
			ui.PrintInfo("Cancelled.")
			return nil
		}
	}

	for _, o := range tree.Orphans {
		if _, err := git.Run("branch", "-D", o.Name); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to delete %s: %s", o.Name, err))
		} else {
			ui.PrintSuccess(fmt.Sprintf("Deleted %s", o.Name))
			stack.RemoveBranch(o.Name)
		}
	}
	return nil
}

func resolveBranches(pattern string) []string {
	out := git.RunUnchecked("branch", "--format=%(refname:short)")
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
	remoteOut := git.RunUnchecked("branch", "-r", "--format=%(refname:short)")
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

func mergedBranches(into string) map[string]bool {
	out := git.RunUnchecked("branch", "--merged", into)
	set := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(strings.TrimLeft(line, "* "))
		if name != "" {
			set[name] = true
		}
	}
	return set
}
