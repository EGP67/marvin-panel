package mood

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// worst returns a snapshot with every placeholder at its widest (D-059 Step 6). spaceGiB
// selects the mount variant: 0 % free (used 100) or 9999.9 GiB free on a huge mount.
func worst(spaceGiB bool) *model.Snapshot {
	s := idle()
	s.CPU.TotalPct, s.CPU.FreqGHz, s.CPU.Load1, s.CPU.IowaitPct, s.CPU.TempC = fp(100), fp(9.9), fp(99.99), fp(100), fp(999)
	s.Connections.Established = ip(99999)
	big := i64(999 * 1048576)
	s.DiskIO.ReadBps, s.DiskIO.WriteBps = big, big
	s.Network[0].RxBps, s.Network[0].TxBps = big, big
	s.Network[0].RxErr, s.Network[0].TxErr = ip(99999), ip(0)
	s.Memory.UsedPct = fp(100)
	up := int64(9999*86400 + 5)
	s.Host.UptimeSeconds = &up
	s.Temps.NvmeC = fp(999)
	for i := range s.GPUs {
		s.GPUs[i].TempC = fp(999)
	}
	s.Fans[0].RPM = ip(9999)
	s.PanicCount = ip(99999)
	if spaceGiB {
		free := int64(99999) * (1 << 30) / 10 // 9999.9 GiB
		s.Storage[1].UsedBytes, s.Storage[1].FreeBytes = i64(free*999), i64(free)
	} else {
		s.Storage[1].UsedBytes, s.Storage[1].FreeBytes = i64(100), i64(0)
	}
	return s
}

func TestEveryLineWithinBudgetAtWorstCase(t *testing.T) {
	zone := time.FixedZone("X", 0)
	for _, spaceGiB := range []bool{false, true} {
		v := view{s: worst(spaceGiB), now: time.Date(2026, 10, 1, 23, 59, 0, 0, zone), dwell: time.Hour}
		for _, l := range append(append([]line{}, table...), fallbackLine) {
			text, ok := render(l.tpl, v)
			if !ok {
				if l.id == "S1" {
					continue // {b} needs a null fan; checked below
				}
				t.Errorf("%s: a placeholder did not resolve at worst case", l.id)
				continue
			}
			if out, fits := wrap(text); !fits {
				t.Errorf("%s over budget (%d runes): %q -> %q", l.id, runes(text), text, out)
			}
		}
	}
	s := worst(false)
	s.Fans[1].RPM = nil
	if text, ok := render(table[indexOf(t, "S1")].tpl, view{s: s}); !ok || !fitsBudget(text) {
		t.Errorf("S1 at worst case: %q", text)
	}
	if !strings.Contains(mustRender(t, "D4", view{s: worst(false)}), "/srv/hogdata IS 100% FULL. 0% LEFT.") {
		t.Error("D4 at 0% free")
	}
	if !strings.Contains(mustRender(t, "D5", view{s: worst(true)}), "ONLY 9999.9 GIB REMAIN ON /srv/hogdata.") {
		t.Error("D5 at 9999.9 GiB")
	}
	// GPU pools at worst case (n 999, util 100, vram 24.0).
	gs := []model.GPUState{
		{TempC: fp(999), UtilPct: fp(100), MemUsedMiB: ip(12288)},
		{TempC: fp(999), UtilPct: fp(100), MemUsedMiB: ip(12288)},
	}
	for kind, pool := range model.GPULinePools {
		for _, tpl := range append(append([]string{}, pool...), model.GPUHotShort) {
			text, ok := model.RenderGPULine(tpl, gs)
			if !ok || runes(text) < 1 || runes(text) > 38 {
				t.Errorf("GPU %s %q -> %q (%d runes)", kind, tpl, text, runes(text))
			}
		}
	}
	if text, _ := model.RenderGPULine("{vram} GIB OF MODEL, DOING NOTHING.", gs); text != "24.0 GIB OF MODEL, DOING NOTHING." {
		t.Errorf("vram: %q", text)
	}
}

func fitsBudget(text string) bool { _, ok := wrap(text); return ok }

func mustRender(t *testing.T, id string, v view) string {
	t.Helper()
	text, ok := render(table[indexOf(t, id)].tpl, v)
	if !ok {
		t.Fatalf("%s did not render", id)
	}
	return text
}

// TestConditions makes every line's condition true and then false.
func TestConditions(t *testing.T) {
	type mut func(v *view)
	set := func(f func(s *model.Snapshot)) mut { return func(v *view) { f(v.s) } }
	hotGPU := set(func(s *model.Snapshot) { s.GPUs[0].TempC = fp(85) })
	cool := set(func(*model.Snapshot) {})
	doomHot := set(func(s *model.Snapshot) { s.CPU.TempC = fp(92) })
	iowait := set(func(s *model.Snapshot) { s.CPU.IowaitPct = fp(20) })
	lowSpace := set(func(s *model.Snapshot) { s.Storage[2].UsedBytes, s.Storage[2].FreeBytes = i64(98), i64(2) })
	failing := set(func(s *model.Snapshot) { s.Smart.State = "failing" })
	noFans := set(func(s *model.Snapshot) { s.Fans[0].RPM = nil })
	hour := func(h int) mut { return func(v *view) { v.now = time.Date(2026, 10, 1, h, 30, 0, 0, time.UTC) } }
	upDays := func(d int64) mut {
		return set(func(s *model.Snapshot) { u := d * 86400; s.Host.UptimeSeconds = &u })
	}
	both := func(a, b mut) mut { return func(v *view) { a(v); b(v) } }
	cases := map[string][2]mut{
		"B1":  {cool, set(func(s *model.Snapshot) { s.CPU.TotalPct = nil })},
		"B2":  {func(v *view) { v.dwell = 11 * time.Minute }, func(v *view) { v.dwell = 10 * time.Minute }},
		"B3":  {cool, set(func(s *model.Snapshot) { s.CPU.TotalPct = nil })},
		"B4":  {set(func(s *model.Snapshot) { s.CPU.FreqGHz = fp(2.9) }), set(func(s *model.Snapshot) { s.CPU.FreqGHz = fp(3.0) })},
		"B5":  {cool, set(func(s *model.Snapshot) { s.Connections.Established = nil })},
		"B6":  {set(func(s *model.Snapshot) { s.DiskIO.ReadBps = i64(1048575) }), set(func(s *model.Snapshot) { s.DiskIO.ReadBps = i64(1048576) })},
		"B7":  {set(func(s *model.Snapshot) { s.Memory.UsedPct = fp(29.9) }), set(func(s *model.Snapshot) { s.Memory.UsedPct = fp(30) })},
		"B8":  {cool, set(func(s *model.Snapshot) { s.CPU.TotalPct = fp(5) })},
		"B9":  {cool, noFans},
		"B10": {upDays(1), upDays(0)},
		"C1":  {cool, failing},
		"C2":  {cool, set(func(s *model.Snapshot) { s.GPUs[1].ThermalBand = nil })},
		"C3":  {cool, set(func(s *model.Snapshot) { s.CPU.TempC = nil })},
		"C4":  {cool, set(func(s *model.Snapshot) { s.Smart.State = "unknown" })},
		"C5":  {cool, set(func(s *model.Snapshot) { s.Storage[0].State = band(model.BandWarn) })},
		"C6":  {cool, set(func(s *model.Snapshot) { s.Network[0].RxErr = ip(1) })},
		"C7":  {cool, set(func(s *model.Snapshot) { s.Fans[1].Verdict = "stalled" })},
		"C8":  {upDays(3), upDays(0)},
		"C9":  {cool, set(func(s *model.Snapshot) { s.Temps.NvmeThermalBand = band(model.BandWarn) })},
		"C10": {cool, func(v *view) { v.s.PanicCount = nil }},
		"M1":  {cool, set(func(s *model.Snapshot) { s.CPU.TotalPct = nil })},
		"M2":  {cool, set(func(s *model.Snapshot) { s.CPU.TempC = nil })},
		"M3":  {cool, set(func(s *model.Snapshot) { s.CPU.Load1 = nil })},
		"M4":  {set(func(s *model.Snapshot) { s.CPU.TotalPct = fp(50) }), set(func(s *model.Snapshot) { s.CPU.TotalPct = fp(49.9) })},
		"M5":  {cool, set(func(s *model.Snapshot) { s.CPU.FreqGHz = nil })},
		"M6":  {set(func(s *model.Snapshot) { s.CPU.TempC = fp(79.9) }), set(func(s *model.Snapshot) { s.CPU.TempC = fp(80) })},
		"M7":  {cool, set(func(*model.Snapshot) {})}, // always; false case is n/a
		"M8":  {set(func(s *model.Snapshot) { s.Memory.UsedPct = fp(50) }), set(func(s *model.Snapshot) { s.Memory.UsedPct = fp(49.9) })},
		"M9":  {cool, noFans},
		"M10": {cool, set(func(s *model.Snapshot) { s.CPU.Load1 = nil })},
		"A1":  {iowait, cool},
		"A2":  {hotGPU, cool},
		"A3":  {iowait, cool},
		"A4":  {both(iowait, set(func(s *model.Snapshot) { s.DiskIO.WriteBps = i64(1048576) })), iowait},
		"A5":  {both(iowait, set(func(s *model.Snapshot) { s.DiskIO.ReadBps = i64(2 * 1048576) })), both(iowait, set(func(s *model.Snapshot) { s.DiskIO.ReadBps = i64(1000) }))},
		"A6":  {hotGPU, cool},
		"A7":  {hotGPU, cool},
		"A8":  {hotGPU, both(hotGPU, noFans)},
		"A9":  {set(func(s *model.Snapshot) { s.Temps.NvmeC = fp(70) }), set(func(s *model.Snapshot) { s.Temps.NvmeC = fp(69) })},
		"A10": {iowait, cool},
		"D1":  {lowSpace, cool},
		"D2":  {failing, cool},
		"D3":  {doomHot, hotGPU},
		"D4":  {lowSpace, cool},
		"D5":  {lowSpace, cool},
		"D6":  {failing, cool},
		"D7":  {failing, cool},
		"D8":  {doomHot, cool},
		"D9":  {doomHot, cool},
		"D10": {doomHot, both(doomHot, noFans)},
		"S1":  {noFans, cool},
		"S2":  {cool, set(func(s *model.Snapshot) { s.Network[0].RxBps = nil })},
		"S3":  {hour(3), hour(6)},
		"S4":  {cool, set(func(s *model.Snapshot) { s.Network[0].TxBps = nil })},
		"S5":  {set(func(s *model.Snapshot) { s.Network[0].TxErr = ip(2) }), cool},
		"S6":  {hour(0), hour(6)},
		"S7":  {upDays(7), upDays(6)},
		"S8":  {func(v *view) { v.s.PanicCount = ip(1) }, func(v *view) { v.s.PanicCount = ip(0) }},
		"S9":  {hour(8), hour(9)},
	}
	for _, l := range table {
		c, ok := cases[l.id]
		if !ok {
			t.Errorf("%s has no condition case", l.id)
			continue
		}
		for i, want := range []bool{true, false} {
			if l.id == "M7" && !want {
				continue
			}
			v := view{s: idle(), now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), dwell: time.Minute}
			v.s.PanicCount = ip(0)
			c[i](&v)
			if _, got := l.text(v); got != want {
				t.Errorf("%s: eligible=%v, want %v", l.id, got, want)
			}
		}
	}
}

func TestNullPlaceholderMakesLineIneligible(t *testing.T) {
	cases := map[string]func(s *model.Snapshot){
		"B3":  func(s *model.Snapshot) { s.CPU.TotalPct = nil },
		"B5":  func(s *model.Snapshot) { s.Connections.Established = nil },
		"C7":  func(s *model.Snapshot) { s.Fans[0].RPM = nil },
		"S4":  func(s *model.Snapshot) { s.Network[0].TxBps = nil },
		"M3":  func(s *model.Snapshot) { s.CPU.Load1 = nil },
		"C10": func(s *model.Snapshot) { s.PanicCount = nil },
	}
	for id, f := range cases {
		s := idle()
		s.PanicCount = ip(0)
		f(s)
		if _, ok := table[indexOf(t, id)].text(view{s: s, now: t0}); ok {
			t.Errorf("%s spoke with a null placeholder", id)
		}
	}
}

func TestFamilies(t *testing.T) {
	want := map[string]string{
		"B1": "idle", "B2": "dwell", "B3": "idle", "B4": "freq", "B5": "conn", "B6": "disk", "B7": "memory", "B8": "idle", "B9": "load", "B10": "uptime",
		"C1": "nominal", "C2": "nominal", "C3": "cputemp", "C4": "smart", "C5": "space", "C6": "conn", "C7": "fans", "C8": "uptime", "C9": "nominal", "C10": "panic",
		"M1": "cpu-load", "M2": "probability", "M3": "load", "M4": "cpu-load", "M5": "freq", "M6": "cputemp", "M7": "load", "M8": "memory", "M9": "fans", "M10": "load",
		"A1": "iowait", "A2": "heat", "A3": "iowait", "A4": "disk", "A5": "disk", "A6": "heat", "A7": "heat", "A8": "heat", "A9": "nvme", "A10": "iowait",
		"D1": "space", "D2": "smart", "D3": "doom-heat", "D4": "space", "D5": "space", "D6": "smart", "D7": "smart", "D8": "doom-heat", "D9": "doom-heat", "D10": "doom-heat",
		"S1": "fans", "S2": "network", "S3": "time", "S4": "network", "S5": "network", "S6": "time", "S7": "uptime", "S8": "panic", "S9": "time",
	}
	if len(table) != len(want) {
		t.Errorf("table has %d lines, want %d", len(table), len(want))
	}
	for _, l := range table {
		if want[l.id] != l.family {
			t.Errorf("%s family %q, want %q", l.id, l.family, want[l.id])
		}
		gpuHeat := l.id == "A2" || l.id == "A6" || l.id == "A7"
		if l.gpuHeat != gpuHeat {
			t.Errorf("%s gpuHeat=%v", l.id, l.gpuHeat)
		}
	}
	if fallbackLine.family != "fallback" {
		t.Error("F family")
	}
}

// TestPoolMatchesMarvinMD keeps docs/MARVIN.md "Line pool" and the code in step (D-059).
func TestPoolMatchesMarvinMD(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "MARVIN.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	start := strings.Index(doc, "## Line pool (D-059)")
	if start < 0 {
		t.Fatal(`docs/MARVIN.md has no "## Line pool (D-059)" section`)
	}
	section := doc[start+1:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	row := regexp.MustCompile(`(?m)^\| (B\d+|C\d+|M\d+|A\d+|D\d+|S\d+|F) \| ([^|]+) \| ([^|]+) \| ([^|]+) \|$`)
	docLines := map[string][2]string{}
	for _, m := range row.FindAllStringSubmatch(section, -1) {
		docLines[m[1]] = [2]string{strings.TrimSpace(m[2]), strings.TrimSpace(m[3])}
	}
	code := append(append([]line{}, table...), fallbackLine)
	for _, l := range code {
		d, ok := docLines[l.id]
		if !ok {
			t.Errorf("%s missing from MARVIN.md Line pool", l.id)
			continue
		}
		if d[0] != l.family || d[1] != l.tpl {
			t.Errorf("%s drift:\n code %s | %s\n doc  %s | %s", l.id, l.family, l.tpl, d[0], d[1])
		}
	}
	if len(docLines) != len(code) {
		ids := map[string]bool{}
		for _, l := range code {
			ids[l.id] = true
		}
		for id := range docLines {
			if !ids[id] {
				t.Errorf("MARVIN.md lists %s, absent from the code", id)
			}
		}
	}
	gpuRow := regexp.MustCompile(`(?m)^\| (stale|hot|thinking|loaded|empty)( \(n >= 100\))? \| ([^|]+) \|$`)
	docGPU := map[string]bool{}
	for _, m := range gpuRow.FindAllStringSubmatch(section, -1) {
		docGPU[strings.TrimSpace(m[3])] = true
	}
	codeGPU := map[string]bool{model.GPUHotShort: true}
	for _, pool := range model.GPULinePools {
		for _, tpl := range pool {
			codeGPU[tpl] = true
		}
	}
	for tpl := range codeGPU {
		if !docGPU[tpl] {
			t.Errorf("GPU template %q missing from MARVIN.md", tpl)
		}
	}
	for tpl := range docGPU {
		if !codeGPU[tpl] {
			t.Errorf("MARVIN.md GPU template %q absent from the code", tpl)
		}
	}
}
