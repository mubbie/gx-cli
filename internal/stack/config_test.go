package stack

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// chdir changes to dir and registers a cleanup to restore the original directory.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

func setupStackRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "Initial commit")
	return dir
}

func writeStackConfig(t *testing.T, dir string, config map[string]interface{}) {
	t.Helper()
	gxDir := filepath.Join(dir, ".git", "gx")
	if err := os.MkdirAll(gxDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gxDir, "stack.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestLoadEmpty(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Branches) != 0 {
		t.Errorf("expected empty branches, got %d", len(cfg.Branches))
	}
}

func TestLoadExisting(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	writeStackConfig(t, dir, map[string]interface{}{
		"branches": map[string]interface{}{
			"feature/a": map[string]interface{}{"parent": "main", "parent_head": "abc123"},
		},
		"metadata": map[string]interface{}{"main_branch": "main"},
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Branches["feature/a"] == nil {
		t.Error("feature/a should be in config")
	}
	if cfg.Branches["feature/a"].Parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", cfg.Branches["feature/a"].Parent)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	cfg := &Config{
		Branches: map[string]*BranchMeta{
			"feature/x": {Parent: "main", ParentHead: "abc"},
		},
		Meta: Metadata{MainBranch: "main"},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}
	if loaded.Branches["feature/x"] == nil {
		t.Error("feature/x should exist after save+load")
	}
}

func TestRecordRelationship(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	// Create the branch first
	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", "feature/a").CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b feature/a failed: %s\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "checkout", "main").CombinedOutput(); err != nil {
		t.Fatalf("git checkout main failed: %s\n%s", err, out)
	}

	if err := RecordRelationship("feature/a", "main"); err != nil {
		t.Fatalf("RecordRelationship failed: %v", err)
	}
	p := Parent("feature/a")
	if p != "main" {
		t.Errorf("expected parent 'main', got '%s'", p)
	}
}

func TestChildren(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	writeStackConfig(t, dir, map[string]interface{}{
		"branches": map[string]interface{}{
			"feature/a": map[string]interface{}{"parent": "main", "parent_head": "x"},
			"feature/b": map[string]interface{}{"parent": "main", "parent_head": "y"},
		},
		"metadata": map[string]interface{}{"main_branch": "main"},
	})
	children := Children("main")
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}
}

func TestStackChain(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	writeStackConfig(t, dir, map[string]interface{}{
		"branches": map[string]interface{}{
			"feature/a": map[string]interface{}{"parent": "main", "parent_head": "x"},
			"feature/b": map[string]interface{}{"parent": "feature/a", "parent_head": "y"},
			"feature/c": map[string]interface{}{"parent": "feature/b", "parent_head": "z"},
		},
		"metadata": map[string]interface{}{"main_branch": "main"},
	})
	chain := StackChain("feature/c")
	expected := []string{"main", "feature/a", "feature/b", "feature/c"}
	if len(chain) != len(expected) {
		t.Fatalf("expected chain length %d, got %d: %v", len(expected), len(chain), chain)
	}
	for i, name := range expected {
		if chain[i] != name {
			t.Errorf("chain[%d] = %s, want %s", i, chain[i], name)
		}
	}
}

func TestCycleDetection(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	writeStackConfig(t, dir, map[string]interface{}{
		"branches": map[string]interface{}{
			"feature/a": map[string]interface{}{"parent": "feature/b", "parent_head": "x"},
			"feature/b": map[string]interface{}{"parent": "feature/a", "parent_head": "y"},
		},
		"metadata": map[string]interface{}{"main_branch": "main"},
	})
	// Should not hang
	chain := StackChain("feature/a")
	if len(chain) > 3 {
		t.Errorf("cycle should be detected, chain too long: %v", chain)
	}
}

func TestConfigExists(t *testing.T) {
	dir := setupStackRepo(t)
	chdir(t, dir)
	if ConfigExists() {
		t.Error("config should not exist yet")
	}
	cfg := &Config{Branches: map[string]*BranchMeta{}, Meta: Metadata{MainBranch: "main"}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if !ConfigExists() {
		t.Error("config should exist after save")
	}
}
