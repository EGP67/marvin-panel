package model

import (
	"fmt"
	"math"
)

// AgreementTolerance is the allowed gap, in percentage points, between
// cpu.total_pct and the mean of the per-thread values (D-006, D-050).
const AgreementTolerance = 12.0

// CheckAgreement enforces the D-050 agreement invariant on s: per-thread arrays
// have one entry per thread and, when cpu.total_pct is non-null, the last
// hist_pct entry equals it and the mean of the non-null per_thread_pct entries
// is within AgreementTolerance of it.
func CheckAgreement(s *Snapshot) error {
	c := &s.CPU
	if len(c.PerThreadPct) != c.Threads {
		return fmt.Errorf("per_thread_pct has %d entries, threads = %d", len(c.PerThreadPct), c.Threads)
	}
	if len(c.PerThreadSev) != c.Threads {
		return fmt.Errorf("per_thread_sev has %d entries, threads = %d", len(c.PerThreadSev), c.Threads)
	}
	if c.TotalPct == nil {
		return nil
	}
	total := *c.TotalPct
	if n := len(c.HistPct); n == 0 || c.HistPct[n-1] == nil || *c.HistPct[n-1] != total {
		return fmt.Errorf("last hist_pct entry does not equal total_pct %v", total)
	}
	sum, n := 0.0, 0
	for _, v := range c.PerThreadPct {
		if v != nil {
			sum += *v
			n++
		}
	}
	if n == 0 {
		return nil
	}
	if mean := sum / float64(n); math.Abs(mean-total) > AgreementTolerance {
		return fmt.Errorf("per-thread mean %.1f differs from total_pct %v by more than %v", mean, total, AgreementTolerance)
	}
	return nil
}
