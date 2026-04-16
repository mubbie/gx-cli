// Package stack manages the .git/gx/stack.json configuration and branch tree.
package stack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mubbie/gx-cli/internal/git"
)

// BranchMeta stores the relationship and state for a stacked branch.
type BranchMeta struct {
	Parent     string `json:"parent"`
	ParentHead string `json:"parent_head"`
	PRNumber   *int   `json:"pr_number,omitempty"`
}

// Metadata stores global config info.
type Metadata struct {
	MainBranch  string `json:"main_branch"`
	LastUpdated string `json:"last_updated"`
}

// Config is the in-memory representation of stack.json.
type Config struct {
	Branches map[string]*BranchMeta `json:"branches"`
	Meta     Metadata               `json:"metadata"`
}

// configDir returns the path to .git/gx/.
func configDir() (string, error) {
	root, err := git.RepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".git", "gx"), nil
}

// configPath returns the path to .git/gx/stack.json.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "stack.json"), nil
}

// Load reads stack.json from disk. Returns empty config if missing.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{
			Branches: make(map[string]*BranchMeta),
			Meta:     Metadata{MainBranch: git.HeadBranch()},
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt stack.json: %w", err)
	}
	if cfg.Branches == nil {
		cfg.Branches = make(map[string]*BranchMeta)
	}

	// Migrate legacy {"relationships": {"child": "parent"}} format
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		if _, hasRel := raw["relationships"]; hasRel {
			if _, hasBranches := raw["branches"]; !hasBranches {
				var relationships map[string]string
				if err := json.Unmarshal(raw["relationships"], &relationships); err == nil {
					cfg.Branches = make(map[string]*BranchMeta, len(relationships))
					for child, parent := range relationships {
						cfg.Branches[child] = &BranchMeta{Parent: parent, ParentHead: ""}
					}
				}
			}
		}
	}
	if cfg.Meta.MainBranch == "" {
		cfg.Meta.MainBranch = git.HeadBranch()
	}
	return &cfg, nil
}

// Save writes the config to .git/gx/stack.json.
func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	c.Meta.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	// Atomic write: temp file + rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RecordRelationship adds or updates a parent-child relationship.
func RecordRelationship(child, parent string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	parentHead, _ := git.Run("rev-parse", parent)
	cfg.Branches[child] = &BranchMeta{Parent: parent, ParentHead: parentHead}
	return cfg.Save()
}

// UpdateParentHead updates the stored parent HEAD for a branch.
func UpdateParentHead(child, newHead string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	if meta, ok := cfg.Branches[child]; ok {
		meta.ParentHead = newHead
		return cfg.Save()
	}
	return nil
}

// ParentOf returns the parent from an already-loaded config.
func (c *Config) ParentOf(branch string) string {
	if meta, ok := c.Branches[branch]; ok {
		return meta.Parent
	}
	return ""
}

// ParentHeadOf returns the stored parent HEAD SHA from an already-loaded config.
func (c *Config) ParentHeadOf(branch string) string {
	if meta, ok := c.Branches[branch]; ok {
		return meta.ParentHead
	}
	return ""
}

// ChildrenOf returns all branches that have the given branch as their parent.
func (c *Config) ChildrenOf(branch string) []string {
	var result []string
	for name, meta := range c.Branches {
		if meta.Parent == branch {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

// StackChainOf walks up from branch to root, returns [root, ..., branch].
func (c *Config) StackChainOf(branch string) []string {
	chain := []string{branch}
	visited := map[string]bool{branch: true}
	current := branch
	for {
		meta, ok := c.Branches[current]
		if !ok {
			break
		}
		if visited[meta.Parent] {
			break
		}
		visited[meta.Parent] = true
		chain = append(chain, meta.Parent)
		current = meta.Parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// DescendantsOf walks down from branch, returns all descendants in order.
func (c *Config) DescendantsOf(branch string) []string {
	var result []string
	visited := map[string]bool{branch: true}
	var walk func(current string)
	walk = func(current string) {
		for _, child := range c.ChildrenOf(current) {
			if visited[child] {
				continue
			}
			visited[child] = true
			result = append(result, child)
			walk(child)
		}
	}
	walk(branch)
	return result
}

// Parent returns the parent branch name, or empty string.
func Parent(branch string) string {
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.ParentOf(branch)
}

// ParentHead returns the stored parent HEAD SHA, or empty string.
func ParentHead(branch string) string {
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.ParentHeadOf(branch)
}

// Children returns all branches that have the given branch as their parent.
func Children(branch string) []string {
	cfg, err := Load()
	if err != nil {
		return nil
	}
	return cfg.ChildrenOf(branch)
}

// StackChain walks up from branch to root, returns [root, ..., branch].
func StackChain(branch string) []string {
	cfg, err := Load()
	if err != nil {
		return []string{branch}
	}
	return cfg.StackChainOf(branch)
}

// Descendants walks down from branch, returns all descendants in order.
func Descendants(branch string) []string {
	cfg, err := Load()
	if err != nil {
		return nil
	}
	return cfg.DescendantsOf(branch)
}

// RemoveBranch removes a branch from config (as child and parent).
func RemoveBranch(branch string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	delete(cfg.Branches, branch)
	for name, meta := range cfg.Branches {
		if meta.Parent == branch {
			delete(cfg.Branches, name)
		}
	}
	return cfg.Save()
}

// CleanDeleted removes entries for branches that no longer exist locally.
func CleanDeleted() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	changed := false
	for name := range cfg.Branches {
		if !git.BranchExists(name) {
			delete(cfg.Branches, name)
			changed = true
		}
	}
	if changed {
		return cfg.Save()
	}
	return nil
}

// ConfigExists returns true if .git/gx/stack.json exists.
func ConfigExists() bool {
	path, err := configPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
