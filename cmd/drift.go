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
		Use:   "drift [target]",
		Short: "Show how far your branch has diverged from the HEAD branch",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDrift,
	}
	cmd.Flags().Bool("full", false, "Show all commits (no truncation)")
	cmd.Flags().BoolP("parent", "p", false, "Compare against stack parent")
	rootCmd.AddCommand(cmd)
}

func runDrift(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	useParent, _ := cmd.Flags().GetBool("parent")
	full, _ := cmd.Flags().GetBool("full")

	if useParent && len(args) > 0 {
		ui.PrintError("Cannot use --parent with an explicit target branch.")
		return nil
	}

	var target string
	if useParent {
		p := stack.Parent(current)
		if p == "" {
			ui.PrintError(fmt.Sprintf("%s is not in the stack. Use `gx drift` without --parent.", current))
			return nil
		}
		target = p
	} else if len(args) > 0 {
		target = args[0]
	} else {
		target = git.HeadBranch()
	}

	if current == target {
		ui.PrintInfo(fmt.Sprintf("You're on %s. Switch to a feature branch to see drift.", target))
		return nil
	}

	mb, err := git.MergeBase("HEAD", target)
	if err != nil {
		ui.PrintError(fmt.Sprintf("No common ancestor between %s and %s.", current, target))
		return nil
	}

	ahead, behind := git.AheadBehind("HEAD", target)

	fmt.Println()
	if ahead == 0 && behind == 0 {
		ui.PrintSuccess(fmt.Sprintf("No divergence. Your branch is based on the latest %s.", target))
		return nil
	}

	aheadStr := ui.AddStyle.Render(fmt.Sprintf("%d ahead", ahead))
	behindStr := ui.DelStyle.Render(fmt.Sprintf("%d behind", behind))
	fmt.Printf("%s is %s, %s %s\n\n", ui.BranchStyle.Render(current), aheadStr, behindStr, ui.BranchStyle.Render(target))

	maxCommits := 20
	if full {
		maxCommits = 0
	}

	if ahead > 0 {
		fmt.Printf("%s\n", ui.BoldStyle.Render(fmt.Sprintf("Commits on your branch (not on %s):", target)))
		showCommits(mb, "HEAD", maxCommits)
		if !full && ahead > 20 {
			fmt.Printf("  (and %d more, use --full to see all)\n", ahead-20)
		}
		fmt.Println()
	}

	if behind > 0 {
		fmt.Printf("%s\n", ui.BoldStyle.Render(fmt.Sprintf("Commits on %s (not on your branch):", target)))
		showCommits(mb, target, maxCommits)
		if !full && behind > 20 {
			fmt.Printf("  (and %d more, use --full to see all)\n", behind-20)
		}
		fmt.Println()
	}

	stat := git.RunUnchecked("diff", "--stat", mb+"..HEAD")
	if stat != "" {
		lines := strings.Split(stat, "\n")
		if len(lines) > 0 {
			fmt.Printf("Files diverged: %s\n\n", strings.TrimSpace(lines[len(lines)-1]))
		}
	}
	return nil
}

func showCommits(from, to string, limit int) {
	args := []string{"log", "--format=%h\t%aI\t%an\t%s", from + ".." + to}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-%d", limit))
	}
	out := git.RunUnchecked(args...)
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		age := git.TimeAgo(parts[1])
		fmt.Printf("  %s  %s %s\n", ui.HashStyle.Render(parts[0]), ui.DateStyle.Render(fmt.Sprintf("%-12s", age)), parts[3])
	}
}
