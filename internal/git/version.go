package git

import (
	"fmt"
	"strconv"
	"strings"
)

// Version returns the git version as (major, minor, patch).
func Version() (int, int, int) {
	out := RunUnchecked("--version")
	for _, part := range strings.Fields(out) {
		if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
			nums := strings.Split(part, ".")
			major, _ := strconv.Atoi(nums[0])
			minor := 0
			patch := 0
			if len(nums) > 1 {
				minor, _ = strconv.Atoi(nums[1])
			}
			if len(nums) > 2 {
				patch, _ = strconv.Atoi(nums[2])
			}
			return major, minor, patch
		}
	}
	return 0, 0, 0
}

// SupportsUpdateRefs returns true if git >= 2.38.
func SupportsUpdateRefs() bool {
	major, minor, _ := Version()
	return major > 2 || (major == 2 && minor >= 38)
}

// VersionString returns the git version as a string like "2.41.0".
func VersionString() string {
	major, minor, patch := Version()
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}
