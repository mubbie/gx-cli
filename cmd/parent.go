package cmd

import (
	"fmt"
	"os"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "parent",
		Short: "Print the parent branch name (for scripting)",
		RunE:  runParent,
	})
}

func runParent(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		os.Exit(1)
	}

	current, err := git.CurrentBranch()
	if err != nil {
		os.Exit(1)
	}

	head := git.HeadBranch()
	if current == head {
		os.Exit(1)
	}

	parent := stack.Parent(current)
	if parent == "" {
		parent = head
	}
	fmt.Println(parent)
	return nil
}
