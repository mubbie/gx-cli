package cmd

import (
	"fmt"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "up", Short: "Move up the stack (to child branch)", RunE: runUp})
	rootCmd.AddCommand(&cobra.Command{Use: "down", Short: "Move down the stack (to parent branch)", RunE: runDown})
	rootCmd.AddCommand(&cobra.Command{Use: "top", Short: "Jump to the top of the stack", RunE: runTop})
	rootCmd.AddCommand(&cobra.Command{Use: "bottom", Short: "Jump to the bottom of the stack", RunE: runBottom})
}

func runUp(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}
	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}
	children := stack.Children(current)
	if len(children) == 0 {
		ui.PrintInfo("Already at the top of the stack.")
		return nil
	}
	if len(children) > 1 {
		ui.PrintInfo(fmt.Sprintf("Multiple branches stacked on %s:", current))
		for _, c := range children {
			ui.PrintInfo("  " + c)
		}
		ui.PrintInfo("Use `gx switch` to pick one.")
		return nil
	}
	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes. They may conflict with the target branch.")
	}
	if _, err := git.Run("checkout", children[0]); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
		return nil
	}
	ui.PrintSuccess(fmt.Sprintf("Moved up: %s -> %s", current, children[0]))
	return nil
}

func runDown(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}
	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}
	parent := stack.Parent(current)
	if parent == "" {
		ui.PrintInfo(fmt.Sprintf("%s is not in the stack. Use `gx switch` to navigate.", current))
		return nil
	}
	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes. They may conflict with the target branch.")
	}
	if _, err := git.Run("checkout", parent); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
		return nil
	}
	if parent == git.HeadBranch() {
		ui.PrintInfo(fmt.Sprintf("Moved down to %s (trunk).", parent))
	} else {
		ui.PrintSuccess(fmt.Sprintf("Moved down: %s -> %s", current, parent))
	}
	return nil
}

func runTop(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}
	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	target := current
	visited := map[string]bool{current: true}
	for {
		children := stack.Children(target)
		if len(children) == 0 {
			break
		}
		if len(children) > 1 {
			ui.PrintInfo(fmt.Sprintf("Stack branches at %s. Cannot determine a single top.", target))
			ui.PrintInfo("Use `gx switch` to pick a specific branch.")
			return nil
		}
		if visited[children[0]] {
			ui.PrintWarning("Cycle detected in stack config.")
			break
		}
		visited[children[0]] = true
		target = children[0]
	}

	if target == current {
		ui.PrintInfo("Already at the top of the stack.")
		return nil
	}
	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes.")
	}
	if _, err := git.Run("checkout", target); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
		return nil
	}
	ui.PrintSuccess(fmt.Sprintf("Jumped to top: %s -> %s", current, target))
	return nil
}

func runBottom(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}
	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	head := git.HeadBranch()
	parent := stack.Parent(current)

	// On trunk or not in stack: enter the stack
	if current == head || parent == "" {
		children := stack.Children(current)
		if len(children) == 0 {
			ui.PrintInfo("No stack branches found.")
			return nil
		}
		if len(children) == 1 {
			if !git.IsClean() {
				ui.PrintWarning("You have uncommitted changes.")
			}
			if _, err := git.Run("checkout", children[0]); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
				return nil
			}
			ui.PrintSuccess(fmt.Sprintf("Jumped to bottom: %s -> %s", current, children[0]))
		} else {
			ui.PrintInfo(fmt.Sprintf("Multiple stacks branching from %s:", current))
			for _, c := range children {
				ui.PrintInfo("  " + c)
			}
			ui.PrintInfo("Use `gx switch` to pick one.")
		}
		return nil
	}

	// Walk down toward trunk
	target := current
	visited := map[string]bool{current: true}
	for {
		p := stack.Parent(target)
		if p == "" || p == head {
			break
		}
		if visited[p] {
			ui.PrintWarning("Cycle detected in stack config.")
			break
		}
		visited[p] = true
		if stack.Parent(p) == "" || stack.Parent(p) == head {
			target = p
			break
		}
		target = p
	}

	if target == current {
		ui.PrintInfo("Already at the bottom of the stack.")
		return nil
	}
	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes.")
	}
	if _, err := git.Run("checkout", target); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
		return nil
	}
	ui.PrintSuccess(fmt.Sprintf("Jumped to bottom: %s -> %s", current, target))
	return nil
}
