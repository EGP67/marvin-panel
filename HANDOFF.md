# HEART OF GOLD — Marvin telemetry panel (hog.local)

## What it is
A permanent portrait 1080x1920 telemetry panel behind the glass side panel of an Ubuntu
homelab server (hog.local). Rendered as MARVIN: bleak, sardonic, self-pitying,
occasionally useful. The title is "HEART OF GOLD" (the ship); every spoken line is
Marvin's and stays attributed to him.

## Status (as of 2026-09-26)
- Panel: NOT yet purchased. Candidate: PeakDo U3 SE 7-inch (native 1920x1080 landscape,
  mounted portrait and rotated by X per D-026; mini-HDMI 1.4 input; USB-C PD power from
  the PSU through a SATA-to-USB 5 V adapter). Constraints: docs/PANEL_BUYING.md.
  Pre-purchase HDMI probe (TASKS.md O1) is deferred: no spare monitor available yet.
- Display path: motherboard HDMI = CPU integrated graphics (amdgpu, PCI 0000:11:00.0,
  boot_vga=1, /dev/dri/card0 + renderD128). The two RTX 3060s are never used by the panel.
- Code: T1 scaffold and T2 fixtures committed (39b0ea9). No collectors, model, mood
  engine or web page yet.
- Docs: owner review 2026-09-26 recorded as D-014..D-033. CLAUDE.md is the agent entry
  point (D-030). Doc sync steps 3-5 in progress.
- Gate: make verify passes under Claude Code, including govulncheck.
- Host facts verified 2026-09-26: Ubuntu 24.04.5, kernel 6.8.0-142, MSI BIOS 1.P5,
  go1.27.1, NTP synchronized, boots to multi-user.target, Xorg + amdgpu DDX + xinit
  installed, lightdm masked, no window manager.
- Deployment: nothing is installed for the panel except the SMART exporter
  (marvin-smart.service + .timer, /usr/local/sbin/marvin-smart), which exists on the host
  but is not yet in deploy/ (doc step 5). Panel services will run as heartofgold (D-023).

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
              Driver "amdgpu"; rotation declared in /etc/X11/xorg.conf.d (D-026). The
              kiosk sees only /dev/dri/card0 + renderD128 (D-023).
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
internal/model/            planned T3: Snapshot, ring buffers, color bands, severity
internal/collect/          planned T4-T8: one file per source
internal/mood/             planned T9: state machine, hysteresis, cooldowns, PANIC COUNT
fixtures/                  deterministic snapshots: calm busy hot dying
web/index.html             planned T10: SVG + value-binding JS
deploy/                    systemd units — STALE until T10b (D-031)
tools/                     discover.sh, discover-sudo.sh, hdmi-probe.sh (owner-run)

## Quality gate — MANDATORY on every commit
make verify runs:  golangci-lint run ./...
                   go test -race -count=1 ./...
                   govulncheck ./...
                   gitleaks detect --source . --redact --exit-code 1
                   gitleaks git --pre-commit --staged --redact --exit-code 1 .   (staged changes, D-039)
Stage first, then verify, then commit (CLAUDE.md rule 3). A task is not complete until make verify exits 0. govulncheck needs network; on failure
follow CLAUDE.md workflow rule 7.

## Definition of done
The panel shows mockup.svg with hog.local hardware (D-032):
title | phrase box | PROCESSOR ("AMD RYZEN 5 9600X" per D-020; total util % colored by
the CPU temperature band 60/90 per D-016; "n GHZ | TEMP n°F / m°C"; 120 s utilization
graph; 12 per-thread dot matrices) | GRAPHICS (total + GPU0/GPU1 RTX 3060, VRAM, F/C,
sparklines) | MEMORY | NETWORK — WIFI (wlp14s0, bars against a stated scale, D-018) |
STORAGE · I/O | SPACE (/, /srv/hogdata, /boot, D-019) | THERMALS (CPU, GPU0, GPU1,
NVME M.2 in F and C; amber >=70 °C, red >=90 °C per D-016; FAN BANK 1/2 per D-027) |
footer "UPTIME … · PANIC COUNT n" (D-017) and DON'T PANIC.
Runs at boot. Survives 24 h with flat RSS. Under 2% of one core idle. make verify clean.

## Escalation triggers
See CLAUDE.md.
