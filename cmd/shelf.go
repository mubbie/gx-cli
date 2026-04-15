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
	index    int
	id       string
	hash     string
	time     string
	msg      string
	branch   string
	numFiles int
	adds     int
	dels     int
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

		// Get file stats (shortstat is one line: " 3 files changed, 12 insertions(+), 5 deletions(-)")
		numFiles, adds, dels := 0, 0, 0
		statLine := git.RunUnchecked("stash", "show", "--shortstat", parts[0])
		if statLine != "" {
			fmt.Sscanf(strings.TrimSpace(statLine), "%d", &numFiles)
			if i := strings.Index(statLine, "insertion"); i > 0 {
				// walk back to find the number
				sub := strings.TrimSpace(statLine[:i])
				if ci := strings.LastIndex(sub, ","); ci >= 0 {
					fmt.Sscanf(strings.TrimSpace(sub[ci+1:]), "%d", &adds)
				} else if ci := strings.LastIndex(sub, " "); ci >= 0 {
					fmt.Sscanf(strings.TrimSpace(sub[ci+1:]), "%d", &adds)
				}
			}
			if i := strings.Index(statLine, "deletion"); i > 0 {
				sub := strings.TrimSpace(statLine[:i])
				if ci := strings.LastIndex(sub, ","); ci >= 0 {
					fmt.Sscanf(strings.TrimSpace(sub[ci+1:]), "%d", &dels)
				}
			}
		}

		entries = append(entries, stashEntry{
			index:    idx,
			id:       parts[0],
			hash:     parts[1],
			time:     parts[2],
			msg:      msg,
			branch:   branch,
			numFiles: numFiles,
			adds:     adds,
			dels:     dels,
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
			for _, s := range filtered {
				renderStashEntry(s)
			}
		}

		fmt.Println()
		fmt.Println(ui.DimStyle.Render("  <n>a = apply  <n>p = pop (apply+drop)  <n>d = drop"))
		fmt.Println(ui.DimStyle.Render("  text = filter  q = cancel"))

		choice := ui.ReadLine()
		if choice == "" || strings.ToLower(choice) == "q" {
			return nil
		}

		// Parse: number + optional action letter (e.g. "0p", "2a", "1d", or just "0")
		choice = strings.TrimSpace(choice)
		action := ""
		numStr := choice

		if len(choice) > 0 {
			last := choice[len(choice)-1]
			if last == 'p' || last == 'a' || last == 'd' {
				action = string(last)
				numStr = strings.TrimSpace(choice[:len(choice)-1])
			}
		}

		if n, err := strconv.Atoi(numStr); err == nil {
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

			// If no action letter, default to apply
			if action == "" {
				action = "a"
			}

			switch action {
			case "p":
				if _, err := git.Run("stash", "pop", selected.id); err != nil {
					ui.PrintError(fmt.Sprintf("Pop failed: %s", err))
				} else {
					ui.PrintSuccess(fmt.Sprintf("Popped %s (applied and dropped)", selected.id))
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
			}
		}

		query = strings.ToLower(choice)
	}
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

	for _, s := range stashes {
		renderStashEntry(s)
	}
	fmt.Println()
	return nil
}

func renderStashEntry(s stashEntry) {
	msg := s.msg
	if len(msg) > 60 {
		msg = msg[:57] + "..."
	}

	// Line 1: index, age, message
	fmt.Printf("  %s  %s  %s\n",
		ui.HashStyle.Render(fmt.Sprintf("%2d", s.index)),
		ui.DateStyle.Render(fmt.Sprintf("%-16s", s.time)),
		msg)

	// Line 2: branch + stats (indented under message)
	var details []string
	if s.branch != "" {
		details = append(details, ui.BranchStyle.Render(s.branch))
	}
	if s.numFiles > 0 {
		stat := fmt.Sprintf("%d file%s", s.numFiles, ui.Plural(s.numFiles))
		if s.adds > 0 {
			stat += ui.AddStyle.Render(fmt.Sprintf(" +%d", s.adds))
		}
		if s.dels > 0 {
			stat += ui.DelStyle.Render(fmt.Sprintf(" -%d", s.dels))
		}
		details = append(details, stat)
	}
	if len(details) > 0 {
		fmt.Printf("      %s  %s\n", strings.Repeat(" ", 16), ui.DimStyle.Render(strings.Join(details, "  ")))
	}
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
