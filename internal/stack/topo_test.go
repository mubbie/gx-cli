package stack

import (
	"testing"
)

func TestTopoSort(t *testing.T) {
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

	// Out of order input
	result := TopoSort([]string{"feature/c", "feature/a", "main", "feature/b"})

	indexOf := func(name string) int {
		for i, n := range result {
			if n == name {
				return i
			}
		}
		return -1
	}

	for _, name := range []string{"main", "feature/a", "feature/b", "feature/c"} {
		if indexOf(name) == -1 {
			t.Fatalf("branch %q not found in result: %v", name, result)
		}
	}

	if indexOf("main") > indexOf("feature/a") {
		t.Error("main should come before feature/a")
	}
	if indexOf("feature/a") > indexOf("feature/b") {
		t.Error("feature/a should come before feature/b")
	}
	if indexOf("feature/b") > indexOf("feature/c") {
		t.Error("feature/b should come before feature/c")
	}
}
