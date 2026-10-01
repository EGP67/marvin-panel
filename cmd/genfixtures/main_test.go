package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"hog.local/marvin-panel/internal/model"
)

// Key sets per docs/SCHEMA.md.
var (
	wantTopKeys = []string{
		"schema", "scenario", "mood", "generated_at", "host", "phrase", "gpu_line", "cpu",
		"memory", "gpus", "temps", "fans", "network", "connections", "disk_io", "storage",
		"smart", "panic_count",
	}
	wantGPUKeys = []string{
		"name", "display_name", "uuid", "mem_total_mib", "mem_used_mib", "util_pct", "temp_c",
		"thermal_band", "power_w", "power_limit_w", "hist_util_pct",
	}
	wantNetKeys    = []string{"if", "rx_bps", "tx_bps", "rx_err", "tx_err"}
	wantDiskIOKeys = []string{"device", "read_bps", "write_bps", "read_iops", "write_iops", "queue_avg", "in_flight"}
	wantSmartKeys  = []string{"state", "percentage_used", "unsafe_shutdowns", "age_seconds"}
	wantMounts     = []string{"/", "/srv/hogdata", "/boot"}
)

func render(t *testing.T, s spec) []byte {
	t.Helper()
	blob, err := json.MarshalIndent(build(s), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(blob, '\n')
}

func TestGeneratedIsDeterministic(t *testing.T) {
	for _, s := range scenarios {
		if a, b := render(t, s), render(t, s); string(a) != string(b) {
			t.Errorf("%s: two builds differ", s.name)
		}
	}
}

func TestCommittedFixturesMatchGenerator(t *testing.T) {
	for _, s := range scenarios {
		want := render(t, s)
		got, err := os.ReadFile(filepath.Join("..", "..", "fixtures", s.name+".json"))
		if err != nil {
			t.Fatalf("missing fixtures/%s.json: %v (run go run ./cmd/genfixtures -out fixtures)", s.name, err)
		}
		if string(got) != string(want) {
			t.Errorf("fixtures/%s.json is stale; regenerate it", s.name)
		}
	}
}

func decodeAll(t *testing.T) map[string]map[string]any {
	t.Helper()
	out := make(map[string]map[string]any)
	for _, s := range scenarios {
		var doc map[string]any
		if err := json.Unmarshal(render(t, s), &doc); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		out[s.name] = doc
	}
	return out
}

func sub(t *testing.T, doc map[string]any, key string) map[string]any {
	t.Helper()
	m, ok := doc[key].(map[string]any)
	if !ok {
		t.Fatalf("missing object %q", key)
	}
	return m
}

func arr[T any](t *testing.T, doc map[string]any, key string) []T {
	t.Helper()
	a, ok := doc[key].([]T)
	if !ok {
		t.Fatalf("missing array %q", key)
	}
	return a
}

func num(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	v, ok := m[key].(float64)
	if !ok {
		t.Fatalf("missing number %q", key)
	}
	return v
}

// optInt decodes an int|null field.
func optInt(m map[string]any, key string) *int {
	if v, ok := m[key].(float64); ok {
		return ptr(int(v))
	}
	return nil
}

func mount(t *testing.T, doc map[string]any, mnt string) map[string]any {
	t.Helper()
	for _, e := range arr[any](t, doc, "storage") {
		if m := e.(map[string]any); m["mount"] == mnt {
			return m
		}
	}
	t.Fatalf("missing mount %q", mnt)
	return nil
}

func checkKeys(t *testing.T, name, what string, m map[string]any, want []string) {
	t.Helper()
	w := append([]string(nil), want...)
	sort.Strings(w)
	if got := mapKeys(m); strings.Join(got, ",") != strings.Join(w, ",") {
		t.Errorf("%s: %s keys = %v, want %v", name, what, got, w)
	}
}

func walkPct(t *testing.T, name string, v any, path string) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			walkPct(t, name, child, path+"/"+k)
		}
	case []any:
		for _, child := range node {
			walkPct(t, name, child, path+"[]")
		}
	case float64:
		if strings.HasSuffix(path, "_pct") || strings.HasSuffix(path, "_pct[]") {
			if node < 0 || node > 100 || math.Round(node*10)/10 != node {
				t.Errorf("%s: %s = %v is not a clamped 1-decimal percent", name, path, node)
			}
		}
	}
}

func walkKeys(v any, visit func(string)) {
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			visit(k)
			walkKeys(child, visit)
		}
	case []any:
		for _, child := range node {
			walkKeys(child, visit)
		}
	}
}

func TestFanVerdict(t *testing.T) {
	cases := []struct {
		rpm, max *int
		band     string
		want     string
	}{
		{nil, nil, "ok", "unknown"},
		{ptr(0), ptr(2900), "ok", "stalled"},
		{ptr(2900), ptr(2900), "warn", "not_cooling"},
		{ptr(2900), ptr(2900), "ok", "ok"},
		{ptr(1500), ptr(2900), "warn", "ok"},
	}
	for _, tc := range cases {
		if got := fanVerdict(tc.rpm, tc.max, tc.band); got != tc.want {
			t.Errorf("fanVerdict(%v, %v, %s) = %s, want %s", optStr(tc.rpm), optStr(tc.max), tc.band, got, tc.want)
		}
	}
}

func optStr(p *int) any {
	if p == nil {
		return "nil"
	}
	return *p
}

func TestGPULine(t *testing.T) {
	g := func(temp, util float64, mem int) gpuState {
		return gpuState{tempC: ptr(temp), utilPct: ptr(util), memUsedMiB: mem}
	}
	cases := []struct {
		name string
		gs   []gpuState
		want string
	}{
		{"stale", []gpuState{{tempC: nil, utilPct: ptr(50.0)}, g(40, 0, 0)}, "THE BRAINS ARE NOT ANSWERING."},
		{"hot", []gpuState{g(79.4, 88, 10112), g(82.1, 94, 11904)}, "THINKING THIS HARD RUNS AT 82 DEGREES."},
		{"thinking", []gpuState{g(58, 62, 9420), g(61.5, 71, 11800)}, "SOMEONE ASKED IT SOMETHING. NOT ME."},
		{"loaded", []gpuState{g(44, 0, 8099), g(36, 0, 11243)}, "MODEL LOADED. NOBODY ASKS IT ANYTHING."},
		{"empty", []gpuState{g(35, 0, 5), g(33, 1, 5)}, "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY."},
	}
	for _, tc := range cases {
		if got := gpuLine(tc.gs); got != tc.want {
			t.Errorf("%s: gpuLine = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestInvariants(t *testing.T) {
	docs := decodeAll(t)
	for name, doc := range docs {
		if doc["schema"] != model.SchemaVersion {
			t.Errorf("%s: schema = %v", name, doc["schema"])
		}
		checkKeys(t, name, "top-level", doc, wantTopKeys)

		cpu := sub(t, doc, "cpu")
		cpuTemp := num(t, cpu, "temp_c")
		if cpu["band"] != string(model.CPUBand(cpuTemp)) {
			t.Errorf("%s: cpu.band %v inconsistent with temp %v", name, cpu["band"], cpuTemp)
		}
		if cpu["thermal_band"] != string(model.ThermalBand(cpuTemp)) {
			t.Errorf("%s: cpu.thermal_band %v inconsistent with temp %v", name, cpu["thermal_band"], cpuTemp)
		}
		if cpu["model_display"] != "AMD RYZEN 5 9600X" {
			t.Errorf("%s: cpu.model_display = %v", name, cpu["model_display"])
		}
		if th := cpu["threads"].(float64); len(arr[any](t, cpu, "per_thread_pct")) != int(th) {
			t.Errorf("%s: per_thread_pct length != threads", name)
		}
		if h := arr[any](t, cpu, "hist_pct"); len(h) != 120 {
			t.Errorf("%s: hist_pct length = %d, want 120", name, len(h))
		}

		gpus := arr[any](t, doc, "gpus")
		if len(gpus) != 2 {
			t.Errorf("%s: %d gpus, want 2", name, len(gpus))
		}
		states := make([]gpuState, 0, len(gpus))
		for _, e := range gpus {
			g := e.(map[string]any)
			checkKeys(t, name, "gpus[]", g, wantGPUKeys)
			if g["thermal_band"] != string(model.ThermalBand(num(t, g, "temp_c"))) {
				t.Errorf("%s: gpu thermal_band inconsistent for %v", name, g["uuid"])
			}
			if g["display_name"] != "RTX 3060" {
				t.Errorf("%s: gpu display_name = %v", name, g["display_name"])
			}
			if u := g["uuid"]; u != gpu0UUID && u != gpu1UUID {
				t.Errorf("%s: unexpected gpu uuid", name)
			}
			states = append(states, gpuState{
				tempC: ptr(num(t, g, "temp_c")), utilPct: ptr(num(t, g, "util_pct")),
				memUsedMiB: int(num(t, g, "mem_used_mib")),
			})
		}

		gl, _ := doc["gpu_line"].(string)
		if n := utf8.RuneCountInString(gl); n < 1 || n > 38 {
			t.Errorf("%s: gpu_line %q is %d runes, want 1-38", name, gl, n)
		}
		if want := gpuLine(states); gl != want {
			t.Errorf("%s: gpu_line = %q, want %q", name, gl, want)
		}

		temps := sub(t, doc, "temps")
		if temps["nvme_thermal_band"] != string(model.ThermalBand(num(t, temps, "nvme_c"))) {
			t.Errorf("%s: nvme_thermal_band inconsistent", name)
		}
		if num(t, temps, "nvme_max_c") != 83.85 {
			t.Errorf("%s: nvme_max_c = %v, want 83.85", name, temps["nvme_max_c"])
		}

		nets := arr[any](t, doc, "network")
		if len(nets) != 1 {
			t.Fatalf("%s: %d network entries, want 1", name, len(nets))
		}
		n0 := nets[0].(map[string]any)
		checkKeys(t, name, "network[0]", n0, wantNetKeys)
		if n0["if"] != "wlp14s0" {
			t.Errorf("%s: network[0].if = %v", name, n0["if"])
		}

		dio := sub(t, doc, "disk_io")
		checkKeys(t, name, "disk_io", dio, wantDiskIOKeys)
		if dio["device"] != "nvme0n1" {
			t.Errorf("%s: disk_io.device = %v", name, dio["device"])
		}

		storage := arr[any](t, doc, "storage")
		if len(storage) != len(wantMounts) {
			t.Errorf("%s: %d storage entries, want %d", name, len(storage), len(wantMounts))
		}
		for i, e := range storage {
			m := e.(map[string]any)
			if i < len(wantMounts) && m["mount"] != wantMounts[i] {
				t.Errorf("%s: storage[%d].mount = %v, want %s", name, i, m["mount"], wantMounts[i])
			}
			used, free, total := num(t, m, "used_bytes"), num(t, m, "free_bytes"), num(t, m, "total_bytes")
			if used+free != total {
				t.Errorf("%s: %v used+free != total", name, m["mount"])
			}
			if want := string(model.MountState(num(t, m, "used_pct"))); m["state"] != want {
				t.Errorf("%s: %v state %v != %v at %v%%", name, m["mount"], m["state"], want, m["used_pct"])
			}
		}

		mem := sub(t, doc, "memory")
		if num(t, mem, "used_bytes")+num(t, mem, "cache_bytes") > num(t, mem, "total_bytes") {
			t.Errorf("%s: memory used + cache > total", name)
		}

		fans := arr[any](t, doc, "fans")
		if len(fans) != 2 {
			t.Errorf("%s: %d fans, want 2", name, len(fans))
		}
		cpuThermal, _ := cpu["thermal_band"].(string)
		for _, e := range fans {
			f := e.(map[string]any)
			rpm, maxRPM := optInt(f, "rpm"), optInt(f, "max_rpm")
			if want := fanVerdict(rpm, maxRPM, cpuThermal); f["verdict"] != want {
				t.Errorf("%s: fan %v verdict %v != %v", name, f["bank"], f["verdict"], want)
			}
			if rpm != nil || maxRPM != nil || f["verdict"] != "unknown" {
				t.Errorf("%s: fan %v must be rpm null, max_rpm null, verdict unknown (D-027)", name, f["bank"])
			}
		}

		lines := arr[any](t, sub(t, doc, "phrase"), "lines")
		if len(lines) < 1 || len(lines) > 2 {
			t.Errorf("%s: %d phrase lines, want 1-2", name, len(lines))
		}
		total := 0
		for _, l := range lines {
			s, _ := l.(string)
			n := utf8.RuneCountInString(s)
			total += n
			if n > 52 {
				t.Errorf("%s: phrase line %q is %d runes, want <= 52", name, s, n)
			}
		}
		if total > 100 {
			t.Errorf("%s: phrase total %d runes, want <= 100", name, total)
		}

		pc, ok := doc["panic_count"].(float64)
		if !ok || pc < 0 {
			t.Errorf("%s: panic_count = %v, want non-null >= 0", name, doc["panic_count"])
		}

		smart := sub(t, doc, "smart")
		checkKeys(t, name, "smart", smart, wantSmartKeys)
		switch smart["state"] {
		case "ok", "failing":
		case "unknown":
			for _, k := range []string{"percentage_used", "unsafe_shutdowns", "age_seconds"} {
				if smart[k] != nil {
					t.Errorf("%s: smart unknown but %s = %v", name, k, smart[k])
				}
			}
		default:
			t.Errorf("%s: smart.state = %v", name, smart["state"])
		}
		walkKeys(smart, func(k string) {
			if k == "serial_number" || k == "wwn" || k == "uuid" {
				t.Errorf("%s: forbidden key %q under smart", name, k)
			}
		})

		walkPct(t, name, doc, "")

		for k := range doc {
			if k == "temp_f" || k == "f" {
				t.Errorf("%s: fahrenheit field %q on wire", name, k)
			}
		}
	}
}

func TestMoodMatrix(t *testing.T) {
	docs := decodeAll(t)
	moods := map[string]string{"calm": "bored", "busy": "melancholic", "hot": "aggrieved", "dying": "doomed"}
	for name, want := range moods {
		if got := docs[name]["mood"]; got != want {
			t.Errorf("%s: mood = %v, want %v", name, got, want)
		}
	}
	if l := num(t, sub(t, docs["calm"], "cpu"), "load1"); l/threads >= 0.02 {
		t.Errorf("calm load/threads = %.3f, bored requires < 0.02", l/threads)
	}
	if l := num(t, sub(t, docs["busy"], "cpu"), "load1"); l/threads <= 0.85 {
		t.Errorf("busy load/threads = %.3f, melancholic requires > 0.85", l/threads)
	}
	hot := sub(t, docs["hot"], "cpu")
	if num(t, hot, "temp_c") <= 80 || num(t, hot, "iowait_pct") <= 15 {
		t.Error("hot must hold aggrieved triggers: temp > 80C and iowait > 15%")
	}
	dying := sub(t, docs["dying"], "cpu")
	if num(t, dying, "temp_c") < 90 {
		t.Error("dying CPU must be >= 90C")
	}
	if 100-num(t, mount(t, docs["dying"], "/srv/hogdata"), "used_pct") >= 5 {
		t.Error("dying hogdata must have < 5% free")
	}

	busy := docs["busy"]
	if rx := num(t, arr[any](t, busy, "network")[0].(map[string]any), "rx_bps"); rx < 118*1048576 {
		t.Errorf("busy wlp14s0 rx_bps = %v, want >= 118 MiB/s", rx)
	}
	if bc := sub(t, busy, "cpu"); bc["band"] != "warn" || bc["thermal_band"] != "ok" {
		t.Errorf("busy cpu band/thermal_band = %v/%v, want warn/ok", bc["band"], bc["thermal_band"])
	}
	if g1 := arr[any](t, docs["hot"], "gpus")[1].(map[string]any); num(t, g1, "temp_c") != 82.1 {
		t.Errorf("hot gpus[1].temp_c = %v, want 82.1", g1["temp_c"])
	}
	if st := mount(t, docs["hot"], "/srv/hogdata")["state"]; st != "warn" {
		t.Errorf("hot /srv/hogdata state = %v, want warn", st)
	}
	if up := num(t, mount(t, docs["dying"], "/srv/hogdata"), "used_pct"); up != 97.4 {
		t.Errorf("dying /srv/hogdata used_pct = %v, want 97.4", up)
	}
	if pc := docs["calm"]["panic_count"]; pc != 0.0 {
		t.Errorf("calm panic_count = %v, want 0", pc)
	}
	if pc, _ := docs["dying"]["panic_count"].(float64); pc <= 0 {
		t.Errorf("dying panic_count = %v, want > 0", docs["dying"]["panic_count"])
	}
	if st := sub(t, docs["hot"], "smart")["state"]; st != "unknown" {
		t.Errorf("hot smart.state = %v, want unknown", st)
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
