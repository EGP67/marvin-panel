# Ordered tasks
One fresh coder session per task. make verify must exit 0 and the architect must APPROVE
before committing. Do not start a task whose dependencies are uncommitted.

T0  DISCOVERY (done by hand, read-only)   bash tools/discover.sh > docs/DISCOVERY.md
    then read sections 5, 9, 11 aloud to yourself: display adapter ownership, fan RPM
    existence, port availability. Answer the BIOS IGD question before any purchase.
T1  SCAFFOLD (already complete in this repo) flags --addr --fixture --oneshot, /healthz,
    graceful shutdown, go.mod, Makefile, .golangci.yml, .gitleaks.toml, make verify green.
T2  FIXTURES   fixtures/{calm,busy,hot,dying}.json deterministic. --fixture serves them at
    1Hz round-robin; --oneshot prints one snapshot to stdout. Move HTTP plumbing to
    internal/server. Verify: go run ./cmd/marvind --oneshot --fixture fixtures/busy.json | jq .
    Status 2026-09-26: committed 39b0ea9 (owner-approved; R1-R6 addressed).
T3  MODEL      internal/model Snapshot struct, ring buffers, temp band (<60/60-90/>=90C),
    severity ramp (<40/40-70/>70), F from C helper, agreement invariant test.
    Gate addition (D-013): forbidigo in .golangci.yml bans http.Get, http.Post, http.Client,
    http.DefaultClient and net.Dial outside internal/server, so "nothing phones home" is
    enforced by make verify, not prose.
    Verify: go test -race ./internal/model
T4  COLLECT-CPU two-sample per-thread %, model string, thread+physical counts, freq, loadavg,
    120-sample ring buffer. Verify: total % within 5 points of mpstat 1 2; unit tests pass.
T5  COLLECT-MEMNET mem.go + net.go with wrap guards and link ceiling.
    Verify: transfer ~500MB, confirm RX bar and "% OF LINK" track iftop/sar.
T6  COLLECT-DISK space via Statfs, IO deltas from /proc/diskstats, IOPS, queue, iowait.
    Verify: dd a 2GB file, watch WRITE track iostat 1.
T7  COLLECT-GPU nvidia-smi once per tick, 750ms timeout, stale marking, binding by name/UUID
    per docs/DATA.md. Must never treat the iGPU as GPU0/GPU1.
    Verify: simulate a hung nvidia-smi; assert tick interval <1.2s and status STALE.
T8  COLLECT-TEMPFAN hwmon label matching with logged winner; fan banks may legitimately be
    absent -> "NO TELEMETRY". Verify: prints chosen sensor path for CPU, GPU0, GPU1, NVMe.
T9  MOOD        hysteresis 90s, line cooldown 10min, family cooldown 3min, danger repeat 60s.
    Verify: table-driven tests over scripted timelines assert the mood sequence.
T10 VIEW        web/index.html = mockup.svg + binding JS. null -> "NO TELEMETRY" text.
    Served BY marvind at GET / from 127.0.0.1:8042 (D-002, D-011) — same origin, not
    file://; a path-traversal test must prove GET /../../etc/passwd cannot escape web/.
    Kiosk engine is locked Brave Origin 154.1.96.59 (D-001) for these screenshots.
    Verify: cycle all four fixtures in a browser, screenshot each.
T11a DISPLAY    systemd units enabled, blanking disabled, kiosk autostart (X :0 on tty1,
    BusID "PCI:17:0:0", Driver "amdgpu", PrivateDevices=yes, BindPaths card0+renderD128).
    Verify: journalctl -u marvind clean; systemctl status shows no restarts; marvind never
    opens /dev/nvidia* and never appears in nvidia-smi --query-compute-apps (D-009).
T11b SOAK       24h soak with flat RSS (D-010). Verify: RSS growth over 24h within noise.
