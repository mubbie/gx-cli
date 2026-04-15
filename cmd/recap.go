package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "recap [@author]",
		Short: "Show what you (or someone else) did recently",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRecap,
	}
	cmd.Flags().IntP("days", "d", 1, "Number of days to look back")
	cmd.Flags().Bool("all", false, "Show all contributors")
	cmd.Flags().Int("limit", 100, "Max commits to display")
	rootCmd.AddCommand(cmd)
}

func runRecap(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	days, _ := cmd.Flags().GetInt("days")
	allAuthors, _ := cmd.Flags().GetBool("all")
	limit, _ := cmd.Flags().GetInt("limit")

	var author string
	if len(args) > 0 {
		author = strings.TrimPrefix(args[0], "@")
	} else if !allAuthors {
		author = git.RunUnchecked("config", "user.name")
		if author == "" {
			ui.PrintError("Cannot determine your git user. Set git config user.name or specify an author.")
			return nil
		}
	}

	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02T15:04:05")

	root, _ := git.RepoRoot()
	repoName := ""
	if root != "" {
		parts := strings.Split(strings.ReplaceAll(root, "\\", "/"), "/")
		repoName = parts[len(parts)-1]
	}

	if allAuthors {
		return recapAll(since, days, limit, repoName)
	}

	return recapAuthor(author, since, days, limit, repoName)
}

func recapAuthor(author, since string, days, limit int, repoName string) error {
	gitArgs := []string{"log", "--all", "--format=%h\t%aI\t%s", "--since=" + since, fmt.Sprintf("-%d", limit), "--author=" + author}
	out := git.RunUnchecked(gitArgs...)
	if out == "" {
		ui.PrintInfo(fmt.Sprintf("No activity in the last %d day%s.", days, ui.Plural(days)))
		return nil
	}

	currentUser := git.RunUnchecked("config", "user.name")
	display := author
	if author == currentUser {
		display = "Your"
	} else {
		display = author + "'s"
	}

	fmt.Println()
	dayLabel := fmt.Sprintf("%d day%s", days, ui.Plural(days))
	fmt.Printf("%s\n\n", ui.BoldStyle.Render(fmt.Sprintf("%s activity in the last %s (%s):", display, dayLabel, repoName)))

	commitCount := 0
	now := time.Now()
	var currentDate string

	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		commitCount++

		t, _ := time.Parse(time.RFC3339, parts[1])
		dateLabel := formatDateLabel(t, now)
		if dateLabel != currentDate {
			currentDate = dateLabel
			fmt.Printf("  %s\n", ui.BoldStyle.Render(dateLabel+":"))
		}
		fmt.Printf("    %s  %s  %s\n", ui.DimStyle.Render(parts[0]), t.Format("15:04"), parts[2])
	}

	fmt.Printf("\n  %d commits\n", commitCount)
	return nil
}

func recapAll(since string, days, limit int, repoName string) error {
	gitArgs := []string{"log", "--all", "--format=%h\t%aI\t%an\t%s", "--since=" + since, fmt.Sprintf("-%d", limit)}
	out := git.RunUnchecked(gitArgs...)
	if out == "" {
		ui.PrintInfo(fmt.Sprintf("No activity in the last %d day%s.", days, ui.Plural(days)))
		return nil
	}

	currentUser := git.RunUnchecked("config", "user.name")

	// Group by author
	type commit struct{ hash, date, msg string }
	byAuthor := map[string][]commit{}
	var order []string
	seen := map[string]bool{}

	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		author := parts[2]
		if !seen[author] {
			order = append(order, author)
			seen[author] = true
		}
		byAuthor[author] = append(byAuthor[author], commit{parts[0], parts[1], parts[3]})
	}

	fmt.Println()
	dayLabel := fmt.Sprintf("%d day%s", days, ui.Plural(days))
	fmt.Printf("%s\n\n", ui.BoldStyle.Render(fmt.Sprintf("Team activity in the last %s (%s):", dayLabel, repoName)))

	total := 0
	for _, author := range order {
		commits := byAuthor[author]
		display := author
		if author == currentUser {
			display = "You"
		}
		fmt.Printf("  %s (%d commit%s):\n", ui.BoldStyle.Render(display), len(commits), ui.Plural(len(commits)))
		for _, c := range commits {
			t, _ := time.Parse(time.RFC3339, c.date)
			fmt.Printf("    %s  %s  %s\n", ui.DimStyle.Render(c.hash), t.Format("15:04"), c.msg)
		}
		fmt.Println()
		total += len(commits)
	}

	fmt.Printf("  %d commits from %d contributor%s\n", total, len(order), ui.Plural(len(order)))
	return nil
}

func formatDateLabel(t, now time.Time) string {
	diff := now.YearDay() - t.YearDay()
	if now.Year() != t.Year() {
		diff = 999
	}
	switch {
	case diff == 0:
		return "Today"
	case diff == 1:
		return "Yesterday"
	default:
		return t.Format("Jan 2")
	}
}
