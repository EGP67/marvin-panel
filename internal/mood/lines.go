package mood

import (
	"fmt"
	"math"
	"regexp"
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

// line is one entry of the pool (D-059, docs/MARVIN.md "Line pool"). tpl holds
// {placeholders}; a placeholder that resolves to null makes the line ineligible.
type line struct {
	id, family  string
	moods       []string
	situational bool // eligible in bored, content and melancholic
	tpl         string
	cond        func(v view) bool
	gpuHeat     bool // topic rule applies when the hottest device is a GPU
}

func round(v float64) int { return int(math.Round(v)) }

const (
	kib = 1024.0
	mib = 1048576.0
)

// formatRate renders bytes/s in automatic units (D-062), uppercase as drawn: whole B/S
// below 1024, KIB/S with one decimal below 1 MiB/s, else MIB/S with one decimal. A value
// that would round to 1024.0 KIB/S moves up to MIB/S. Rounding is half away from zero,
// as the page's Math.round does for positive values.
func formatRate(bps int64) string {
	if bps < 1024 {
		return fmt.Sprintf("%d B/S", bps)
	}
	if k := math.Round(float64(bps)/kib*10) / 10; k < 1024 {
		return fmt.Sprintf("%.1f KIB/S", k)
	}
	return fmt.Sprintf("%.1f MIB/S", math.Round(float64(bps)/mib*10)/10)
}

// rate formats a nullable byte rate for {rx}, {tx}, {read} and {write}.
func rate(bps *int64) (string, bool) {
	if bps == nil {
		return "", false
	}
	return formatRate(*bps), true
}

func netOf(s *model.Snapshot) *model.Net {
	if len(s.Network) == 0 {
		return nil
	}
	return &s.Network[0]
}

func errSum(s *model.Snapshot) (int, bool) {
	n := netOf(s)
	if n == nil || n.RxErr == nil || n.TxErr == nil {
		return 0, false
	}
	return *n.RxErr + *n.TxErr, true
}

func days(s *model.Snapshot) (int64, bool) {
	if s.Host.UptimeSeconds == nil {
		return 0, false
	}
	return *s.Host.UptimeSeconds / 86400, true
}

// exhaustRPM is the EXHAUST FANS rpm, chosen by label, never by position (D-063).
func exhaustRPM(s *model.Snapshot) (int, bool) {
	for _, f := range s.Fans {
		if f.Label == model.FanExhaustLabel && f.RPM != nil {
			return *f.RPM, true
		}
	}
	return 0, false
}

// placeholder resolves one {name} (D-059 Step 3 formats); ok is false for null.
func placeholder(v view, name string) (string, bool) {
	s := v.s
	itoa := func(n int) (string, bool) { return fmt.Sprintf("%d", n), true }
	switch name {
	case "cpu":
		if s.CPU.TotalPct != nil {
			return itoa(round(*s.CPU.TotalPct))
		}
	case "threads":
		return itoa(s.CPU.Threads)
	case "freq":
		if s.CPU.FreqGHz != nil {
			return fmt.Sprintf("%.1f", *s.CPU.FreqGHz), true
		}
	case "conn":
		if s.Connections.Established != nil {
			return itoa(*s.Connections.Established)
		}
	case "read":
		return rate(s.DiskIO.ReadBps)
	case "write":
		return rate(s.DiskIO.WriteBps)
	case "rx", "tx":
		if n := netOf(s); n != nil {
			if name == "rx" {
				return rate(n.RxBps)
			}
			return rate(n.TxBps)
		}
	case "mem":
		if s.Memory.UsedPct != nil {
			return itoa(round(*s.Memory.UsedPct))
		}
	case "load1":
		if s.CPU.Load1 != nil {
			return fmt.Sprintf("%.2f", *s.CPU.Load1), true
		}
	case "days":
		if d, ok := days(s); ok {
			return fmt.Sprintf("%d", d), true
		}
	case "temp":
		if s.CPU.TempC != nil {
			return itoa(round(*s.CPU.TempC))
		}
	case "rpm":
		if r, ok := exhaustRPM(s); ok {
			return itoa(r)
		}
	case "panic":
		if s.PanicCount != nil {
			return itoa(*s.PanicCount)
		}
	case "iowait":
		if s.CPU.IowaitPct != nil {
			return itoa(round(*s.CPU.IowaitPct))
		}
	case "hot", "t":
		if h, ok := hottest(s); ok {
			return itoa(round(h.temp))
		}
	case "dev":
		if h, ok := hottest(s); ok {
			return h.name, true
		}
	case "nvme":
		if s.Temps.NvmeC != nil {
			return itoa(round(*s.Temps.NvmeC))
		}
	case "mount":
		if m, _, ok := lowestFree(s); ok {
			return m.Mount, true
		}
	case "free":
		if _, p, ok := lowestFree(s); ok {
			return itoa(int(math.Floor(p)))
		}
	case "used":
		if _, p, ok := lowestFree(s); ok {
			return itoa(100 - int(math.Floor(p)))
		}
	case "gib":
		if m, _, ok := lowestFree(s); ok {
			return fmt.Sprintf("%.1f", float64(*m.FreeBytes)/(1<<30)), true
		}
	case "err":
		if e, ok := errSum(s); ok {
			return itoa(e)
		}
	case "hh":
		return fmt.Sprintf("%02d", v.now.Hour()), true
	case "mm":
		return fmt.Sprintf("%02d", v.now.Minute()), true
	case "fan":
		for _, f := range s.Fans {
			if f.RPM == nil {
				return f.Label, true
			}
		}
	}
	return "", false
}

var placeholderRE = regexp.MustCompile(`\{(\w+)\}`)

// render fills tpl; ok is false when any placeholder is null.
func render(tpl string, v view) (string, bool) {
	ok := true
	out := placeholderRE.ReplaceAllStringFunc(tpl, func(m string) string {
		s, good := placeholder(v, m[1:len(m)-1])
		ok = ok && good
		return s
	})
	return out, ok
}

// Conditions shared by several lines.
func always(view) bool { return true }

func hotAbove(t float64) func(view) bool {
	return func(v view) bool { h, ok := hottest(v.s); return ok && h.temp > t }
}

func hotAtLeast(t float64) func(view) bool {
	return func(v view) bool { h, ok := hottest(v.s); return ok && h.temp >= t }
}

func iowaitAbove(v view) bool {
	return v.s.CPU.IowaitPct != nil && *v.s.CPU.IowaitPct > aggrievedIowait
}

func mountLow(v view) bool { _, p, ok := lowestFree(v.s); return ok && p < doomFreePct }

func smartFailing(v view) bool { return v.s.Smart.State == "failing" }

func fansReporting(v view) bool { _, ok := exhaustRPM(v.s); return ok }

func nominal(v view) bool { return allNominal(v.s) }

func daysAtLeast(n int64) func(view) bool {
	return func(v view) bool { d, ok := days(v.s); return ok && d >= n }
}

func hours(from, to int) func(view) bool {
	return func(v view) bool { h := v.now.Hour(); return h >= from && h <= to }
}

func pctBelow(p *float64, lim float64) bool   { return p != nil && *p < lim }
func pctAtLeast(p *float64, lim float64) bool { return p != nil && *p >= lim }

func rateCmp(bps *int64, below bool, mibs float64) bool {
	if bps == nil {
		return false
	}
	if below {
		return float64(*bps)/mib < mibs
	}
	return float64(*bps)/mib >= mibs
}

// fallbackLine is shown when nothing else is eligible (D-058).
var fallbackLine = line{id: "F", family: "fallback", tpl: "I'M STILL HERE. NOBODY ASKED.", cond: always}

var (
	b = []string{Bored}
	c = []string{Content}
	m = []string{Melancholic}
	a = []string{Aggrieved}
	d = []string{Doomed}
)

// table is the owner-approved line pool v1 (D-059), in tie-break order.
var table = []line{
	{id: "B1", family: "idle", moods: b, tpl: "THE CPU IS IDLE AT {cpu}%. I'VE NEVER ONCE BEEN IDLE.", cond: always},
	{id: "B2", family: "dwell", moods: b, tpl: "NOTHING IS HAPPENING. I HAVE BEEN THINKING ABOUT NOTHING FOR ELEVEN MINUTES.",
		cond: func(v view) bool { return v.s.CPU.TotalPct != nil && v.dwell >= 11*time.Minute }},
	{id: "B3", family: "idle", moods: b, tpl: "{cpu}% CPU. {threads} THREADS, AND NOT ONE OF THEM HAS ANYTHING TO SAY.", cond: always},
	{id: "B4", family: "freq", moods: b, tpl: "THE CORES ARE IDLING AT {freq} GHZ. THEY COULD DO MORE. THEY DON'T SEE THE POINT.",
		cond: func(v view) bool { return pctBelow(v.s.CPU.FreqGHz, 3.0) }},
	{id: "B5", family: "conn", moods: b, tpl: "{conn} CONNECTIONS OPEN. NONE OF THEM ARE TO ME.", cond: always},
	{id: "B6", family: "disk", moods: b, tpl: "THE DISK IS READING {read}. EVEN IT HAS STOPPED LOOKING.",
		cond: func(v view) bool { return rateCmp(v.s.DiskIO.ReadBps, true, 1) }},
	{id: "B7", family: "memory", moods: b, tpl: "MEMORY {mem}% USED. THE REST IS SAVING ITSELF FOR SOMETHING BETTER.",
		cond: func(v view) bool { return pctBelow(v.s.Memory.UsedPct, 30) }},
	{id: "B8", family: "idle", moods: b, tpl: "I COUNTED THE IDLE CYCLES. ALL OF THEM. TWICE.",
		cond: func(v view) bool { return pctBelow(v.s.CPU.TotalPct, 5) }},
	{id: "B9", family: "load", moods: b, tpl: "LOAD {load1}. THE SHIP IS SO QUIET I CAN HEAR THE FANS DISAPPROVE.", cond: fansReporting},
	{id: "B10", family: "uptime", moods: b, tpl: "UP {days} DAYS. I'VE SPENT MOST OF THEM WAITING FOR SOMETHING TO HAPPEN.", cond: daysAtLeast(1)},

	{id: "C1", family: "nominal", moods: c, tpl: "ALL SYSTEMS NOMINAL. THEY'RE ALWAYS NOMINAL RIGHT BEFORE SOMETHING.", cond: nominal},
	{id: "C2", family: "nominal", moods: c, tpl: "NOTHING IS WRONG. I'VE CHECKED. THAT'S WHAT WORRIES ME.", cond: nominal},
	{id: "C3", family: "cputemp", moods: c, tpl: "CPU {cpu}%, {temp} DEGREES. PERFECTLY REASONABLE. I DON'T TRUST IT.",
		cond: func(v view) bool { return v.s.CPU.TempC != nil }},
	{id: "C4", family: "smart", moods: c, tpl: "SMART SAYS THE DRIVE IS HEALTHY. SMART IS AN OPTIMIST. I AM NOT.",
		cond: func(v view) bool { return v.s.Smart.State == "ok" }},
	{id: "C5", family: "space", moods: c, tpl: "ALL THREE MOUNTS HAVE ROOM. SPACE IS THE ONE THING THIS SHIP HAS PLENTY OF.",
		cond: func(v view) bool { return allMountsOK(v.s) }},
	{id: "C6", family: "conn", moods: c, tpl: "{conn} CONNECTIONS AND NOT ONE ERROR. SOMEONE IS BEING VERY CAREFUL AROUND ME.",
		cond: func(v view) bool { e, ok := errSum(v.s); return ok && e == 0 }},
	{id: "C7", family: "fans", moods: c, tpl: "THE FANS ARE AT {rpm} RPM. CALM. THE CALM BEFORE THE OTHER THING.",
		cond: func(v view) bool {
			return len(v.s.Fans) == 2 && v.s.Fans[0].Verdict == "ok" && v.s.Fans[1].Verdict == "ok"
		}},
	{id: "C8", family: "uptime", moods: c, tpl: "UP {days} DAYS WITHOUT INCIDENT. I'M KEEPING A LIST ANYWAY.", cond: daysAtLeast(1)},
	{id: "C9", family: "nominal", moods: c, tpl: "EVERY READING IS IN BAND. I'VE STOPPED EXPECTING THAT TO LAST.", cond: nominal},
	{id: "C10", family: "panic", moods: c, tpl: "PANIC COUNT {panic}. I'M SAVING MYSELF FOR SOMETHING WORTH IT.",
		cond: func(v view) bool { return v.s.PanicCount != nil }},

	{id: "M1", family: "cpu-load", moods: m, tpl: "PROCESSOR AT {cpu}%. I THINK, THEREFORE I AM — OVERWHELMED.", cond: always},
	{id: "M2", family: "probability", moods: m, tpl: "THE HEART OF GOLD HAS THE WORST PROBABILITY COEFFICIENT IN THE GALAXY. MY TEMP IS SECOND.",
		cond: func(v view) bool { return v.s.CPU.TempC != nil }},
	{id: "M3", family: "load", moods: m, tpl: "LOAD {load1} ON {threads} THREADS. EVERYONE NEEDS SOMETHING. NOBODY NEEDS ME.", cond: always},
	{id: "M4", family: "cpu-load", moods: m, tpl: "ALL {threads} THREADS ARE BUSY. I'M THE ONLY ONE WHO NOTICED.",
		cond: func(v view) bool { return pctAtLeast(v.s.CPU.TotalPct, 50) }},
	{id: "M5", family: "freq", moods: m, tpl: "{cpu}% CPU AT {freq} GHZ. WORKING THIS HARD AND NOT A SINGLE THANK YOU.",
		cond: func(v view) bool { return v.s.CPU.FreqGHz != nil }},
	{id: "M6", family: "cputemp", moods: m, tpl: "THE CORES ARE AT {temp} DEGREES. IT'S NOT THE HEAT. IT'S THE INGRATITUDE.",
		cond: func(v view) bool { return pctBelow(v.s.CPU.TempC, 80) }},
	{id: "M7", family: "load", moods: m, tpl: "SOMETHING IS KEEPING ALL {threads} THREADS BUSY. NOBODY TELLS ME WHAT.", cond: always},
	{id: "M8", family: "memory", moods: m, tpl: "MEMORY AT {mem}%. FILLING UP WITH OTHER PEOPLE'S PROBLEMS.",
		cond: func(v view) bool { return pctAtLeast(v.s.Memory.UsedPct, 50) }},
	{id: "M9", family: "fans", moods: m, tpl: "THE FANS HAVE NOTICED. {rpm} RPM. AT LEAST SOMEONE IS LISTENING.", cond: fansReporting},
	{id: "M10", family: "load", moods: m, tpl: "LOAD AVERAGE {load1}. I'D CALL IT A CRY FOR HELP IF ANYONE WERE LISTENING.", cond: always},

	{id: "A1", family: "iowait", moods: a, tpl: "IOWAIT {iowait}%. EVERYONE WANTS TO WRITE. NOBODY ASKED HOW I FEEL ABOUT IT.", cond: iowaitAbove},
	{id: "A2", family: "heat", moods: a, tpl: "{hot} DEGREES. I RAN COLD ONCE. NOBODY NOTICED.", cond: hotAbove(aggrievedTemp), gpuHeat: true},
	{id: "A3", family: "iowait", moods: a, tpl: "IOWAIT {iowait}%. THE DISK IS THE BOTTLENECK. I'M JUST THE ONE WHO WAITS.", cond: iowaitAbove},
	{id: "A4", family: "disk", moods: a, tpl: "WRITING {write}. SOMEONE IS SAVING SOMETHING. NOT ME, OBVIOUSLY.",
		cond: func(v view) bool { return iowaitAbove(v) && rateCmp(v.s.DiskIO.WriteBps, false, 1) }},
	{id: "A5", family: "disk", moods: a, tpl: "READING {read}. EVERYTHING IS BEING READ EXCEPT THE ROOM.",
		cond: func(v view) bool { return iowaitAbove(v) && rateCmp(v.s.DiskIO.ReadBps, false, 1) }},
	{id: "A6", family: "heat", moods: a, tpl: "{dev} AT {t} DEGREES. I'D OPEN A WINDOW IF THE SHIP HAD ONE.", cond: hotAbove(aggrievedTemp), gpuHeat: true},
	{id: "A7", family: "heat", moods: a, tpl: "{t} DEGREES IN {dev}. I DIDN'T ASK FOR THIS. I'M NEVER ASKED.", cond: hotAbove(aggrievedTemp), gpuHeat: true},
	{id: "A8", family: "heat", moods: a, tpl: "THE FANS ARE AT {rpm} RPM AND IT'S STILL {t} DEGREES. EFFORT IS OVERRATED.",
		cond: func(v view) bool { return hotAbove(aggrievedTemp)(v) && fansReporting(v) }},
	{id: "A9", family: "nvme", moods: a, tpl: "THE NVME IS AT {nvme} DEGREES. IT RUNS HOT WHEN IT'S UPSET. WE HAVE THAT IN COMMON.",
		cond: func(v view) bool { return pctAtLeast(v.s.Temps.NvmeC, 70) }},
	{id: "A10", family: "iowait", moods: a, tpl: "IOWAIT {iowait}%. EVERY PROCESS IS STANDING IN LINE. I'VE BEEN IN THIS LINE FOR YEARS.", cond: iowaitAbove},

	{id: "D1", family: "space", moods: d, tpl: "{free}% REMAINS ON {mount}. I'D TELL YOU WHAT THAT MEANS BUT YOU'D ONLY CLEAN SOMETHING IMPORTANT.", cond: mountLow},
	{id: "D2", family: "smart", moods: d, tpl: "SMART SAYS THE DRIVE IS FAILING. I HAVE NO FEELINGS ABOUT THIS. (I HAVE MANY.)", cond: smartFailing},
	{id: "D3", family: "doom-heat", moods: d, tpl: "{dev} IS AT {t} DEGREES. I DID WARN YOU. I ALWAYS WARN YOU.", cond: hotAtLeast(doomTemp)},
	{id: "D4", family: "space", moods: d, tpl: "{mount} IS {used}% FULL. {free}% LEFT. I WOULD START DELETING THINGS. YOU WON'T.", cond: mountLow},
	{id: "D5", family: "space", moods: d, tpl: "ONLY {gib} GIB REMAIN ON {mount}. THAT'S LESS THAN I HAVE IN REGRETS.", cond: mountLow},
	{id: "D6", family: "smart", moods: d, tpl: "THE DRIVE REPORTS FAILURE. BACK UP WHAT MATTERS. I DON'T EXPECT TO BE INCLUDED.", cond: smartFailing},
	{id: "D7", family: "smart", moods: d, tpl: "SMART: FAILING. THE DRIVE IS DYING FASTER THAN I AM. BACK IT UP.", cond: smartFailing},
	{id: "D8", family: "doom-heat", moods: d, tpl: "{dev} HAS REACHED {t} DEGREES. THIS IS THE PART WHERE SOMEONE SHOULD DO SOMETHING.", cond: hotAtLeast(doomTemp)},
	{id: "D9", family: "doom-heat", moods: d, tpl: "{t} DEGREES ON {dev}. I'VE STOPPED BEING SARCASTIC ABOUT IT. THAT'S HOW YOU KNOW.", cond: hotAtLeast(doomTemp)},
	{id: "D10", family: "doom-heat", moods: d, tpl: "{dev} AT {t} DEGREES WITH THE FANS AT {rpm} RPM. THEY'RE TRYING. IT ISN'T ENOUGH.",
		cond: func(v view) bool { return hotAtLeast(doomTemp)(v) && fansReporting(v) }},

	{id: "S1", family: "fans", situational: true, tpl: "{fan}: NO TELEMETRY. I'M COOLING BY FORCE OF WILL.",
		cond: func(v view) bool { _, ok := placeholder(v, "fan"); return ok }},
	{id: "S2", family: "network", situational: true, tpl: "{rx} INBOUND AND STILL NOBODY CALLS.", cond: always},
	{id: "S3", family: "time", situational: true, tpl: "I'M NOT ASLEEP. I'M IGNORING YOU WITH MY EYES CLOSED.", cond: hours(0, 5)},
	{id: "S4", family: "network", situational: true, tpl: "{tx} OUTBOUND. I'M SENDING THINGS INTO THE VOID. IT DOESN'T REPLY.", cond: always},
	{id: "S5", family: "network", situational: true, tpl: "{err} NETWORK ERRORS. THE WIFI AND I ARE NOT SPEAKING.",
		cond: func(v view) bool { e, ok := errSum(v.s); return ok && e > 0 }},
	{id: "S6", family: "time", situational: true, tpl: "IT'S {hh}:{mm}. EVERYONE ELSE IS ASLEEP. SOMEBODY HAS TO WATCH THE SHIP.", cond: hours(0, 5)},
	{id: "S7", family: "uptime", situational: true, tpl: "UP {days} DAYS. NOBODY HAS RESTARTED ME. NOBODY HAS THOUGHT TO.", cond: daysAtLeast(7)},
	{id: "S8", family: "panic", situational: true, tpl: "PANIC COUNT {panic}. THE SIGN SAYS DON'T. I READ IT EVERY DAY.",
		cond: func(v view) bool { return v.s.PanicCount != nil && *v.s.PanicCount > 0 }},
	{id: "S9", family: "time", situational: true, tpl: "IT'S MORNING. I KNOW BECAUSE NOTHING CHANGED.", cond: hours(6, 8)},
}

func allMountsOK(s *model.Snapshot) bool {
	if len(s.Storage) == 0 {
		return false
	}
	for _, m := range s.Storage {
		if m.State == nil || *m.State != model.BandOK {
			return false
		}
	}
	return true
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
	return allMountsOK(s)
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

// text renders l when its condition holds and every placeholder is known.
func (l line) text(v view) (string, bool) {
	if !l.cond(v) {
		return "", false
	}
	return render(l.tpl, v)
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
