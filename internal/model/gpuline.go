package model

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

// GPUState is the per-card input to the GPU line; nil means no telemetry.
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

// GPUHotShort replaces the first hot template at n >= 100 so the line stays within 38
// runes (D-058).
const GPUHotShort = "THINKING THIS HARD: {n} DEGREES."

// GPULinePools are the owner-approved templates per state (D-059, docs/MARVIN.md "Line
// pool"); the first of each is the fixture line.
var GPULinePools = map[GPULineKind][]string{
	GPUStale:    {"THE BRAINS ARE NOT ANSWERING.", "NO WORD FROM THE CARDS. TYPICAL.", "THE BRAINS WENT QUIET. SO DID I."},
	GPUHot:      {"THINKING THIS HARD RUNS AT {n} DEGREES.", "{n} DEGREES OF PURE THOUGHT. NOT MINE."},
	GPUThinking: {"SOMEONE ASKED IT SOMETHING. NOT ME.", "THE BRAINS ARE BUSY. WITH OTHERS.", "{util}% BUSY ANSWERING SOMEONE ELSE."},
	GPULoaded:   {"MODEL LOADED. NOBODY ASKS IT ANYTHING.", "{vram} GIB OF MODEL, DOING NOTHING.", "A MODEL IN MEMORY. WAITING. LIKE ME."},
	GPUEmpty:    {"BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY.", "NOTHING LOADED. NOTHING ASKED.", "TWO IDLE CARDS. I KNOW THE FEELING."},
}

// GPULineKindOf classifies the cards, first match wins (D-037).
func GPULineKindOf(gs []GPUState) GPULineKind {
	maxTemp, sumUtil := math.Inf(-1), 0.0
	for _, g := range gs {
		if g.TempC == nil || g.UtilPct == nil {
			return GPUStale
		}
		maxTemp = math.Max(maxTemp, *g.TempC)
		sumUtil += *g.UtilPct
	}
	if maxTemp >= 80 {
		return GPUHot
	}
	if len(gs) > 0 && sumUtil/float64(len(gs)) >= 20 {
		return GPUThinking
	}
	for _, g := range gs {
		if g.MemUsedMiB != nil && *g.MemUsedMiB >= 1024 {
			return GPULoaded
		}
	}
	return GPUEmpty
}

var gpuPlaceholder = regexp.MustCompile(`\{(\w+)\}`)

// RenderGPULine fills a pool template: {n} hottest card temperature, {util} mean
// utilization, {vram} total memory used in GiB (1 dp). ok is false when a value is null.
func RenderGPULine(tpl string, gs []GPUState) (string, bool) {
	ok := true
	val := func(name string) string {
		switch name {
		case "n":
			hot := math.Inf(-1)
			for _, g := range gs {
				if g.TempC == nil {
					ok = false
					return ""
				}
				hot = math.Max(hot, *g.TempC)
			}
			if len(gs) == 0 {
				ok = false
				return ""
			}
			return fmt.Sprintf("%d", int(math.Round(hot)))
		case "util":
			sum := 0.0
			for _, g := range gs {
				if g.UtilPct == nil {
					ok = false
					return ""
				}
				sum += *g.UtilPct
			}
			if len(gs) == 0 {
				ok = false
				return ""
			}
			return fmt.Sprintf("%d", int(math.Round(sum/float64(len(gs)))))
		case "vram":
			mib := 0
			for _, g := range gs {
				if g.MemUsedMiB == nil {
					ok = false
					return ""
				}
				mib += *g.MemUsedMiB
			}
			return fmt.Sprintf("%.1f", float64(mib)/1024)
		}
		ok = false
		return ""
	}
	if tpl == GPULinePools[GPUHot][0] {
		if n, err := strconv.Atoi(val("n")); ok && err == nil && n >= 100 {
			tpl = GPUHotShort
		}
	}
	out := gpuPlaceholder.ReplaceAllStringFunc(tpl, func(m string) string { return val(m[1 : len(m)-1]) })
	return out, ok
}

// GPULineState classifies the cards and renders the state's first pool line (the
// fixture line). Shared by genfixtures and the engine's first selection.
func GPULineState(gs []GPUState) (GPULineKind, string) {
	k := GPULineKindOf(gs)
	s, _ := RenderGPULine(GPULinePools[k][0], gs)
	return k, s
}

// GPULine is the text of GPULineState.
func GPULine(gs []GPUState) string {
	_, s := GPULineState(gs)
	return s
}
