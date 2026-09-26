// Command genfixtures writes the four deterministic fixture snapshots
// (calm, busy, hot, dying) that drive panel development before T4 collectors exist.
// Values mirror docs/DISCOVERY.md hardware facts; every derived field (band, state,
// verdict, used_pct, F-eligibility) is computed here, never hand-written.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
)

const (
	schema     = "marvin/v1"
	cpuModel   = "AMD Ryzen 5 9600X 6-Core Processor"
	threads    = 12
	physCores  = 6
	hostName   = "hog"
	kernel     = "6.8.0-139-generic"
	gpuName    = "NVIDIA GeForce RTX 3060"
	gpu0UUID   = "GPU-23f0baf7-c957-0754-f922-d20a294ec2a7"
	gpu1UUID   = "GPU-b4124ee5-ff18-3b39-facd-544010000b6a"
	gpuMemMiB  = 12288
	gpuLimitW  = 170.0
	rootDev    = "/dev/mapper/ubuntu--vg-ubuntu--lv"
	dataDev    = "/dev/mapper/ubuntu--vg-hogdata"
	rootTotal  = 214748364800
	dataTotal  = 1782369484800
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
	smMHz      int
	memUsedMiB int
}

type ioSpec struct {
	readB    float64
	writeB   float64
	rIOPS    int
	wIOPS    int
	inFlight int
}

type netSpec struct {
	name     string
	rxBps    float64
	txBps    float64
	linkMbps *int
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
	rootIO                  ioSpec
	dataIO                  ioSpec
	nets                    [2]netSpec
	estab                   int
	fanRPM                  [2]*int
	fanMaxRPM               [2]int
}

var scenarios = []spec{
	{
		name: "calm", mood: "bored", generatedAt: "2026-09-25T12:00:00Z", seed: 11,
		uptimeSec: 15480, load1: 0.12, load5: 0.21, load15: 0.35,
		iowaitPct: 0.2, cpuTotalPct: 3.4, cpuFreqGHz: 1.38, cpuTempC: 42.3, nvmeTempC: 35.9,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 0, tempC: 44, powerW: 11.8, smMHz: 210, memUsedMiB: 8099},
			{uuid: gpu1UUID, utilPct: 0, tempC: 36, powerW: 9.8, smMHz: 210, memUsedMiB: 11243},
		},
		memUsed: 9545129984, memCache: 70089203712, swapUsed: 0,
		rootUsedPct: 23, dataUsedPct: 22,
		rootIO: ioSpec{readB: 4096, writeB: 12288, rIOPS: 1, wIOPS: 2},
		dataIO: ioSpec{readB: 8192, writeB: 45056, rIOPS: 2, wIOPS: 5},
		nets: [2]netSpec{
			{name: "enp13s0", rxBps: 15400, txBps: 9200, linkMbps: ptr(1000)},
			{name: "wlp14s0", rxBps: 2210000, txBps: 310000},
		},
		estab: 14, fanMaxRPM: [2]int{0, 0},
	},
	{
		name: "busy", mood: "melancholic", generatedAt: "2026-09-25T13:00:00Z", seed: 22,
		uptimeSec: 93240, load1: 11.4, load5: 10.2, load15: 8.7,
		iowaitPct: 3.1, cpuTotalPct: 78.2, cpuFreqGHz: 5.31, cpuTempC: 67.4, nvmeTempC: 44.2,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 62, tempC: 58, powerW: 148.2, smMHz: 1957, memUsedMiB: 9420},
			{uuid: gpu1UUID, utilPct: 71, tempC: 61.5, powerW: 155.9, smMHz: 1980, memUsedMiB: 11800},
		},
		memUsed: 103079215104, memCache: 20971520000, swapUsed: 0,
		rootUsedPct: 24, dataUsedPct: 40,
		rootIO: ioSpec{readB: 61440000, writeB: 40960000, rIOPS: 480, wIOPS: 320, inFlight: 3},
		dataIO: ioSpec{readB: 18432000, writeB: 51200000, rIOPS: 120, wIOPS: 410, inFlight: 2},
		nets: [2]netSpec{
			{name: "enp13s0", rxBps: 123731968, txBps: 8388608, linkMbps: ptr(1000)},
			{name: "wlp14s0", rxBps: 1100000, txBps: 96000},
		},
		estab: 22, fanMaxRPM: [2]int{0, 0},
	},
	{
		name: "hot", mood: "aggrieved", generatedAt: "2026-09-25T14:00:00Z", seed: 33,
		uptimeSec: 172980, load1: 13.8, load5: 12.9, load15: 11.4,
		iowaitPct: 18.6, cpuTotalPct: 94.1, cpuFreqGHz: 4.87, cpuTempC: 84.4, nvmeTempC: 51.8,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 88, tempC: 79.4, powerW: 166.4, smMHz: 2017, memUsedMiB: 10112},
			{uuid: gpu1UUID, utilPct: 94, tempC: 82.1, powerW: 169.8, smMHz: 2032, memUsedMiB: 11904},
		},
		memUsed: 118111600640, memCache: 12884901888, swapUsed: 805306368,
		rootUsedPct: 25, dataUsedPct: 82,
		rootIO: ioSpec{readB: 199229440, writeB: 122880000, rIOPS: 1100, wIOPS: 900, inFlight: 27},
		dataIO: ioSpec{readB: 71680000, writeB: 92160000, rIOPS: 620, wIOPS: 780, inFlight: 19},
		nets: [2]netSpec{
			{name: "enp13s0", rxBps: 96468992, txBps: 41943040, linkMbps: ptr(1000)},
			{name: "wlp14s0", rxBps: 740000, txBps: 512000},
		},
		estab: 19, fanMaxRPM: [2]int{0, 0},
	},
	{
		name: "dying", mood: "doomed", generatedAt: "2026-09-25T15:00:00Z", seed: 44,
		uptimeSec: 259240, load1: 17.2, load5: 15.8, load15: 14.9,
		iowaitPct: 24.3, cpuTotalPct: 88.6, cpuFreqGHz: 5.42, cpuTempC: 93.7, nvmeTempC: 58.3,
		gpus: [2]gpuSpec{
			{uuid: gpu0UUID, utilPct: 100, tempC: 91.3, powerW: 169.9, smMHz: 1500, memUsedMiB: 12180},
			{uuid: gpu1UUID, utilPct: 99, tempC: 88.7, powerW: 168.2, smMHz: 1531, memUsedMiB: 12220},
		},
		memUsed: 125304578048, memCache: 5242880000, swapUsed: 3435973837,
		rootUsedPct: 61, dataUsedPct: 97.4,
		rootIO: ioSpec{readB: 297984000, writeB: 199680000, rIOPS: 1400, wIOPS: 1200, inFlight: 64},
		dataIO: ioSpec{readB: 92160000, writeB: 245760000, rIOPS: 700, wIOPS: 1500, inFlight: 58},
		nets: [2]netSpec{
			{name: "enp13s0", rxBps: 50331648, txBps: 3145728, linkMbps: ptr(1000)},
			{name: "wlp14s0", rxBps: 640000, txBps: 481280},
		},
		estab: 41, fanRPM: [2]*int{ptr(2884), ptr(2908)}, fanMaxRPM: [2]int{2900, 2920},
	},
}

func ptr(v int) *int { return &v }

func r1(v float64) float64 { return math.Round(v*10) / 10 }

func band(t float64) string {
	switch {
	case t >= 90:
		return "danger"
	case t >= 60:
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

func fanVerdict(rpm *int, maxRPM int, cpuBand string) string {
	switch {
	case rpm == nil:
		return "unknown"
	case *rpm == 0:
		return "stalled"
	case maxRPM > 0 && float64(*rpm) >= 0.99*float64(maxRPM) && cpuBand != "ok":
		return "not_cooling"
	default:
		return "ok"
	}
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

func mountEntry(mnt, dev string, total int64, usedPct float64, io ioSpec) map[string]any {
	used := int64(math.Round(float64(total) * usedPct / 100))
	return map[string]any{
		"device": dev, "mount": mnt, "fs": "ext4",
		"total_bytes": total, "used_bytes": used, "free_bytes": total - used,
		"used_pct": r1(usedPct), "state": mountState(r1(usedPct)),
		"read_bps": int64(io.readB), "write_bps": int64(io.writeB),
		"read_iops": io.rIOPS, "write_iops": io.wIOPS, "in_flight": io.inFlight,
	}
}

func build(s spec) map[string]any {
	rnd := rand.New(rand.NewSource(s.seed))
	cpuBand := band(s.cpuTempC)
	gpus := make([]any, 0, len(s.gpus))
	for _, g := range s.gpus {
		gpus = append(gpus, map[string]any{
			"name": gpuName, "uuid": g.uuid,
			"mem_total_mib": gpuMemMiB, "mem_used_mib": g.memUsedMiB,
			"util_pct": r1(g.utilPct), "temp_c": r1(g.tempC), "band": band(g.tempC),
			"power_w": r1(g.powerW), "power_limit_w": gpuLimitW,
			"sm_clock_mhz": g.smMHz, "hist_util_pct": series(rnd, gpuHistLen, g.utilPct, 18),
		})
	}
	nets := make([]any, 0, len(s.nets))
	for _, n := range s.nets {
		linkKnown := n.linkMbps != nil
		var linkMB any
		if linkKnown {
			linkMB = *n.linkMbps
		}
		nets = append(nets, map[string]any{
			"if": n.name, "rx_bps": int64(n.rxBps), "tx_bps": int64(n.txBps),
			"link_mbps": linkMB, "link_known": linkKnown,
			"rx_err": 0, "tx_err": 0,
		})
	}
	fans := make([]any, 0, 2)
	for i := 0; i < 2; i++ {
		var rpm, maxRPM any
		if s.fanRPM[i] != nil {
			rpm = *s.fanRPM[i]
			maxRPM = s.fanMaxRPM[i]
		}
		fans = append(fans, map[string]any{
			"bank": i + 1, "label": fmt.Sprintf("FAN BANK %d", i+1),
			"rpm": rpm, "max_rpm": maxRPM,
			"verdict": fanVerdict(s.fanRPM[i], s.fanMaxRPM[i], cpuBand),
		})
	}
	perThread := make([]float64, threads)
	for i := range perThread {
		v := s.cpuTotalPct + (rnd.Float64()-0.5)*(s.cpuTotalPct+6)
		perThread[i] = r1(math.Max(0, math.Min(100, v)))
	}
	return map[string]any{
		"schema": schema, "scenario": s.name, "mood": s.mood,
		"generated_at": s.generatedAt,
		"host":         map[string]any{"hostname": hostName, "kernel": kernel, "uptime_seconds": s.uptimeSec},
		"cpu": map[string]any{
			"model": cpuModel, "threads": threads, "physical_cores": physCores,
			"total_pct": r1(s.cpuTotalPct), "per_thread_pct": perThread,
			"freq_ghz": r1(s.cpuFreqGHz),
			"load1":    r1(s.load1), "load5": r1(s.load5), "load15": r1(s.load15),
			"iowait_pct": r1(s.iowaitPct),
			"temp_c":     r1(s.cpuTempC), "band": cpuBand,
			"hist_pct": series(rnd, cpuHistLen, s.cpuTotalPct, s.cpuTotalPct+18),
		},
		"memory": map[string]any{
			"total_bytes": int64(memTotal), "used_bytes": s.memUsed,
			"cache_bytes": s.memCache, "used_pct": r1(float64(s.memUsed) / float64(memTotal) * 100),
			"swap_total_bytes": int64(swapTotal), "swap_used_bytes": s.swapUsed,
		},
		"gpus": gpus,
		"temps": map[string]any{
			"nvme_c": r1(s.nvmeTempC), "nvme_band": band(s.nvmeTempC),
			"nvme_sensor": "Composite", "nvme_max_c": 83.9,
		},
		"fans":        fans,
		"network":     nets,
		"connections": map[string]any{"established": s.estab},
		"storage":     []any{mountEntry("/", rootDev, rootTotal, s.rootUsedPct, s.rootIO), mountEntry("/srv/hogdata", dataDev, dataTotal, s.dataUsedPct, s.dataIO)},
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
