package cmd

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "who [path]",
		Short: "Show who knows a file, directory, or repo best",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runWho,
	}
	cmd.Flags().IntP("number", "n", 5, "Number of contributors to show")
	cmd.Flags().Bool("no-limit", false, "Remove the 200-file cap for directory analysis")
	rootCmd.AddCommand(cmd)
}

func runWho(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	n, _ := cmd.Flags().GetInt("number")

	if len(args) == 0 {
		return whoRepo(n)
	}

	path := args[0]
	info, err := os.Stat(path)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Path not found: %s", path))
		return nil
	}
	if info.IsDir() {
		noLimit, _ := cmd.Flags().GetBool("no-limit")
		return whoDir(path, n, noLimit)
	}
	return whoFile(path, n)
}

// --- File-level: git blame (fast, ~100ms per file) ---

type blameAuthor struct {
	name     string
	email    string
	lines    int
	lastDate string
}

func whoFile(path string, n int) error {
	sp := ui.StartSpinner(fmt.Sprintf("Analyzing %s...", path))

	out, err := git.Run("blame", "--line-porcelain", path)
	if err != nil {
		sp.Stop()
		ui.PrintError(fmt.Sprintf("Failed to blame %s: %s", path, err))
		return nil
	}

	authors := map[string]*blameAuthor{}
	total := 0
	var curName, curEmail string

	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "author "):
			curName = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-mail "):
			curEmail = strings.Trim(strings.TrimPrefix(line, "author-mail "), "<>")
		case strings.HasPrefix(line, "author-tz "):
			if curName != "" && curName != "Not Committed Yet" {
				a, exists := authors[curName]
				if !exists {
					a = &blameAuthor{name: curName, email: strings.ToLower(curEmail)}
					authors[curName] = a
				}
				a.lines++
				total++
			}
		}
	}

	// Get last edit dates in parallel
	var wg sync.WaitGroup
	for _, a := range authors {
		wg.Add(1)
		go func(a *blameAuthor) {
			defer wg.Done()
			date := git.RunUnchecked("log", "-1", "--format=%aI", "--author="+a.name, "--", path)
			a.lastDate = date
		}(a)
	}
	wg.Wait()

	sp.Stop()

	if total == 0 {
		ui.PrintInfo(fmt.Sprintf("No blame data for %s", path))
		return nil
	}

	var sorted []*blameAuthor
	for _, a := range authors {
		sorted = append(sorted, a)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].lines > sorted[j].lines })

	var rows [][]string
	for i, a := range sorted {
		if i >= n {
			break
		}
		pct := fmt.Sprintf("%.1f%%", float64(a.lines)/float64(total)*100)
		lastEdit := ""
		if a.lastDate != "" {
			lastEdit = git.TimeAgo(a.lastDate)
		}
		rows = append(rows, []string{
			ui.DimStyle.Render(fmt.Sprintf("%d", i+1)),
			a.name,
			ui.BoldStyle.Render(fmt.Sprintf("%d", a.lines)),
			pct,
			ui.DimStyle.Render(a.email),
			ui.DateStyle.Render(lastEdit),
		})
	}

	fmt.Println()
	ui.PrintTable([]string{"#", "Author", "Lines", "%", "Email", "Last Edit"}, rows,
		fmt.Sprintf("Ownership of %s (%d lines)", path, total))
	return nil
}

// --- Directory-level: concurrent blame across files ---

func whoDir(dir string, n int, noLimit bool) error {
	sp := ui.StartSpinner(fmt.Sprintf("Analyzing %s...", dir))

	filesOut, err := git.Run("ls-files", dir)
	if err != nil || filesOut == "" {
		sp.Stop()
		ui.PrintInfo(fmt.Sprintf("No tracked files in %s", dir))
		return nil
	}

	allFiles := strings.Split(strings.TrimSpace(filesOut), "\n")
	files := allFiles
	maxFiles := 200
	if noLimit {
		maxFiles = len(files)
	}
	truncated := false
	if len(files) > maxFiles {
		truncated = true
		files = files[:maxFiles]
	}

	if truncated {
		sp.Stop()
		ui.PrintWarning(fmt.Sprintf("Analyzing first %d of %d files. Use --no-limit for all (slower).", maxFiles, len(allFiles)))
		sp = ui.StartSpinner(fmt.Sprintf("Blaming %d files...", len(files)))
	}

	type result struct {
		counts map[string]int
		emails map[string]string
	}
	results := make(chan result, len(files))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			out := git.RunUnchecked("blame", "--line-porcelain", f)
			counts := map[string]int{}
			emails := map[string]string{}
			var name, email string
			for _, line := range strings.Split(out, "\n") {
				switch {
				case strings.HasPrefix(line, "author "):
					name = strings.TrimPrefix(line, "author ")
				case strings.HasPrefix(line, "author-mail "):
					email = strings.Trim(strings.TrimPrefix(line, "author-mail "), "<>")
				case strings.HasPrefix(line, "author-tz "):
					if name != "" && name != "Not Committed Yet" {
						counts[name]++
						if _, exists := emails[name]; !exists {
							emails[name] = strings.ToLower(email)
						}
					}
				}
			}
			results <- result{counts, emails}
		}(file)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	totalCounts := map[string]int{}
	totalEmails := map[string]string{}
	filesTouched := map[string]int{}
	for r := range results {
		for name, lines := range r.counts {
			totalCounts[name] += lines
			filesTouched[name]++
		}
		for name, email := range r.emails {
			if _, exists := totalEmails[name]; !exists {
				totalEmails[name] = email
			}
		}
	}

	sp.Stop()

	totalLines := 0
	for _, c := range totalCounts {
		totalLines += c
	}
	if totalLines == 0 {
		ui.PrintInfo(fmt.Sprintf("No blame data for %s", dir))
		return nil
	}

	type entry struct {
		name  string
		lines int
		files int
		email string
	}
	var sorted []entry
	for name, lines := range totalCounts {
		sorted = append(sorted, entry{name, lines, filesTouched[name], totalEmails[name]})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].lines > sorted[j].lines })

	var buf bytes.Buffer
	var rows [][]string
	for i, e := range sorted {
		if i >= n {
			break
		}
		pct := fmt.Sprintf("%.1f%%", float64(e.lines)/float64(totalLines)*100)
		rows = append(rows, []string{
			ui.DimStyle.Render(fmt.Sprintf("%d", i+1)),
			e.name,
			ui.BoldStyle.Render(fmt.Sprintf("%d", e.lines)),
			pct,
			ui.DimStyle.Render(fmt.Sprintf("%d", e.files)),
			ui.DimStyle.Render(e.email),
		})
	}

	fmt.Fprintln(&buf)
	ui.PrintTableTo(&buf, []string{"#", "Author", "Lines", "%", "Files", "Email"}, rows,
		fmt.Sprintf("Ownership of %s (%d files, %d lines)", dir, len(files), totalLines))
	fmt.Print(buf.String())
	return nil
}

// --- Repo-level: git shortlog (fast, commits only) ---

func whoRepo(n int) error {
	sp := ui.StartSpinner("Analyzing contributors...")

	out, err := git.Run("shortlog", "-sne", "HEAD")
	sp.Stop()
	if err != nil || out == "" {
		ui.PrintInfo("No contributors found.")
		return nil
	}

	type entry struct {
		name    string
		email   string
		commits int
	}
	var sorted []entry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		var commits int
		fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &commits)
		nameEmail := strings.TrimSpace(parts[1])
		name, email := nameEmail, ""
		if idx := strings.Index(nameEmail, "<"); idx >= 0 {
			name = strings.TrimSpace(nameEmail[:idx])
			end := strings.Index(nameEmail, ">")
			if end > idx {
				email = strings.ToLower(nameEmail[idx+1 : end])
			}
		}
		sorted = append(sorted, entry{name, email, commits})
	}

	var rows [][]string
	for i, e := range sorted {
		if i >= n {
			break
		}
		rows = append(rows, []string{
			ui.DimStyle.Render(fmt.Sprintf("%d", i+1)),
			e.name,
			ui.BoldStyle.Render(fmt.Sprintf("%d", e.commits)),
			ui.DimStyle.Render(e.email),
		})
	}

	fmt.Println()
	ui.PrintTable([]string{"#", "Author", "Commits", "Email"}, rows, "Top contributors")
	return nil
}
