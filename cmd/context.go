package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Short:   "Repo status at a glance",
		RunE:    runContext,
	}
	rootCmd.AddCommand(cmd)
}

func runContext(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	fmt.Println()

	// Branch
	branch, err := git.CurrentBranch()
	if err != nil {
		shortHash := git.RunUnchecked("rev-parse", "--short", "HEAD")
		fmt.Printf("%s  (detached HEAD at %s)\n", ui.LabelStyle.Render("Branch:"), ui.HashStyle.Render(shortHash))
		ui.PrintWarning("You are in detached HEAD state.")
	} else {
		fmt.Printf("%s  %s\n", ui.LabelStyle.Render("Branch:"), ui.BranchStyle.Render(branch))

		// Tracking
		tracking := git.RunUnchecked("rev-parse", "--abbrev-ref", branch+"@{upstream}")
		if tracking != "" {
			ahead, behind := git.AheadBehind(branch, tracking)
			if ahead == 0 && behind == 0 {
				fmt.Printf("%s  %s %s\n", ui.LabelStyle.Render("Tracking:"), tracking, ui.DimStyle.Render("(up to date)"))
			} else {
				parts := ""
				if ahead > 0 {
					parts += ui.AddStyle.Render(fmt.Sprintf("%d ahead", ahead))
				}
				if behind > 0 {
					if parts != "" {
						parts += ", "
					}
					parts += ui.DelStyle.Render(fmt.Sprintf("%d behind", behind))
				}
				fmt.Printf("%s  %s (%s)\n", ui.LabelStyle.Render("Tracking:"), tracking, parts)
			}
		}

		// vs HEAD branch
		headBranch := git.HeadBranch()
		if branch != headBranch {
			ahead, behind := git.AheadBehind(branch, headBranch)
			aheadStr := ui.AddStyle.Render(fmt.Sprintf("%d ahead", ahead))
			behindStr := ui.DelStyle.Render(fmt.Sprintf("%d behind", behind))
			fmt.Printf("%s  %s, %s\n", ui.LabelStyle.Render(fmt.Sprintf("vs %s:", headBranch)), aheadStr, behindStr)
		}
	}

	fmt.Println()

	// Last commit
	_, shortHash, message, _, date := git.LastCommit()
	if shortHash != "" {
		fmt.Printf("%s  %s \"%s\" %s\n", ui.LabelStyle.Render("Last commit:"), ui.HashStyle.Render(shortHash), message, ui.DateStyle.Render("("+git.TimeAgo(date)+")"))
	} else {
		fmt.Printf("%s  %s\n", ui.LabelStyle.Render("Last commit:"), ui.DimStyle.Render("No commits yet"))
	}

	fmt.Println()

	// Working tree
	status := git.RunUnchecked("status", "--porcelain")
	if status == "" {
		fmt.Printf("%s %s\n", ui.LabelStyle.Render("Working tree:"), ui.SuccessStyle.Render("clean"))
	} else {
		modified, staged, untracked := 0, 0, 0
		for _, line := range strings.Split(status, "\n") {
			if len(line) < 2 {
				continue
			}
			if line[:2] == "??" {
				untracked++
			} else {
				if line[0] != ' ' && line[0] != '?' {
					staged++
				}
				if line[1] != ' ' && line[1] != '?' {
					modified++
				}
			}
		}
		fmt.Println(ui.LabelStyle.Render("Working tree:"))
		if modified > 0 {
			fmt.Printf("  %s  %s\n", ui.LabelStyle.Render("Modified:"), ui.WarningStyle.Render(fmt.Sprintf("%d file%s", modified, ui.Plural(modified))))
		}
		if staged > 0 {
			fmt.Printf("  %s  %s\n", ui.LabelStyle.Render("Staged:"), ui.AddStyle.Render(fmt.Sprintf("%d file%s", staged, ui.Plural(staged))))
		}
		if untracked > 0 {
			fmt.Printf("  %s  %s\n", ui.LabelStyle.Render("Untracked:"), ui.DimStyle.Render(fmt.Sprintf("%d file%s", untracked, ui.Plural(untracked))))
		}
	}

	fmt.Println()

	// Stash
	stashCount := git.StashCount()
	if stashCount > 0 {
		fmt.Printf("%s  %d entr%s\n", ui.LabelStyle.Render("Stash:"), stashCount, ui.PluralIES(stashCount))
	} else {
		fmt.Printf("%s  %s\n", ui.LabelStyle.Render("Stash:"), ui.DimStyle.Render("empty"))
	}

	// Active operations
	root, _ := git.RepoRoot()
	if root != "" {
		if git.FileExists(filepath.Join(root, ".git", git.MergeHeadPath)) {
			fmt.Println()
			ui.PrintWarning("Merge in progress")
		} else if git.DirExists(filepath.Join(root, ".git", git.RebaseMergePath)) || git.DirExists(filepath.Join(root, ".git", git.RebaseApplyPath)) {
			fmt.Println()
			ui.PrintWarning("Rebase in progress")
		} else if git.FileExists(filepath.Join(root, ".git", git.CherryPickHeadPath)) {
			fmt.Println()
			ui.PrintWarning("Cherry-pick in progress")
		}
	}

	fmt.Println()
	return nil
}

