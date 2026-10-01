// Package mood is Marvin's engine (T9, D-058): mood with hysteresis, phrase selection
// with cooldowns, GPU-line pacing and the persisted PANIC COUNT. It reads a measured
// model.Snapshot and fills the engine-chosen fields.
package mood

import (
	"time"

	"hog.local/marvin-panel/internal/model"
)

// Mood names (docs/MARVIN.md).
const (
	Doomed      = "doomed"
	Aggrieved   = "aggrieved"
	Melancholic = "melancholic"
	Bored       = "bored"
	Content     = "content"
)

// Parameters fixed by D-058.
const (
	Hysteresis      = 90 * time.Second
	MelancholicHold = 120 * time.Second
	doomTemp        = 90.0
	aggrievedTemp   = 80.0
	aggrievedIowait = 15.0
	doomFreePct     = 5.0
	melancholicLoad = 0.85
	boredLoad       = 0.02
)

// device is one temperature the mood rules watch.
type device struct {
	name string
	temp float64
}

// temps returns the non-null CPU, GPU0, GPU1 and NVMe temperatures in that order.
func temps(s *model.Snapshot) []device {
	var out []device
	if s.CPU.TempC != nil {
		out = append(out, device{"CPU", *s.CPU.TempC})
	}
	for i, g := range s.GPUs {
		if g.TempC != nil && i < 2 {
			out = append(out, device{[]string{"GPU0", "GPU1"}[i], *g.TempC})
		}
	}
	if s.Temps.NvmeC != nil {
		out = append(out, device{"NVME", *s.Temps.NvmeC})
	}
	return out
}

// hottest is the highest non-null temperature (first device on ties); ok is false
// when none is known.
func hottest(s *model.Snapshot) (device, bool) {
	var best device
	ok := false
	for _, d := range temps(s) {
		if !ok || d.temp > best.temp {
			best, ok = d, true
		}
	}
	return best, ok
}

// freePct is 100*free/(used+free) for a mount, false when unknown.
func freePct(m model.Mount) (float64, bool) {
	if m.UsedBytes == nil || m.FreeBytes == nil || *m.UsedBytes+*m.FreeBytes <= 0 {
		return 0, false
	}
	return 100 * float64(*m.FreeBytes) / float64(*m.UsedBytes+*m.FreeBytes), true
}

// lowestFree is the mount with the least free space, false when none is known.
func lowestFree(s *model.Snapshot) (model.Mount, float64, bool) {
	var best model.Mount
	bestPct, ok := 0.0, false
	for _, m := range s.Storage {
		if p, known := freePct(m); known && (!ok || p < bestPct) {
			best, bestPct, ok = m, p, true
		}
	}
	return best, bestPct, ok
}

func loadRatio(s *model.Snapshot) (float64, bool) {
	if s.CPU.Load1 == nil || s.CPU.Threads <= 0 {
		return 0, false
	}
	return *s.CPU.Load1 / float64(s.CPU.Threads), true
}

// candidate is the mood the snapshot asks for; melancholicHeld reports whether the
// load condition has held MelancholicHold. Null inputs never trigger.
func candidate(s *model.Snapshot, melancholicHeld bool) string {
	if _, p, ok := lowestFree(s); ok && p < doomFreePct {
		return Doomed
	}
	if s.Smart.State == "failing" {
		return Doomed
	}
	if h, ok := hottest(s); ok && h.temp >= doomTemp {
		return Doomed
	}
	if s.CPU.IowaitPct != nil && *s.CPU.IowaitPct > aggrievedIowait {
		return Aggrieved
	}
	if h, ok := hottest(s); ok && h.temp > aggrievedTemp {
		return Aggrieved
	}
	if melancholicHeld {
		return Melancholic
	}
	if r, ok := loadRatio(s); ok && r < boredLoad {
		return Bored
	}
	return Content
}

// moodState is the hysteresis machine.
type moodState struct {
	current      string
	since        time.Time // when current was adopted
	adopted      bool
	pending      string
	pendingSince time.Time
	loadSince    time.Time // start of the continuous melancholic load condition
	loadHigh     bool
}

// step advances the machine; entered reports a confirmed hysteresis change (never the
// initial adoption).
func (m *moodState) step(s *model.Snapshot, now time.Time) (entered bool) {
	if r, ok := loadRatio(s); ok && r > melancholicLoad {
		if !m.loadHigh {
			m.loadHigh, m.loadSince = true, now
		}
	} else {
		m.loadHigh = false
	}
	c := candidate(s, m.loadHigh && now.Sub(m.loadSince) >= MelancholicHold)
	if !m.adopted {
		m.current, m.since, m.adopted, m.pending = c, now, true, ""
		return false
	}
	switch {
	case c == m.current:
		m.pending = ""
	case c != m.pending:
		m.pending, m.pendingSince = c, now
	case now.Sub(m.pendingSince) >= Hysteresis:
		m.current, m.since, m.pending = c, now, ""
		return true
	}
	return false
}

// dwell is how long the current mood has been held.
func (m *moodState) dwell(now time.Time) time.Duration { return now.Sub(m.since) }
