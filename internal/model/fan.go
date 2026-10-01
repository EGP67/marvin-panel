package model

// FanVerdict implements SCHEMA invariant 6; it keys on cpu.thermal_band and never
// reports not_cooling while that band is null. Shared by genfixtures and the live
// collector (D-057).
func FanVerdict(rpm, maxRPM *int, cpuThermalBand *Band) string {
	switch {
	case rpm == nil:
		return "unknown"
	case *rpm == 0:
		return "stalled"
	case maxRPM != nil && *maxRPM > 0 && float64(*rpm) >= 0.99*float64(*maxRPM) &&
		cpuThermalBand != nil && *cpuThermalBand != BandOK:
		return "not_cooling"
	default:
		return "ok"
	}
}
