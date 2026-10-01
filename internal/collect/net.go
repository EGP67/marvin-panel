package collect

import (
	"path/filepath"
	"strconv"
)

// netCounters are the cumulative wlp14s0 statistics.
type netCounters struct {
	rxBytes, txBytes, rxErr, txErr *int64
}

func readNet(root, iface string) netCounters {
	dir := filepath.Join(root, "sys/class/net", iface, "statistics")
	read := func(name string) *int64 {
		s, err := readTrim(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil
		}
		return &v
	}
	return netCounters{read("rx_bytes"), read("tx_bytes"), read("rx_errors"), read("tx_errors")}
}

// rate is Δ/seconds rounded to whole bytes; nil on a missing sample, a negative delta
// (counter wrap or reset) or no elapsed time.
func rate(prev, cur *int64, seconds float64) *int64 {
	if prev == nil || cur == nil || *cur < *prev || seconds <= 0 {
		return nil
	}
	v := int64(float64(*cur-*prev)/seconds + 0.5)
	return &v
}

func toInt(p *int64) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
