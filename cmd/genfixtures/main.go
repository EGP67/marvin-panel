// Command genfixtures writes the four deterministic fixture snapshots
// (calm, busy, hot, dying) that drive panel development before T4 collectors exist.
// Values mirror docs/DISCOVERY.md hardware facts and the D-037 totals; every derived
// field (band, thermal_band, state, verdict, used_pct, display names, phrase and GPU
// lines) is computed here, never hand-written. The wire contract is docs/SCHEMA.md.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

const (
	schema     = "marvin/v1"
	cpuModel   = "AMD Ryzen 5 9600X 6-Core Processor"
	threads    = 12
	physCores  = 6
	hostName   = "hog"
	kernel     = "6.8.0-142-generic"
	gpuName    = "NVIDIA GeForce RTX 3060"
	gpu0UUID   = "GPU-23f0baf7-c957-0754-f922-d20a294ec2a7"
	gpu1UUID   = "GPU-b4124ee5-ff18-3b39-facd-544010000b6a"
	gpuMemMiB  = 12288
	gpuLimitW  = 170.0
	netIf      = "wlp14s0"
	diskDev    = "nvme0n1"
	rootDev    = "/dev/mapper/ubuntu--vg-ubuntu--lv"
	dataDev    = "/dev/mapper/ubuntu--vg-hogdata"
	bootDev    = "/dev/nvme0n1p2"
	rootFS     = "ext4"
	dataFS     = "ext4"
	bootFS     = "ext4"
	rootTotal  = 211527139328  // 197 GiB (D-037)
	dataTotal  = 1759218604442 // 1.6 TiB (D-037), rounded to whole bytes
	bootTotal  = 2147483648    // 2.0 GiB (D-037)
	bootUsed   = 58.0
	nvmeMaxC   = 83.85
	memTotal   = 132535803904
	swapTotal  = 8589934592
	cpuHistLen = 120
	gpuHistLen = 30
)

type gpuSpec struct {
	uuid       string
	utilPct    float64
	tempC      float64
	powerW     float64
	memUsedMiB int
}

type netSpec struct {
	rxBps float64
	txBps float64
}

type diskIOSpec struct {
	readB    float64
	writeB   float64
	rIOPS    int
	wIOPS    int
	inFlight int
	queueAvg float64
}

// smartSpec: nil counters mean "no telemetry" (state unknown).
type smartSpec struct {
	state           string
	percentageUsed  *int
	unsafeShutdowns *int
	ageSeconds      *int
}

type spec struct {
	name, mood, generatedAt string
	seed                    int64
	uptimeSec               int64
	load1, load5, load15    float64
	iowaitPct               float64
	cpuTotalPct             float64
	cpuFreqGHz              float64
	cpuTempC                float64
	nvmeTempC               float64
	gpus                    [2]gpuSpec
	memUsed                 int64
	memCache                int64
	swapUsed                int64
	rootUsedPct             float64
	dataUsedPct             float64
	net                     netSpec
	diskIO                  diskIOSpec
	estab                   int
	smart                   smartSpec
	panicCount              int
}

func smartOK(age int) smartSpec {
	return smartSpec{state: "ok", percentageUsed: ptr(1), unsafeShutdowns: ptr(12), ageSeconds: ptr(age)}
}

var scenarios = []spec{
	{
		name: "calm", mood: "bored", generatedAt: "2026-09-25T12:00:00Z", seed: 11,
		uptimeSec: 15480, load1: 0.12, load5: 0.21, load15: 0.35,
		iowaitPct: 0.2, cpuTotalPct: 3.4, cpuFreqGHz: 1.38, cpuTempC: 42.3, nvmeTempC: 35.9,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 0, tempC: 44, powerW: 11.8, memUsedMiB: 8099},
			{uuid: gpu1UUID, utilPct: 0, tempC: 36, powerW: 9.8, memUsedMiB: 11243},
		},
		memUsed: 9545129984, memCache: 70089203712, swapUsed: 0,
		rootUsedPct: 23, dataUsedPct: 22,
		net:    netSpec{rxBps: 2210000, txBps: 310000},
		diskIO: diskIOSpec{readB: 12288, writeB: 57344, rIOPS: 3, wIOPS: 7, inFlight: 0, queueAvg: 0.0},
		estab:  14, smart: smartOK(312), panicCount: 0,
	},
	{
		name: "busy", mood: "melancholic", generatedAt: "2026-09-25T13:00:00Z", seed: 22,
		uptimeSec: 93240, load1: 11.4, load5: 10.2, load15: 8.7,
		iowaitPct: 3.1, cpuTotalPct: 78.2, cpuFreqGHz: 5.31, cpuTempC: 67.4, nvmeTempC: 44.2,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 62, tempC: 58, powerW: 148.2, memUsedMiB: 9420},
			{uuid: gpu1UUID, utilPct: 71, tempC: 61.5, powerW: 155.9, memUsedMiB: 11800},
		},
		memUsed: 103079215104, memCache: 20971520000, swapUsed: 0,
		rootUsedPct: 24, dataUsedPct: 40,
		net:    netSpec{rxBps: 123731968, txBps: 8388608},
		diskIO: diskIOSpec{readB: 79872000, writeB: 92160000, rIOPS: 600, wIOPS: 730, inFlight: 5, queueAvg: 0.4},
		estab:  22, smart: smartOK(604), panicCount: 1,
	},
	{
		name: "hot", mood: "aggrieved", generatedAt: "2026-09-25T14:00:00Z", seed: 33,
		uptimeSec: 172980, load1: 13.8, load5: 12.9, load15: 11.4,
		iowaitPct: 18.6, cpuTotalPct: 94.1, cpuFreqGHz: 4.87, cpuTempC: 84.4, nvmeTempC: 51.8,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 88, tempC: 79.4, powerW: 166.4, memUsedMiB: 10112},
			{uuid: gpu1UUID, utilPct: 94, tempC: 82.1, powerW: 169.8, memUsedMiB: 11904},
		},
		memUsed: 118111600640, memCache: 12884901888, swapUsed: 805306368,
		rootUsedPct: 25, dataUsedPct: 82,
		net:    netSpec{rxBps: 740000, txBps: 512000},
		diskIO: diskIOSpec{readB: 270909440, writeB: 215040000, rIOPS: 1720, wIOPS: 1680, inFlight: 46, queueAvg: 3.1},
		estab:  19, smart: smartSpec{state: "unknown"}, panicCount: 1,
	},
	{
		name: "dying", mood: "doomed", generatedAt: "2026-09-25T15:00:00Z", seed: 44,
		uptimeSec: 259240, load1: 17.2, load5: 15.8, load15: 14.9,
		iowaitPct: 24.3, cpuTotalPct: 88.6, cpuFreqGHz: 5.42, cpuTempC: 93.7, nvmeTempC: 58.3,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 100, tempC: 91.3, powerW: 169.9, memUsedMiB: 12180},
			{uuid: gpu1UUID, utilPct: 99, tempC: 88.7, powerW: 168.2, memUsedMiB: 12220},
		},
		memUsed: 125304578048, memCache: 5242880000, swapUsed: 3435973837,
		rootUsedPct: 61, dataUsedPct: 97.4,
		net:    netSpec{rxBps: 640000, txBps: 481280},
		diskIO: diskIOSpec{readB: 390144000, writeB: 445440000, rIOPS: 2100, wIOPS: 2700, inFlight: 122, queueAvg: 8.7},
		estab:  41, smart: smartOK(128), panicCount: 2,
	},
}

func ptr[T any](v T) *T { return &v }

func r1(v float64) float64 { return math.Round(v*10) / 10 }

// cpuBand is the PROCESSOR big-number band (D-016, SCHEMA invariant 3).
func cpuBand(c float64) string {
	switch {
	case c >= 90:
		return "danger"
	case c >= 60:
		return "warn"
	default:
		return "ok"
	}
}

// thermalBand is the THERMALS band for every thermal reading (D-016, SCHEMA invariant 4).
func thermalBand(c float64) string {
	switch {
	case c >= 90:
		return "danger"
	case c >= 70:
		return "warn"
	default:
		return "ok"
	}
}

func mountState(usedPct float64) string {
	switch {
	case usedPct >= 90:
		return "danger"
	case usedPct >= 80:
		return "warn"
	default:
		return "ok"
	}
}

// fanVerdict implements SCHEMA invariant 6; it keys on cpu.thermal_band.
func fanVerdict(rpm, maxRPM *int, cpuThermalBand string) string {
	switch {
	case rpm == nil:
		return "unknown"
	case *rpm == 0:
		return "stalled"
	case maxRPM != nil && *maxRPM > 0 && float64(*rpm) >= 0.99*float64(*maxRPM) && cpuThermalBand != "ok":
		return "not_cooling"
	default:
		return "ok"
	}
}

func modelDisplay(raw string) string {
	return strings.ToUpper(strings.TrimSuffix(raw, " 6-Core Processor"))
}

func gpuDisplay(raw string) string {
	return strings.TrimPrefix(raw, "NVIDIA GeForce ")
}

type gpuState struct {
	tempC, utilPct *float64
	memUsedMiB     int
}

// gpuLine picks the GRAPHICS box line (D-037, docs/MARVIN.md "GPU line"); first match wins.
func gpuLine(gs []gpuState) string {
	maxTemp, sumUtil := math.Inf(-1), 0.0
	for _, g := range gs {
		if g.tempC == nil || g.utilPct == nil {
			return "THE BRAINS ARE NOT ANSWERING."
		}
		maxTemp = math.Max(maxTemp, *g.tempC)
		sumUtil += *g.utilPct
	}
	if maxTemp >= 80 {
		return fmt.Sprintf("THINKING THIS HARD RUNS AT %d DEGREES.", int(math.Round(maxTemp)))
	}
	if len(gs) > 0 && sumUtil/float64(len(gs)) >= 20 {
		return "SOMEONE ASKED IT SOMETHING. NOT ME."
	}
	for _, g := range gs {
		if g.memUsedMiB >= 1024 {
			return "MODEL LOADED. NOBODY ASKS IT ANYTHING."
		}
	}
	return "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY."
}

// phraseLines returns the scenario's seed line (docs/MARVIN.md), numbers taken from
// the fixture's own wire values.
func phraseLines(scenario string, cpuTotalPct, cpuTempC, dataUsedPct float64) []string {
	switch scenario {
	case "calm":
		return []string{fmt.Sprintf("THE CPU IS IDLE AT %d%%. I'VE NEVER ONCE BEEN IDLE.", int(math.Round(cpuTotalPct)))}
	case "busy":
		return []string{
			"THE HEART OF GOLD HAS THE WORST PROBABILITY",
			"COEFFICIENT IN THE GALAXY. MY TEMP IS SECOND.",
		}
	case "hot":
		return []string{fmt.Sprintf("%d DEGREES. I RAN COLD ONCE. NOBODY NOTICED.", int(math.Round(cpuTempC)))}
	case "dying":
		return []string{
			fmt.Sprintf("%d%% REMAINS ON /srv/hogdata. I'D TELL YOU WHAT THAT", int(math.Round(100-dataUsedPct))),
			"MEANS BUT YOU'D ONLY CLEAN SOMETHING IMPORTANT.",
		}
	}
	panic("genfixtures: no phrase for scenario " + scenario)
}

func series(rnd *rand.Rand, n int, mean, spread float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		v := mean + (rnd.Float64()-0.5)*spread
		v = math.Max(0, math.Min(100, v))
		out[i] = r1(v)
	}
	return out
}

func mountEntry(mnt, dev, fs string, total int64, usedPct float64) map[string]any {
	used := int64(math.Round(float64(total) * usedPct / 100))
	return map[string]any{
		"device": dev, "mount": mnt, "fs": fs,
		"total_bytes": total, "used_bytes": used, "free_bytes": total - used,
		"used_pct": r1(usedPct), "state": mountState(r1(usedPct)),
	}
}

func intOrNull(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func build(s spec) map[string]any {
	rnd := rand.New(rand.NewSource(s.seed))
	cpuTemp := r1(s.cpuTempC)
	cpuThermal := thermalBand(cpuTemp)
	gpus := make([]any, 0, len(s.gpus))
	states := make([]gpuState, 0, len(s.gpus))
	for _, g := range s.gpus {
		temp, util := r1(g.tempC), r1(g.utilPct)
		states = append(states, gpuState{tempC: ptr(temp), utilPct: ptr(util), memUsedMiB: g.memUsedMiB})
		gpus = append(gpus, map[string]any{
			"name": gpuName, "display_name": gpuDisplay(gpuName), "uuid": g.uuid,
			"mem_total_mib": gpuMemMiB, "mem_used_mib": g.memUsedMiB,
			"util_pct": util, "temp_c": temp, "thermal_band": thermalBand(temp),
			"power_w": r1(g.powerW), "power_limit_w": gpuLimitW,
			"hist_util_pct": series(rnd, gpuHistLen, g.utilPct, 18),
		})
	}
	// Every fan is unobserved until O2 decides (D-027).
	fans := make([]any, 0, 2)
	for i := 0; i < 2; i++ {
		fans = append(fans, map[string]any{
			"bank": i + 1, "label": fmt.Sprintf("FAN BANK %d", i+1),
			"rpm": nil, "max_rpm": nil,
			"verdict": fanVerdict(nil, nil, cpuThermal),
		})
	}
	perThread := make([]float64, threads)
	for i := range perThread {
		v := s.cpuTotalPct + (rnd.Float64()-0.5)*(s.cpuTotalPct+6)
		perThread[i] = r1(math.Max(0, math.Min(100, v)))
	}
	nvmeTemp := r1(s.nvmeTempC)
	return map[string]any{
		"schema": schema, "scenario": s.name, "mood": s.mood,
		"generated_at": s.generatedAt,
		"host":         map[string]any{"hostname": hostName, "kernel": kernel, "uptime_seconds": s.uptimeSec},
		"phrase":       map[string]any{"lines": phraseLines(s.name, r1(s.cpuTotalPct), cpuTemp, r1(s.dataUsedPct))},
		"gpu_line":     gpuLine(states),
		"cpu": map[string]any{
			"model": cpuModel, "model_display": modelDisplay(cpuModel),
			"threads": threads, "physical_cores": physCores,
			"total_pct": r1(s.cpuTotalPct), "per_thread_pct": perThread,
			"freq_ghz": r1(s.cpuFreqGHz),
			"load1":    r1(s.load1), "load5": r1(s.load5), "load15": r1(s.load15),
			"iowait_pct": r1(s.iowaitPct),
			"temp_c":     cpuTemp, "band": cpuBand(cpuTemp), "thermal_band": cpuThermal,
			"hist_pct": series(rnd, cpuHistLen, s.cpuTotalPct, s.cpuTotalPct+18),
		},
		"memory": map[string]any{
			"total_bytes": int64(memTotal), "used_bytes": s.memUsed,
			"cache_bytes": s.memCache, "used_pct": r1(float64(s.memUsed) / float64(memTotal) * 100),
			"swap_total_bytes": int64(swapTotal), "swap_used_bytes": s.swapUsed,
		},
		"gpus": gpus,
		"temps": map[string]any{
			"nvme_c": nvmeTemp, "nvme_thermal_band": thermalBand(nvmeTemp),
			"nvme_sensor": "Composite", "nvme_max_c": nvmeMaxC,
		},
		"fans": fans,
		"network": []any{map[string]any{
			"if": netIf, "rx_bps": int64(s.net.rxBps), "tx_bps": int64(s.net.txBps),
			"rx_err": 0, "tx_err": 0,
		}},
		"connections": map[string]any{"established": s.estab},
		"disk_io": map[string]any{
			"device": diskDev, "read_bps": int64(s.diskIO.readB), "write_bps": int64(s.diskIO.writeB),
			"read_iops": s.diskIO.rIOPS, "write_iops": s.diskIO.wIOPS,
			"queue_avg": r1(s.diskIO.queueAvg), "in_flight": s.diskIO.inFlight,
		},
		"storage": []any{
			mountEntry("/", rootDev, rootFS, rootTotal, s.rootUsedPct),
			mountEntry("/srv/hogdata", dataDev, dataFS, dataTotal, s.dataUsedPct),
			mountEntry("/boot", bootDev, bootFS, bootTotal, bootUsed),
		},
		"smart": map[string]any{
			"state":            s.smart.state,
			"percentage_used":  intOrNull(s.smart.percentageUsed),
			"unsafe_shutdowns": intOrNull(s.smart.unsafeShutdowns),
			"age_seconds":      intOrNull(s.smart.ageSeconds),
		},
		"panic_count": s.panicCount,
	}
}

func main() {
	out := flag.String("out", "fixtures", "directory to write fixtures into")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "genfixtures:", err)
		os.Exit(1)
	}
	for _, s := range scenarios {
		blob, err := json.MarshalIndent(build(s), "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "genfixtures:", err)
			os.Exit(1)
		}
		path := filepath.Join(*out, s.name+".json")
		if err := os.WriteFile(path, append(blob, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "genfixtures:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "genfixtures: wrote", path)
	}
}
