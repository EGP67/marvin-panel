// Package collect reads live host telemetry from /proc, /sys, statfs and the SMART
// handoff file and assembles marvin/v1 snapshots (T4-T6, D-055). Every path is joined
// to an injectable root so tests run against captured text in testdata/.
package collect

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// Interim engine values until T7/T9 (D-055).
const (
	Scenario    = "live"
	interimMood = "content"
	gpuName     = "NVIDIA GeForce RTX 3060"
	gpuUnbound  = "unbound"
	netIf       = "wlp14s0"
	diskDev     = "nvme0n1"
	cpuHistLen  = 120
	gpuHistLen  = 30
	smartEvery  = 5 * time.Second
	smartPath   = "var/lib/marvin/smart.json"
)

// interimPhrase is the fans seed line (docs/MARVIN.md), speakable while fans are null,
// wrapped to the 52-character line budget.
var interimPhrase = []string{"FAN BANK 1: NO TELEMETRY.", "I'M COOLING BY FORCE OF WILL."}

// Options configures a Collector; zero values mean the real host.
type Options struct {
	Root   string           // filesystem root for /proc, /sys and /var paths
	Statfs StatfsFunc       // defaults to syscall.Statfs
	Now    func() time.Time // defaults to time.Now (monotonic)
	Log    *slog.Logger     // defaults to slog.Default()
}

// Collector owns the previous samples between ticks. It is not safe for concurrent
// use: one goroutine calls Sample.
type Collector struct {
	root   string
	statfs StatfsFunc
	now    func() time.Time
	log    *slog.Logger

	cpus     []int
	cores    int
	model    string
	hostname string
	kernel   string

	prevAt    time.Time
	havePrev  bool
	prevAgg   cpuTimes
	prevPer   map[int]cpuTimes
	prevNet   netCounters
	prevRead  *int64
	prevWrite *int64
	hist      *model.Ring[*float64]

	smart     *smartReading
	smartAt   time.Time
	smartRead bool

	errs map[string]string
}

// New reads the static facts (online threads, physical cores, model, host identity).
// It returns an error above MaxThreads (D-006).
func New(o Options) (*Collector, error) {
	c := &Collector{root: o.Root, statfs: o.Statfs, now: o.Now, log: o.Log, errs: map[string]string{}}
	if c.root == "" {
		c.root = "/"
	}
	if c.statfs == nil {
		c.statfs = syscall.Statfs
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.log == nil {
		c.log = slog.Default()
	}
	online, err := readTrim(c.path("sys/devices/system/cpu/online"))
	if err != nil {
		return nil, err
	}
	if c.cpus, err = parseOnline(online); err != nil {
		return nil, err
	}
	if n := len(c.cpus); n > MaxThreads {
		return nil, fmt.Errorf("%d threads online; the panel supports at most %d (D-006)", n, MaxThreads)
	}
	if c.cores, err = physicalCores(c.root, c.cpus); err != nil {
		return nil, err
	}
	info, err := os.ReadFile(c.path("proc/cpuinfo"))
	if err != nil {
		return nil, fmt.Errorf("cpuinfo: %w", err)
	}
	if c.model, err = cpuModel(bytes.NewReader(info)); err != nil {
		return nil, err
	}
	// Identity strings stay non-null (D-050); a read failure leaves them empty.
	c.hostname, err = readTrim(c.path("proc/sys/kernel/hostname"))
	c.note("hostname", err)
	c.kernel, err = readTrim(c.path("proc/sys/kernel/osrelease"))
	c.note("osrelease", err)
	c.hist = model.NewRing[*float64](cpuHistLen)
	return c, nil
}

// Threads is the online thread count.
func (c *Collector) Threads() int { return len(c.cpus) }

func (c *Collector) path(rel string) string { return filepath.Join(c.root, rel) }

// note logs a source's error once per state change (and its recovery), never per tick.
func (c *Collector) note(source string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if c.errs[source] == msg {
		return
	}
	c.errs[source] = msg
	if err != nil {
		c.log.Warn("collector source failed", "source", source, "err", msg)
	} else {
		c.log.Info("collector source recovered", "source", source)
	}
}

func (c *Collector) read(source, rel string) ([]byte, bool) {
	b, err := os.ReadFile(c.path(rel))
	c.note(source, err)
	return b, err == nil
}

// Sample reads every source once and returns the snapshot. The first call has null
// deltas (the startup shape, D-050).
func (c *Collector) Sample() model.Snapshot {
	at := c.now()
	secs := 0.0
	if c.havePrev {
		secs = at.Sub(c.prevAt).Seconds()
	}
	s := model.Snapshot{
		Schema:      model.SchemaVersion,
		Scenario:    Scenario,
		Mood:        interimMood,
		GeneratedAt: at.UTC().Format(time.RFC3339),
		Phrase:      model.Phrase{Lines: interimPhrase},
		Host:        model.Host{Hostname: c.hostname, Kernel: c.kernel},
	}
	if b, ok := c.read("uptime", "proc/uptime"); ok {
		up, err := parseUptime(string(b))
		c.note("uptime", err)
		if err == nil {
			s.Host.UptimeSeconds = &up
		}
	}

	s.CPU = c.sampleCPU()

	if b, ok := c.read("meminfo", "proc/meminfo"); ok {
		s.Memory = memory(parseMeminfo(bytes.NewReader(b)))
	}

	nc := readNet(c.root, netIf)
	n := model.Net{If: netIf, RxErr: toInt(nc.rxErr), TxErr: toInt(nc.txErr)}
	if c.havePrev {
		n.RxBps, n.TxBps = rate(c.prevNet.rxBytes, nc.rxBytes, secs), rate(c.prevNet.txBytes, nc.txBytes, secs)
	}
	s.Network = []model.Net{n}
	c.prevNet = nc

	s.Connections = model.Connections{Established: c.established()}

	s.DiskIO = model.DiskIO{Device: diskDev}
	var rd, wr *int64
	if b, ok := c.read("diskstats", "proc/diskstats"); ok {
		r, w, err := diskSectors(bytes.NewReader(b), diskDev)
		c.note("diskstats", err)
		if err == nil {
			r, w = r*512, w*512
			rd, wr = &r, &w
		}
	}
	if c.havePrev {
		s.DiskIO.ReadBps, s.DiskIO.WriteBps = rate(c.prevRead, rd, secs), rate(c.prevWrite, wr, secs)
	}
	c.prevRead, c.prevWrite = rd, wr

	s.Storage = c.space()
	s.Smart = c.sampleSmart(at)

	s.GPUs = make([]model.GPU, 2)
	states := make([]model.GPUState, 2)
	for i := range s.GPUs {
		s.GPUs[i] = model.GPU{
			Name: gpuName, DisplayName: model.GPUDisplayName(gpuName), UUID: gpuUnbound,
			HistUtilPct: make([]*float64, gpuHistLen),
		}
	}
	s.GPULine = model.GPULine(states)
	s.Fans = []model.Fan{
		{Bank: 1, Label: "FAN BANK 1", Verdict: "unknown"},
		{Bank: 2, Label: "FAN BANK 2", Verdict: "unknown"},
	}

	c.prevAt, c.havePrev = at, true
	if err := model.CheckAgreement(&s); err != nil {
		c.log.Warn("agreement invariant failed", "err", err)
	}
	return s
}

func (c *Collector) sampleCPU() model.CPU {
	cpu := model.CPU{
		Model: c.model, ModelDisplay: model.CPUModelDisplay(c.model),
		Threads: len(c.cpus), PhysicalCores: c.cores,
		PerThreadPct: make([]*float64, len(c.cpus)),
		PerThreadSev: make([]*model.Band, len(c.cpus)),
	}
	if b, ok := c.read("cpufreq", "sys/devices/system/cpu/cpufreq/policy0/scaling_cur_freq"); ok {
		g, err := freqGHz(string(b))
		c.note("cpufreq", err)
		if err == nil {
			cpu.FreqGHz = &g
		}
	}
	if b, ok := c.read("loadavg", "proc/loadavg"); ok {
		l, err := parseLoadavg(string(b))
		c.note("loadavg", err)
		if err == nil {
			cpu.Load1, cpu.Load5, cpu.Load15 = &l[0], &l[1], &l[2]
		}
	}
	var total *float64
	if b, ok := c.read("stat", "proc/stat"); ok {
		agg, per, err := parseProcStat(bytes.NewReader(b))
		c.note("stat", err)
		if err == nil {
			if c.prevPer != nil {
				total = busyPct(c.prevAgg, agg)
				cpu.IowaitPct = iowaitPct(c.prevAgg, agg)
				for i, id := range c.cpus {
					p, okp := c.prevPer[id]
					q, okq := per[id]
					if okp && okq {
						cpu.PerThreadPct[i] = busyPct(p, q)
					}
				}
			}
			c.prevAgg, c.prevPer = agg, per
		} else {
			c.prevPer = nil
		}
	}
	for i, v := range cpu.PerThreadPct {
		if v != nil {
			b := model.Severity(*v)
			cpu.PerThreadSev[i] = &b
		}
	}
	cpu.TotalPct = total
	c.hist.Push(total)
	vals := c.hist.Values()
	cpu.HistPct = append(make([]*float64, cpuHistLen-len(vals)), vals...)
	return cpu
}

func (c *Collector) established() *int {
	total := 0
	for i, rel := range []string{"proc/net/tcp", "proc/net/tcp6"} {
		b, err := os.ReadFile(c.path(rel))
		if err != nil {
			if i == 1 && os.IsNotExist(err) {
				continue // no IPv6
			}
			c.note("tcp", err)
			return nil
		}
		n, err := countEstablished(bytes.NewReader(b))
		if err != nil {
			c.note("tcp", err)
			return nil
		}
		total += n
	}
	c.note("tcp", nil)
	return &total
}

func (c *Collector) space() []model.Mount {
	mi := map[string]mountInfo{}
	if b, ok := c.read("mountinfo", "proc/self/mountinfo"); ok {
		mi = parseMountinfo(bytes.NewReader(b))
	}
	out := make([]model.Mount, 0, len(Mounts))
	for _, mp := range Mounts {
		m, err := mount(mp, mi[mp], c.statfs)
		c.note("statfs "+mp, err)
		out = append(out, m)
	}
	return out
}

// sampleSmart re-reads the handoff file at most every smartEvery.
func (c *Collector) sampleSmart(at time.Time) model.Smart {
	if !c.smartRead || at.Sub(c.smartAt) >= smartEvery {
		r, err := readSmart(c.path(smartPath))
		c.note("smart", err)
		c.smartAt, c.smartRead = at, true
		c.smart = nil
		if err == nil {
			c.smart = &r
		}
	}
	return smartAt(c.smart, at)
}
