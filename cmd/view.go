package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "view",
		Short: "Show the current stack at a glance",
		RunE:  runView,
	})
}

func runView(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	current, err := git.CurrentBranch()
	if err != nil {
		ui.PrintError("Not on a branch. Checkout a branch to view its stack.")
		return nil
	}

	cfg, err := stack.Load()
	if err != nil {
		ui.PrintError(err.Error())
		return nil
	}
	trunk := cfg.Meta.MainBranch
	if trunk == "" {
		trunk = git.HeadBranch()
	}

	// On trunk
	if current == trunk {
		showTrunkView(cfg, trunk)
		return nil
	}

	// Not in stack
	if _, ok := cfg.Branches[current]; !ok {
		fmt.Println()
		ui.PrintInfo(fmt.Sprintf("Current branch (%s) is not part of a stack.", current))
		ahead, behind := git.AheadBehind(current, trunk)
		fmt.Printf("  vs %s: %d ahead, %d behind\n", trunk, ahead, behind)
		fmt.Println()
		fmt.Println("  Tip: Use `gx stack` to start stacking, or `gx graph` to see all branches.")
		return nil
	}

	// Get full chain
	chain := stack.StackChain(current)
	descendants := stack.Descendants(current)

	var allBranches []string
	seen := map[string]bool{}
	for _, b := range chain {
		if b != trunk && !seen[b] {
			allBranches = append(allBranches, b)
			seen[b] = true
		}
	}
	for _, b := range descendants {
		if !seen[b] {
			allBranches = append(allBranches, b)
			seen[b] = true
		}
	}

	hasGH := isGHAvailable()

	fmt.Println()
	fmt.Println(ui.BoldStyle.Render(trunk))
	for _, branch := range allBranches {
		meta := cfg.Branches[branch]
		parent := trunk
		if meta != nil {
			parent = meta.Parent
		}
		ahead, _ := git.AheadBehind(branch, parent)
		age := git.TimeAgo(git.RunUnchecked("log", "-1", "--format=%aI", branch))

		parts := []string{fmt.Sprintf("  <- %-30s", branch)}

		if hasGH {
			pr := getPRStatus(branch)
			if pr != "" {
				parts = append(parts, fmt.Sprintf("%-22s", pr))
			} else {
				parts = append(parts, fmt.Sprintf("%-22s", "  no PR"))
			}
		}

		parts = append(parts, fmt.Sprintf("%d ahead   ", ahead))
		parts = append(parts, age)

		if branch == current {
			parts = append(parts, "  "+ui.SuccessStyle.Bold(true).Render("<"))
		}
		fmt.Println(strings.Join(parts, ""))
	}
	fmt.Println()
	return nil
}

func showTrunkView(cfg *stack.Config, trunk string) {
	fmt.Println()
	fmt.Printf("You're on %s (trunk).\n\n", ui.BoldStyle.Render(trunk))

	// Find direct children
	var children []string
	for name, meta := range cfg.Branches {
		if meta.Parent == trunk {
			children = append(children, name)
		}
	}
	sort.Strings(children)

	if len(children) == 0 {
		fmt.Println("  No stacked branches yet. Use `gx stack` to start.")
		return
	}

	fmt.Printf("Stacks branching from %s:\n", trunk)
	for _, child := range children {
		// Walk down the chain
		chain := []string{child}
		cur := child
		visited := map[string]bool{child: true}
		for {
			kids := stack.Children(cur)
			if len(kids) == 0 {
				break
			}
			if visited[kids[0]] {
				break
			}
			visited[kids[0]] = true
			chain = append(chain, kids[0])
			cur = kids[0]
		}
		if len(chain) == 1 {
			fmt.Printf("  <- %s\n", child)
		} else {
			fmt.Printf("  <- %s   (%d branches)\n", strings.Join(chain, " -> "), len(chain))
		}
	}
	fmt.Println()
	fmt.Println("Use `gx graph` to see the full tree.")
}

func isGHAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func getPRStatus(branch string) string {
	cmd := exec.Command("gh", "pr", "view", branch, "--json", "number,state,reviewDecision")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var pr struct {
		Number         int    `json:"number"`
		State          string `json:"state"`
		ReviewDecision string `json:"reviewDecision"`
	}
	if json.Unmarshal(out, &pr) != nil {
		return ""
	}
	switch {
	case pr.State == "MERGED":
		return fmt.Sprintf("#%d  + merged", pr.Number)
	case pr.ReviewDecision == "APPROVED":
		return fmt.Sprintf("#%d  + approved", pr.Number)
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return fmt.Sprintf("#%d  x changes", pr.Number)
	case pr.State == "OPEN":
		return fmt.Sprintf("#%d  o reviewing", pr.Number)
	default:
		return fmt.Sprintf("#%d  %s", pr.Number, strings.ToLower(pr.State))
	}
}
