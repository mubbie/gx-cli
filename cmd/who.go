package cmd

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

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

	if len(args) == 0 {
		return whoRepo(n, since)
	}
	ui.PrintInfo("File/directory-level who is coming soon. Showing repo level.")
	return whoRepo(n, since)
}

func whoRepo(n int, since string) error {
	sp := ui.StartSpinner("Analyzing contributors (this may take a moment)...")

	// Run both git commands in parallel
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

		// Lines and percentage
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
	}

	// Stop spinner AFTER all data is ready, BEFORE printing
	sp.Stop()

	fmt.Fprintln(&buf)
	// Print table
	ui.PrintTableTo(&buf, []string{"#", "Author", "Lines", "%", "Commits", "Last Active"}, rows, "Top contributors")

	if currentName != "" || currentEmail != "" {
		fmt.Fprintf(&buf, "\n%s\n", ui.DimStyle.Render(fmt.Sprintf("You: %s <%s>", currentName, currentEmail)))
	}

	// Print all at once
	fmt.Print(buf.String())
	return nil
}

