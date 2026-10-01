package collect

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"hog.local/marvin-panel/internal/model"
)

func readTestdata(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func f64(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestCPUDeltaMath(t *testing.T) {
	a1, p1, err := parseProcStat(bytes.NewReader(readTestdata(t, "root/proc/stat")))
	if err != nil {
		t.Fatal(err)
	}
	a2, p2, err := parseProcStat(bytes.NewReader(readTestdata(t, "stat2")))
	if err != nil {
		t.Fatal(err)
	}
	if len(p1) != 12 || len(p2) != 12 {
		t.Fatalf("per-cpu lines %d/%d, want 12", len(p1), len(p2))
	}
	// Hand-computed: every thread +100 ticks; busy 10..60 (x2) => 420/1200 = 35.0%.
	if got := f64(busyPct(a1, a2)); got != 35.0 {
		t.Errorf("total = %v, want 35", got)
	}
	if got := f64(iowaitPct(a1, a2)); got != 5.0 {
		t.Errorf("iowait = %v, want 5", got)
	}
	for i := 0; i < 12; i++ {
		want := float64(10 * (i%6 + 1))
		if got := f64(busyPct(p1[i], p2[i])); got != want {
			t.Errorf("cpu%d = %v, want %v", i, got, want)
		}
	}
}

func TestCPUWrapGuard(t *testing.T) {
	prev := cpuTimes{total: 1000, idle: 800, iowait: 50}
	cases := map[string]cpuTimes{
		"total backwards":  {total: 900, idle: 700, iowait: 50},
		"idle backwards":   {total: 1100, idle: 700, iowait: 50},
		"iowait backwards": {total: 1100, idle: 900, iowait: 10},
		"no time":          prev,
	}
	for name, cur := range cases {
		if p := busyPct(prev, cur); p != nil {
			t.Errorf("%s: busy = %v, want null", name, *p)
		}
	}
	if p := iowaitPct(prev, cpuTimes{total: 1100, idle: 900, iowait: 10}); p != nil {
		t.Errorf("iowait wrap = %v, want null", *p)
	}
}

func TestParseOnline(t *testing.T) {
	cases := map[string]int{"0-11": 12, "0": 1, "0-3,5,7-9": 8, "0,2,4\n": 3}
	for in, want := range cases {
		got, err := parseOnline(in)
		if err != nil || len(got) != want {
			t.Errorf("parseOnline(%q) = %v, %v; want %d cpus", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "a-b", "5-3"} {
		if _, err := parseOnline(bad); err == nil {
			t.Errorf("parseOnline(%q): want error", bad)
		}
	}
}

func TestPhysicalCores(t *testing.T) {
	cpus, err := parseOnline("0-11")
	if err != nil {
		t.Fatal(err)
	}
	n, err := physicalCores(filepath.Join("testdata", "root"), cpus)
	if err != nil || n != 6 {
		t.Fatalf("physicalCores = %d, %v; want 6", n, err)
	}
}

func TestRefuseAboveTwelveThreads(t *testing.T) {
	root := copyRoot(t)
	write(t, root, "sys/devices/system/cpu/online", "0-15\n")
	_, err := New(Options{Root: root, Log: quiet()})
	if err == nil || !strings.Contains(err.Error(), "16 threads") {
		t.Fatalf("New with 16 threads: err = %v, want refusal naming 16", err)
	}
}

func TestMemoryCacheClamp(t *testing.T) {
	m := memory(parseMeminfo(bytes.NewReader(readTestdata(t, "root/proc/meminfo"))))
	if *m.UsedBytes != (129429496-120108096)*1024 || *m.UsedPct != 7.2 {
		t.Errorf("used = %d (%v%%)", *m.UsedBytes, *m.UsedPct)
	}
	if *m.CacheBytes != (60000000+8000000-500000)*1024 {
		t.Errorf("cache = %d", *m.CacheBytes)
	}
	// Cache larger than total - used clamps; Shmem larger than the rest clamps to 0.
	big := memory(map[string]int64{"MemTotal": 100, "MemAvailable": 40, "Cached": 90, "SReclaimable": 10, "Shmem": 0})
	small := memory(map[string]int64{"MemTotal": 100, "MemAvailable": 40, "Cached": 5, "SReclaimable": 0, "Shmem": 50})
	if *big.CacheBytes != 40 || *small.CacheBytes != 0 {
		t.Errorf("clamp: big %d (want 40), small %d (want 0)", *big.CacheBytes, *small.CacheBytes)
	}
	if none := memory(map[string]int64{}); none.TotalBytes != nil || none.UsedPct != nil || none.CacheBytes != nil {
		t.Error("missing meminfo must give null fields")
	}
}

func TestNetRateAndWrap(t *testing.T) {
	a, b := int64(1000), int64(3048)
	if r := rate(&a, &b, 2); r == nil || *r != 1024 {
		t.Errorf("rate = %v, want 1024", r)
	}
	if r := rate(&b, &a, 1); r != nil {
		t.Errorf("wrap rate = %d, want null", *r)
	}
	if r := rate(nil, &b, 1); r != nil {
		t.Errorf("first-sample rate = %d, want null", *r)
	}
}

func TestCountEstablished(t *testing.T) {
	n4, err := countEstablished(bytes.NewReader(readTestdata(t, "root/proc/net/tcp")))
	if err != nil || n4 != 3 {
		t.Errorf("tcp = %d, %v; want 3", n4, err)
	}
	n6, err := countEstablished(bytes.NewReader(readTestdata(t, "root/proc/net/tcp6")))
	if err != nil || n6 != 1 {
		t.Errorf("tcp6 = %d, %v; want 1", n6, err)
	}
}

func TestDiskSectors(t *testing.T) {
	r, w, err := diskSectors(bytes.NewReader(readTestdata(t, "root/proc/diskstats")), "nvme0n1")
	if err != nil || r != 3749200 || w != 242818 {
		t.Errorf("nvme0n1 = %d/%d, %v", r, w, err)
	}
	if _, _, err := diskSectors(bytes.NewReader(readTestdata(t, "root/proc/diskstats")), "sda"); err == nil {
		t.Error("missing device: want error")
	}
}

func fakeStatfs(st syscall.Statfs_t, fail map[string]bool) StatfsFunc {
	return func(path string, out *syscall.Statfs_t) error {
		if fail[path] {
			return errors.New("statfs: no such mount")
		}
		*out = st
		return nil
	}
}

func TestStatfsMath(t *testing.T) {
	st := syscall.Statfs_t{Blocks: 1000, Bfree: 300, Bavail: 250, Frsize: 4096, Bsize: 4096}
	m, err := mount("/", mountInfo{device: "/dev/x", fs: "ext4"}, fakeStatfs(st, nil))
	if err != nil {
		t.Fatal(err)
	}
	if *m.TotalBytes != 4096000 || *m.UsedBytes != 700*4096 || *m.FreeBytes != 250*4096 {
		t.Errorf("total/used/free = %d/%d/%d", *m.TotalBytes, *m.UsedBytes, *m.FreeBytes)
	}
	if *m.UsedPct != 73.7 || *m.State != model.BandOK { // 700 / 950
		t.Errorf("used_pct = %v state %v", *m.UsedPct, *m.State)
	}
	bad, err := mount("/boot", mountInfo{}, fakeStatfs(st, map[string]bool{"/boot": true}))
	if err == nil || bad.TotalBytes != nil || bad.UsedPct != nil || bad.State != nil || bad.Device != "unknown" {
		t.Errorf("failed statfs: %+v, %v", bad, err)
	}
}

func TestMountinfoLookup(t *testing.T) {
	mi := parseMountinfo(bytes.NewReader(readTestdata(t, "root/proc/self/mountinfo")))
	want := map[string]mountInfo{
		"/":            {"/dev/mapper/ubuntu--vg-ubuntu--lv", "ext4"},
		"/srv/hogdata": {"/dev/mapper/ubuntu--vg-hogdata", "ext4"},
		"/boot":        {"/dev/nvme0n1p2", "ext4"},
	}
	for mp, w := range want {
		if mi[mp] != w {
			t.Errorf("%s = %+v, want %+v", mp, mi[mp], w)
		}
	}
}

func TestSmart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "smart.json")
	now := time.Now()
	writeSmart := func(body string, age time.Duration) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-age)
		if err := os.Chtimes(path, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	get := func() model.Smart {
		r, err := readSmart(path)
		if err != nil {
			return smartAt(nil, now)
		}
		return smartAt(&r, now)
	}
	fixture := string(readTestdata(t, "root/var/lib/marvin/smart.json"))

	writeSmart(fixture, 120*time.Second)
	s := get()
	if s.State != "ok" || *s.PercentageUsed != 1 || *s.UnsafeShutdowns != 24 || *s.AgeSeconds != 120 {
		t.Errorf("ok: %+v", s)
	}
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"FAKESERIAL", "serial", "wwn", "uuid", "eui64"} {
		if strings.Contains(strings.ToLower(string(out)), strings.ToLower(leak)) {
			t.Errorf("SMART output leaks %q: %s", leak, out)
		}
	}

	writeSmart(strings.Replace(fixture, `"passed": true`, `"passed": false`, 1), 10*time.Second)
	if s := get(); s.State != "failing" || s.PercentageUsed == nil {
		t.Errorf("failing: %+v", s)
	}
	writeSmart(fixture, SmartMaxAge+time.Second)
	if s := get(); s.State != "unknown" || s.PercentageUsed != nil || s.AgeSeconds != nil || s.UnsafeShutdowns != nil {
		t.Errorf("stale: %+v", s)
	}
	writeSmart("{not json", 0)
	if s := get(); s.State != "unknown" || s.AgeSeconds != nil {
		t.Errorf("unparsable: %+v", s)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if s := get(); s.State != "unknown" || s.AgeSeconds != nil {
		t.Errorf("missing: %+v", s)
	}
}

func TestHost(t *testing.T) {
	if up, err := parseUptime("1835.94 21888.86\n"); err != nil || up != 1835 {
		t.Errorf("uptime = %d, %v", up, err)
	}
	if _, err := parseUptime(""); err == nil {
		t.Error("empty uptime: want error")
	}
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// copyRoot copies testdata/root to a temp dir so a test can change files between samples.
func copyRoot(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := filepath.Join("testdata", "root")
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestFullSnapshot drives two samples one second apart through a copied root.
func TestFullSnapshot(t *testing.T) {
	root := copyRoot(t)
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	st := syscall.Statfs_t{Blocks: 1000, Bfree: 300, Bavail: 250, Frsize: 4096}
	c, err := New(Options{Root: root, Now: func() time.Time { return clock }, Statfs: fakeStatfs(st, nil), Log: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	first := c.Sample()
	if first.CPU.TotalPct != nil || first.Network[0].RxBps != nil || first.DiskIO.ReadBps != nil || first.CPU.IowaitPct != nil {
		t.Error("first sample: deltas must be null")
	}
	for _, v := range first.CPU.HistPct {
		if v != nil {
			t.Fatal("first sample: hist_pct must be all null")
		}
	}

	write(t, root, "proc/stat", string(readTestdata(t, "stat2")))
	write(t, root, "proc/diskstats", string(readTestdata(t, "diskstats2")))
	var n2 map[string]int64
	if err := json.Unmarshal(readTestdata(t, "net2.json"), &n2); err != nil {
		t.Fatal(err)
	}
	for k, v := range n2 {
		write(t, root, "sys/class/net/wlp14s0/statistics/"+k, itoa(v)+"\n")
	}
	clock = clock.Add(time.Second)
	s := c.Sample()

	if err := model.CheckAgreement(&s); err != nil {
		t.Errorf("agreement: %v", err)
	}
	if f64(s.CPU.TotalPct) != 35.0 || *s.CPU.HistPct[119] != 35.0 || s.CPU.HistPct[117] != nil {
		t.Errorf("cpu total %v, hist tail %v/%v", f64(s.CPU.TotalPct), s.CPU.HistPct[118], s.CPU.HistPct[119])
	}
	if *s.CPU.PerThreadSev[5] != model.BandWarn || *s.CPU.PerThreadSev[0] != model.BandOK { // 60%, 10%
		t.Errorf("severity: %v %v", *s.CPU.PerThreadSev[0], *s.CPU.PerThreadSev[5])
	}
	if *s.DiskIO.ReadBps != 2048*512 || *s.DiskIO.WriteBps != 4096*512 {
		t.Errorf("disk %d/%d", *s.DiskIO.ReadBps, *s.DiskIO.WriteBps)
	}
	if *s.Network[0].RxBps != 2097152 || *s.Network[0].TxBps != 262144 || *s.Network[0].TxErr != 2 {
		t.Errorf("net %+v", s.Network[0])
	}
	if *s.Connections.Established != 4 || *s.Host.UptimeSeconds != 1835 || s.Host.Hostname != "hog" {
		t.Errorf("conn %d uptime %d host %q", *s.Connections.Established, *s.Host.UptimeSeconds, s.Host.Hostname)
	}
	if s.CPU.PhysicalCores != 6 || s.CPU.Threads != 12 || s.CPU.ModelDisplay != "AMD RYZEN 5 9600X" || *s.CPU.FreqGHz != 5.4 {
		t.Errorf("cpu identity %d/%d %q %v", s.CPU.PhysicalCores, s.CPU.Threads, s.CPU.ModelDisplay, f64(s.CPU.FreqGHz))
	}

	// D-050 / D-055 nullability and interim values.
	if s.Scenario != "live" || s.Mood != "content" || s.PanicCount != nil || s.GeneratedAt != "2026-10-01T12:00:01Z" {
		t.Errorf("top: %q %q %v %q", s.Scenario, s.Mood, s.PanicCount, s.GeneratedAt)
	}
	if strings.Join(s.Phrase.Lines, " ") != "FAN BANK 1: NO TELEMETRY. I'M COOLING BY FORCE OF WILL." {
		t.Errorf("phrase %q", s.Phrase.Lines)
	}
	if s.GPULine != "THE BRAINS ARE NOT ANSWERING." {
		t.Errorf("gpu_line %q", s.GPULine)
	}
	if s.CPU.TempC != nil || s.CPU.Band != nil || s.CPU.ThermalBand != nil || s.Temps.NvmeC != nil || s.Temps.NvmeSensor != nil {
		t.Error("temperatures must be null until T8")
	}
	for _, g := range s.GPUs {
		if g.UUID != "unbound" || g.DisplayName != "RTX 3060" || g.UtilPct != nil || g.MemTotalMiB != nil || len(g.HistUtilPct) != 30 {
			t.Errorf("gpu %+v", g)
		}
	}
	for i, f := range s.Fans {
		if f.RPM != nil || f.Verdict != "unknown" || f.Bank != i+1 {
			t.Errorf("fan %+v", f)
		}
	}
	if len(s.Storage) != 3 || s.Storage[1].Mount != "/srv/hogdata" || s.Storage[2].Device != "/dev/nvme0n1p2" {
		t.Errorf("storage %+v", s.Storage)
	}
	if s.Smart.State == "" {
		t.Error("smart.state must be set")
	}

	// Strict round trip: the live snapshot is a valid marvin/v1 document.
	blob, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(blob))
	dec.DisallowUnknownFields()
	var back model.Snapshot
	if err := dec.Decode(&back); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("FAKESERIAL")) {
		t.Error("snapshot leaks the SMART serial")
	}
}

func itoa(v int64) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
