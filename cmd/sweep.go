package cmd

import (
	"fmt"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Clean up merged branches, prune stale refs, and tidy up",
		RunE:  runSweep,
	}
	cmd.Flags().Bool("dry-run", false, "Show what would be cleaned")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompts")
	rootCmd.AddCommand(cmd)
}

func runSweep(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	head := git.HeadBranch()
	current, _ := git.CurrentBranch()

	fmt.Println()
	fmt.Println(ui.BoldStyle.Render("Scanning for cleanup opportunities..."))
	fmt.Println()

	// Merged branches
	mergedOut := git.RunUnchecked("branch", "--merged", head)
	var merged []string
	for _, line := range strings.Split(mergedOut, "\n") {
		name := strings.TrimSpace(strings.TrimLeft(line, "* "))
		if name != "" && name != head && name != current {
			merged = append(merged, name)
		}
	}

	// Squash-merged detection
	allOut := git.RunUnchecked("branch", "--format=%(refname:short)")
	mergedSet := map[string]bool{}
	for _, b := range merged {
		mergedSet[b] = true
	}
	var squashMerged []string
	for _, line := range strings.Split(allOut, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == head || name == current || mergedSet[name] {
			continue
		}
		cherry := git.RunUnchecked("cherry", head, name)
		if cherry == "" {
			continue
		}
		allMerged := true
		for _, cl := range strings.Split(cherry, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(cl), "-") {
				allMerged = false
				break
			}
		}
		if allMerged {
			squashMerged = append(squashMerged, name)
		}
	}

	// Stale refs
	pruneOut := git.RunUnchecked("remote", "prune", "origin", "--dry-run")
	var staleRefs []string
	for _, line := range strings.Split(pruneOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "origin/") {
			for _, word := range strings.Fields(line) {
				if strings.Contains(word, "origin/") {
					staleRefs = append(staleRefs, word)
					break
				}
			}
		}
	}

	if len(merged) == 0 && len(squashMerged) == 0 && len(staleRefs) == 0 {
		ui.PrintSuccess("Nothing to clean up. Repository is tidy!")
		return nil
	}

	if len(merged) > 0 {
		fmt.Println(ui.BoldStyle.Render("Merged branches (safe to delete):"))
		for _, b := range merged {
			fmt.Printf("  %s\n", ui.BranchStyle.Render(b))
		}
		fmt.Println()
	}
	if len(squashMerged) > 0 {
		fmt.Println(ui.BoldStyle.Render("Likely squash-merged branches:"))
		for _, b := range squashMerged {
			fmt.Printf("  %s\n", ui.BranchStyle.Render(b))
		}
		fmt.Println()
	}
	if len(staleRefs) > 0 {
		fmt.Println(ui.BoldStyle.Render("Stale remote tracking refs:"))
		for _, r := range staleRefs {
			fmt.Printf("  %s\n", ui.DimStyle.Render(r))
		}
		fmt.Println()
	}

	fmt.Printf("%s %s merged, %s likely squash-merged, %s stale refs\n\n",
		ui.BoldStyle.Render("Summary:"),
		ui.BoldStyle.Render(fmt.Sprintf("%d", len(merged))),
		ui.BoldStyle.Render(fmt.Sprintf("%d", len(squashMerged))),
		ui.BoldStyle.Render(fmt.Sprintf("%d", len(staleRefs))))

	if dryRun {
		fmt.Printf("%s would delete %d branches, %d stale refs\n", ui.WarningStyle.Render("DRY RUN"), len(merged)+len(squashMerged), len(staleRefs))
		return nil
	}

	if len(merged) > 0 && (yes || ui.Confirm("Delete merged branches?")) {
		for _, b := range merged {
			if _, err := git.Run("branch", "-d", b); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to delete %s: %s", b, err))
			} else {
				ui.PrintSuccess(fmt.Sprintf("Deleted %s", b))
			}
		}
	}

	if len(squashMerged) > 0 && (yes || ui.Confirm("Delete likely squash-merged branches?")) {
		for _, b := range squashMerged {
			if _, err := git.Run("branch", "-D", b); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to delete %s: %s", b, err))
			} else {
				ui.PrintSuccess(fmt.Sprintf("Deleted %s", b))
			}
		}
	}

	if len(staleRefs) > 0 && (yes || ui.Confirm("Prune stale remote tracking refs?")) {
		if _, err := git.Run("remote", "prune", "origin"); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to prune: %s", err))
		} else {
			ui.PrintSuccess(fmt.Sprintf("Pruned %d stale remote tracking refs", len(staleRefs)))
		}
	}

	stack.CleanDeleted()
	fmt.Println()
	ui.PrintSuccess("Cleanup complete.")
	return nil
}
