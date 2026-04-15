package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "switch [- | branch]",
		Short: "Branch switcher with search and rich context",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSwitch,
	})
}

type branchInfo struct {
	name    string
	date    string
	author  string
	current bool
}

func runSwitch(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	// gx switch -
	if len(args) == 1 && args[0] == "-" {
		if _, err := git.Run("switch", "-"); err != nil {
			if strings.Contains(err.Error(), "no previous") || strings.Contains(err.Error(), "invalid") {
				ui.PrintError("No previous branch to switch to.")
			} else {
				ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
			}
			return nil
		}
		current, _ := git.CurrentBranch()
		ui.PrintSuccess(fmt.Sprintf("Switched to %s", current))
		return nil
	}

	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes. They may conflict with the target branch.")
	}

	branches := getBranches()
	if len(branches) == 0 {
		ui.PrintInfo("No branches to switch to.")
		return nil
	}

	current, _ := git.CurrentBranch()
	var selectable []branchInfo
	for _, b := range branches {
		if b.name != current {
			selectable = append(selectable, b)
		}
	}

	if len(selectable) == 0 {
		ui.PrintInfo("No other branches to switch to.")
		return nil
	}

	if len(selectable) == 1 {
		if _, err := git.Run("switch", selectable[0].name); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
			return nil
		}
		ui.PrintSuccess(fmt.Sprintf("Switched to %s", selectable[0].name))
		return nil
	}

	selected := pickBranch(selectable)
	if selected == "" {
		ui.PrintInfo("Cancelled.")
		return nil
	}

	if _, err := git.Run("switch", selected); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to switch: %s", err))
		return nil
	}
	ui.PrintSuccess(fmt.Sprintf("Switched to %s", selected))
	return nil
}

func getBranches() []branchInfo {
	out := git.RunUnchecked("branch", "--format=%(refname:short)\t%(authordate:iso8601)\t%(authorname)", "--sort=-committerdate")
	current, _ := git.CurrentBranch()
	var branches []branchInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 || parts[0] == "" {
			continue
		}
		branches = append(branches, branchInfo{
			name:    parts[0],
			date:    parts[1],
			author:  parts[2],
			current: parts[0] == current,
		})
	}
	return branches
}

func pickBranch(branches []branchInfo) string {
	query := ""
	reader := newLineReader()

	for {
		var filtered []branchInfo
		if query == "" {
			filtered = branches
		} else {
			for _, b := range branches {
				if strings.Contains(strings.ToLower(b.name), query) {
					filtered = append(filtered, b)
				}
			}
		}

		fmt.Println()
		if len(filtered) == 0 {
			fmt.Println(ui.DimStyle.Render("  No branches match your search."))
		} else {
			for i, b := range filtered {
				age := git.TimeAgo(b.date)
				fmt.Printf("  %3d  %-40s %-15s %s\n", i+1, b.name, age, b.author)
			}
		}

		fmt.Println()
		if len(filtered) > 0 {
			fmt.Println(ui.DimStyle.Render("Enter a number to switch, text to filter, q to cancel"))
		} else {
			fmt.Println(ui.DimStyle.Render("Enter text to filter, q to cancel"))
		}

		choice := reader.read()
		if choice == "" || strings.ToLower(choice) == "q" {
			return ""
		}

		// Try as number
		if n, err := strconv.Atoi(choice); err == nil {
			idx := n - 1
			if idx >= 0 && idx < len(filtered) {
				return filtered[idx].name
			}
			fmt.Printf("  %s\n", ui.ErrorStyle.Render(fmt.Sprintf("Invalid number. Pick 1-%d.", len(filtered))))
			continue
		}

		query = strings.ToLower(choice)
	}
}

type lineReader struct{}

func newLineReader() lineReader { return lineReader{} }

func (lineReader) read() string {
	fmt.Print("> ")
	var line string
	fmt.Scanln(&line)
	return strings.TrimSpace(line)
}
