package stack

import (
	"sort"
	"strings"

	"github.com/mubbie/gx-cli/internal/git"
)

// BranchNode represents a branch in the visual tree.
type BranchNode struct {
	Name     string
	SHA      string
	IsHead   bool
	IsOrphan bool
	IsMerged bool
	Ahead    int
	Behind   int
	Children []*BranchNode
}

// BranchStack is the full branch hierarchy.
type BranchStack struct {
	Roots      []*BranchNode
	AllNodes   map[string]*BranchNode
	MainBranch string
	Orphans    []*BranchNode
}

// BuildTree constructs the full branch hierarchy with self-healing.
func BuildTree() *BranchStack {
	mainBranch := git.HeadBranch()
	cfg, _ := Load()

	// Get all local branches with SHAs
	output := git.RunUnchecked("for-each-ref", "--format=%(refname:short)\t%(objectname)", "refs/heads/")
	branchSHAs := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && parts[0] != "" {
			branchSHAs[parts[0]] = parts[1]
		}
	}

	if len(branchSHAs) == 0 {
		return &BranchStack{MainBranch: mainBranch, AllNodes: make(map[string]*BranchNode)}
	}

	// Clean stale entries
	for name := range cfg.Branches {
		if _, exists := branchSHAs[name]; !exists {
			delete(cfg.Branches, name)
		}
	}

	// Clean stale parent references (parent branch was deleted)
	changed := false
	for name, meta := range cfg.Branches {
		if _, exists := branchSHAs[meta.Parent]; !exists {
			// Parent no longer exists, default to trunk
			if meta.Parent != mainBranch {
				cfg.Branches[name] = &BranchMeta{Parent: mainBranch, ParentHead: meta.ParentHead}
				changed = true
			}
		}
	}
	if changed {
		_ = cfg.Save()
	}

	// No self-healing: only show branches explicitly in stack.json.
	// Use `gx stack` to add branches to the stack.

	// Build nodes only for trunk + branches in the stack config
	current, _ := git.CurrentBranch()
	nodes := make(map[string]*BranchNode)

	// Always include trunk
	if sha, exists := branchSHAs[mainBranch]; exists {
		nodes[mainBranch] = &BranchNode{Name: mainBranch, SHA: sha, IsHead: mainBranch == current}
	}
	// Include all stacked branches and their parents
	for name, meta := range cfg.Branches {
		if sha, exists := branchSHAs[name]; exists {
			nodes[name] = &BranchNode{Name: name, SHA: sha, IsHead: name == current}
		}
		if _, exists := nodes[meta.Parent]; !exists {
			if sha, exists := branchSHAs[meta.Parent]; exists {
				nodes[meta.Parent] = &BranchNode{Name: meta.Parent, SHA: sha, IsHead: meta.Parent == current}
			}
		}
	}

	// Wire parent-child links
	for childName, meta := range cfg.Branches {
		if child, ok := nodes[childName]; ok {
			if parent, ok := nodes[meta.Parent]; ok {
				parent.Children = append(parent.Children, child)
			}
		}
	}

	// Calculate ahead/behind and merged status
	// Batch merged check: one call per unique parent instead of per child
	mergedCache := map[string]map[string]bool{} // parent -> set of merged branches
	for childName, meta := range cfg.Branches {
		child, childOK := nodes[childName]
		_, parentOK := nodes[meta.Parent]
		if childOK && parentOK {
			child.Ahead, child.Behind = git.AheadBehind(childName, meta.Parent)

			// Cache merged branches per parent
			if _, ok := mergedCache[meta.Parent]; !ok {
				mergedSet := map[string]bool{}
				out := git.RunUnchecked("branch", "--merged", meta.Parent)
				for _, line := range strings.Split(out, "\n") {
					name := strings.TrimSpace(strings.TrimLeft(line, "* "))
					if name != "" {
						mergedSet[name] = true
					}
				}
				mergedCache[meta.Parent] = mergedSet
			}
			if mergedCache[meta.Parent][childName] {
				child.IsMerged = true
			}
		}
	}

	// Classify roots and orphans
	inConfig := make(map[string]bool)
	for name := range cfg.Branches {
		inConfig[name] = true
	}

	var roots, orphans []*BranchNode
	for name, node := range nodes {
		if !inConfig[name] {
			if len(node.Children) > 0 || name == mainBranch {
				roots = append(roots, node)
			} else if name != mainBranch {
				node.IsOrphan = true
				orphans = append(orphans, node)
			}
		}
	}

	sort.Slice(roots, func(i, j int) bool {
		if roots[i].Name == mainBranch {
			return true
		}
		if roots[j].Name == mainBranch {
			return false
		}
		return roots[i].Name < roots[j].Name
	})

	var sortChildren func(n *BranchNode, seen map[string]bool)
	sortChildren = func(n *BranchNode, seen map[string]bool) {
		if seen[n.Name] {
			return
		}
		seen[n.Name] = true
		sort.Slice(n.Children, func(i, j int) bool {
			return n.Children[i].Name < n.Children[j].Name
		})
		for _, child := range n.Children {
			sortChildren(child, seen)
		}
	}
	seen := make(map[string]bool)
	for _, root := range roots {
		sortChildren(root, seen)
	}

	return &BranchStack{
		Roots:      roots,
		AllNodes:   nodes,
		MainBranch: mainBranch,
		Orphans:    orphans,
	}
}

