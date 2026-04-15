package cmd

import (
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
	name    string
	emails  map[string]bool
	commits int
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
	// File or directory level not ported yet in this phase
	ui.PrintInfo("File/directory-level who is coming soon. Showing repo level.")
	return whoRepo(n, since)
}

func whoRepo(n int, since string) error {
	gitArgs := []string{"shortlog", "-sne", "--all"}
	if since != "" {
		gitArgs = append(gitArgs, "--since="+since)
	}
	out, err := git.Run(gitArgs...)
	if err != nil || out == "" {
		ui.PrintInfo("No contributors found.")
		return nil
	}

	// Parse entries
	type rawEntry struct {
		name    string
		email   string
		commits int
	}
	var raw []rawEntry
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
		raw = append(raw, rawEntry{name, email, commits})
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
	}

	// Sort by commits
	var sorted []*contributor
	for _, g := range groups {
		sorted = append(sorted, g)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].commits > sorted[j].commits
	})

	// Current user
	currentName := git.RunUnchecked("config", "user.name")
	currentEmail := strings.ToLower(git.RunUnchecked("config", "user.email"))

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
			displayName = "You"
		}

		var emailList []string
		for e := range c.emails {
			emailList = append(emailList, e)
		}
		sort.Strings(emailList)

		lastActive := getAuthorLastEdit(c.name, ".")
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			displayName,
			strings.Join(emailList, ", "),
			fmt.Sprintf("%d", c.commits),
			lastActive,
		})
	}

	fmt.Println()
	ui.PrintTable([]string{"#", "Author", "Email", "Commits", "Last Active"}, rows, "Top contributors")

	if currentName != "" || currentEmail != "" {
		fmt.Printf("\n%s\n", ui.DimStyle.Render(fmt.Sprintf("You: %s <%s>", currentName, currentEmail)))
	}
	return nil
}

func getAuthorLastEdit(author, path string) string {
	out := git.RunUnchecked("log", "-1", "--author="+author, "--format=%aI", "--", path)
	if out == "" {
		return "unknown"
	}
	return git.TimeAgo(out)
}
