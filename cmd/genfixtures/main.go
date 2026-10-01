// Command genfixtures writes the five deterministic fixture snapshots
// (calm, busy, hot, dying, startup) that serve development and tests; live collectors
// exist since LIVE-1 (D-055). Values mirror docs/DISCOVERY.md hardware facts and live statfs totals; every
// derived field (band, thermal_band, severity, state, used_pct, display names) is
// computed via internal/model (bands, fan verdicts, GPU lines), never hand-written; the
// phrase is chosen here until T9 owns it. The wire contract is docs/SCHEMA.md (D-050).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"hog.local/marvin-panel/internal/model"
)

const (
	cpuModel  = "AMD Ryzen 5 9600X 6-Core Processor"
	threads   = 12
	physCores = 6
	hostName  = "hog"
	kernel    = "6.8.0-142-generic"
	gpuName   = "NVIDIA GeForce RTX 3060"
	gpu0UUID  = "GPU-23f0baf7-c957-0754-f922-d20a294ec2a7"
	gpu1UUID  = "GPU-b4124ee5-ff18-3b39-facd-544010000b6a"
	gpuMemMiB = 12288
	gpuLimitW = 170.0
	netIf     = "wlp14s0"
	diskDev   = "nvme0n1"
	rootDev   = "/dev/mapper/ubuntu--vg-ubuntu--lv"
	dataDev   = "/dev/mapper/ubuntu--vg-hogdata"
	bootDev   = "/dev/nvme0n1p2"
	rootFS    = "ext4"
	dataFS    = "ext4"
	bootFS    = "ext4"
	// Filesystem totals as statfs reports them (df -B1), hog.local 2026-10-01 (decided in D-049 doc commit).
	// Not partition sizes: /boot's partition is 2.0 GiB, its ext4 filesystem 2040373248 bytes.
	rootTotal  = 210779168768
	dataTotal  = 1753240817664
	bootTotal  = 2040373248
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
	readB  float64
	writeB float64
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
	// startup marks marvind's first tick: deltas, histories, temperatures and GPUs
	// have no reading yet and are null (D-050).
	startup              bool
	seed                 int64
	uptimeSec            int64
	load1, load5, load15 float64
	iowaitPct            float64
	cpuTotalPct          float64
	cpuFreqGHz           float64
	cpuTempC             float64
	nvmeTempC            float64
	gpus                 [2]gpuSpec
	memUsed              int64
	memCache             int64
	swapUsed             int64
	rootUsedPct          float64
	dataUsedPct          float64
	net                  netSpec
	diskIO               diskIOSpec
	estab                int
	smart                smartSpec
	panicCount           int
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
		diskIO: diskIOSpec{readB: 12288, writeB: 57344},
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
		diskIO: diskIOSpec{readB: 79872000, writeB: 92160000},
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
		diskIO: diskIOSpec{readB: 270909440, writeB: 215040000},
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
		diskIO: diskIOSpec{readB: 390144000, writeB: 445440000},
		estab:  41, smart: smartOK(128), panicCount: 2,
	},
	{
		// Instant reads as calm; everything that needs a second sample is null.
		name: "startup", mood: "content", generatedAt: "2026-09-25T11:00:00Z", seed: 55,
		startup:   true,
		uptimeSec: 41, load1: 0.12, load5: 0.21, load15: 0.35, cpuFreqGHz: 1.38,
		gpus:    [2]gpuSpec{{uuid: gpu0UUID}, {uuid: gpu1UUID}},
		memUsed: 9545129984, memCache: 70089203712, swapUsed: 0,
		rootUsedPct: 23, dataUsedPct: 22,
		estab: 14, smart: smartSpec{state: "unknown"},
	},
}

func ptr[T any](v T) *T { return &v }

func r1(v float64) float64 { return math.Round(v*10) / 10 }

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
	case "startup":
		// docs/MARVIN.md fans seed line, wrapped to the 52-character line budget.
		return []string{"FAN BANK 1: NO TELEMETRY.", "I'M COOLING BY FORCE OF WILL."}
	}
	panic("genfixtures: no phrase for scenario " + scenario)
}

func series(rnd *rand.Rand, n int, mean, spread float64) []*float64 {
	out := make([]*float64, n)
	for i := range out {
		v := mean + (rnd.Float64()-0.5)*spread
		v = math.Max(0, math.Min(100, v))
		out[i] = ptr(r1(v))
	}
	return out
}

// sev is model.Severity lifted over null.
func sev(pct *float64) *model.Band {
	if pct == nil {
		return nil
	}
	return ptr(model.Severity(*pct))
}

// recenter shifts the per-thread values so their mean equals total (D-050 agreement),
// then clamps to 0-100 and rounds to one decimal.
func recenter(vals []*float64, total float64) {
	sum := 0.0
	for _, v := range vals {
		sum += *v
	}
	shift := total - sum/float64(len(vals))
	for i, v := range vals {
		vals[i] = ptr(r1(math.Max(0, math.Min(100, *v+shift))))
	}
}

func mountEntry(mnt, dev, fs string, total int64, usedPct float64) model.Mount {
	used := int64(math.Round(float64(total) * usedPct / 100))
	return model.Mount{
		Device: dev, Mount: mnt, FS: fs,
		TotalBytes: ptr(total), UsedBytes: ptr(used), FreeBytes: ptr(total - used),
		UsedPct: ptr(r1(usedPct)), State: ptr(model.MountState(r1(usedPct))),
	}
}

// base returns the snapshot as marvind's first tick sees it: identity, constants and
// instant reads filled, every delta, history, temperature and GPU reading null.
func base(s spec) model.Snapshot {
	gpus := make([]model.GPU, 0, len(s.gpus))
	for _, g := range s.gpus {
		gpus = append(gpus, model.GPU{
			Name: gpuName, DisplayName: model.GPUDisplayName(gpuName), UUID: g.uuid,
			HistUtilPct: make([]*float64, gpuHistLen),
		})
	}
	// Every fan is unobserved until O2 decides (D-027).
	fans := make([]model.Fan, 0, 2)
	for i := 0; i < 2; i++ {
		fans = append(fans, model.Fan{
			Bank: i + 1, Label: fmt.Sprintf("FAN BANK %d", i+1),
			Verdict: model.FanVerdict(nil, nil, nil),
		})
	}
	return model.Snapshot{
		Schema: model.SchemaVersion, Scenario: s.name, Mood: s.mood,
		GeneratedAt: s.generatedAt,
		Host:        model.Host{Hostname: hostName, Kernel: kernel, UptimeSeconds: ptr(s.uptimeSec)},
		Phrase:      model.Phrase{Lines: phraseLines(s.name, r1(s.cpuTotalPct), r1(s.cpuTempC), r1(s.dataUsedPct))},
		CPU: model.CPU{
			Model: cpuModel, ModelDisplay: model.CPUModelDisplay(cpuModel),
			Threads: threads, PhysicalCores: physCores,
			PerThreadPct: make([]*float64, threads), PerThreadSev: make([]*model.Band, threads),
			FreqGHz: ptr(r1(s.cpuFreqGHz)),
			Load1:   ptr(r1(s.load1)), Load5: ptr(r1(s.load5)), Load15: ptr(r1(s.load15)),
			HistPct: make([]*float64, cpuHistLen),
		},
		Memory: model.Memory{
			TotalBytes: ptr(int64(memTotal)), UsedBytes: ptr(s.memUsed),
			CacheBytes: ptr(s.memCache), UsedPct: ptr(r1(float64(s.memUsed) / float64(memTotal) * 100)),
			SwapTotalBytes: ptr(int64(swapTotal)), SwapUsedBytes: ptr(s.swapUsed),
		},
		GPUs:        gpus,
		Fans:        fans,
		Network:     []model.Net{{If: netIf, RxErr: ptr(0), TxErr: ptr(0)}},
		Connections: model.Connections{Established: ptr(s.estab)},
		DiskIO:      model.DiskIO{Device: diskDev},
		Storage: []model.Mount{
			mountEntry("/", rootDev, rootFS, rootTotal, s.rootUsedPct),
			mountEntry("/srv/hogdata", dataDev, dataFS, dataTotal, s.dataUsedPct),
			mountEntry("/boot", bootDev, bootFS, bootTotal, bootUsed),
		},
		Smart: model.Smart{
			State:           s.smart.state,
			PercentageUsed:  s.smart.percentageUsed,
			UnsafeShutdowns: s.smart.unsafeShutdowns,
			AgeSeconds:      s.smart.ageSeconds,
		},
	}
}

func build(s spec) model.Snapshot {
	snap := base(s)
	states := make([]model.GPUState, 0, len(s.gpus))
	if s.startup {
		for _, g := range snap.GPUs {
			states = append(states, model.GPUState{TempC: g.TempC, UtilPct: g.UtilPct, MemUsedMiB: g.MemUsedMiB})
		}
		snap.GPULine = model.GPULine(states)
		return snap
	}

	rnd := rand.New(rand.NewSource(s.seed))
	for i, g := range s.gpus {
		temp, util := r1(g.tempC), r1(g.utilPct)
		states = append(states, model.GPUState{TempC: ptr(temp), UtilPct: ptr(util), MemUsedMiB: ptr(g.memUsedMiB)})
		snap.GPUs[i] = model.GPU{
			Name: gpuName, DisplayName: model.GPUDisplayName(gpuName), UUID: g.uuid,
			MemTotalMiB: ptr(gpuMemMiB), MemUsedMiB: ptr(g.memUsedMiB),
			UtilPct: ptr(util), UtilSev: sev(ptr(util)),
			TempC: ptr(temp), ThermalBand: ptr(model.ThermalBand(temp)),
			PowerW: ptr(r1(g.powerW)), PowerLimitW: ptr(gpuLimitW),
			HistUtilPct: series(rnd, gpuHistLen, g.utilPct, 18),
		}
	}
	snap.GPULine = model.GPULine(states)

	total := r1(s.cpuTotalPct)
	cpuTemp := r1(s.cpuTempC)
	cpu := &snap.CPU
	for i := range cpu.PerThreadPct {
		v := s.cpuTotalPct + (rnd.Float64()-0.5)*(s.cpuTotalPct+6)
		cpu.PerThreadPct[i] = ptr(r1(math.Max(0, math.Min(100, v))))
	}
	recenter(cpu.PerThreadPct, total)
	for i, v := range cpu.PerThreadPct {
		cpu.PerThreadSev[i] = sev(v)
	}
	cpu.HistPct = series(rnd, cpuHistLen, s.cpuTotalPct, s.cpuTotalPct+18)
	cpu.HistPct[cpuHistLen-1] = ptr(total)
	cpu.TotalPct = ptr(total)
	cpu.IowaitPct = ptr(r1(s.iowaitPct))
	cpu.TempC = ptr(cpuTemp)
	cpu.Band = ptr(model.CPUBand(cpuTemp))
	cpu.ThermalBand = ptr(model.ThermalBand(cpuTemp))
	for i := range snap.Fans {
		f := &snap.Fans[i]
		f.Verdict = model.FanVerdict(f.RPM, f.MaxRPM, cpu.ThermalBand)
	}

	nvmeTemp := r1(s.nvmeTempC)
	snap.Temps = model.Temps{
		NvmeC: ptr(nvmeTemp), NvmeThermalBand: ptr(model.ThermalBand(nvmeTemp)),
		NvmeSensor: ptr("Composite"), NvmeMaxC: ptr(nvmeMaxC),
	}
	snap.Network[0].RxBps = ptr(int64(s.net.rxBps))
	snap.Network[0].TxBps = ptr(int64(s.net.txBps))
	d := &snap.DiskIO
	d.ReadBps, d.WriteBps = ptr(int64(s.diskIO.readB)), ptr(int64(s.diskIO.writeB))
	snap.PanicCount = ptr(s.panicCount)
	if err := model.CheckAgreement(&snap); err != nil {
		panic(fmt.Sprintf("genfixtures: %s violates agreement: %v", s.name, err))
	}
	return snap
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
