package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

const (
	undoMergeConflict = "merge_conflict"
	undoRebase        = "rebase"
	undoStage         = "stage"
	undoAmend         = "amend"
	undoMergeCommit   = "merge_commit"
	undoCommit        = "commit"
)

func init() {
	undoCmd := &cobra.Command{
		Use:   "undo",
		Short: "Smart undo. Detects the last git action and reverses it.",
		RunE:  runUndo,
	}
	undoCmd.Flags().Bool("dry-run", false, "See what would be undone")
	undoCmd.Flags().Bool("history", false, "Show undo/redo history")
	rootCmd.AddCommand(undoCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "redo",
		Short: "Redo the last undo",
		RunE:  runRedo,
	})
}

type undoState struct {
	Type      string
	Desc      string
	Command   string
	ActionMsg string
}

func detectUndoState() *undoState {
	root, err := git.RepoRoot()
	if err != nil {
		return nil
	}

	// Priority 1: Merge conflict
	if git.FileExists(filepath.Join(root, ".git", "MERGE_HEAD")) {
		return &undoState{undoMergeConflict, "merge conflict in progress", "git merge --abort", "Abort the merge. Returns to pre-merge state."}
	}

	// Priority 2: Rebase
	if git.DirExists(filepath.Join(root, ".git", "rebase-merge")) || git.DirExists(filepath.Join(root, ".git", "rebase-apply")) {
		return &undoState{undoRebase, "rebase in progress", "git rebase --abort", "Abort the rebase. Returns to pre-rebase state."}
	}

	// Priority 3: Staged files
	staged := git.RunUnchecked("diff", "--cached", "--name-only")
	if staged != "" {
		count := len(strings.Split(staged, "\n"))
		return &undoState{undoStage, fmt.Sprintf("%d staged file%s", count, ui.Plural(count)), "git reset HEAD", "Unstage all files. Changes stay in your working tree."}
	}

	// Priority 4+: Reflog
	reflog, _ := git.Lines("reflog", "--format=%gs", "-n", "10")
	for _, action := range reflog {
		if strings.Contains(strings.ToLower(action), "amend") {
			return &undoState{undoAmend, "amended commit", "git reset --soft HEAD@{1}", "Restore pre-amend state. Your changes will be preserved."}
		}
		if strings.HasPrefix(strings.ToLower(action), "commit") {
			_, short, msg, _, date := git.LastCommit()
			if short == "" {
				continue
			}
			// Check merge commit
			parents := git.RunUnchecked("rev-list", "--parents", "-n", "1", "HEAD")
			if len(strings.Fields(parents)) > 2 {
				return &undoState{undoMergeCommit, fmt.Sprintf("merge commit \"%s\" (%s)", msg, short), "git reset --hard HEAD~1", "Reset to before the merge commit. WARNING: hard reset."}
			}
			return &undoState{undoCommit, fmt.Sprintf("commit \"%s\" (%s, %s)", msg, short, git.TimeAgo(date)), "git reset --soft HEAD~1", "Soft reset to previous commit. Your changes will be preserved in staging."}
		}
		continue
	}
	return nil
}

func runUndo(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	history, _ := cmd.Flags().GetBool("history")
	if history {
		showUndoHistory()
		return nil
	}

	state := detectUndoState()
	if state == nil {
		ui.PrintInfo("Nothing to undo.")
		return nil
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

	fmt.Println()
	fmt.Printf("%s %s\n", ui.BoldStyle.Render("Detected:"), state.Desc)
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.LabelStyle.Render("Action:"), state.ActionMsg)
	fmt.Printf("  %s  %s\n", ui.LabelStyle.Render("Command:"), ui.DimStyle.Render(state.Command))

	if dryRun {
		ui.PrintDryRun([]string{
			"Would run: " + state.Command,
			"Result:    " + state.ActionMsg,
		})
		return nil
	}

	if !ui.Confirm("Proceed with undo?") {
		ui.PrintInfo("Cancelled.")
		return nil
	}

	preRef := git.RunUnchecked("rev-parse", "HEAD")

	parts := strings.Fields(strings.TrimPrefix(state.Command, "git "))
	if _, err := git.Run(parts...); err != nil {
		ui.PrintError(fmt.Sprintf("Undo failed: %s", err))
		return nil
	}

	postRef := git.RunUnchecked("rev-parse", "HEAD")
	saveUndoHistory(state.Type, "Undo "+state.Desc, state.Command, preRef, postRef)

	fmt.Println()
	switch state.Type {
	case undoStage:
		ui.PrintSuccess("Unstaged files. Your changes are preserved in the working tree.")
	case undoCommit:
		ui.PrintSuccess("Undone. Your changes from that commit are now staged.")
		ui.PrintInfo("Run `gx redo` to reverse this.")
	case undoMergeConflict:
		ui.PrintSuccess("Merge aborted. Working tree restored to pre-merge state.")
	case undoRebase:
		ui.PrintSuccess("Rebase aborted. Working tree restored to pre-rebase state.")
	case undoAmend:
		ui.PrintSuccess("Restored pre-amend state.")
	case undoMergeCommit:
		ui.PrintSuccess("Merge commit undone.")
	default:
		ui.PrintSuccess("Undone.")
	}
	return nil
}

func runRedo(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	entries := loadUndoHistory()
	if len(entries) == 0 {
		ui.PrintInfo("Nothing to redo.")
		return nil
	}

	var last *undoEntry
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].Undone {
			last = &entries[i]
			break
		}
	}
	if last == nil {
		ui.PrintInfo("Nothing to redo.")
		return nil
	}

	currentHead := git.RunUnchecked("rev-parse", "HEAD")
	if last.PostRef != "" && currentHead != last.PostRef {
		ui.PrintError("Cannot redo. Repo state has changed since last undo.")
		fmt.Printf("  Expected HEAD at %s, but found %s.\n", safeShort(last.PostRef, 7), safeShort(currentHead, 7))
		ui.PrintInfo("Use `gx undo --history` to review past actions.")
		return nil
	}

	fmt.Println()
	fmt.Printf("%s %s\n", ui.BoldStyle.Render("Redoing:"), last.Desc)

	if !ui.Confirm("Proceed with redo?") {
		ui.PrintInfo("Cancelled.")
		return nil
	}

	if last.PreRef == "" {
		ui.PrintError("Cannot redo. No pre-state reference found.")
		return nil
	}

	if !git.IsClean() {
		ui.PrintWarning("You have uncommitted changes that would be lost by redo.")
		if !ui.Confirm("Proceed anyway?") {
			ui.PrintInfo("Cancelled.")
			return nil
		}
	}

	if last.Action == "stage" {
		if _, err := git.Run("add", "-A"); err != nil {
			ui.PrintError(fmt.Sprintf("Redo failed: %s", err))
			return nil
		}
	} else {
		if _, err := git.Run("reset", "--hard", last.PreRef); err != nil {
			ui.PrintError(fmt.Sprintf("Redo failed: %s", err))
			return nil
		}
	}

	last.Undone = true
	saveUndoEntries(entries)

	fmt.Println()
	ui.PrintSuccess("Redone.")
	return nil
}

// Undo history persistence

type undoEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action_detected"`
	Desc      string `json:"description"`
	Command   string `json:"undo_command"`
	PreRef    string `json:"pre_state_ref"`
	PostRef   string `json:"post_state_ref"`
	Undone    bool   `json:"undone"`
}

func undoHistoryPath() string {
	root, err := git.RepoRoot()
	if err != nil {
		return ""
	}
	return filepath.Join(root, ".git", "gx", "undo_history.json")
}

func loadUndoHistory() []undoEntry {
	path := undoHistoryPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var wrapper struct {
		Entries []undoEntry `json:"entries"`
	}
	if json.Unmarshal(data, &wrapper) != nil {
		return nil
	}
	return wrapper.Entries
}

func saveUndoEntries(entries []undoEntry) {
	path := undoHistoryPath()
	if path == "" {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	if len(entries) > 50 {
		entries = entries[len(entries)-50:]
	}
	data, _ := json.MarshalIndent(map[string]any{"entries": entries}, "", "  ")
	os.WriteFile(path, data, 0o644)
}

func saveUndoHistory(action, desc, command, preRef, postRef string) {
	entries := loadUndoHistory()
	entries = append(entries, undoEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Action:    action,
		Desc:      desc,
		Command:   command,
		PreRef:    preRef,
		PostRef:   postRef,
	})
	saveUndoEntries(entries)
}

func safeShort(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func showUndoHistory() {
	entries := loadUndoHistory()
	if len(entries) == 0 {
		ui.PrintInfo("No undo/redo history.")
		return
	}

	fmt.Println()
	fmt.Println(ui.BoldStyle.Render("Undo/Redo History (last 10):"))
	fmt.Println()

	start := 0
	if len(entries) > 10 {
		start = len(entries) - 10
	}

	var rows [][]string
	num := 1
	for i := len(entries) - 1; i >= start; i-- {
		e := entries[i]
		status := "active"
		if e.Undone {
			status = "redone"
		}
		rows = append(rows, []string{fmt.Sprintf("%d", num), git.TimeAgo(e.Timestamp), e.Action, e.Desc, status})
		num++
	}
	ui.PrintTable([]string{"#", "Time", "Action", "Description", "Status"}, rows, "")
}
