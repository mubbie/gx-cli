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
		Use:   "sync [branches...]",
		Short: "Rebase and push a chain of stacked branches in sequence",
		RunE:  runSync,
	}
	cmd.Flags().Bool("stack", false, "Auto-detect and sync the current branch's full stack")
	cmd.Flags().Bool("dry-run", false, "Show what would happen")
	rootCmd.AddCommand(cmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	original, _ := git.CurrentBranch()
	stackFlag, _ := cmd.Flags().GetBool("stack")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	cfg, err := stack.Load()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	var chain []string
	if stackFlag || len(args) == 0 {
		chain = autoDetectChain(cfg)
	} else {
		chain = stack.TopoSortWith(cfg, args)
	}

	if len(chain) < 2 {
		ui.PrintInfo("Need at least 2 branches (root + 1 child) to sync.")
		return nil
	}

	root := chain[0]
	syncBranches := chain[1:]

	fmt.Println()
	fmt.Printf("%s %s\n\n", ui.BoldStyle.Render("Syncing stack:"), strings.Join(chain, " -> "))

	useUpdateRefs := git.SupportsUpdateRefs()

	if dryRun {
		strategy := "--update-refs"
		if !useUpdateRefs {
			strategy = "--onto (fallback)"
		}
		major, minor, _ := git.Version()
		ui.PrintDryRun([]string{
			fmt.Sprintf("Would sync stack: %s", strings.Join(chain, " -> ")),
			"",
			fmt.Sprintf("Git %d.%d detected, will use %s strategy", major, minor, strategy),
			fmt.Sprintf("Push %s with --force-with-lease", strings.Join(syncBranches, ", ")),
		})
		return nil
	}

	if len(syncBranches) >= 5 {
		if !ui.Confirm(fmt.Sprintf("Sync %d branches?", len(syncBranches))) {
			ui.PrintInfo("Cancelled.")
			return nil
		}
	}

	// --update-refs only works on linear chains; validate that each branch
	// in the chain is the parent of the next one.
	if useUpdateRefs && !isLinearChain(cfg, chain) {
		ui.PrintInfo("Stack has siblings. Falling back to --onto iteration.")
		useUpdateRefs = false
	}

	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes. Stash or commit them before syncing.")
		return nil
	}

	var success bool
	if useUpdateRefs {
		success = syncUpdateRefs(chain, root)
	} else {
		if !git.SupportsUpdateRefs() {
			ui.PrintInfo("Using --onto fallback (Git < 2.38).")
		}
		// else: already printed "Stack has siblings" message
		success = syncOnto(chain)
	}

	if success {
		// Update parent heads (batch: load once, update all, save once)
		for i, branch := range syncBranches {
			parentRef, _ := git.Run("rev-parse", chain[i])
			if meta, ok := cfg.Branches[branch]; ok {
				meta.ParentHead = parentRef
			}
		}
		cfg.Save()

		// Push
		fmt.Println()
		fmt.Println("  Pushing updated branches...")
		for _, branch := range syncBranches {
			if _, err := git.Run("push", "--force-with-lease", "origin", branch); err != nil {
				ui.PrintWarning(fmt.Sprintf("Failed to push %s: %s", branch, err))
			} else {
				ui.PrintSuccess(fmt.Sprintf("Pushed %s", branch))
			}
		}
		fmt.Println()
		ui.PrintSuccess(fmt.Sprintf("Stack sync complete. %d branches updated.", len(syncBranches)))
	}

	// Return to original branch
	if original != "" {
		git.Run("checkout", original)
	}
	return nil
}

func syncUpdateRefs(chain []string, root string) bool {
	tip := chain[len(chain)-1]
	fmt.Printf("  Rebasing stack onto %s (using --update-refs)...\n", root)

	if _, err := git.Run("checkout", tip); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to checkout %s: %s", tip, err))
		return false
	}

	_, err := git.RunCombined("rebase", "--update-refs", root)
	if err != nil {
		handleRebaseConflict(chain)
		return false
	}

	for _, branch := range chain[1:] {
		ui.PrintSuccess(fmt.Sprintf("Rebased %s", branch))
	}
	return true
}

func syncOnto(chain []string) bool {
	fmt.Println("  Rebasing stack (using --onto fallback)...")

	// Capture all pre-rebase SHAs so --onto has correct old bases
	preRebaseSHA := make(map[string]string, len(chain))
	for _, b := range chain {
		sha, _ := git.Run("rev-parse", b)
		preRebaseSHA[b] = sha
	}

	for i := 1; i < len(chain); i++ {
		parent := chain[i-1]
		branch := chain[i]

		var err error
		if i == 1 {
			_, err = git.RunCombined("rebase", parent, branch)
		} else {
			newParentSHA, _ := git.Run("rev-parse", parent)
			_, err = git.RunCombined("rebase", "--onto", newParentSHA, preRebaseSHA[parent], branch)
		}

		if err != nil {
			handleRebaseConflict(chain[i:])
			return false
		}
		ui.PrintSuccess(fmt.Sprintf("Rebased %s", branch))
	}
	return true
}

func autoDetectChain(cfg *stack.Config) []string {
	current, err := git.CurrentBranch()
	if err != nil {
		return nil
	}
	chainUp := cfg.StackChainOf(current)
	descendants := cfg.DescendantsOf(current)
	return append(chainUp, descendants...)
}

// isLinearChain returns true if each branch in chain[1:] has exactly one child
// (the next branch in the chain), meaning there are no siblings.
func isLinearChain(cfg *stack.Config, chain []string) bool {
	for i := 0; i < len(chain)-1; i++ {
		children := cfg.ChildrenOf(chain[i])
		if len(children) != 1 || children[0] != chain[i+1] {
			return false
		}
	}
	return true
}
