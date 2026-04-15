package stack

import (
	"fmt"
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

	// Self-heal: detect parents for unknown branches
	changed := false
	for branch := range branchSHAs {
		if branch == mainBranch {
			continue
		}
		if meta, ok := cfg.Branches[branch]; ok {
			if _, exists := branchSHAs[meta.Parent]; exists {
				continue
			}
		}
		if detected := detectParent(branch, branchSHAs); detected != "" {
			parentHead, _ := git.MergeBase(branch, detected)
			cfg.Branches[branch] = &BranchMeta{Parent: detected, ParentHead: parentHead}
			changed = true
		}
	}
	if changed {
		_ = cfg.Save()
	}

	// Build nodes
	current, _ := git.CurrentBranch()
	nodes := make(map[string]*BranchNode)
	for name, sha := range branchSHAs {
		nodes[name] = &BranchNode{Name: name, SHA: sha, IsHead: name == current}
	}

	// Wire parent-child links
	for childName, meta := range cfg.Branches {
		if child, ok := nodes[childName]; ok {
			if parent, ok := nodes[meta.Parent]; ok {
				parent.Children = append(parent.Children, child)
			}
		}
	}

	// Calculate ahead/behind
	for childName, meta := range cfg.Branches {
		child, childOK := nodes[childName]
		_, parentOK := nodes[meta.Parent]
		if childOK && parentOK {
			child.Ahead, child.Behind = git.AheadBehind(childName, meta.Parent)

			merged := git.RunUnchecked("branch", "--merged", meta.Parent)
			for _, line := range strings.Split(merged, "\n") {
				if strings.TrimSpace(strings.TrimLeft(line, "* ")) == childName {
					child.IsMerged = true
					break
				}
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

func detectParent(branch string, branchSHAs map[string]string) string {
	var bestParent string
	bestDistance := int(^uint(0) >> 1) // max int

	for candidate := range branchSHAs {
		if candidate == branch {
			continue
		}
		mb, err := git.MergeBase(branch, candidate)
		if err != nil || mb == "" {
			continue
		}
		countStr, err := git.Run("rev-list", "--count", mb+".."+branch)
		if err != nil {
			continue
		}
		var distance int
		if _, err := fmt.Sscanf(countStr, "%d", &distance); err == nil {
			if distance < bestDistance {
				bestDistance = distance
				bestParent = candidate
			}
		}
	}
	return bestParent
}
