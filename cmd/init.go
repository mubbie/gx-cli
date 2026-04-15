package cmd

import (
	"fmt"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize gx stacking for this repo",
		RunE:  runInit,
	}
	cmd.Flags().String("trunk", "", "Explicitly set the trunk branch")
	cmd.Flags().Bool("force", false, "Re-initialize (preserves relationships)")
	rootCmd.AddCommand(cmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	trunk, _ := cmd.Flags().GetString("trunk")
	force, _ := cmd.Flags().GetBool("force")

	if stack.ConfigExists() && !force {
		cfg, err := stack.Load()
		if err != nil {
			ui.PrintError(err.Error())
			return nil
		}
		fmt.Println()
		ui.PrintInfo("gx is already initialized.")
		fmt.Printf("  Trunk: %s\n", cfg.Meta.MainBranch)
		fmt.Printf("  Tracked branches: %d\n", len(cfg.Branches))
		fmt.Println("  Config: .git/gx/stack.json")
		fmt.Println()
		fmt.Println("  Run with --force to re-initialize.")
		return nil
	}

	if trunk != "" {
		if !git.BranchExists(trunk) {
			ui.PrintError(fmt.Sprintf("Branch '%s' does not exist.", trunk))
			return nil
		}
	} else {
		trunk = git.HeadBranch()
	}

	if force && stack.ConfigExists() {
		ui.PrintWarning("Re-initializing gx. Existing branch relationships will be preserved.")
		cfg, _ := stack.Load()
		cfg.Meta.MainBranch = trunk
		if err := cfg.Save(); err != nil {
			ui.PrintError(err.Error())
			return nil
		}
		fmt.Println()
		ui.PrintSuccess(fmt.Sprintf("Re-initialized gx with trunk branch: %s", trunk))
		return nil
	}

	cfg := &stack.Config{
		Branches: make(map[string]*stack.BranchMeta),
		Meta:     stack.Metadata{MainBranch: trunk},
	}
	if err := cfg.Save(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	fmt.Println()
	ui.PrintSuccess("Initialized gx in this repo.")
	fmt.Printf("  Trunk: %s\n", trunk)
	fmt.Println("  Stack config: .git/gx/stack.json")
	fmt.Println()
	fmt.Println("  Get started:")
	fmt.Printf("    gx stack feature/my-thing %s    Create your first stacked branch\n", trunk)
	fmt.Println("    gx graph                          View your stack")
	return nil
}
