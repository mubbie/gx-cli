package git

import (
	"strconv"
	"strings"
	"sync"
)

var (
	versionOnce  sync.Once
	versionCache [3]int
)

// Version returns the git version as (major, minor, patch).
func Version() (int, int, int) {
	versionOnce.Do(func() {
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
				versionCache = [3]int{major, minor, patch}
				return
			}
		}
	})
	return versionCache[0], versionCache[1], versionCache[2]
}

// SupportsUpdateRefs returns true if git >= 2.38.
func SupportsUpdateRefs() bool {
	major, minor, _ := Version()
	return major > 2 || (major == 2 && minor >= 38)
}
