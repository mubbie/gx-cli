package cmd

import (
	"bytes"
	"fmt"

	"github.com/mubbie/gx-cli/internal/git"
	"github.com/mubbie/gx-cli/internal/stack"
	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "graph",
		Short: "Visualize the branch stack tree",
		RunE:  runGraph,
	})
}

func runGraph(cmd *cobra.Command, args []string) error {
	if err := git.EnsureRepo(); err != nil {
		ui.PrintError(err.Error())
		return nil
	}

	sp := ui.StartSpinner("Building branch tree...")
	tree := stack.BuildTree()

	if len(tree.Roots) == 0 && len(tree.Orphans) == 0 {
		sp.Stop()
		fmt.Println("No branches found.")
		return nil
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, ui.BoldStyle.Render("Branch Stack:"))
	fmt.Fprintln(&buf)

	for i, root := range tree.Roots {
		isLast := i == len(tree.Roots)-1 && len(tree.Orphans) == 0
		renderNodeTo(&buf, root, "", isLast)
	}

	if len(tree.Orphans) > 0 {
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, ui.WarningStyle.Bold(true).Render("Orphaned Branches:"))
		for i, orphan := range tree.Orphans {
			renderNodeTo(&buf, orphan, "", i == len(tree.Orphans)-1)
		}
	}

	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, ui.DimStyle.Render("Legend: * current branch  + merged  (+ahead/-behind)  ! orphaned"))
	fmt.Fprintln(&buf, ui.DimStyle.Render("Relationships stored in .git/gx/stack.json"))
	fmt.Fprintln(&buf)

	sp.Stop()
	fmt.Print(buf.String())
	return nil
}

func renderNodeTo(buf *bytes.Buffer, node *stack.BranchNode, prefix string, isLast bool) {
	connector := ui.DimStyle.Render("|-- ")
	if isLast {
		connector = ui.DimStyle.Render("`-- ")
	}

	// Status indicators
	indicators := ""
	if node.IsHead {
		indicators += "  " + ui.HeadMarker.Render("* HEAD")
	}
	if node.IsMerged {
		indicators += "  " + ui.DimStyle.Render("+ merged")
	} else if node.IsOrphan {
		indicators += "  " + ui.WarningStyle.Render("! orphaned")
	} else if node.Ahead > 0 || node.Behind > 0 {
		indicators += "  " + ui.BranchStyle.Render(fmt.Sprintf("(+%d/-%d)", node.Ahead, node.Behind))
	}

	// Branch name color
	var name string
	switch {
	case node.IsHead:
		name = ui.HeadMarker.Render(node.Name)
	case node.IsMerged:
		name = ui.DimStyle.Render(node.Name)
	case node.IsOrphan:
		name = ui.WarningStyle.Render(node.Name)
	default:
		name = ui.BranchStyle.Render(node.Name)
	}

	fmt.Fprintf(buf, "%s%s%s%s\n", prefix, connector, name, indicators)

	childPrefix := prefix + ui.DimStyle.Render("|") + "   "
	if isLast {
		childPrefix = prefix + "    "
	}
	for i, child := range node.Children {
		renderNodeTo(buf, child, childPrefix, i == len(node.Children)-1)
	}
}
