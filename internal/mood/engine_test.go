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
	s.Fans = []model.Fan{{Bank: 1, Label: "FAN BANK 1", RPM: ip(1000), MaxRPM: ip(2000), Verdict: "ok"},
		{Bank: 2, Label: "FAN BANK 2", RPM: ip(1100), MaxRPM: ip(2000), Verdict: "ok"}}
	s.Network = []model.Net{{If: "wlp14s0", RxBps: i64(2000), TxBps: i64(1000), RxErr: ip(0), TxErr: ip(0)}}
	s.DiskIO = model.DiskIO{Device: "nvme0n1"}
	s.Connections = model.Connections{Established: ip(10)}
	return s
}

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

func TestRotationAndExactLineCooldown(t *testing.T) {
	e := newEngine(t, "")
	b1 := "THE CPU IS IDLE AT 1%. I'VE NEVER ONCE BEEN IDLE."
	s2 := "0.0 MIB/S INBOUND AND STILL NOBODY CALLS."
	seen := map[int]string{}
	for i := 0; i <= 840; i++ {
		seen[i] = phrase(tick(e, idle(), t0.Add(time.Duration(i)*time.Second)))
	}
	if seen[0] != b1 || seen[119] != b1 || seen[120] != s2 {
		t.Fatalf("rotation at 120 s: %q / %q / %q", seen[0], seen[119], seen[120])
	}
	for i := 120; i < 720; i++ {
		if seen[i] == b1 {
			t.Fatalf("B1 repeated at %d s, inside its 10 min cooldown", i)
		}
	}
	// At 720 s the bored dwell passes 11 min: B2 (never shown) wins least-recently-shown;
	// B1, out of cooldown since 719 s, returns at the next rotation.
	if !strings.HasPrefix(seen[720], "NOTHING IS HAPPENING") || seen[840] != b1 {
		t.Errorf("at 720 s %q (want B2), at 840 s %q (want B1)", seen[720], seen[840])
	}
}

func TestFamilyCooldown(t *testing.T) {
	e := newEngine(t, "")
	mk := func(id, fam string) line {
		return line{id: id, family: fam, moods: []string{Bored}, render: func(view) (string, bool) { return "LINE " + id + ".", true }}
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
	// GPU0 at 85 °C: aggrieved; the GPU line is hot, so A2 (about GPU heat) is excluded and
	// iowait is calm: nothing is eligible -> fallback.
	s := idle()
	s.GPUs[0].TempC = fp(85)
	out := tick(newEngine(t, ""), s, t0)
	if out.Mood != Aggrieved || !strings.HasPrefix(out.GPULine, "THINKING THIS HARD") {
		t.Fatalf("setup: %q %q", out.Mood, out.GPULine)
	}
	if phrase(out) != "I'M STILL HERE. NOBODY ASKED." {
		t.Errorf("topic rule + nothing eligible: %q", phrase(out))
	}
	// CPU hotter than the GPU: A2 is about the CPU and may speak.
	s.CPU.TempC = fp(86)
	if p := phrase(tick(newEngine(t, ""), s, t0)); p != "86 DEGREES. I RAN COLD ONCE. NOBODY NOTICED." {
		t.Errorf("A2 about the CPU: %q", p)
	}
	// Doomed GPU heat: D3 still speaks while the GPU line is hot.
	d := idle()
	d.GPUs[1].TempC = fp(92)
	if p := phrase(tick(newEngine(t, ""), d, t0)); p != "GPU1 IS AT 92 DEGREES. I DID WARN YOU. I ALWAYS WARN YOU." {
		t.Errorf("D3 with GPU hot: %q", p)
	}
}

func TestDoomReassertIgnoresCooldown(t *testing.T) {
	s := idle()
	s.Storage[1].UsedBytes, s.Storage[1].FreeBytes = i64(97), i64(3)
	e := newEngine(t, "")
	d1 := "3% REMAINS ON /srv/hogdata. I'D TELL YOU WHAT THAT MEANS BUT YOU'D ONLY CLEAN SOMETHING IMPORTANT."
	for i := 0; i <= 900; i += 1 {
		if p := phrase(tick(e, s, t0.Add(time.Duration(i)*time.Second))); p != d1 {
			t.Fatalf("at %d s: %q, want D1 re-asserting (cooldown ignored)", i, p)
		}
	}
	// Two triggers alternate every 60 s; each names its mount or device.
	s.CPU.TempC = fp(95)
	got := texts(newEngine(t, ""), s, t0, 180, 0, 59, 60, 120, 180)
	if got[0] != d1 || got[59] != d1 || !strings.HasPrefix(got[60], "CPU IS AT 95 DEGREES") || got[120] != d1 || !strings.HasPrefix(got[180], "CPU IS AT 95") {
		t.Errorf("doomed alternation: %v", got)
	}
}

func TestNightLine(t *testing.T) {
	zone := time.FixedZone("EDT", -4*3600)
	night := time.Date(2026, 10, 1, 3, 0, 0, 0, zone)
	got := texts(newEngine(t, ""), idle(), night, 240, 240)
	if got[240] != "I'M NOT ASLEEP. I'M IGNORING YOU WITH MY EYES CLOSED." {
		t.Errorf("night at 03:04 local: %q", got[240])
	}
	day := time.Date(2026, 10, 1, 6, 0, 0, 0, zone)
	for i, p := range texts(newEngine(t, ""), idle(), day, 600, 0, 120, 240, 360, 480, 600) {
		if strings.Contains(p, "ASLEEP") {
			t.Errorf("night line at 06:%02d", i/60)
		}
	}
}

func TestGPULinePacing(t *testing.T) {
	e := newEngine(t, "")
	tick(e, idle(), t0)
	busy := idle()
	busy.GPUs[0].UtilPct, busy.GPUs[1].UtilPct = fp(50), fp(50)
	var s *model.Snapshot
	for i := 1; i <= 30; i++ {
		s = tick(e, busy, t0.Add(time.Duration(i)*time.Second))
		if i == 30 && s.GPULine != "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY." {
			t.Fatalf("state changed before 30 s held: %q", s.GPULine)
		}
	}
	if s = tick(e, busy, t0.Add(31*time.Second)); s.GPULine != "SOMEONE ASKED IT SOMETHING. NOT ME." {
		t.Fatalf("after 30 s held: %q", s.GPULine)
	}
	// Within the hot state the number refreshes only every 5 min.
	hot := idle()
	hot.GPUs[0].TempC = fp(85)
	run(e, hot, t0.Add(100*time.Second), 31*time.Second) // enters hot at 130 s
	hot.GPUs[0].TempC = fp(88)
	if s = run(e, hot, t0.Add(131*time.Second), 299*time.Second); s.GPULine != "THINKING THIS HARD RUNS AT 85 DEGREES." {
		t.Fatalf("before the 5 min refresh: %q", s.GPULine)
	}
	if s = tick(e, hot, t0.Add(430*time.Second)); s.GPULine != "THINKING THIS HARD RUNS AT 88 DEGREES." {
		t.Errorf("at the 5 min refresh: %q", s.GPULine)
	}
}

func TestEveryLineWithinBudgetAtWorstCase(t *testing.T) {
	s := idle()
	s.CPU.TotalPct, s.CPU.IowaitPct, s.CPU.TempC = fp(100), fp(100), fp(999)
	s.Temps.NvmeC = fp(999)
	s.Storage[1].UsedBytes, s.Storage[1].FreeBytes = i64(100), i64(0)
	s.Smart.State = "failing"
	s.Fans[0].RPM = nil
	s.Network[0].RxBps = i64(999 * 1048576)
	night := time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC)
	all := append(append([]line{}, table...), fallbackLine)
	for _, l := range all {
		for _, m := range []string{Doomed, Aggrieved, Melancholic, Bored, Content} {
			v := view{s: s, mood: m, dwell: 20 * time.Minute, now: night}
			text, ok := l.render(v)
			if !ok {
				if l.id == "C1" {
					continue // C1 is false with a failing drive; checked below
				}
				t.Errorf("%s: not true at worst case", l.id)
				continue
			}
			if _, fits := wrap(text); !fits {
				t.Errorf("%s over budget: %q", l.id, text)
			}
		}
	}
	if _, fits := wrap("ALL SYSTEMS NOMINAL. THEY'RE ALWAYS NOMINAL RIGHT BEFORE SOMETHING."); !fits {
		t.Error("C1 over budget")
	}
	if text, _ := table[7].render(view{s: s}); !strings.HasPrefix(text, "0% REMAINS ON /srv/hogdata.") {
		t.Errorf("D1 at 0%%: %q", text)
	}
	if _, fits := wrap(strings.Repeat("A", 53)); fits {
		t.Error("a 53-rune word must be over budget")
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
