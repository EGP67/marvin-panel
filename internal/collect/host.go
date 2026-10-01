package collect

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// parseUptime returns the floor of the first /proc/uptime field.
func parseUptime(s string) (int64, error) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, fmt.Errorf("/proc/uptime: empty")
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, fmt.Errorf("/proc/uptime: %w", err)
	}
	return int64(math.Floor(v)), nil
}
