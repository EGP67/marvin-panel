package model

import "math"

// Band is a three-level color/severity state.
type Band string

// Band values.
const (
	BandOK     Band = "ok"
	BandWarn   Band = "warn"
	BandDanger Band = "danger"
)

// CPUBand is the PROCESSOR big-number band: ok < 60 <= warn < 90 <= danger (D-016).
func CPUBand(c float64) Band {
	switch {
	case c >= 90:
		return BandDanger
	case c >= 60:
		return BandWarn
	default:
		return BandOK
	}
}

// ThermalBand is the THERMALS band for every thermal reading: ok < 70 <= warn < 90 <= danger (D-016).
func ThermalBand(c float64) Band {
	switch {
	case c >= 90:
		return BandDanger
	case c >= 70:
		return BandWarn
	default:
		return BandOK
	}
}

// MountState is the SPACE row state on used_pct: ok < 80 <= warn < 90 <= danger (SCHEMA invariant 5).
func MountState(usedPct float64) Band {
	switch {
	case usedPct >= 90:
		return BandDanger
	case usedPct >= 80:
		return BandWarn
	default:
		return BandOK
	}
}

// Severity is the per-thread utilization ramp: ok < 40 <= warn <= 70 < danger (GEOMETRY).
func Severity(pct float64) Band {
	switch {
	case pct > 70:
		return BandDanger
	case pct >= 40:
		return BandWarn
	default:
		return BandOK
	}
}

// CToF converts Celsius to whole Fahrenheit for display only; Fahrenheit is never on the wire.
func CToF(c float64) int {
	return int(math.Round(c*9/5 + 32))
}
