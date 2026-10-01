package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
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
		"thermal_band", "power_w", "power_limit_w", "hist_util_pct", "util_sev",
	}
	wantNetKeys    = []string{"if", "rx_bps", "tx_bps", "rx_err", "tx_err"}
	wantDiskIOKeys = []string{"device", "read_bps", "write_bps"}
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

// optNum decodes a number|null field; ok is false for null.
func optNum(m map[string]any, key string) (float64, bool) {
	v, ok := m[key].(float64)
	return v, ok
}

// optF decodes a number|null field as a pointer.
func optF(m map[string]any, key string) *float64 {
	if v, ok := m[key].(float64); ok {
		return &v
	}
	return nil
}

// lift applies a band function to a nullable input (null in -> null out).
func lift(f func(float64) model.Band, v *float64) any {
	if v == nil {
		return nil
	}
	return string(f(*v))
}

// typedAll strictly decodes every rendered fixture into model.Snapshot.
func typedAll(t *testing.T) map[string]model.Snapshot {
	t.Helper()
	out := make(map[string]model.Snapshot)
	for _, s := range scenarios {
		var snap model.Snapshot
		dec := json.NewDecoder(strings.NewReader(string(render(t, s))))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&snap); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		out[s.name] = snap
	}
	return out
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

func TestInvariants(t *testing.T) {
	docs := decodeAll(t)
	if len(docs) != 5 {
		t.Fatalf("%d scenarios, want 5", len(docs))
	}
	for name, doc := range docs {
		if doc["schema"] != model.SchemaVersion {
			t.Errorf("%s: schema = %v", name, doc["schema"])
		}
		checkKeys(t, name, "top-level", doc, wantTopKeys)

		cpu := sub(t, doc, "cpu")
		cpuTemp := optF(cpu, "temp_c")
		if want := lift(model.CPUBand, cpuTemp); cpu["band"] != want {
			t.Errorf("%s: cpu.band %v, want %v for temp %v", name, cpu["band"], want, cpu["temp_c"])
		}
		if want := lift(model.ThermalBand, cpuTemp); cpu["thermal_band"] != want {
			t.Errorf("%s: cpu.thermal_band %v, want %v for temp %v", name, cpu["thermal_band"], want, cpu["temp_c"])
		}
		if cpu["model_display"] != "AMD RYZEN 5 9600X" {
			t.Errorf("%s: cpu.model_display = %v", name, cpu["model_display"])
		}
		th := int(num(t, cpu, "threads"))
		ptp, pts := arr[any](t, cpu, "per_thread_pct"), arr[any](t, cpu, "per_thread_sev")
		if len(ptp) != th || len(pts) != th {
			t.Errorf("%s: per_thread_pct/sev lengths %d/%d, threads %d", name, len(ptp), len(pts), th)
		}
		for i := range ptp {
			var v *float64
			if f, ok := ptp[i].(float64); ok {
				v = &f
			}
			if want := lift(model.Severity, v); i < len(pts) && pts[i] != want {
				t.Errorf("%s: per_thread_sev[%d] = %v, want %v for %v", name, i, pts[i], want, ptp[i])
			}
		}
		if h := arr[any](t, cpu, "hist_pct"); len(h) != cpuHistLen {
			t.Errorf("%s: hist_pct length = %d, want %d", name, len(h), cpuHistLen)
		}

		gpus := arr[any](t, doc, "gpus")
		if len(gpus) != 2 {
			t.Errorf("%s: %d gpus, want 2", name, len(gpus))
		}
		states := make([]model.GPUState, 0, len(gpus))
		for _, e := range gpus {
			g := e.(map[string]any)
			checkKeys(t, name, "gpus[]", g, wantGPUKeys)
			if want := lift(model.ThermalBand, optF(g, "temp_c")); g["thermal_band"] != want {
				t.Errorf("%s: gpu thermal_band %v, want %v", name, g["thermal_band"], want)
			}
			if want := lift(model.Severity, optF(g, "util_pct")); g["util_sev"] != want {
				t.Errorf("%s: gpu util_sev %v, want %v", name, g["util_sev"], want)
			}
			if g["display_name"] != "RTX 3060" {
				t.Errorf("%s: gpu display_name = %v", name, g["display_name"])
			}
			if u := g["uuid"]; u != gpu0UUID && u != gpu1UUID {
				t.Errorf("%s: unexpected gpu uuid", name)
			}
			if h := arr[any](t, g, "hist_util_pct"); len(h) != gpuHistLen {
				t.Errorf("%s: hist_util_pct length = %d, want %d", name, len(h), gpuHistLen)
			}
			states = append(states, model.GPUState{
				TempC: optF(g, "temp_c"), UtilPct: optF(g, "util_pct"), MemUsedMiB: optInt(g, "mem_used_mib"),
			})
		}

		gl, _ := doc["gpu_line"].(string)
		if n := utf8.RuneCountInString(gl); n < 1 || n > 38 {
			t.Errorf("%s: gpu_line %q is %d runes, want 1-38", name, gl, n)
		}
		if want := model.GPULine(states); gl != want {
			t.Errorf("%s: gpu_line = %q, want %q", name, gl, want)
		}

		temps := sub(t, doc, "temps")
		if want := lift(model.ThermalBand, optF(temps, "nvme_c")); temps["nvme_thermal_band"] != want {
			t.Errorf("%s: nvme_thermal_band %v, want %v", name, temps["nvme_thermal_band"], want)
		}
		if mx, ok := optNum(temps, "nvme_max_c"); ok && mx != 83.85 {
			t.Errorf("%s: nvme_max_c = %v, want 83.85", name, mx)
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
			used, uok := optNum(m, "used_bytes")
			free, fok := optNum(m, "free_bytes")
			total, tok := optNum(m, "total_bytes")
			if uok && fok && tok && used+free != total {
				t.Errorf("%s: %v used+free != total", name, m["mount"])
			}
			if want := lift(model.MountState, optF(m, "used_pct")); m["state"] != want {
				t.Errorf("%s: %v state %v != %v at %v%%", name, m["mount"], m["state"], want, m["used_pct"])
			}
		}

		mem := sub(t, doc, "memory")
		used, uok := optNum(mem, "used_bytes")
		cache, cok := optNum(mem, "cache_bytes")
		total, tok := optNum(mem, "total_bytes")
		if uok && cok && tok && used+cache > total {
			t.Errorf("%s: memory used + cache > total", name)
		}

		fans := arr[any](t, doc, "fans")
		if len(fans) != 2 {
			t.Errorf("%s: %d fans, want 2", name, len(fans))
		}
		var cpuThermal *model.Band
		if b, ok := cpu["thermal_band"].(string); ok {
			cpuThermal = ptr(model.Band(b))
		}
		for _, e := range fans {
			f := e.(map[string]any)
			rpm, maxRPM := optInt(f, "rpm"), optInt(f, "max_rpm")
			if want := model.FanVerdict(rpm, maxRPM, cpuThermal); f["verdict"] != want {
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
		runes := 0
		for _, l := range lines {
			s, _ := l.(string)
			n := utf8.RuneCountInString(s)
			runes += n
			if n > 52 {
				t.Errorf("%s: phrase line %q is %d runes, want <= 52", name, s, n)
			}
		}
		if runes > 100 {
			t.Errorf("%s: phrase total %d runes, want <= 100", name, runes)
		}

		if pc, ok := optNum(doc, "panic_count"); ok && pc < 0 {
			t.Errorf("%s: panic_count = %v, want null or >= 0", name, pc)
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

func TestAgreement(t *testing.T) {
	for name, snap := range typedAll(t) {
		if err := model.CheckAgreement(&snap); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestStartup checks the first-tick fixture against D-050 (4d).
func TestStartup(t *testing.T) {
	s := typedAll(t)["startup"]
	isNil := func(what string, null bool) {
		t.Helper()
		if !null {
			t.Errorf("startup: %s must be null", what)
		}
	}
	notNil := func(what string, null bool) {
		t.Helper()
		if null {
			t.Errorf("startup: %s must be non-null", what)
		}
	}
	c := s.CPU
	isNil("cpu.total_pct", c.TotalPct == nil)
	isNil("cpu.iowait_pct", c.IowaitPct == nil)
	isNil("cpu.temp_c", c.TempC == nil)
	isNil("cpu.band", c.Band == nil)
	isNil("cpu.thermal_band", c.ThermalBand == nil)
	for i := range c.PerThreadPct {
		isNil("cpu.per_thread_pct[]", c.PerThreadPct[i] == nil)
		isNil("cpu.per_thread_sev[]", c.PerThreadSev[i] == nil)
	}
	for _, v := range c.HistPct {
		isNil("cpu.hist_pct[]", v == nil)
	}
	for _, g := range s.GPUs {
		isNil("gpus[].mem_total_mib", g.MemTotalMiB == nil)
		isNil("gpus[].mem_used_mib", g.MemUsedMiB == nil)
		isNil("gpus[].util_pct", g.UtilPct == nil)
		isNil("gpus[].util_sev", g.UtilSev == nil)
		isNil("gpus[].temp_c", g.TempC == nil)
		isNil("gpus[].thermal_band", g.ThermalBand == nil)
		isNil("gpus[].power_w", g.PowerW == nil)
		isNil("gpus[].power_limit_w", g.PowerLimitW == nil)
		for _, v := range g.HistUtilPct {
			isNil("gpus[].hist_util_pct[]", v == nil)
		}
	}
	isNil("temps.nvme_c", s.Temps.NvmeC == nil)
	isNil("temps.nvme_thermal_band", s.Temps.NvmeThermalBand == nil)
	isNil("temps.nvme_sensor", s.Temps.NvmeSensor == nil)
	isNil("temps.nvme_max_c", s.Temps.NvmeMaxC == nil)
	n := s.Network[0]
	isNil("network.rx_bps", n.RxBps == nil)
	isNil("network.tx_bps", n.TxBps == nil)
	d := s.DiskIO
	isNil("disk_io.read_bps", d.ReadBps == nil)
	isNil("disk_io.write_bps", d.WriteBps == nil)
	isNil("smart.percentage_used", s.Smart.PercentageUsed == nil)
	isNil("smart.unsafe_shutdowns", s.Smart.UnsafeShutdowns == nil)
	isNil("smart.age_seconds", s.Smart.AgeSeconds == nil)
	isNil("panic_count", s.PanicCount == nil)
	if s.Smart.State != "unknown" {
		t.Errorf("startup: smart.state = %q, want unknown", s.Smart.State)
	}

	notNil("host.uptime_seconds", s.Host.UptimeSeconds == nil)
	if s.Host.UptimeSeconds != nil && *s.Host.UptimeSeconds != 41 {
		t.Errorf("startup: uptime_seconds = %d, want 41", *s.Host.UptimeSeconds)
	}
	notNil("cpu.freq_ghz", c.FreqGHz == nil)
	notNil("cpu.load1", c.Load1 == nil)
	notNil("cpu.load5", c.Load5 == nil)
	notNil("cpu.load15", c.Load15 == nil)
	m := s.Memory
	notNil("memory.total_bytes", m.TotalBytes == nil)
	notNil("memory.used_bytes", m.UsedBytes == nil)
	notNil("memory.cache_bytes", m.CacheBytes == nil)
	notNil("memory.used_pct", m.UsedPct == nil)
	notNil("memory.swap_total_bytes", m.SwapTotalBytes == nil)
	notNil("memory.swap_used_bytes", m.SwapUsedBytes == nil)
	for _, mt := range s.Storage {
		notNil("storage[].total_bytes", mt.TotalBytes == nil)
		notNil("storage[].used_bytes", mt.UsedBytes == nil)
		notNil("storage[].free_bytes", mt.FreeBytes == nil)
		notNil("storage[].used_pct", mt.UsedPct == nil)
		notNil("storage[].state", mt.State == nil)
	}
	notNil("connections.established", s.Connections.Established == nil)
	notNil("network.rx_err", n.RxErr == nil)
	notNil("network.tx_err", n.TxErr == nil)

	calm := typedAll(t)["calm"]
	if !reflect.DeepEqual(s.Memory, calm.Memory) || !reflect.DeepEqual(s.Storage, calm.Storage) {
		t.Error("startup: memory and storage must equal calm's")
	}
	if s.GPULine != "THE BRAINS ARE NOT ANSWERING." {
		t.Errorf("startup: gpu_line = %q", s.GPULine)
	}
	if got := strings.Join(s.Phrase.Lines, " "); got != "FAN BANK 1: NO TELEMETRY. I'M COOLING BY FORCE OF WILL." {
		t.Errorf("startup: phrase = %q", got)
	}
}

func TestMoodMatrix(t *testing.T) {
	docs := decodeAll(t)
	moods := map[string]string{"calm": "bored", "busy": "melancholic", "hot": "aggrieved", "dying": "doomed", "startup": "content"}
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
