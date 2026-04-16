package cmd

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

var conflictRe = regexp.MustCompile(`Merge conflict in (.+)`)

func init() {
	cmd := &cobra.Command{
		Use:   "conflicts [target]",
		Short: "Preview merge conflicts before merging, without touching the working tree",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConflicts,
	}
	cmd.Flags().Bool("dry-run", false, "Always read-only (supported for consistency)")
	rootCmd.AddCommand(cmd)
}

func runConflicts(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	target := git.HeadBranch()
	if len(args) > 0 {
		target = args[0]
	}

	if current == target {
		ui.PrintInfo(fmt.Sprintf("You're already on %s. Switch to a feature branch to check conflicts.", target))
		return nil
	}

	if !git.BranchExists(target) {
		ui.PrintError(fmt.Sprintf("Branch '%s' does not exist.", target))
		return nil
	}

	fmt.Println()
	fmt.Printf("Checking %s against %s...\n\n", ui.BoldStyle.Render(current), ui.BoldStyle.Render(target))

	sp := ui.StartSpinner("Checking for conflicts...")

	// Try new-style merge-tree (Git >= 2.38), fall back to old 3-arg form
	out, err := git.Run("merge-tree", "--write-tree", "HEAD", target)
	if err != nil {
		// --write-tree not supported; try old 3-arg form: git merge-tree <merge-base> HEAD <target>
		mb, mbErr := git.MergeBase("HEAD", target)
		if mbErr == nil && mb != "" {
			out, err = git.Run("merge-tree", mb, "HEAD", target)
		}
	}

	var conflicts []string
	cleanFiles := 0

	if err != nil {
		// Parse error output for conflicts
		errStr := err.Error()
		matches := conflictRe.FindAllStringSubmatch(errStr, -1)
		for _, m := range matches {
			if len(m) > 1 {
				conflicts = append(conflicts, strings.TrimSpace(m[1]))
			}
		}
	} else {
		// Check output for CONFLICT lines
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "CONFLICT") {
				matches := conflictRe.FindStringSubmatch(line)
				if len(matches) > 1 {
					conflicts = append(conflicts, strings.TrimSpace(matches[1]))
				}
			}
		}
	}

	// Count total files
	mb, _ := git.MergeBase("HEAD", target)
	if mb != "" {
		stat := git.RunUnchecked("diff", "--stat", mb+".."+target)
		total := 0
		for _, line := range strings.Split(stat, "\n") {
			if strings.Contains(line, "|") {
				total++
			}
		}
		cleanFiles = total - len(conflicts)
		if cleanFiles < 0 {
			cleanFiles = 0
		}
	}

	var buf bytes.Buffer

	if len(conflicts) == 0 {
		sp.Stop()
		ui.PrintSuccess("No conflicts. Clean merge.")
		if mb != "" {
			stat := git.RunUnchecked("diff", "--stat", mb+".."+target)
			if stat != "" {
				lines := strings.Split(stat, "\n")
				if len(lines) > 0 {
					fmt.Printf("  %s\n", strings.TrimSpace(lines[len(lines)-1]))
				}
			}
		}
		return nil
	}

	fmt.Fprintf(&buf, "%s %d conflict%s found\n\n", ui.ErrorStyle.Render("X"), len(conflicts), ui.Plural(len(conflicts)))
	for _, f := range conflicts {
		// Try to get authors
		ourAuthor := git.RunUnchecked("log", "-1", "--format=%an", "--", f)
		theirAuthor := git.RunUnchecked("log", "-1", "--format=%an", target, "--", f)
		authors := ""
		if ourAuthor != "" && theirAuthor != "" && ourAuthor != theirAuthor {
			authors = fmt.Sprintf("     (%s + %s)", ourAuthor, theirAuthor)
		}
		fmt.Fprintf(&buf, "  %s%s\n", f, authors)
	}

	if cleanFiles > 0 {
		fmt.Fprintf(&buf, "\n  %d other file%s merge cleanly\n", cleanFiles, ui.Plural(cleanFiles))
	}

	sp.Stop()
	fmt.Print(buf.String())
	return nil
}
