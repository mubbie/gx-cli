package cmd

import (
	"fmt"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

type handoffCommit struct{ hash, msg string }

func init() {
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: "Generate a branch summary for PRs, Slack, or standups",
		RunE:  runHandoff,
	}
	cmd.Flags().String("against", "", "Compare against a specific branch")
	cmd.Flags().BoolP("copy", "c", false, "Copy output to clipboard")
	cmd.Flags().Bool("markdown", false, "Output in markdown format")
	cmd.Flags().Bool("md", false, "Output in markdown format")
	rootCmd.AddCommand(cmd)
}

func runHandoff(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	against, _ := cmd.Flags().GetString("against")
	copyFlag, _ := cmd.Flags().GetBool("copy")
	markdown, _ := cmd.Flags().GetBool("markdown")
	md, _ := cmd.Flags().GetBool("md")
	if md {
		markdown = true
	}

	// Determine base
	isStacked := false
	var base string
	if against != "" {
		if !git.BranchExists(against) {
			ui.PrintError(fmt.Sprintf("Branch '%s' does not exist.", against))
			return nil
		}
		base = against
	} else {
		p := stack.Parent(current)
		if p != "" {
			base = p
			isStacked = true
		} else {
			base = git.HeadBranch()
		}
	}

	// Gather commits
	logOut := git.RunUnchecked("log", "--oneline", "--no-decorate", base+"..HEAD")
	var commits []handoffCommit
	for _, line := range strings.Split(logOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, handoffCommit{parts[0], parts[1]})
		}
	}

	if len(commits) == 0 {
		ui.PrintInfo(fmt.Sprintf("No changes between %s and %s.", current, base))
		return nil
	}

	// File stats
	statOut := git.RunUnchecked("diff", "--stat", base+"...HEAD")
	statSummary := ""
	if statOut != "" {
		lines := strings.Split(statOut, "\n")
		statSummary = strings.TrimSpace(lines[len(lines)-1])
	}

	// Files
	filesOut := git.RunUnchecked("diff", "--name-only", base+"...HEAD")
	var files []string
	for _, f := range strings.Split(filesOut, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}

	// Format
	var output string
	if markdown {
		output = formatHandoffMD(current, base, commits, statSummary, files)
	} else {
		output = formatHandoffPlain(current, base, isStacked, commits, statSummary, files)
	}

	fmt.Println(output)

	if copyFlag {
		if ui.CopyToClipboard(output) {
			fmt.Println()
			ui.PrintSuccess("Copied to clipboard.")
		} else {
			fmt.Println()
			ui.PrintWarning("Could not copy to clipboard.")
		}
	}
	return nil
}

func formatHandoffPlain(branch, base string, isStacked bool, commits []handoffCommit, stat string, files []string) string {
	var b strings.Builder
	if isStacked {
		fmt.Fprintf(&b, "%s (on %s)\n", ui.BranchStyle.Render(branch), ui.DimStyle.Render(base))
	} else {
		fmt.Fprintf(&b, "%s (vs %s)\n", ui.BranchStyle.Render(branch), ui.DimStyle.Render(base))
	}
	fmt.Fprintf(&b, "\n%s\n", ui.BoldStyle.Render(fmt.Sprintf("Commits (%d):", len(commits))))
	for _, c := range commits {
		fmt.Fprintf(&b, "  %s  %s\n", ui.HashStyle.Render(c.hash), c.msg)
	}
	// Color the stat summary line
	colored := colorStatLine(stat)
	fmt.Fprintf(&b, "\n%s\n", colored)
	fmt.Fprintf(&b, "\n%s\n", ui.BoldStyle.Render("Files:"))
	for _, f := range files {
		fmt.Fprintf(&b, "  %s\n", ui.FileStyle.Render(f))
	}
	return b.String()
}

func colorStatLine(stat string) string {
	// Color insertions green and deletions red in stat summary
	parts := strings.Split(stat, ",")
	for i, p := range parts {
		trimmed := strings.TrimSpace(p)
		if strings.Contains(trimmed, "insertion") {
			parts[i] = " " + ui.AddStyle.Render(trimmed)
		} else if strings.Contains(trimmed, "deletion") {
			parts[i] = " " + ui.DelStyle.Render(trimmed)
		}
	}
	return strings.Join(parts, ",")
}

func formatHandoffMD(branch, base string, commits []handoffCommit, stat string, files []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n", branch)
	fmt.Fprintf(&b, "**Base:** %s · **%d commit%s** · %s\n", base, len(commits), ui.Plural(len(commits)), stat)
	fmt.Fprintf(&b, "\n### Commits\n")
	for _, c := range commits {
		fmt.Fprintf(&b, "- `%s` %s\n", c.hash, c.msg)
	}
	fmt.Fprintf(&b, "\n### Files Changed\n")
	for _, f := range files {
		fmt.Fprintf(&b, "- `%s`\n", f)
	}
	return b.String()
}

