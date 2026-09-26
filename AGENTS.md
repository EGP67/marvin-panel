# AGENTS.md — how to work on marvin-panel

HEART OF GOLD is a permanent portrait 1080x1920 telemetry panel behind the glass side
panel of an Ubuntu homelab server (hog.local), rendering host metrics in Marvin's bleak,
sardonic voice. A single dependency-free Go binary, marvind, collects telemetry and
serves snapshot.json plus the static front end on 127.0.0.1:8042; a locked Brave kiosk
displays it. Development is fixture-driven: deterministic snapshots drive the panel until
T4-T8 collectors replace them.

## Reading order (stop at the first file your task does not touch)
1. AGENTS.md (this file)
2. HANDOFF.md — current truth: status, architecture, hard constraints
3. docs/DECISIONS.md — closed arguments; never re-litigate an ADOPTED row in code
4. TASKS.md — what to build, in order, with acceptance criteria
5. docs/SCHEMA.md, docs/GEOMETRY.md, docs/DATA.md, docs/MARVIN.md — only when the task
   touches the wire contract, pixels, sources, or voice respectively

## The files
| path | what it is | status |
|---|---|---|
| AGENTS.md | this map | NORMATIVE process |
| HANDOFF.md | project truth: status, architecture, constraints | NORMATIVE |
| TASKS.md | ordered tasks + acceptance criteria | NORMATIVE |
| docs/DECISIONS.md | decision register (D-001..D-013, P-001..P-003) | NORMATIVE; ADOPTED closed, PENDING OWNER = do not act |
| docs/SCHEMA.md | snapshot.json wire contract "marvin/v1" | NORMATIVE (T2; R4 unconfirmed by architect) |
| docs/GEOMETRY.md | coordinates, fonts, colour rules, budgets | NORMATIVE |
| docs/DATA.md | metric -> source map + handling rules | NORMATIVE |
| docs/MARVIN.md | voice, mood triggers, seed lines + speakability | NORMATIVE for voice only (D-004) |
| docs/PANEL_BUYING.md | purchase + install constraints | reference |
| docs/DISCOVERY.md | 632-line read-only system survey | REFERENCE; read only the section you need; do not re-run it |
| mockup.svg | the panel as drawn | **NORMATIVE GEOMETRY-ONLY AND DO NOT EDIT** — its data strings (RTX 4090, A5000, 48.0 GiB, 410 W, eth0, /tank, 47% OF LINK) are placeholders and must never be copied into fixtures; changes need owner approval (P-001..P-003) |
| Makefile | verify pipeline | NORMATIVE gate |
| go.mod | module hog.local/marvin-panel, go 1.23 | NORMATIVE: zero dependencies, keep it |
| .golangci.yml | lint config (forbidigo arrives at T3) | NORMATIVE gate |
| .gitleaks.toml | secret rules; real GPU UUIDs are allowlisted ONLY in cmd/genfixtures/main.go and fixtures/*.json | NORMATIVE gate |
| cmd/marvind/ | daemon: fixture serve, --oneshot, healthz, shutdown | code |
| cmd/genfixtures/ | sole author of fixtures/*.json; real hardware values come from DISCOVERY.md and live here, nowhere else | code |
| internal/server/ | verbatim fixture replay + round-robin | code |
| internal/collect/, internal/model/, internal/mood/ | T3-T9, do not create early | planned |
| fixtures/ | calm busy hot dying; regenerate, never hand-edit | generated |
| web/ | index.html (empty until T10; served at GET /, D-011) | code |
| deploy/ | systemd units (marvind Description per D-004) | code |
| prompts/ | coder + architect system prompts | reference |
| tools/ | discover.sh, discover-sudo.sh, hdmi-probe.sh | tooling |

## Hard constraints
- Portrait 1080x1920; no keyboard/mouse/interaction, ever. Always on.
- Never endanger the host: dedicated non-privileged account (NOT uid 1000), CPU/RAM
  capped, restart always.
- Unavailable metric renders as visible text ("NO TELEMETRY"), never a silent zero;
  null is never replaced by 0.
- No Nvidia GPU work for the panel: no /dev/nvidia*, never in nvidia-smi
  --query-compute-apps (D-009); kiosk sees only card0 + renderD128.
- Loopback only, port 8042 (D-002); never touch reserved ports (D-003); nothing phones
  home (D-013); ALL logging to STDERR.
- go.mod stays dependency-free; stdlib only.
- Never write the drive serial number into any file in this repo.
- NEVER `git add -A` or `git commit -a`; stage only the files your task names.

## Quality gate — mandatory, no exceptions
`make verify` = golangci-lint + `go test -race` + govulncheck + gitleaks.
govulncheck needs network: the coder sandbox sets HTTPS_PROXY=http://127.0.0.1:8889
which nothing listens on, so a proxy-refused govulncheck failure is the sandbox, not the
code — say so in those words; the owner's own `make verify` terminal output is the one
that counts. Never skip silently.

## Definition of done
Panel shows (HANDOFF.md); runs at boot; 24h flat RSS; <2% of one core idle; every task's
Verify command actually run and its output pasted; make verify clean; architect APPROVE
before commit.

## Escalation triggers — stop and ask rather than attempt a 4th time
1. Same task fails 3 times. 2. Multi-file data-model refactor.
3. Display/kiosk/rotation/EDID plumbing. 4. Code that works but is not understood.
5. Anything touching mockup.svg, or a PENDING OWNER decision. 6. An ADOPTED decision you
believe is wrong: write the argument, let the owner rule.

## Session ending (standing instruction)
A coder session ends with the single word READY on its own final line only when every
command passed, or BLOCKED.
