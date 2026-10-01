package collect

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"hog.local/marvin-panel/internal/model"
)

// parseMeminfo returns the kB values of /proc/meminfo converted to bytes.
func parseMeminfo(r io.Reader) map[string]int64 {
	out := map[string]int64{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			continue
		}
		if len(f) > 1 && f[1] == "kB" {
			n *= 1024
		}
		out[k] = n
	}
	return out
}

// memory builds the MEMORY cell (D-034); a field is nil when its inputs are missing.
func memory(mi map[string]int64) model.Memory {
	var m model.Memory
	get := func(k string) *int64 {
		if v, ok := mi[k]; ok {
			return &v
		}
		return nil
	}
	total, avail := get("MemTotal"), get("MemAvailable")
	m.TotalBytes = total
	if total != nil && avail != nil {
		used := *total - *avail
		m.UsedBytes = &used
		if *total > 0 {
			p := r1(100 * float64(used) / float64(*total))
			m.UsedPct = &p
		}
		cached, srecl, shmem := get("Cached"), get("SReclaimable"), get("Shmem")
		if cached != nil && srecl != nil && shmem != nil {
			c := *cached + *srecl - *shmem
			c = max(0, min(c, *total-used))
			m.CacheBytes = &c
		}
	}
	st, sf := get("SwapTotal"), get("SwapFree")
	m.SwapTotalBytes = st
	if st != nil && sf != nil {
		su := *st - *sf
		m.SwapUsedBytes = &su
	}
	return m
}
