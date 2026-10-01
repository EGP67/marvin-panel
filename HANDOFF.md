# HEART OF GOLD — Marvin telemetry panel (hog.local)

## What it is
A permanent portrait 1080x1920 telemetry panel behind the glass side panel of an Ubuntu
homelab server (hog.local). Rendered as MARVIN: bleak, sardonic, self-pitying,
occasionally useful. The title is "HEART OF GOLD" (the ship); every spoken line is
Marvin's and stays attributed to him.

## Status (as of 2026-10-01)
- Panel: SunFounder 7" 1024x600 HDMI is the interim production display (D-044); facts
  below under "Development/interim display". PeakDo U3 SE 7-inch purchase deferred to a
  cost/benefit decision after install; it remains a drop-in (docs/PANEL_BUYING.md).
  O1 (HDMI probe): done for the SunFounder. O3 (PeakDo wake test): deferred with the
  PeakDo purchase (D-044).
- Display path: motherboard HDMI = CPU integrated graphics (amdgpu, PCI 0000:11:00.0,
  boot_vga=1). DRM card, renderD and i2c-N numbers are not stable across boots (D-041):
  the iGPU was card0 when the docs were written and card1 on 2026-09-29 boots. Always
  address it by PCI address. The two RTX 3060s (0000:01:00.0, 0000:06:00.0) are never
  used by the panel. Output names: kernel connector HDMI-A-1, X output HDMI-A-0 (D-045
  note).
- Phase 1 typography frozen (D-047); dashboard is at-a-glance.
- Code: T1 scaffold and T2 fixtures committed (39b0ea9); T2b schema amendments
  (155dabb, a4fac41, D-049 docs); T3 model (a40a1e3, D-050); T10 page (embedded web/).
  No collectors or mood engine yet.
- Docs: owner review 2026-09-26 recorded as D-014..D-033. CLAUDE.md is the agent entry
  point (D-030). Doc sync steps 3-5 done (342dc42, 9a9f058, a031e8f, f02df6c).
- Gate: make verify passes under Claude Code, including govulncheck.
- Remote: origin = git@github.com:EGP67/marvin-panel.git (private GitHub), branch
  master; full history pushed 2026-09-26 (D-040).
- Host facts verified 2026-09-26: Ubuntu 24.04.5, kernel 6.8.0-142, MSI BIOS 1.P5,
  go1.27.1, NTP synchronized, boots to multi-user.target, Xorg + amdgpu DDX + xinit
  installed, lightdm masked, no window manager.
- Deployment: units and targets exist (D-052): deploy/ holds marvind.service,
  heartofgold-kiosk.service, the Xorg config, the polkit rule and the kiosk session; the
  Makefile has deploy, restart, restart-kiosk, install-service-prereqs and
  install-service. Panel services run as heartofgold (D-023) from /opt/heartofgold. The
  SMART exporter (marvin-smart.service + .timer, /usr/local/sbin/marvin-smart) exists on
  the host; verbatim copies are in deploy/ (f02df6c).
- Development/interim display (D-044), observed 2026-09-29: EDID mfg "TXD", model
  "HDMI", EDID 1.3, native 1024x600, also accepts 1920x1080 / 1280x720 and lower.
  DDC/CI: Novatek controller, capabilities model "FALCON", MCCS 2.0 (detect reports VCP
  2.1), firmware 2.32, on "AMDGPU DM i2c hw bus 0". VCP 10 brightness: writes accepted
  and read back, no visible change observed at 10 ft. VCP D6 off/on works; the
  controller stays awake while off. The capabilities list looks like a generic template
  (input source reads VGA-1; VGA/DVI only; audio advertised). No kernel backlight
  device. USB carries power + touch only. Cold-start artifacts for ~20 min after
  power-up from cold (not seen when warm). The panel survives reboot and power-off/on
  (blanks, returns). DDC policy: D-043.
- Observation 2026-09-29..10-01: on USB power only, no cold-start artifacts on three
  consecutive mornings (previously ~20 min of artifacts on the barrel supply). Cause not
  established: either the barrel adapter caused them, or the panel no longer truly
  cold-starts because hog's USB port powers it continuously and it only sleeps when it
  has no signal.
- ddcutil 1.4.1 installed (owner, 2026-09-29). i2c-dev is built into the kernel.
- Watch item: amdgpu logs "REG_WAIT timeout 1us * 100000 tries - optc1_wait_for_state
  line:839" on each X start/stop/mode set; not seen during steady display. It becomes a
  defect if it appears during steady operation.

## Hard constraints
- Portrait 1080x1920. No keyboard, mouse or touch input, no interaction of any kind,
  ever. Always on.
- Must never endanger the host it monitors: dedicated non-privileged account heartofgold
  (D-023), CPU/RAM capped, restart always.
- Any unavailable metric renders as visible text saying so. Never a silent zero.
- The two Nvidia GPUs are reserved for inference. Rendering order (D-028): software
  raster first, iGPU acceleration as fallback, never Nvidia. marvind's only Nvidia access
  is the per-tick nvidia-smi child (D-008, D-024).
- Marvin runs on the box he reports.

## Architecture (DECIDED — register: docs/DECISIONS.md)
1. marvind    Single Go binary. Collects, computes, owns state + mood + phrase choice.
              Serves JSON on 127.0.0.1:8042 (D-002: named constant, fail loudly on bind
              failure, never auto-increment; reserved ports D-015). No outbound network
              (D-013), no auth (loopback). Never talks to marvinweb.service (D-004); unit
              marvind.service, Description "HEART OF GOLD telemetry daemon (NOT
              marvinweb)". Reads the SMART handoff /var/lib/marvin/smart.json (D-012).
              Its only write path is /var/lib/heartofgold/ for the lifetime PANIC COUNT
              (D-017).
2. front end  One static HTML page embedding the SVG from mockup.svg, served BY marvind
              from the same loopback origin at GET / (D-011), never file://. Polls
              /snapshot.json at 1 Hz and mutates SVG attributes/text in place. No
              framework, no bundler, no build step for web/. Path-traversal test
              required (GET /../../etc/passwd must not escape web/).
3. display    Brave Origin (package brave-origin, /usr/bin/brave-origin, D-014) in
              --kiosk mode, started by heartofgold-kiosk.service via xinit on display :0,
              vt1, tty1, as heartofgold (D-025). Screen pinned to BusID "PCI:17:0:0",
              Driver "amdgpu", AutoAddGPU off (D-041); input ignored (D-042); mode
              and rotation declared in /etc/X11/xorg.conf.d (D-026, D-045). The
              kiosk sees only the iGPU's card + render nodes (D-023, D-041).
4. rationale  Mutating attributes on an existing SVG is nearly free; regenerating whole
              frames is not. Collection and presentation stay independently testable,
              which is what makes fixture-driven development and CI checks possible.

## Repository layout
CLAUDE.md                  agent entry point — read first
README.md                  human quick start
HANDOFF.md                 this file
TASKS.md                   ordered task list with acceptance criteria
mockup.svg                 NORMATIVE visual reference (edits only per owner decision)
docs/DECISIONS.md          decision register; highest authority
docs/SCHEMA.md             snapshot wire contract "marvin/v1"
docs/GEOMETRY.md           coordinate + color contract
docs/DATA.md               metric -> source map and handling rules
docs/MARVIN.md             voice rules, mood triggers, seed lines
docs/PANEL_BUYING.md       display purchase + install constraints
docs/DISCOVERY.md          read-only system survey (2026-09-25)
cmd/marvind/               daemon (T1 flags/healthz/shutdown; T2 fixture serve, --oneshot)
cmd/genfixtures/           sole author of fixtures/*.json (seeded, byte-deterministic)
internal/server/           HTTP handlers, fixture replay
internal/model/            Snapshot types, bands, severity, Ring, agreement (T3)
internal/collect/          planned T4-T8: one file per source
internal/mood/             planned T9: state machine, hysteresis, cooldowns, PANIC COUNT
fixtures/                  deterministic snapshots: calm busy hot dying startup
web/                       index.html (mockup SVG + b-* ids) + app.js, embedded (T10)
deploy/                    units, Xorg config, polkit rule, kiosk session (D-052)
tools/                     discover.sh, discover-sudo.sh, hdmi-probe.sh (owner-run)

## Quality gate — MANDATORY on every commit
make verify runs:  golangci-lint run ./...
                   go test -race -count=1 ./...
                   govulncheck ./...
                   gitleaks detect --source . --redact --exit-code 1
                   gitleaks git --pre-commit --staged --redact --exit-code 1 .   (staged changes, D-039)
Stage first, then verify, then commit (CLAUDE.md rule 3). A task is not complete until
make verify exits 0. govulncheck needs network; on failure follow CLAUDE.md workflow
rule 7.
Clone setup: git config core.hooksPath .githooks (D-048); git config user.email
<GitHub noreply address> in this repo only — GitHub email privacy rejects pushes that
expose the private address (GH007).

## Definition of done
The panel shows mockup.svg with hog.local hardware (D-032):
title | phrase box | PROCESSOR ("AMD RYZEN 5 9600X" per D-020; total util % colored by
the CPU temperature band 60/90 per D-016; "n GHZ | TEMP n°F / m°C"; 120 s utilization
graph; 12 per-thread dot matrices) | GRAPHICS (total + GPU0/GPU1 RTX 3060, VRAM, F/C,
sparklines) | MEMORY | NETWORK (header WIFI, CON/ERR counters; wlp14s0, bars against a
stated scale; D-018, D-046, D-049) | STORAGE · I/O | SPACE (/, /srv/hogdata, /boot as
percent bars; D-019, D-049) | THERMALS (CPU, GPU0, GPU1,
NVME M.2 in F and C; amber >=70 °C, red >=90 °C per D-016; FAN BANK 1/2 per D-027) |
footer "UPTIME … · PANIC COUNT n" (D-017) and DON'T PANIC.
Runs at boot. Survives 24 h with flat RSS. Under 2% of one core idle. make verify clean.

## Escalation triggers
See CLAUDE.md.
