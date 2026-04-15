package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

var shelfCmd = &cobra.Command{
	Use:   "shelf",
	Short: "Stash manager",
	RunE:  runShelfInteractive,
}

func init() {
	pushCmd := &cobra.Command{
		Use:   "push [message]",
		Short: "Stash current work with a descriptive message",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runShelfPush,
	}
	pushCmd.Flags().BoolP("include-untracked", "u", false, "Also stash untracked files")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Non-interactive stash list",
		RunE:  runShelfList,
	}

	showCmd := &cobra.Command{
		Use:   "show <number>",
		Short: "Show diff for a stash",
		Args:  cobra.ExactArgs(1),
		RunE:  runShelfShow,
	}

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Drop all stashes",
		RunE:  runShelfClear,
	}
	clearCmd.Flags().Bool("dry-run", false, "Show what would be dropped")

	shelfCmd.AddCommand(pushCmd, listCmd, showCmd, clearCmd)
	rootCmd.AddCommand(shelfCmd)
}

type stashEntry struct {
	index  int
	id     string
	hash   string
	time   string
	msg    string
	branch string
}

func getStashList() []stashEntry {
	out := git.RunUnchecked("stash", "list", "--format=%gd|%H|%ar|%s")
	if out == "" {
		return nil
	}
	var entries []stashEntry
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		idx := 0
		if i := strings.Index(parts[0], "{"); i >= 0 {
			end := strings.Index(parts[0], "}")
			if end > i {
				fmt.Sscanf(parts[0][i+1:end], "%d", &idx)
			}
		}
		branch := ""
		msg := parts[3]
		if strings.HasPrefix(msg, "On ") {
			if colon := strings.Index(msg, ":"); colon > 3 {
				branch = msg[3:colon]
			}
		} else if strings.HasPrefix(msg, "WIP on ") {
			if colon := strings.Index(msg, ":"); colon > 7 {
				branch = msg[7:colon]
			}
		}
		entries = append(entries, stashEntry{
			index:  idx,
			id:     parts[0],
			hash:   parts[1],
			time:   parts[2],
			msg:    msg,
			branch: branch,
		})
	}
	return entries
}

func runShelfInteractive(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	stashes := getStashList()
	if len(stashes) == 0 {
		ui.PrintInfo("No stashes. Use `gx shelf push` to stash your work.")
		return nil
	}

	query := ""
	reader := newLineReader()

	for {
		var filtered []stashEntry
		if query == "" {
			filtered = stashes
		} else {
			for _, s := range stashes {
				if strings.Contains(strings.ToLower(s.msg), query) ||
					strings.Contains(strings.ToLower(s.branch), query) {
					filtered = append(filtered, s)
				}
			}
		}

		fmt.Printf("\n%d stash%s:\n\n", len(filtered), ui.PluralES(len(filtered)))

		if len(filtered) == 0 {
			fmt.Println(ui.DimStyle.Render("  No stashes match your search."))
		} else {
			maxMsgLen := 50
			for _, s := range filtered {
				msg := s.msg
				if len(msg) > maxMsgLen {
					msg = msg[:maxMsgLen-3] + "..."
				}
				branchStr := ""
				if s.branch != "" {
					branchStr = ui.BranchStyle.Render(s.branch)
				}
				fmt.Printf("  %s  %s  %s\n",
					ui.DimStyle.Render(fmt.Sprintf("%3d", s.index)),
					ui.DateStyle.Render(fmt.Sprintf("%-15s", s.time)),
					msg)
				if branchStr != "" {
					fmt.Printf("       %s\n", branchStr)
				}
			}
		}

		fmt.Println()
		if len(filtered) > 0 {
			fmt.Println(ui.DimStyle.Render("Enter number to select, text to filter, q to cancel"))
			fmt.Println(ui.DimStyle.Render("Then: p=pop  a=apply  d=drop  s=show diff"))
		} else {
			fmt.Println(ui.DimStyle.Render("Enter text to filter, q to cancel"))
		}

		choice := reader.read()
		if choice == "" || strings.ToLower(choice) == "q" {
			return nil
		}

		// Try as number
		if n, err := strconv.Atoi(choice); err == nil {
			// Find the stash with this index
			var selected *stashEntry
			for i := range filtered {
				if filtered[i].index == n {
					selected = &filtered[i]
					break
				}
			}
			if selected == nil {
				ui.PrintError(fmt.Sprintf("No stash with index %d.", n))
				continue
			}

			fmt.Printf("\n  Selected: %s %s\n", ui.HashStyle.Render(selected.id), selected.msg)
			fmt.Println(ui.DimStyle.Render("  p=pop  a=apply  d=drop  s=show diff  q=cancel"))
			action := reader.read()

			switch strings.ToLower(action) {
			case "p":
				if _, err := git.Run("stash", "pop", selected.id); err != nil {
					ui.PrintError(fmt.Sprintf("Pop failed: %s", err))
				} else {
					ui.PrintSuccess(fmt.Sprintf("Popped %s", selected.id))
				}
				return nil
			case "a":
				if _, err := git.Run("stash", "apply", selected.id); err != nil {
					ui.PrintError(fmt.Sprintf("Apply failed: %s", err))
				} else {
					ui.PrintSuccess(fmt.Sprintf("Applied %s (stash kept)", selected.id))
				}
				return nil
			case "d":
				if !ui.Confirm(fmt.Sprintf("Drop %s?", selected.id)) {
					ui.PrintInfo("Cancelled.")
					continue
				}
				if _, err := git.Run("stash", "drop", selected.id); err != nil {
					ui.PrintError(fmt.Sprintf("Drop failed: %s", err))
				} else {
					ui.PrintSuccess(fmt.Sprintf("Dropped %s", selected.id))
				}
				return nil
			case "s":
				showStashDiff(selected.id)
				continue
			default:
				continue
			}
		}

		// Otherwise treat as search filter
		query = strings.ToLower(choice)
	}
}

func showStashDiff(id string) {
	diff, err := git.Run("stash", "show", "-p", id)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to load diff: %s", err))
		return
	}

	fmt.Println()
	// Show stat summary first
	stat := git.RunUnchecked("stash", "show", "--stat", id)
	if stat != "" {
		for _, line := range strings.Split(stat, "\n") {
			if strings.Contains(line, "|") {
				parts := strings.SplitN(line, "|", 2)
				fmt.Printf("  %s|%s\n", ui.FileStyle.Render(parts[0]), parts[1])
			} else if strings.TrimSpace(line) != "" {
				fmt.Printf("  %s\n", line)
			}
		}
	}

	fmt.Println()
	// Show colored diff
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff "):
			fmt.Println(ui.BranchStyle.Render(line))
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			fmt.Println(ui.BoldStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			fmt.Println(ui.InfoStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			fmt.Println(ui.AddStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			fmt.Println(ui.DelStyle.Render(line))
		default:
			fmt.Println(line)
		}
	}
	fmt.Println()
}

func runShelfShow(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		ui.PrintError(fmt.Sprintf("Invalid stash number: %s", args[0]))
		return nil
	}

	id := fmt.Sprintf("stash@{%d}", n)
	showStashDiff(id)
	return nil
}

func runShelfPush(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	includeUntracked, _ := cmd.Flags().GetBool("include-untracked")

	if git.IsClean() {
		if includeUntracked {
			untracked := git.RunUnchecked("ls-files", "--others", "--exclude-standard")
			if untracked == "" {
				ui.PrintInfo("Nothing to stash. Working tree is clean.")
				return nil
			}
		} else {
			ui.PrintInfo("Nothing to stash. Working tree is clean.")
			return nil
		}
	}

	message := ""
	if len(args) > 0 {
		message = args[0]
	} else {
		branch, _ := git.CurrentBranch()
		if branch == "" {
			branch = "unknown"
		}
		message = fmt.Sprintf("gx-shelf: %s %s", branch, time.Now().UTC().Format("2006-01-02 15:04"))
	}

	gitArgs := []string{"stash", "push", "-m", message}
	if includeUntracked {
		gitArgs = []string{"stash", "push", "-u", "-m", message}
	}

	if _, err := git.Run(gitArgs...); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to stash: %s", err))
		return nil
	}

	ui.PrintSuccess(fmt.Sprintf("Stashed working directory: \"%s\"", message))
	fmt.Println("  Run `gx shelf` to browse.")
	return nil
}

func runShelfList(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	stashes := getStashList()
	if len(stashes) == 0 {
		ui.PrintInfo("No stashes.")
		return nil
	}

	fmt.Printf("\n%d stash%s:\n\n", len(stashes), ui.PluralES(len(stashes)))

	var rows [][]string
	for _, s := range stashes {
		b := s.branch
		if b == "" {
			b = ui.DimStyle.Render("--")
		} else {
			b = ui.BranchStyle.Render(b)
		}
		msg := s.msg
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		rows = append(rows, []string{ui.HashStyle.Render(fmt.Sprintf("%d", s.index)), ui.DateStyle.Render(s.time), b, msg})
	}
	ui.PrintTable([]string{"#", "Age", "Branch", "Message"}, rows, "")
	return nil
}

func runShelfClear(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	stashes := getStashList()
	if len(stashes) == 0 {
		ui.PrintInfo("No stashes to clear.")
		return nil
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

	fmt.Printf("\nThis will permanently delete ALL %d stashes:\n\n", len(stashes))
	for _, s := range stashes {
		fmt.Printf("  %s  %s %s\n", ui.HashStyle.Render(s.id), ui.DateStyle.Render(fmt.Sprintf("%-15s", s.time)), s.msg)
	}
	fmt.Println()

	if dryRun {
		ui.PrintDryRun([]string{fmt.Sprintf("Would drop %d stashes.", len(stashes))})
		return nil
	}

	ui.PrintWarning("This cannot be undone.")
	if !ui.Confirm("Drop all stashes?") {
		ui.PrintInfo("Cancelled.")
		return nil
	}

	if _, err := git.Run("stash", "clear"); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to clear stashes: %s", err))
		return nil
	}
	ui.PrintSuccess("All stashes cleared.")
	return nil
}
