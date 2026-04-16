package stack

import "sort"

// TopoSortWith sorts branches so parents always come before children,
// using an already-loaded config to avoid redundant Load calls.
func TopoSortWith(cfg *Config, branches []string) []string {
	branchSet := make(map[string]bool)
	for _, b := range branches {
		branchSet[b] = true
	}

	childrenOf := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, b := range branches {
		childrenOf[b] = nil
		inDegree[b] = 0
	}

	for _, b := range branches {
		if meta, ok := cfg.Branches[b]; ok {
			if branchSet[meta.Parent] {
				childrenOf[meta.Parent] = append(childrenOf[meta.Parent], b)
				inDegree[b]++
			}
		}
	}

	// Kahn's algorithm
	var queue []string
	for _, b := range branches {
		if inDegree[b] == 0 {
			queue = append(queue, b)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		children := childrenOf[current]
		sort.Strings(children)
		for _, child := range children {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	// Append any unresolved (cycle) branches
	if len(result) < len(branches) {
		resolved := make(map[string]bool)
		for _, b := range result {
			resolved[b] = true
		}
		for _, b := range branches {
			if !resolved[b] {
				result = append(result, b)
			}
		}
	}

	return result
}

// TopoSort sorts branches so parents always come before children.
func TopoSort(branches []string) []string {
	cfg, err := Load()
	if err != nil {
		return branches
	}
	return TopoSortWith(cfg, branches)
}
