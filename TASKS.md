# Ordered tasks
One fresh Claude Code session per task. make verify must exit 0 and the owner approves
before committing. Do not start a task whose dependencies are uncommitted.
O-tasks are owner-run (sudo or physical access); Claude Web supplies their steps.

## Done
T0  DISCOVERY   docs/DISCOVERY.md (2026-09-25). Corrections verified 2026-09-26 are
    recorded in HANDOFF.md, docs/DATA.md and docs/DECISIONS.md.
T1  SCAFFOLD    flags --addr --fixture --oneshot, /healthz, graceful shutdown, go.mod,
    Makefile, .golangci.yml, .gitleaks.toml, make verify green.
T2  FIXTURES    fixtures/{calm,busy,hot,dying}.json deterministic; --fixture serves them
    at 1 Hz round-robin; --oneshot prints one snapshot. Committed 39b0ea9.

## Owner tasks
O1  HDMI PROBE (before buying the panel; deferred, no spare monitor). Any HDMI monitor
    on the motherboard port with tools/hdmi-probe.sh. Record: cold-boot seconds to first
    image, whether BIOS/POST and the tty1 console appear on the iGPU, hotplug result.
O2  FAN EXPERIMENT (D-027). Reversible nct6683 probe with snapshot and revert steps.
    The result decides T8 fans.
O3  PANEL WAKE TEST (after purchase). PeakDo powered from the PSU 5 V rail: it must
    show the kiosk without a button press after a cold boot, a reboot, and a 10-minute
    kiosk stop.

## Build
T2b SCHEMA AMENDMENTS  Apply the docs/SCHEMA.md changes from doc step 3b (D-016 thermal
    band, D-017 panic_count, D-018 network interface, D-019 mounts, D-027 fans) in
    cmd/genfixtures and its tests; regenerate fixtures byte-deterministically.
    Also remove docs/SCHEMA.md from the .gitleaks.toml nvidia-gpu-uuid allowlist.
    Verify: go run ./cmd/marvind --oneshot --fixture fixtures/busy.json | jq .
T3  MODEL       internal/model: Snapshot struct, ring buffers, two temperature color
    functions (D-016: CPU big number 60/90, THERMALS 70/90), severity ramp
    (<40/40-70/>70), F from C helper, model-string display forms (D-020), agreement
    invariant test. Gate addition (D-013): forbidigo bans http.Get, http.Post,
    http.Client, http.DefaultClient and net.Dial outside internal/server.
    Verify: go test -race ./internal/model
T4  COLLECT-CPU two-sample per-thread %, model string, thread count from
    /sys/devices/system/cpu/online, physical cores, frequency, loadavg (mood input only;
    no longer displayed, D-017), 120-sample ring. Refuse to start above 12 threads
    (D-006). Verify: total % within 5 points of mpstat 1 2; unit tests pass.
T5  COLLECT-MEMNET mem.go + net.go on wlp14s0 (D-018) with counter-wrap guards and no
    link ceiling. Verify: scp ~500 MB between the laptop and hog over Wi-Fi; RX/TX track
    sar -n DEV 1; report peak rates so the owner can set the scale S.
T6  COLLECT-DISK Statfs for /, /srv/hogdata, /boot (D-019); IO deltas from
    /proc/diskstats (nvme0n1), IOPS, queue, iowait. Verify: dd a 2 GB file on
    /srv/hogdata and watch WRITE track iostat 1; report peak read/write for the D-021
    scale review.
T7  COLLECT-GPU one nvidia-smi per tick, 750 ms timeout, stale marking, bound by UUID,
    never by index, never the iGPU (D-008, D-024). Verify: simulate a hung nvidia-smi;
    assert tick interval <1.2 s and status STALE.
T8  COLLECT-TEMPFAN hwmon matched by name first (k10temp Tctl -> CPU, nvme Composite ->
    NVMe; amdgpu and mt7921_phy0 excluded); log the winning sensor path. Fans follow the
    O2 result; until then fans are null and render "NO TELEMETRY".
    Verify: print the chosen sensor path for CPU, GPU0, GPU1, NVMe.
T9  MOOD        hysteresis 90 s, line cooldown 10 min, family cooldown 3 min, danger
    repeat 60 s; lifetime PANIC COUNT persisted in /var/lib/heartofgold/ with atomic
    writes (D-017). Verify: table-driven tests over scripted timelines assert the mood
    sequence and the persisted count across a simulated restart.
T10 VIEW        web/index.html = mockup.svg (after doc step 4) + binding JS; null ->
    "NO TELEMETRY" text. Served BY marvind at GET / from 127.0.0.1:8042 (D-002, D-011)
    with the path-traversal test. Verify: cycle all four fixtures in brave-origin
    (D-014) using a throwaway --user-data-dir under /tmp, screenshot each; the owner
    reviews label fit for NETWORK — WIFI (D-018) and the SPACE rows (D-019).
T10b DEPLOY     (D-031) Makefile targets deploy, restart, install-service-prereqs,
    install-service. Rewrite deploy/: marvind.service (heartofgold, D-004 Description,
    device policy per D-024, StateDirectory=heartofgold) and heartofgold-kiosk.service
    (D-014, D-023, D-025, D-026); SMART units from deploy/ (doc step 5).
    Verify: systemd-analyze verify on each unit; the owner runs the install targets.
T11a DISPLAY    (panel installed) kiosk enabled, screen blanking and DPMS off, rotation
    set, O3 passes. Verify: journalctl -u marvind clean; no restarts; D-024 acceptance.
T11b SOAK       24 h soak with flat RSS (D-010). Verify: RSS growth within noise.
