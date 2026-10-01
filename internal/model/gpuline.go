package model

import (
	"fmt"
	"math"
)

// GPUState is the per-card input to GPULine; nil means no telemetry.
type GPUState struct {
	TempC, UtilPct *float64
	MemUsedMiB     *int
}

// GPULine picks the GRAPHICS box line (D-037, docs/MARVIN.md "GPU line"); first match
// wins. Shared by genfixtures and the live collector (D-055).
func GPULine(gs []GPUState) string {
	maxTemp, sumUtil := math.Inf(-1), 0.0
	for _, g := range gs {
		if g.TempC == nil || g.UtilPct == nil {
			return "THE BRAINS ARE NOT ANSWERING."
		}
		maxTemp = math.Max(maxTemp, *g.TempC)
		sumUtil += *g.UtilPct
	}
	if maxTemp >= 80 {
		return fmt.Sprintf("THINKING THIS HARD RUNS AT %d DEGREES.", int(math.Round(maxTemp)))
	}
	if len(gs) > 0 && sumUtil/float64(len(gs)) >= 20 {
		return "SOMEONE ASKED IT SOMETHING. NOT ME."
	}
	for _, g := range gs {
		if g.MemUsedMiB != nil && *g.MemUsedMiB >= 1024 {
			return "MODEL LOADED. NOBODY ASKS IT ANYTHING."
		}
	}
	return "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY."
}
