package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

var shelfCmd = &cobra.Command{
	Use:   "shelf",
	Short: "Visual stash manager",
	RunE:  runShelfList, // Default: show list
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

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Drop all stashes",
		RunE:  runShelfClear,
	}
	clearCmd.Flags().Bool("dry-run", false, "Show what would be dropped")

	shelfCmd.AddCommand(pushCmd, listCmd, clearCmd)
	rootCmd.AddCommand(shelfCmd)
}

type stashEntry struct {
	index int
	id    string
	hash  string
	time  string
	msg   string
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
			b = "--"
		}
		rows = append(rows, []string{fmt.Sprintf("%d", s.index), s.time, b, s.msg})
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
		fmt.Printf("  %s  %-15s %s\n", s.id, s.time, s.msg)
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

