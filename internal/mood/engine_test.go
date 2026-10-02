package mood

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hog.local/marvin-panel/internal/model"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func fp(v float64) *float64 { return &v }
func ip(v int) *int         { return &v }
func i64(v int64) *int64    { return &v }

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func band(b model.Band) *model.Band { return &b }

// idle is hog at rest: bored (load1/threads 0.01), every band ok, fans reporting.
func idle() *model.Snapshot {
	s := &model.Snapshot{Schema: model.SchemaVersion, Scenario: "live", GeneratedAt: "2026-10-01T12:00:00Z"}
	s.CPU = model.CPU{Model: "AMD Ryzen 5 9600X 6-Core Processor", ModelDisplay: "AMD RYZEN 5 9600X",
		Threads: 12, PhysicalCores: 6, TotalPct: fp(1), IowaitPct: fp(0), Load1: fp(0.12), Load5: fp(0.1), Load15: fp(0.1),
		FreqGHz: fp(4.9), TempC: fp(45), Band: band(model.BandOK), ThermalBand: band(model.BandOK)}
	for i := 0; i < 12; i++ {
		s.CPU.PerThreadPct = append(s.CPU.PerThreadPct, fp(1))
		s.CPU.PerThreadSev = append(s.CPU.PerThreadSev, band(model.BandOK))
	}
	s.CPU.HistPct = make([]*float64, 120)
	s.CPU.HistPct[119] = fp(1)
	for i := 0; i < 2; i++ {
		s.GPUs = append(s.GPUs, model.GPU{Name: "NVIDIA GeForce RTX 3060", DisplayName: "RTX 3060", UUID: "unbound",
			TempC: fp(32), ThermalBand: band(model.BandOK), UtilPct: fp(0), UtilSev: band(model.BandOK),
			MemUsedMiB: ip(1), MemTotalMiB: ip(12288), HistUtilPct: make([]*float64, 30)})
	}
	s.Temps = model.Temps{NvmeC: fp(33), NvmeThermalBand: band(model.BandOK), NvmeMaxC: fp(83.85)}
	for _, mp := range []string{"/", "/srv/hogdata", "/boot"} {
		s.Storage = append(s.Storage, model.Mount{Mount: mp, Device: "d", FS: "ext4", TotalBytes: i64(100),
			UsedBytes: i64(30), FreeBytes: i64(70), UsedPct: fp(30), State: band(model.BandOK)})
	}
	s.Smart = model.Smart{State: "ok"}
	s.Fans = []model.Fan{{Bank: 1, Label: model.FanIntakeLabel, RPM: ip(1000), MaxRPM: ip(2000), Verdict: "ok"},
		{Bank: 2, Label: model.FanExhaustLabel, RPM: ip(1100), MaxRPM: ip(2000), Verdict: "ok"}}
	s.Network = []model.Net{{If: "wlp14s0", RxBps: i64(2000), TxBps: i64(1000), RxErr: ip(0), TxErr: ip(0)}}
	s.DiskIO = model.DiskIO{Device: "nvme0n1"}
	s.Connections = model.Connections{Established: ip(10)}
	return s
}

// exhaust is the EXHAUST FANS index in display order (D-063).
const exhaust = 1

func newEngine(t *testing.T, dir string) *Engine {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	return New(Options{StateDir: dir, Log: quiet()})
}

// tick applies a copy of s at t and returns it.
func tick(e *Engine, s *model.Snapshot, t time.Time) *model.Snapshot {
	c := *s
	e.Apply(&c, t)
	return &c
}

// run ticks once a second over [from, from+d) and returns the last snapshot.
func run(e *Engine, s *model.Snapshot, from time.Time, d time.Duration) *model.Snapshot {
	var out *model.Snapshot
	for t := from; t.Before(from.Add(d)); t = t.Add(time.Second) {
		out = tick(e, s, t)
	}
	return out
}

func phrase(s *model.Snapshot) string { return strings.Join(s.Phrase.Lines, " ") }

func TestInitialBoredAdoption(t *testing.T) {
	e := newEngine(t, "")
	s := tick(e, idle(), t0)
	if s.Mood != Bored || *s.PanicCount != 0 {
		t.Fatalf("mood %q panic %v", s.Mood, s.PanicCount)
	}
	if phrase(s) != "THE CPU IS IDLE AT 1%. I'VE NEVER ONCE BEEN IDLE." {
		t.Errorf("phrase %q", phrase(s))
	}
}

func TestBeforeFirstCompleteSample(t *testing.T) {
	e := newEngine(t, "")
	s := idle()
	s.CPU.TotalPct = nil
	out := tick(e, s, t0)
	if out.Mood != Content || phrase(out) != "I'M STILL HERE. NOBODY ASKED." || out.GPULine != "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY." {
		t.Errorf("first tick: %q %q %q", out.Mood, phrase(out), out.GPULine)
	}
	if out = tick(e, idle(), t0.Add(time.Second)); out.Mood != Bored {
		t.Errorf("first complete sample adopts without hysteresis: %q", out.Mood)
	}
}

func TestSpikeShorterThanHysteresis(t *testing.T) {
	e := newEngine(t, "")
	tick(e, idle(), t0)
	hot := idle()
	hot.CPU.TempC = fp(85)
	if s := run(e, hot, t0.Add(time.Second), 60*time.Second); s.Mood != Bored {
		t.Fatalf("85 °C for 60 s changed the mood to %q", s.Mood)
	}
	if s := run(e, idle(), t0.Add(61*time.Second), 200*time.Second); s.Mood != Bored {
		t.Errorf("after the spike: %q", s.Mood)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDoomedEntryAndPanicCount(t *testing.T) {
	dir := t.TempDir()
	e := newEngine(t, dir)
	path := filepath.Join(dir, PanicFile)
	if readFile(t, path) != "0\n" {
		t.Fatalf("first run must create 0, got %q", readFile(t, path))
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, %v", fi.Mode(), err)
	}
	tick(e, idle(), t0)
	hot := idle()
	hot.CPU.TempC = fp(92)
	s := run(e, hot, t0.Add(time.Second), 89*time.Second)
	if s.Mood != Bored {
		t.Fatalf("doomed before 90 s: %q", s.Mood)
	}
	s = run(e, hot, t0.Add(90*time.Second), 6*time.Second) // held 95 s in total
	if s.Mood != Doomed || *s.PanicCount != 1 || readFile(t, path) != "1\n" {
		t.Fatalf("after 95 s at 92 °C: %q count %v file %q", s.Mood, s.PanicCount, readFile(t, path))
	}

	// Restart while still at 92 °C: initial adoption is doomed, the count stays 1.
	e2 := newEngine(t, dir)
	s = run(e2, hot, t0.Add(200*time.Second), 120*time.Second)
	if s.Mood != Doomed || *s.PanicCount != 1 || readFile(t, path) != "1\n" {
		t.Fatalf("restart: %q count %v file %q", s.Mood, s.PanicCount, readFile(t, path))
	}
	// Leave (90 s) and re-enter (90 s): 2.
	s = run(e2, idle(), t0.Add(320*time.Second), 91*time.Second)
	if s.Mood != Bored {
		t.Fatalf("leave doomed: %q", s.Mood)
	}
	s = run(e2, hot, t0.Add(411*time.Second), 91*time.Second)
	if s.Mood != Doomed || *s.PanicCount != 2 || readFile(t, path) != "2\n" {
		t.Fatalf("re-enter: %q count %v file %q", s.Mood, s.PanicCount, readFile(t, path))
	}
}

func TestCorruptPanicFileIsNeverOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, PanicFile)
	if err := os.WriteFile(path, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	e := newEngine(t, dir)
	tick(e, idle(), t0)
	hot := idle()
	hot.CPU.TempC = fp(95)
	s := run(e, hot, t0.Add(time.Second), 100*time.Second)
	if s.Mood != Doomed || s.PanicCount != nil || readFile(t, path) != "x" {
		t.Errorf("corrupt: mood %q count %v file %q", s.Mood, s.PanicCount, readFile(t, path))
	}
}

func TestUnreadableStateDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, PanicFile), []byte("3\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Error(err)
		}
	}()
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	if s := tick(newEngine(t, dir), idle(), t0); s.PanicCount != nil {
		t.Errorf("unreadable dir: count %v, want null", *s.PanicCount)
	}
}

func TestMelancholicNeedsHoldPlusHysteresis(t *testing.T) {
	e := newEngine(t, "")
	busy := idle()
	busy.CPU.Load1 = fp(11) // 0.92 per thread
	if s := tick(e, busy, t0); s.Mood != Content {
		t.Fatalf("initial: %q (melancholic needs 120 s held)", s.Mood)
	}
	if s := run(e, busy, t0.Add(time.Second), 209*time.Second); s.Mood != Content {
		t.Fatalf("at 209 s: %q", s.Mood)
	}
	if s := tick(e, busy, t0.Add(210*time.Second)); s.Mood != Melancholic {
		t.Errorf("at 210 s (120 + 90): %q", s.Mood)
	}
}

func TestNullInputsNeverTrigger(t *testing.T) {
	s := idle()
	s.CPU.TempC, s.CPU.IowaitPct, s.CPU.Load1, s.Temps.NvmeC = nil, nil, nil, nil
	for i := range s.GPUs {
		s.GPUs[i].TempC = nil
	}
	for i := range s.Storage {
		s.Storage[i].UsedBytes, s.Storage[i].FreeBytes = nil, nil
	}
	s.Smart.State = "unknown"
	if out := tick(newEngine(t, ""), s, t0); out.Mood != Content {
		t.Errorf("all-null inputs: %q", out.Mood)
	}
}

// texts returns the phrase at each listed offset (seconds) while ticking every second.
func texts(e *Engine, s *model.Snapshot, start time.Time, until int, at ...int) map[int]string {
	want := map[int]bool{}
	for _, a := range at {
		want[a] = true
	}
	out := map[int]string{}
	for i := 0; i <= until; i++ {
		p := phrase(tick(e, s, start.Add(time.Duration(i)*time.Second)))
		if want[i] {
			out[i] = p
		}
	}
	return out
}

// shownLog runs the engine every second and records which line id was on screen.
func shownLog(e *Engine, s *model.Snapshot, start time.Time, secs int) []string {
	ids := make([]string, secs)
	for i := 0; i < secs; i++ {
		tick(e, s, start.Add(time.Duration(i)*time.Second))
		ids[i] = e.cur.id
	}
	return ids
}

// TestRotationAndCooldowns checks the selection rules over two idle hours: the line changes
// at the 120 s rotation whenever an alternative exists, and a newly selected line was last
// shown >= 10 min ago and its family >= 3 min ago (the line on screen may continue).
func TestRotationAndCooldowns(t *testing.T) {
	e := newEngine(t, "")
	ids := shownLog(e, idle(), t0, 7200)
	if ids[0] != "B1" || ids[119] != "B1" || ids[120] == "B1" {
		t.Fatalf("rotation at 120 s: %s / %s / %s", ids[0], ids[119], ids[120])
	}
	fam := map[string]string{}
	for _, l := range table {
		fam[l.id] = l.family
	}
	lastID, lastFam := map[string]int{}, map[string]int{}
	changes := 0
	for i, id := range ids {
		if i > 0 && id != ids[i-1] {
			changes++
			if at, ok := lastID[id]; ok && i-at < 600 {
				t.Fatalf("%s re-selected at %d s, last shown %d s (exact-line cooldown)", id, i, at)
			}
			if at, ok := lastFam[fam[id]]; ok && i-at < 180 {
				t.Fatalf("%s (family %s) at %d s, family last shown %d s", id, fam[id], i, at)
			}
			if i%120 != 0 {
				t.Fatalf("selection at %d s outside the 120 s rotation", i)
			}
		}
		lastID[id], lastFam[fam[id]] = i, i
	}
	if changes < 30 {
		t.Errorf("only %d rotations in two hours", changes)
	}
}

func TestFamilyCooldown(t *testing.T) {
	e := newEngine(t, "")
	mk := func(id, fam string) line {
		return line{id: id, family: fam, moods: []string{Bored}, tpl: "LINE " + id + ".", cond: always}
	}
	e.lines = []line{mk("X", "fam"), mk("Y", "fam"), mk("Z", "other")}
	got := texts(e, idle(), t0, 360, 0, 120, 240, 360)
	want := map[int]string{0: "LINE X.", 120: "LINE Z.", 240: "LINE Z.", 360: "LINE Y."}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("at %d s: %q, want %q (family cooldown 3 min)", k, got[k], w)
		}
	}
}

func TestFallbackAndTopicRule(t *testing.T) {
	// GPU0 at 85 °C, fans silent, iowait calm: aggrieved; the GPU line is hot, so the
	// GPU-heat lines A2, A6, A7 are excluded and nothing else is true -> fallback.
	s := idle()
	s.GPUs[0].TempC = fp(85)
	s.Fans[0].RPM, s.Fans[1].RPM = nil, nil
	e := newEngine(t, "")
	out := tick(e, s, t0)
	if out.Mood != Aggrieved || !strings.Contains(out.GPULine, "85") {
		t.Fatalf("setup: %q %q", out.Mood, out.GPULine)
	}
	if phrase(out) != "I'M STILL HERE. NOBODY ASKED." {
		t.Errorf("topic rule + nothing eligible: %q", phrase(out))
	}
	v := view{s: s, mood: Aggrieved, now: t0, gpuHot: true}
	for _, l := range table {
		_, ok := e.eligible(&l, v)
		switch l.id {
		case "A2", "A6", "A7":
			if ok {
				t.Errorf("%s about GPU heat must be excluded while the GPU line is hot", l.id)
			}
			if _, ok := e.eligible(&l, view{s: s, mood: Aggrieved, now: t0, gpuHot: false}); !ok {
				t.Errorf("%s must speak when the GPU line is not hot", l.id)
			}
		}
	}
	// With the exhaust fans reporting, A8 (not in the topic set) speaks.
	s.Fans[exhaust].RPM = ip(1200)
	l := table[indexOf(t, "A8")]
	if _, ok := e.eligible(&l, v); !ok {
		t.Error("A8 is not covered by the topic rule")
	}
	// CPU hotter than the GPU: the heat lines are about the CPU and may speak.
	s.CPU.TempC = fp(86)
	if _, ok := e.eligible(&table[indexOf(t, "A6")], v); !ok {
		t.Error("A6 about the CPU must speak")
	}
	// Doomed GPU heat: D lines are never excluded.
	d := idle()
	d.GPUs[1].TempC = fp(92)
	if p := phrase(tick(newEngine(t, ""), d, t0)); !strings.Contains(p, "GPU1") || !strings.Contains(p, "92") {
		t.Errorf("doomed line with the GPU hot: %q", p)
	}
}

func indexOf(t *testing.T, id string) int {
	t.Helper()
	for i, l := range table {
		if l.id == id {
			return i
		}
	}
	t.Fatalf("no line %s", id)
	return -1
}

func TestDoomReassertMatchingTriggerOnly(t *testing.T) {
	s := idle()
	s.Storage[1].UsedBytes, s.Storage[1].FreeBytes = i64(97), i64(3)
	ids := shownLog(newEngine(t, ""), s, t0, 900)
	seen := map[string]bool{}
	for i, id := range ids {
		if id != "D1" && id != "D4" && id != "D5" {
			t.Fatalf("at %d s: %s, want only the space lines (the active trigger)", i, id)
		}
		if i > 0 && id != ids[i-1] && i%60 != 0 {
			t.Fatalf("doomed re-selection at %d s, want every 60 s", i)
		}
		seen[id] = true
	}
	if len(seen) != 3 {
		t.Errorf("space lines shown: %v (re-assert ignores cooldowns, so all three cycle)", seen)
	}
	e := newEngine(t, "")
	tick(e, s, t0)
	if p := phrase(tick(e, s, t0.Add(time.Second))); !strings.Contains(p, "/srv/hogdata") {
		t.Errorf("doomed space line must name the mount: %q", p)
	}
	// Add a heat trigger: heat lines join, each names the device.
	s.CPU.TempC = fp(95)
	e = newEngine(t, "")
	heat := 0
	for i := 0; i < 900; i++ {
		out := tick(e, s, t0.Add(time.Duration(i)*time.Second))
		switch e.cur.family {
		case "doom-heat":
			heat++
			if !strings.Contains(phrase(out), "CPU") {
				t.Fatalf("heat line without the device: %q", phrase(out))
			}
		case "space":
		default:
			t.Fatalf("non-trigger line %s in doomed", e.cur.id)
		}
	}
	if heat == 0 {
		t.Error("heat trigger never spoke")
	}
}

func TestTimeWindows(t *testing.T) {
	zone := time.FixedZone("EDT", -4*3600)
	run := func(hour int) map[string]bool {
		e := newEngine(t, "")
		seen := map[string]bool{}
		for _, id := range shownLog(e, idle(), time.Date(2026, 10, 1, hour, 0, 0, 0, zone), 3600) {
			seen[id] = true
		}
		return seen
	}
	if n := run(3); !n["S3"] || !n["S6"] || n["S9"] {
		t.Errorf("03:00-03:59: %v", n)
	}
	if m := run(7); !m["S9"] || m["S3"] || m["S6"] {
		t.Errorf("07:00-07:59: %v", m)
	}
	if d := run(12); d["S3"] || d["S6"] || d["S9"] {
		t.Errorf("12:00-12:59: %v", d)
	}
	v := view{s: idle(), now: time.Date(2026, 10, 1, 4, 7, 0, 0, zone)}
	if txt, _ := table[indexOf(t, "S6")].text(v); txt != "IT'S 04:07. EVERYONE ELSE IS ASLEEP. SOMEBODY HAS TO WATCH THE SHIP." {
		t.Errorf("S6: %q", txt)
	}
}

func TestGPULinePacingAndPools(t *testing.T) {
	e := newEngine(t, "")
	tick(e, idle(), t0)
	busy := idle()
	busy.GPUs[0].UtilPct, busy.GPUs[1].UtilPct = fp(50), fp(50)
	var s *model.Snapshot
	for i := 1; i <= 30; i++ {
		s = tick(e, busy, t0.Add(time.Duration(i)*time.Second))
	}
	if s.GPULine != "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY." {
		t.Fatalf("state changed before 30 s held: %q", s.GPULine)
	}
	if s = tick(e, busy, t0.Add(31*time.Second)); s.GPULine != "SOMEONE ASKED IT SOMETHING. NOT ME." {
		t.Fatalf("after 30 s held: %q", s.GPULine)
	}
	// Within a state the pool rotates every 5 min, least recently shown first; a line
	// returns only after 10 min.
	e = newEngine(t, "")
	want := map[int]string{
		0: "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY.", 299: "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY.",
		300: "NOTHING LOADED. NOTHING ASKED.", 600: "TWO IDLE CARDS. I KNOW THE FEELING.",
		900: "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY.",
	}
	for i := 0; i <= 900; i++ {
		out := tick(e, idle(), t0.Add(time.Duration(i)*time.Second))
		if w, ok := want[i]; ok && out.GPULine != w {
			t.Errorf("at %d s: %q, want %q", i, out.GPULine, w)
		}
	}
	// Numbers are rendered at selection: hot text refreshes at the 5 min rotation.
	e = newEngine(t, "")
	hot := idle()
	hot.GPUs[0].TempC = fp(85)
	tick(e, hot, t0)
	hot.GPUs[0].TempC = fp(88)
	if s = run(e, hot, t0.Add(time.Second), 298*time.Second); s.GPULine != "THINKING THIS HARD RUNS AT 85 DEGREES." {
		t.Fatalf("before the refresh: %q", s.GPULine)
	}
	if s = tick(e, hot, t0.Add(300*time.Second)); s.GPULine != "88 DEGREES OF PURE THOUGHT. NOT MINE." {
		t.Errorf("at the refresh: %q", s.GPULine)
	}
}

func TestEngineOutputIsValidSnapshot(t *testing.T) {
	e := newEngine(t, "")
	for i, s := range []*model.Snapshot{idle(), func() *model.Snapshot { s := idle(); s.CPU.TempC = fp(95); return s }()} {
		out := run(e, s, t0.Add(time.Duration(i)*time.Hour), 200*time.Second)
		if err := model.CheckAgreement(out); err != nil {
			t.Error(err)
		}
		if out.Mood == "" || len(out.Phrase.Lines) == 0 || out.GPULine == "" || len([]rune(out.GPULine)) > 38 {
			t.Errorf("engine fields: %q %q %q", out.Mood, out.Phrase.Lines, out.GPULine)
		}
		blob, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(blob))
		dec.DisallowUnknownFields()
		var back model.Snapshot
		if err := dec.Decode(&back); err != nil {
			t.Fatal(err)
		}
	}
}
