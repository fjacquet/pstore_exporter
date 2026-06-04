package powerstore

import (
	"strconv"
	"strings"
)

// bulkCapableFromVersion returns true when a PowerStoreOS version string is >= 4.1.
// The bulk CSV metrics API (/latest_five_min_metrics) was introduced in 4.1.0.
func bulkCapableFromVersion(version string) bool {
	major, minor, ok := parseMajorMinor(version)
	if !ok {
		return false
	}
	return major > 4 || (major == 4 && minor >= 1)
}

func parseMajorMinor(version string) (major, minor int, ok bool) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return maj, min, true
}
