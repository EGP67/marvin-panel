package collect

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hog.local/marvin-panel/internal/model"
)

func row(bus, uuid string, util float64) gpuRow {
	t, u, m := 40.0, util, 100
	return gpuRow{name: gpuName, uuid: uuid, bus: bus, temp: &t, util: &u, memUsed: &m}
}

func TestParseGPUCSV(t *testing.T) {
	rows := parseGPUCSV([]byte("0, NVIDIA GeForce RTX 3060, GPU-fake-a, 00000000:01:00.0, 12288, 8099, 44, 3, [N/A], [Not Supported]\n" +
		"garbage line\n" +
		"1, NVIDIA GeForce RTX 3060, GPU-fake-b, 00000000:06:00.0, 12288, x, 36, 0, 9.83, 170.00\n"))
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (malformed line skipped)", len(rows))
	}
	r := rows[0]
	if r.bus != "00000000:01:00.0" || *r.memTotal != 12288 || *r.memUsed != 8099 || *r.temp != 44 || *r.util != 3 {
		t.Errorf("row0 %+v", r)
	}
	if r.power != nil || r.limit != nil {
		t.Error("[N/A] and [Not Supported] must be null")
	}
	if rows[1].memUsed != nil || *rows[1].power != 9.8 || *rows[1].limit != 170 {
		t.Errorf("row1: unparsable must be null, power rounded to 1 dp: %+v", rows[1])
	}
}

func TestGPUBindingByPCIThenUUID(t *testing.T) {
	g := newGPUSet()
	var buses []string
	bound := func(slot int, bus string) { buses = append(buses, bus) }
	// Rows swapped: GPU1's bus first.
	out, _ := g.apply([]gpuRow{row("00000000:06:00.0", "GPU-fake-b", 70), row("00000000:01:00.0", "GPU-fake-a", 10)}, bound)
	if out[0].UUID != "GPU-fake-a" || out[1].UUID != "GPU-fake-b" || *out[0].UtilPct != 10 || *out[1].UtilPct != 70 {
		t.Fatalf("bind by PCI: %v/%v", out[0].UUID, out[1].UUID)
	}
	if len(buses) != 2 {
		t.Errorf("bind logged %d times, want 2", len(buses))
	}
	// Afterwards UUIDs win: a card whose bus changed still lands in its slot.
	out, _ = g.apply([]gpuRow{row("00000000:01:00.0", "GPU-fake-b", 55), row("00000000:06:00.0", "GPU-fake-a", 5)}, bound)
	if *out[0].UtilPct != 5 || *out[1].UtilPct != 55 {
		t.Errorf("UUID tracking: GPU0 %v GPU1 %v", *out[0].UtilPct, *out[1].UtilPct)
	}
	if len(buses) != 2 {
		t.Error("rebinding must not happen after the first bind")
	}
}

func TestGPUMissingCardIsStale(t *testing.T) {
	g := newGPUSet()
	lim, tot := 170.0, 12288
	r0 := row("00000000:01:00.0", "GPU-fake-a", 10)
	r0.limit, r0.memTotal = &lim, &tot
	g.apply([]gpuRow{r0, row("00000000:06:00.0", "GPU-fake-b", 20)}, func(int, string) {})
	out, states := g.apply([]gpuRow{row("00000000:06:00.0", "GPU-fake-b", 20)}, func(int, string) {})
	s := out[0]
	if s.UtilPct != nil || s.TempC != nil || s.PowerW != nil || s.MemUsedMiB != nil || s.ThermalBand != nil || s.UtilSev != nil {
		t.Errorf("missing card must be stale: %+v", s)
	}
	if s.UUID != "GPU-fake-a" || *s.MemTotalMiB != 12288 || *s.PowerLimitW != 170 || s.DisplayName != "RTX 3060" {
		t.Errorf("stale card keeps identity and last good totals: %+v", s)
	}
	if s.HistUtilPct[29] != nil || *s.HistUtilPct[28] != 10 {
		t.Error("stale tick pushes null into the history")
	}
	if model.GPULine(states) != "THE BRAINS ARE NOT ANSWERING." {
		t.Error("a stale card makes the GPU line stale")
	}
	// Before any success: identity defaults, totals null.
	fresh, _ := newGPUSet().apply(nil, func(int, string) {})
	if fresh[0].UUID != "unbound" || fresh[0].MemTotalMiB != nil || fresh[0].PowerLimitW != nil {
		t.Errorf("before first success: %+v", fresh[0])
	}
}

// TestNvidiaSMITimeout is TASKS T7's verify: a hung nvidia-smi makes that tick stale
// and the tick still completes in < 1.2 s.
func TestNvidiaSMITimeout(t *testing.T) {
	c, err := New(Options{Root: copyRoot(t), Log: quiet(), NvidiaSMI: filepath.Join("testdata", "nvidia-smi-slow")})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	s := c.Sample(context.Background())
	if d := time.Since(start); d >= 1200*time.Millisecond {
		t.Errorf("tick took %s, want < 1.2s", d)
	}
	for _, g := range s.GPUs {
		if g.UtilPct != nil || g.TempC != nil {
			t.Errorf("timed-out GPU must be stale: %+v", g)
		}
	}
}

func TestHwmonDiscovery(t *testing.T) {
	root := filepath.Join("testdata", "root")
	p := scanHwmon(root)
	if !strings.Contains(p.cpuTemp, "hwmon2/temp1_input") {
		t.Errorf("cpu = %s, want k10temp Tctl (hwmon2), never amdgpu", p.cpuTemp)
	}
	if !strings.Contains(p.nvmeTemp, "hwmon1/temp1_input") || !strings.Contains(p.nvmeMax, "hwmon1/temp1_max") {
		t.Errorf("nvme = %s / %s, want nvme Composite (hwmon1), never mt7921_phy0", p.nvmeTemp, p.nvmeMax)
	}
	if !strings.HasSuffix(p.fans[0], "hwmon4/fan3_input") || !strings.HasSuffix(p.fans[1], "hwmon4/fan6_input") {
		t.Errorf("fans = %v, want nct6687 fan3/fan6", p.fans)
	}
	v, failed := readHwmon(p)
	if failed || *v.cpuTemp != 42.3 || *v.nvmeTemp != 35.9 || *v.nvmeMax != 83.85 || *v.rpm[0] != 977 || *v.rpm[1] != 1088 {
		t.Errorf("values %v %v %v %v %v (failed=%v)", *v.cpuTemp, *v.nvmeTemp, *v.nvmeMax, *v.rpm[0], *v.rpm[1], failed)
	}
	// No chips at all: every value null, no failure.
	none := scanHwmon(t.TempDir())
	if v, failed := readHwmon(none); failed || v.cpuTemp != nil || v.rpm[0] != nil {
		t.Error("missing chips must give null values")
	}
}

func TestFanMappingAndVerdicts(t *testing.T) {
	ip := func(v int) *int { return &v }
	ok, warn := model.BandOK, model.BandWarn
	f := fans([2]*int{ip(977), ip(0)}, &ok)
	if *f[0].MaxRPM != 2000 || *f[1].MaxRPM != 2000 || f[0].Label != "FAN BANK 1" || f[1].Bank != 2 {
		t.Errorf("mapping %+v", f)
	}
	if f[0].Verdict != "ok" || f[1].Verdict != "stalled" {
		t.Errorf("verdicts %s/%s, want ok/stalled", f[0].Verdict, f[1].Verdict)
	}
	hot := fans([2]*int{ip(1980), ip(1979)}, &warn)
	if hot[0].Verdict != "not_cooling" || hot[1].Verdict != "ok" {
		t.Errorf("at 0.99*2000 with warn: %s/%s, want not_cooling/ok", hot[0].Verdict, hot[1].Verdict)
	}
	if n := fans([2]*int{nil, nil}, &ok); n[0].MaxRPM != nil || n[0].Verdict != "unknown" {
		t.Errorf("null fan: %+v", n[0])
	}
}
