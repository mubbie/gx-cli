package git

import "testing"

func TestVersion(t *testing.T) {
	major, minor, patch := Version()
	if major < 2 {
		t.Errorf("expected git major >= 2, got %d", major)
	}
	if minor < 0 || patch < 0 {
		t.Error("minor and patch should be >= 0")
	}
}

func TestSupportsUpdateRefs(t *testing.T) {
	// Just verify it doesn't panic
	_ = SupportsUpdateRefs()
}
