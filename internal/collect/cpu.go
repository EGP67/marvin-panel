package collect

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MaxThreads is the dot-matrix limit; marvind refuses to start above it (D-006).
const MaxThreads = 12

// cpuTimes is one /proc/stat cpu line reduced to the D-036 quantities.
type cpuTimes struct {
	total, idle, iowait uint64
}

func (c cpuTimes) busy() uint64 { return c.total - c.idle - c.iowait }

// parseProcStat returns the aggregate "cpu" line and the per-cpu "cpuN" lines keyed by N.
func parseProcStat(r io.Reader) (cpuTimes, map[int]cpuTimes, error) {
	var agg cpuTimes
	per := map[int]cpuTimes{}
	found := false
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 9 || !strings.HasPrefix(f[0], "cpu") {
			continue
		}
		var v [8]uint64 // user nice system idle iowait irq softirq steal
		for i := range v {
			n, err := strconv.ParseUint(f[i+1], 10, 64)
			if err != nil {
				return agg, nil, fmt.Errorf("/proc/stat %s: %w", f[0], err)
			}
			v[i] = n
		}
		t := cpuTimes{idle: v[3], iowait: v[4]}
		for _, n := range v {
			t.total += n
		}
		if f[0] == "cpu" {
			agg, found = t, true
			continue
		}
		id, err := strconv.Atoi(strings.TrimPrefix(f[0], "cpu"))
		if err != nil {
			return agg, nil, fmt.Errorf("/proc/stat %s: %w", f[0], err)
		}
		per[id] = t
	}
	if err := sc.Err(); err != nil {
		return agg, nil, fmt.Errorf("/proc/stat: %w", err)
	}
	if !found {
		return agg, nil, fmt.Errorf("/proc/stat: no aggregate cpu line")
	}
	return agg, per, nil
}

// busyPct is 100*Δbusy/Δtotal; nil when any counter went backwards (wrap guard) or
// no time passed.
func busyPct(prev, cur cpuTimes) *float64 {
	if cur.total <= prev.total || cur.idle < prev.idle || cur.iowait < prev.iowait || cur.busy() < prev.busy() {
		return nil
	}
	return pct(float64(cur.busy()-prev.busy()), float64(cur.total-prev.total))
}

// iowaitPct is 100*Δiowait/Δtotal with the same guards.
func iowaitPct(prev, cur cpuTimes) *float64 {
	if cur.total <= prev.total || cur.iowait < prev.iowait {
		return nil
	}
	return pct(float64(cur.iowait-prev.iowait), float64(cur.total-prev.total))
}

func pct(part, whole float64) *float64 {
	v := r1(math.Max(0, math.Min(100, 100*part/whole)))
	return &v
}

func r1(v float64) float64 { return math.Round(v*10) / 10 }

// parseOnline parses a cpu list such as "0-11" or "0-3,5,7-9".
func parseOnline(s string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(strings.TrimSpace(s), ",") {
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			return nil, fmt.Errorf("cpu online %q: %w", s, err)
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil || b < a {
				return nil, fmt.Errorf("cpu online %q: bad range %q", s, part)
			}
		}
		for i := a; i <= b; i++ {
			out = append(out, i)
		}
	}
	return out, nil
}

// physicalCores counts unique (physical_package_id, core_id) pairs over cpus (D-055).
func physicalCores(root string, cpus []int) (int, error) {
	seen := map[string]bool{}
	for _, c := range cpus {
		dir := filepath.Join(root, "sys/devices/system/cpu", fmt.Sprintf("cpu%d", c), "topology")
		pkg, err := readTrim(filepath.Join(dir, "physical_package_id"))
		if err != nil {
			return 0, err
		}
		core, err := readTrim(filepath.Join(dir, "core_id"))
		if err != nil {
			return 0, err
		}
		seen[pkg+"/"+core] = true
	}
	return len(seen), nil
}

// cpuModel returns the first "model name" value in /proc/cpuinfo.
func cpuModel(r io.Reader) (string, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("/proc/cpuinfo: %w", err)
	}
	return "", fmt.Errorf("/proc/cpuinfo: no model name")
}

// parseLoadavg returns load1, load5 and load15.
func parseLoadavg(s string) ([3]float64, error) {
	var out [3]float64
	f := strings.Fields(s)
	if len(f) < 3 {
		return out, fmt.Errorf("/proc/loadavg: %q", s)
	}
	for i := range out {
		v, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return out, fmt.Errorf("/proc/loadavg: %w", err)
		}
		out[i] = v
	}
	return out, nil
}

// freqGHz converts policy0 scaling_cur_freq (kHz) to GHz, one decimal.
func freqGHz(s string) (float64, error) {
	khz, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("scaling_cur_freq: %w", err)
	}
	return r1(khz / 1e6), nil
}

func readTrim(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}
