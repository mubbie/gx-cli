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

func TestVersionString(t *testing.T) {
	s := VersionString()
	if s == "" || s == "0.0.0" {
		t.Errorf("unexpected version string: %s", s)
	}
}

func TestSupportsUpdateRefs(t *testing.T) {
	// Just verify it doesn't panic
	_ = SupportsUpdateRefs()
}
