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
T2b SCHEMA AMENDMENTS  genfixtures and tests per D-016..D-019, D-027, D-034, D-035, D-037;
    fixtures regenerated (155dabb), live statfs totals (a4fac41); gitleaks allowlist narrowed;
    D-046/D-049 layout fixes in mockup.svg and GEOMETRY.md (docs commit).
T3  MODEL       internal/model typed snapshot, bands, severity, CToF, display forms, Ring,
    CheckAgreement; forbidigo gate (a40a1e3); D-050 nullability, severity fields, startup
    fixture (T3b commit).
T10 VIEW        embedded web/ (index.html = mockup SVG + b-* ids, app.js bindings) served
    by marvind at GET / with CSP, traversal and drift tests, --fixture-every; committed
    (T10 commit); owner label review moves to T11a on the real panel.
T10b DEPLOY     deploy/ units, Xorg config, polkit rule, kiosk session and Makefile
    deploy/restart/install targets (D-031, D-052); files and targets committed (T10b
    commit); installed under the owner exception, see report.
T11a DISPLAY    kiosk on the SunFounder, native 1024x600 rotated right (D-054), page
    fit; D-053 slice limits live; acceptance a PASS (no Xorg/brave on the RTX 3060s);
    acceptance b PASS (slice device policy: iGPU open, NVIDIA refused); unattended reboot
    PASS; idle 1.38% of one core; Brave Origin onboarding persisted in the kiosk profile
    (repeat via ssh -X as heartofgold if the profile is ever deleted). Closed 2026-10-01.
    Acceptance c: owner reviewed the native layout on the panel 2026-10-01: orientation
    correct, labels fit; title spacing rebalanced (TOP_TRIM 24); side margins accepted for
    Phase 1 (F8).
T4  COLLECT-CPU  LIVE-1, D-055.
T5  COLLECT-MEMNET  LIVE-1, D-055 (memory, Wi-Fi, connections).
T6  COLLECT-DISK  LIVE-1, D-055; SMART read lands here.

## Owner tasks
O1  HDMI PROBE — DONE for the SunFounder 7" (2026-09-29; see HANDOFF.md, D-044, D-045).
    Original procedure (before buying the panel): any HDMI monitor
    on the motherboard port with tools/hdmi-probe.sh. Record: cold-boot seconds to first
    image, whether BIOS/POST and the tty1 console appear on the iGPU, hotplug result.
O2  FAN EXPERIMENT (D-027). Reversible nct6683 probe with snapshot and revert steps.
    The result decides T8 fans.
O3  PANEL WAKE TEST (after purchase). PeakDo powered from the PSU 5 V rail: it must
    show the kiosk without a button press after a cold boot, a reboot, and a 10-minute
    kiosk stop.
    DEFERRED with the PeakDo purchase (D-044); procedure kept for a future PeakDo.

## Build
Order (owner, 2026-10-01): T10b, T11a, then T4-T9 (display first).
Next: T7 GPU + T8 temps/fans (O2 result), then T9 mood, T11b soak.
T7  COLLECT-GPU one nvidia-smi per tick, 750 ms timeout, stale marking, bound by UUID,
    never by index, never the iGPU (D-008, D-024). Verify: simulate a hung nvidia-smi;
    assert tick interval <1.2 s and status STALE.
T8  COLLECT-TEMPFAN hwmon matched by name first (k10temp Tctl -> CPU, nvme Composite ->
    NVMe; amdgpu and mt7921_phy0 excluded); log the winning sensor path. Fans follow the
    O2 result; until then fans are null and render "NO TELEMETRY".
    Verify: print the chosen sensor path for CPU, GPU0, GPU1, NVMe.
T9  MOOD        hysteresis 90 s, line cooldown 10 min, family cooldown 3 min, danger
    repeat 60 s; lifetime PANIC COUNT persisted in /var/lib/heartofgold/ with atomic
    writes (D-017); the state-driven GPU line (D-037). Verify: table-driven tests over
    scripted timelines assert the mood sequence and the persisted count across a
    simulated restart. Also: gpu_line stays <= 38 characters at any temperature,
    including 3-digit readings (the "hot" template is exactly 38 at 2 digits).
T11b SOAK       24 h soak with flat RSS (D-010). Verify: RSS growth within noise.

## Follow-ups (Phase 1, non-blocking)
F1 Brave window 601x1025 on a 600x1024 screen (1 px overhang, invisible); try 600,1024 once.
   IN PROGRESS: --window-size=600,1024 tried in LIVE-1; result in the LIVE-1 report.
F2 PEAK label inside the plot at 100% peaks: low contrast over the line; owner review.
F3 heartofgold-kiosk.service Requires=marvind.service bounces the kiosk on every marvind
   restart; proposed Wants= (the page covers short outages, MARVIN.md rule 7 exception).
   RESOLVED: Wants=marvind.service (D-055, LIVE-1); takes effect after make install-service.
F4 Push anomaly 2026-10-01: GitHub "cannot lock ref" with the ref already updated during a
   slow pre-push hook; watch.
F5 Docs lines over 90 columns (GEOMETRY, TASKS, HANDOFF): rewrap pass.
F6 Network "% OF S" unbounded (D-051 (5)); decide with T5.
F7 Kiosk slice hits MemoryHigh (reclaim of page cache, no OOM): watch memory.events high and
   CPU during T11b; raise MemoryHigh only on real cost.
F8 Phase 2: narrower side margins (owner 2026-10-01; layout change, GEOMETRY).

## Phase 2 — rotating sections (future, not v1)
Keep the HEART OF GOLD title and separator fixed; fade the body out and the next view
in, cycling: Dashboard, Marvin saying, Processor, Graphics, GPU0 & GPU1, Memory &
Network, Storage & Space, Thermals. Rotation is browser-side; the schema is unchanged,
but see the quote rule below for marvind.
- Timing: Dashboard 11 s; each other view 7 s (Marvin saying, Processor, Graphics,
  GPU0 & GPU1, Memory & Network, Storage & Space, Thermals); cycle = 60 s.
- Transitions are cross-fades: the outgoing view fades out while the incoming view fades
  in simultaneously, with no fade to black.
- Quotes: every appearance of the Marvin view shows a different quote — never the same
  quote twice in a row; exhaust the pool before any repeat (shuffle-bag).
- Readability goals (D-047 verdict): larger big numbers, fatter bars, larger section
  headers, and legible row values, footer, and Marvin lines on dedicated views.
Open design questions: Marvin lines currently come from marvind's snapshot
(phrase.lines, docs/MARVIN.md); the quote rule needs either marvind supplying a fresh
line at least once per 60 s cycle or the page choosing from a line pool, so Phase 2 is
no longer strictly browser-side — decide in Phase 2. Whether red states (THERMALS
>=90 °C, PANIC) pin the rotation to that view; seven new large-type GEOMETRY layouts;
measure fade CPU cost under software raster (D-028).

## Phase 3 — GPU-active view (future; depends on Phase 2)
- Trigger: enter when either RTX 3060 is >=10% utilization for 10 s; exit when both are
  <10% for 30 s (hysteresis prevents flapping).
- While active, the Phase 2 rotation pauses and this view is shown; on exit, cross-fade
  back to the rotation starting at the Dashboard.
- Layout: HEART OF GOLD title and separator fixed. Row 1: CPU and RAM utilization side by
  side. Row 2: Thermals as drawn on the Dashboard. Rows 3-4: one large row per GPU (GPU0,
  then GPU1), each an enlarged Dashboard GPU card: utilization %, model, VRAM,
  temperature, power, history line.
- Data: all fields already exist in the snapshot; no new collection. Browser-side,
  subject to the Phase 2 quote question.
- Open: GEOMETRY for this view; whether per-GPU power is already a per-device schema
  field (confirm against SCHEMA.md; if not, it becomes a schema change).
