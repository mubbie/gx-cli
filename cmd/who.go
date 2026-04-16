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
	cmd.Flags().String("since", "", "Only consider commits after this date")
	cmd.Flags().Bool("no-limit", false, "Remove file cap for directory analysis")
	cmd.Flags().Bool("lines", false, "Show line stats (slower, scans full history)")
	rootCmd.AddCommand(cmd)
}

type contributor struct {
	name       string
	emails     map[string]bool
	commits    int
	added      int
	deleted    int
	lastActive string // ISO date
}

func runWho(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	n, _ := cmd.Flags().GetInt("number")
	since, _ := cmd.Flags().GetString("since")
	showLines, _ := cmd.Flags().GetBool("lines")

	if len(args) == 0 {
		return whoRepo(n, since, showLines)
	}

	path := args[0]
	// Check if it's a file or directory
	info, err := os.Stat(path)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Path not found: %s", path))
		return nil
	}
	if info.IsDir() {
		noLimit, _ := cmd.Flags().GetBool("no-limit")
		return whoDir(path, n, since, noLimit)
	}
	return whoFile(path, n, since)
}

func whoRepo(n int, since string, showLines bool) error {
	sp := ui.StartSpinner("Analyzing contributors...")

	// Always run shortlog (fast) + shortstat with dates in parallel
	shortlogArgs := []string{"shortlog", "-sne", "--all"}
	if since != "" {
		shortlogArgs = append(shortlogArgs, "--since="+since)
	}
	shortstatArgs := []string{"log", "--all", "--shortstat", "--format=%aE|%aI"}
	if since != "" {
		shortstatArgs = append(shortstatArgs, "--since="+since)
	}

	var shortlogOut, numstatOut string
	var err error

	// Run in parallel
	done := make(chan struct{})
	go func() {
		shortlogOut, err = git.Run(shortlogArgs...)
		close(done)
	}()
	numstatOut = git.RunUnchecked(shortstatArgs...)
	<-done

	// Parse shortlog
	type rawEntry struct {
		name    string
		email   string
		commits int
	}
	var raw []rawEntry
	if err == nil && shortlogOut != "" {
		for _, line := range strings.Split(shortlogOut, "\n") {
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
			raw = append(raw, rawEntry{name, email, commits})
		}
	}

	// Parse shortstat to get lines and last-active per email
	linesByEmail := map[string][2]int{}    // email -> [added, deleted]
	lastActiveByEmail := map[string]string{} // email -> ISO date (first seen = most recent)
	if numstatOut != "" {
		var currentEmail, currentDate string
		for _, line := range strings.Split(numstatOut, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Format lines: "email|date" from --format=%aE|%aI
			if strings.Contains(line, "@") && strings.Contains(line, "|") {
				parts := strings.SplitN(line, "|", 2)
				currentEmail = strings.ToLower(parts[0])
				if len(parts) > 1 {
					currentDate = parts[1]
				}
				// Track first (most recent) date per email
				if _, exists := lastActiveByEmail[currentEmail]; !exists {
					lastActiveByEmail[currentEmail] = currentDate
				}
				continue
			}
			// Shortstat lines
			if strings.Contains(line, "file") && strings.Contains(line, "changed") && currentEmail != "" {
				var a, d int
				if idx := strings.Index(line, "insertion"); idx > 0 {
					sub := strings.TrimSpace(line[:idx])
					if ci := strings.LastIndex(sub, " "); ci >= 0 {
						fmt.Sscanf(strings.TrimSpace(sub[ci+1:]), "%d", &a)
					}
				}
				if idx := strings.Index(line, "deletion"); idx > 0 {
					sub := strings.TrimSpace(line[:idx])
					if ci := strings.LastIndex(sub, " "); ci >= 0 {
						fmt.Sscanf(strings.TrimSpace(sub[ci+1:]), "%d", &d)
					}
				}
				stats := linesByEmail[currentEmail]
				stats[0] += a
				stats[1] += d
				linesByEmail[currentEmail] = stats
			}
		}
	}

	if len(raw) == 0 {
		sp.Stop()
		ui.PrintInfo("No contributors found.")
		return nil
	}

	// Union-find dedup
	parent := make([]int, len(raw))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	keyMap := map[string]int{}
	for i, e := range raw {
		keys := []string{}
		if e.email != "" {
			keys = append(keys, "email:"+e.email)
			user := strings.SplitN(e.email, "@", 2)[0]
			if user != "" {
				keys = append(keys, "user:"+user)
			}
		}
		if e.name != "" {
			keys = append(keys, "name:"+strings.ToLower(e.name))
		}
		for _, k := range keys {
			if prev, ok := keyMap[k]; ok {
				union(i, prev)
			} else {
				keyMap[k] = i
			}
		}
	}

	// Merge groups
	groups := map[int]*contributor{}
	groupEmails := map[int]map[string]bool{}
	for i, e := range raw {
		root := find(i)
		if g, ok := groups[root]; ok {
			g.commits += e.commits
			if len(e.name) > len(g.name) {
				g.name = e.name
			}
		} else {
			groups[root] = &contributor{name: e.name, emails: map[string]bool{}, commits: e.commits}
			groupEmails[root] = map[string]bool{}
		}
		if e.email != "" {
			groupEmails[root][e.email] = true
		}
	}
	for root, emails := range groupEmails {
		groups[root].emails = emails
		for email := range emails {
			if stats, ok := linesByEmail[email]; ok {
				groups[root].added += stats[0]
				groups[root].deleted += stats[1]
			}
			if date, ok := lastActiveByEmail[email]; ok {
				if groups[root].lastActive == "" || date > groups[root].lastActive {
					groups[root].lastActive = date
				}
			}
		}
	}

	// Calculate total lines for percentage
	totalLines := 0
	for _, g := range groups {
		totalLines += g.added
	}

	// Sort by lines added (primary), commits (secondary)
	var sorted []*contributor
	for _, g := range groups {
		sorted = append(sorted, g)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].added != sorted[j].added {
			return sorted[i].added > sorted[j].added
		}
		return sorted[i].commits > sorted[j].commits
	})

	// Current user
	currentName := git.RunUnchecked("config", "user.name")
	currentEmail := strings.ToLower(git.RunUnchecked("config", "user.email"))

	// Build output into buffer, then stop spinner and print
	var buf bytes.Buffer

	var rows [][]string
	for i, c := range sorted {
		if i >= n {
			break
		}
		isYou := false
		if currentEmail != "" {
			for e := range c.emails {
				if e == currentEmail {
					isYou = true
					break
				}
			}
		}
		if !isYou && currentName != "" && c.name == currentName {
			isYou = true
		}

		displayName := c.name
		if isYou {
			displayName = ui.SuccessStyle.Bold(true).Render("You")
		}

		var emailList []string
		for e := range c.emails {
			emailList = append(emailList, e)
		}
		sort.Strings(emailList)

		lastActive := "unknown"
		if c.lastActive != "" {
			lastActive = git.TimeAgo(c.lastActive)
		}

		if showLines {
			linesStr := ui.AddStyle.Render(fmt.Sprintf("+%d", c.added)) + " " + ui.DelStyle.Render(fmt.Sprintf("-%d", c.deleted))
			pct := ""
			if totalLines > 0 {
				pct = fmt.Sprintf("%.1f%%", float64(c.added)/float64(totalLines)*100)
			}
			rows = append(rows, []string{
				ui.DimStyle.Render(fmt.Sprintf("%d", i+1)),
				displayName,
				linesStr,
				pct,
				ui.BoldStyle.Render(fmt.Sprintf("%d", c.commits)),
				ui.DateStyle.Render(lastActive),
			})
		} else {
			rows = append(rows, []string{
				ui.DimStyle.Render(fmt.Sprintf("%d", i+1)),
				displayName,
				ui.BoldStyle.Render(fmt.Sprintf("%d", c.commits)),
				ui.DateStyle.Render(lastActive),
			})
		}
	}

	sp.Stop()

	fmt.Fprintln(&buf)
	var headers []string
	if showLines {
		headers = []string{"#", "Author", "Lines", "%", "Commits", "Last Active"}
	} else {
		headers = []string{"#", "Author", "Commits", "Last Active"}
	}
	ui.PrintTableTo(&buf, headers, rows, "Top contributors")

	if currentName != "" || currentEmail != "" {
		fmt.Fprintf(&buf, "\n%s\n", ui.DimStyle.Render(fmt.Sprintf("You: %s <%s>", currentName, currentEmail)))
	}

	// Print all at once
	fmt.Print(buf.String())
	return nil
}

// whoFile shows line ownership for a single file via git blame.
func whoFile(path string, n int, since string) error {
	sp := ui.StartSpinner(fmt.Sprintf("Analyzing %s...", path))

	blameArgs := []string{"blame", "--line-porcelain"}
	if since != "" {
		blameArgs = append(blameArgs, "--since", since)
	}
	blameArgs = append(blameArgs, path)

	out, err := git.Run(blameArgs...)
	sp.Stop()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to blame %s: %s", path, err))
		return nil
	}

	// Parse blame output: count lines per author
	counts := map[string]int{}      // author -> lines
	emails := map[string]string{}   // author -> email
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "author ") {
			name := strings.TrimPrefix(line, "author ")
			if name != "Not Committed Yet" {
				counts[name]++
			}
		}
		if strings.HasPrefix(line, "author-mail ") {
			mail := strings.TrimPrefix(line, "author-mail ")
			mail = strings.Trim(mail, "<>")
			// Find the last author we incremented
			for name := range counts {
				if _, exists := emails[name]; !exists {
					emails[name] = strings.ToLower(mail)
				}
			}
		}
	}

	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		ui.PrintInfo(fmt.Sprintf("No blame data for %s", path))
		return nil
	}

	// Sort by line count
	type entry struct {
		name  string
		lines int
	}
	var sorted []entry
	for name, lines := range counts {
		sorted = append(sorted, entry{name, lines})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].lines > sorted[j].lines
	})

	currentName := git.RunUnchecked("config", "user.name")

	var rows [][]string
	for i, e := range sorted {
		if i >= n {
			break
		}
		displayName := e.name
		if e.name == currentName {
			displayName = ui.SuccessStyle.Bold(true).Render("You")
		}
		pct := fmt.Sprintf("%.1f%%", float64(e.lines)/float64(total)*100)
		rows = append(rows, []string{
			ui.DimStyle.Render(fmt.Sprintf("%d", i+1)),
			displayName,
			ui.BoldStyle.Render(fmt.Sprintf("%d", e.lines)),
			pct,
		})
	}

	fmt.Println()
	ui.PrintTable([]string{"#", "Author", "Lines", "%"}, rows, fmt.Sprintf("Ownership of %s (%d lines)", path, total))
	return nil
}

// whoDir shows line ownership across all files in a directory via git blame.
func whoDir(dir string, n int, since string, noLimit bool) error {
	sp := ui.StartSpinner(fmt.Sprintf("Analyzing %s (this may take a moment)...", dir))

	// Get tracked files
	filesOut, err := git.Run("ls-files", dir)
	if err != nil || filesOut == "" {
		sp.Stop()
		ui.PrintInfo(fmt.Sprintf("No tracked files in %s", dir))
		return nil
	}

	files := strings.Split(strings.TrimSpace(filesOut), "\n")
	maxFiles := 200
	if noLimit {
		maxFiles = len(files)
	}
	if len(files) > maxFiles {
		sp.Stop()
		ui.PrintWarning(fmt.Sprintf("%s contains %d tracked files. Analyzing first %d. Use --no-limit to analyze all.", dir, len(files), maxFiles))
		files = files[:maxFiles]
		sp = ui.StartSpinner(fmt.Sprintf("Analyzing %d files...", len(files)))
	}

	// Blame all files concurrently (capped at 8 workers)
	type blameResult struct {
		counts map[string]int
	}
	results := make(chan blameResult, len(files))
	sem := make(chan struct{}, 8) // concurrency limit
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

			blameArgs := []string{"blame", "--line-porcelain"}
			if since != "" {
				blameArgs = append(blameArgs, "--since", since)
			}
			blameArgs = append(blameArgs, f)

			out := git.RunUnchecked(blameArgs...)
			counts := map[string]int{}
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "author ") {
					name := strings.TrimPrefix(line, "author ")
					if name != "Not Committed Yet" {
						counts[name]++
					}
				}
			}
			results <- blameResult{counts}
		}(file)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate
	totalCounts := map[string]int{}
	filesTouched := map[string]int{}
	for r := range results {
		for name, lines := range r.counts {
			totalCounts[name] += lines
			filesTouched[name]++
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
	}
	var sorted []entry
	for name, lines := range totalCounts {
		sorted = append(sorted, entry{name, lines, filesTouched[name]})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].lines > sorted[j].lines
	})

	currentName := git.RunUnchecked("config", "user.name")

	var rows [][]string
	for i, e := range sorted {
		if i >= n {
			break
		}
		displayName := e.name
		if e.name == currentName {
			displayName = ui.SuccessStyle.Bold(true).Render("You")
		}
		pct := fmt.Sprintf("%.1f%%", float64(e.lines)/float64(totalLines)*100)
		rows = append(rows, []string{
			ui.DimStyle.Render(fmt.Sprintf("%d", i+1)),
			displayName,
			ui.BoldStyle.Render(fmt.Sprintf("%d", e.lines)),
			pct,
			ui.DimStyle.Render(fmt.Sprintf("%d", e.files)),
		})
	}

	fmt.Println()
	ui.PrintTable([]string{"#", "Author", "Lines", "%", "Files Touched"}, rows,
		fmt.Sprintf("Ownership of %s (%d files, %d lines)", dir, len(files), totalLines))
	return nil
}

