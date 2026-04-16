package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
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
	chain := cfg.StackChainOf(current)
	descendants := cfg.DescendantsOf(current)

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
	var prMap map[string]prInfo
	var sp *ui.Spinner
	if hasGH {
		sp = ui.StartSpinner("Fetching PR status...")
		prMap = fetchPRMap()
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, ui.BranchStyle.Render(trunk))
	for _, branch := range allBranches {
		meta := cfg.Branches[branch]
		parent := trunk
		if meta != nil {
			parent = meta.Parent
		}
		ahead, _ := git.AheadBehind(branch, parent)
		age := git.TimeAgo(git.RunUnchecked("log", "-1", "--format=%aI", branch))

		parts := []string{fmt.Sprintf("  <- %-30s", ui.BranchStyle.Render(branch))}

		if hasGH {
			if pr, ok := prMap[branch]; ok {
				parts = append(parts, fmt.Sprintf("%-22s", formatPRStatus(pr)))
			} else {
				parts = append(parts, fmt.Sprintf("%-22s", ui.DimStyle.Render("  no PR")))
			}
		}

		parts = append(parts, ui.AddStyle.Render(fmt.Sprintf("%d ahead", ahead))+"   ")
		parts = append(parts, ui.DateStyle.Render(age))

		if branch == current {
			parts = append(parts, "  "+ui.HeadMarker.Render("<"))
		}
		fmt.Fprintln(&buf, strings.Join(parts, ""))
	}
	fmt.Fprintln(&buf)

	if sp != nil {
		sp.Stop()
	}
	fmt.Print(buf.String())
	return nil
}

func showTrunkView(cfg *stack.Config, trunk string) {
	fmt.Println()
	fmt.Printf("You're on %s %s\n\n", ui.BranchStyle.Render(trunk), ui.DimStyle.Render("(trunk)"))

	// Find direct children
	children := cfg.ChildrenOf(trunk)

	if len(children) == 0 {
		fmt.Println("  No stacked branches yet. Use `gx stack` to start.")
		return
	}

	fmt.Printf("Stacks branching from %s:\n", ui.BranchStyle.Render(trunk))
	for _, child := range children {
		// Walk down the chain
		chain := []string{child}
		cur := child
		visited := map[string]bool{child: true}
		for {
			kids := cfg.ChildrenOf(cur)
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
		coloredChain := make([]string, len(chain))
		for i, c := range chain {
			coloredChain[i] = ui.BranchStyle.Render(c)
		}
		if len(chain) == 1 {
			fmt.Printf("  <- %s\n", coloredChain[0])
		} else {
			fmt.Printf("  <- %s   %s\n", strings.Join(coloredChain, " -> "), ui.DimStyle.Render(fmt.Sprintf("(%d branches)", len(chain))))
		}
	}
	fmt.Println()
	fmt.Println("Use `gx graph` to see the full tree.")
}

func isGHAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

type prInfo struct {
	Number         int    `json:"number"`
	HeadRefName    string `json:"headRefName"`
	State          string `json:"state"`
	ReviewDecision string `json:"reviewDecision"`
}

func fetchPRMap() map[string]prInfo {
	cmd := exec.Command("gh", "pr", "list", "--json", "number,headRefName,state,reviewDecision", "--limit", "100")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var prs []prInfo
	if json.Unmarshal(out, &prs) != nil {
		return nil
	}
	m := make(map[string]prInfo, len(prs))
	for _, pr := range prs {
		m[pr.HeadRefName] = pr
	}
	return m
}

func formatPRStatus(pr prInfo) string {
	switch {
	case pr.State == "MERGED":
		return fmt.Sprintf("#%d  %s", pr.Number, ui.DimStyle.Render("+ merged"))
	case pr.ReviewDecision == "APPROVED":
		return fmt.Sprintf("#%d  %s", pr.Number, ui.SuccessStyle.Render("+ approved"))
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return fmt.Sprintf("#%d  %s", pr.Number, ui.ErrorStyle.Render("x changes"))
	case pr.State == "OPEN":
		return fmt.Sprintf("#%d  %s", pr.Number, ui.WarningStyle.Render("o reviewing"))
	default:
		return fmt.Sprintf("#%d  %s", pr.Number, strings.ToLower(pr.State))
	}
}
