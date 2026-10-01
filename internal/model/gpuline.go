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

// GPULineKind is the GRAPHICS box line state (docs/MARVIN.md "GPU line").
type GPULineKind string

// GPU line states, in evaluation order.
const (
	GPUStale    GPULineKind = "stale"
	GPUHot      GPULineKind = "hot"
	GPUThinking GPULineKind = "thinking"
	GPULoaded   GPULineKind = "loaded"
	GPUEmpty    GPULineKind = "empty"
)

// GPULineState classifies the cards (first match wins) and renders the line (D-037);
// at or above 100 °C the hot line uses the short form so it stays within 38 runes
// (D-058). Shared by genfixtures and the live engine.
func GPULineState(gs []GPUState) (GPULineKind, string) {
	maxTemp, sumUtil := math.Inf(-1), 0.0
	for _, g := range gs {
		if g.TempC == nil || g.UtilPct == nil {
			return GPUStale, "THE BRAINS ARE NOT ANSWERING."
		}
		maxTemp = math.Max(maxTemp, *g.TempC)
		sumUtil += *g.UtilPct
	}
	if maxTemp >= 80 {
		n := int(math.Round(maxTemp))
		if n >= 100 {
			return GPUHot, fmt.Sprintf("THINKING THIS HARD: %d DEGREES.", n)
		}
		return GPUHot, fmt.Sprintf("THINKING THIS HARD RUNS AT %d DEGREES.", n)
	}
	if len(gs) > 0 && sumUtil/float64(len(gs)) >= 20 {
		return GPUThinking, "SOMEONE ASKED IT SOMETHING. NOT ME."
	}
	for _, g := range gs {
		if g.MemUsedMiB != nil && *g.MemUsedMiB >= 1024 {
			return GPULoaded, "MODEL LOADED. NOBODY ASKS IT ANYTHING."
		}
	}
	return GPUEmpty, "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY."
}

// GPULine is the text of GPULineState.
func GPULine(gs []GPUState) string {
	_, s := GPULineState(gs)
	return s
}
