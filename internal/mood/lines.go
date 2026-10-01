package mood

import (
	"fmt"
	"math"
	"strings"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// Phrase budget (docs/GEOMETRY.md): 1-2 lines of <= 52 runes, <= 100 in total.
const (
	lineMax  = 52
	totalMax = 100
)

// view is what a line may look at when it renders.
type view struct {
	s      *model.Snapshot
	mood   string
	dwell  time.Duration
	now    time.Time
	gpuHot bool // the GPU line is showing its hot state (topic rule)
}

// line is one entry of the phrase table. render returns the full text and whether the
// line is true now; every number comes from live values.
type line struct {
	id, family  string
	moods       []string
	situational bool // eligible in bored, content and melancholic
	render      func(v view) (string, bool)
	gpuHeat     func(v view) bool // true when the rendered line is about GPU heat
}

func round(v float64) int { return int(math.Round(v)) }

// fallbackLine is shown when nothing else is eligible (D-058).
var fallbackLine = line{id: "F", family: "fallback", render: func(view) (string, bool) {
	return "I'M STILL HERE. NOBODY ASKED.", true
}}

// table is the owner-approved line pool (D-058, docs/MARVIN.md), in tie-break order.
var table = []line{
	{id: "B1", family: "idle", moods: []string{Bored}, render: func(v view) (string, bool) {
		if v.s.CPU.TotalPct == nil {
			return "", false
		}
		return fmt.Sprintf("THE CPU IS IDLE AT %d%%. I'VE NEVER ONCE BEEN IDLE.", round(*v.s.CPU.TotalPct)), true
	}},
	{id: "B2", family: "dwell", moods: []string{Bored}, render: func(v view) (string, bool) {
		if v.s.CPU.TotalPct == nil || v.dwell < 11*time.Minute {
			return "", false
		}
		return "NOTHING IS HAPPENING. I HAVE BEEN THINKING ABOUT NOTHING FOR ELEVEN MINUTES.", true
	}},
	{id: "C1", family: "nominal", moods: []string{Content}, render: func(v view) (string, bool) {
		if !allNominal(v.s) {
			return "", false
		}
		return "ALL SYSTEMS NOMINAL. THEY'RE ALWAYS NOMINAL RIGHT BEFORE SOMETHING.", true
	}},
	{id: "M1", family: "cpu-load", moods: []string{Melancholic}, render: func(v view) (string, bool) {
		if v.s.CPU.TotalPct == nil {
			return "", false
		}
		return fmt.Sprintf("PROCESSOR AT %d%%. I THINK, THEREFORE I AM — OVERWHELMED.", round(*v.s.CPU.TotalPct)), true
	}},
	{id: "M2", family: "probability", moods: []string{Melancholic}, render: func(v view) (string, bool) {
		if v.s.CPU.TempC == nil {
			return "", false
		}
		return "THE HEART OF GOLD HAS THE WORST PROBABILITY COEFFICIENT IN THE GALAXY. MY TEMP IS SECOND.", true
	}},
	{id: "A1", family: "iowait", moods: []string{Aggrieved}, render: func(v view) (string, bool) {
		w := v.s.CPU.IowaitPct
		if w == nil || *w <= aggrievedIowait {
			return "", false
		}
		return fmt.Sprintf("IOWAIT %d%%. EVERYONE WANTS TO WRITE. NOBODY ASKED HOW I FEEL ABOUT IT.", round(*w)), true
	}},
	{id: "A2", family: "heat", moods: []string{Aggrieved},
		render: func(v view) (string, bool) {
			h, ok := hottest(v.s)
			if !ok || h.temp <= aggrievedTemp {
				return "", false
			}
			return fmt.Sprintf("%d DEGREES. I RAN COLD ONCE. NOBODY NOTICED.", round(h.temp)), true
		},
		gpuHeat: func(v view) bool {
			h, ok := hottest(v.s)
			return ok && strings.HasPrefix(h.name, "GPU")
		}},
	{id: "D1", family: "space", moods: []string{Doomed}, render: func(v view) (string, bool) {
		m, p, ok := lowestFree(v.s)
		if !ok || p >= doomFreePct {
			return "", false
		}
		return fmt.Sprintf("%d%% REMAINS ON %s. I'D TELL YOU WHAT THAT MEANS BUT YOU'D ONLY CLEAN SOMETHING IMPORTANT.",
			int(math.Floor(p)), m.Mount), true
	}},
	{id: "D2", family: "smart", moods: []string{Doomed}, render: func(v view) (string, bool) {
		if v.s.Smart.State != "failing" {
			return "", false
		}
		return "SMART SAYS THE DRIVE IS FAILING. I HAVE NO FEELINGS ABOUT THIS. (I HAVE MANY.)", true
	}},
	{id: "D3", family: "doom-heat", moods: []string{Doomed}, render: func(v view) (string, bool) {
		h, ok := hottest(v.s)
		if !ok || h.temp < doomTemp {
			return "", false
		}
		return fmt.Sprintf("%s IS AT %d DEGREES. I DID WARN YOU. I ALWAYS WARN YOU.", h.name, round(h.temp)), true
	}},
	{id: "S1", family: "fans", situational: true, render: func(v view) (string, bool) {
		for _, f := range v.s.Fans {
			if f.RPM == nil {
				return fmt.Sprintf("FAN BANK %d: NO TELEMETRY. I'M COOLING BY FORCE OF WILL.", f.Bank), true
			}
		}
		return "", false
	}},
	{id: "S2", family: "network", situational: true, render: func(v view) (string, bool) {
		if len(v.s.Network) == 0 || v.s.Network[0].RxBps == nil {
			return "", false
		}
		mib := float64(*v.s.Network[0].RxBps) / 1048576
		n := fmt.Sprintf("%.1f", mib)
		if mib >= 10 {
			n = fmt.Sprintf("%d", round(mib))
		}
		return n + " MIB/S INBOUND AND STILL NOBODY CALLS.", true
	}},
	{id: "S3", family: "night", situational: true, render: func(v view) (string, bool) {
		if h := v.now.Hour(); h > 5 {
			return "", false
		}
		return "I'M NOT ASLEEP. I'M IGNORING YOU WITH MY EYES CLOSED.", true
	}},
}

// allNominal: every thermal band, mount state and smart.state is known and ok.
func allNominal(s *model.Snapshot) bool {
	okBand := func(b *model.Band) bool { return b != nil && *b == model.BandOK }
	if !okBand(s.CPU.ThermalBand) || !okBand(s.Temps.NvmeThermalBand) || s.Smart.State != "ok" {
		return false
	}
	for _, g := range s.GPUs {
		if !okBand(g.ThermalBand) {
			return false
		}
	}
	for _, m := range s.Storage {
		if !okBand(m.State) {
			return false
		}
	}
	return true
}

// allows reports whether the line may speak in mood.
func (l line) allows(mood string) bool {
	if l.situational {
		return mood == Bored || mood == Content || mood == Melancholic
	}
	for _, m := range l.moods {
		if m == mood {
			return true
		}
	}
	return false
}

// wrap splits text at word boundaries into 1-2 lines of <= lineMax runes with a total of
// <= totalMax; ok is false when it does not fit (the line is then ineligible).
func wrap(text string) ([]string, bool) {
	var out []string
	cur := ""
	for _, w := range strings.Fields(text) {
		switch {
		case cur == "":
			cur = w
		case runes(cur)+1+runes(w) <= lineMax:
			cur += " " + w
		default:
			out = append(out, cur)
			cur = w
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	total := 0
	for _, l := range out {
		if runes(l) > lineMax {
			return nil, false
		}
		total += runes(l)
	}
	return out, len(out) >= 1 && len(out) <= 2 && total <= totalMax
}

func runes(s string) int { return len([]rune(s)) }
