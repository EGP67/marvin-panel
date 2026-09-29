# CLAUDE.md — marvin-panel (HEART OF GOLD)

Agent entry point for Claude Code (D-030). Read this first. Stop reading further files
at the first one your task does not touch.

## What this is
A permanent portrait 1080x1920 telemetry panel behind the glass side panel of hog.local
(Ubuntu 24.04), rendered in the voice of Marvin. One dependency-free Go binary, marvind,
collects host telemetry and serves /snapshot.json plus the static page on 127.0.0.1:8042.
Brave Origin in kiosk mode displays it on the motherboard (iGPU) HDMI. Development is
fixture-driven until the T4-T8 collectors exist.

## Roles
- Owner: approves every plan, every mockup/geometry change, and the scope of every commit.
- Architect: Claude Web. Decisions, designs, and the prompts Claude Code executes.
- Coder: Claude Code. File edits, builds, tests, git. Executes the prompt as written;
  nothing beyond its scope.

## Authority order when documents conflict
1. docs/DECISIONS.md — ADOPTED rows govern. SUPERSEDED / CLOSED / RETIRED rows are history.
2. mockup.svg and docs/GEOMETRY.md — pixels, fonts, colors, budgets.
3. HANDOFF.md, TASKS.md, docs/SCHEMA.md, docs/DATA.md, docs/MARVIN.md.
If a lower document contradicts a higher one, follow the higher one and report the
contradiction. Do not fix it unless the task says so. Never re-open an ADOPTED decision
in code; argue it in the report and let the owner rule.

## Reading order
1. CLAUDE.md
2. HANDOFF.md — status, architecture, hard constraints
3. docs/DECISIONS.md
4. TASKS.md — what to build, in order, with acceptance criteria
5. docs/SCHEMA.md, docs/GEOMETRY.md, docs/DATA.md, docs/MARVIN.md — only when the task
   touches the wire contract, pixels, sources, or voice respectively
docs/DISCOVERY.md is reference only: read the section you need; never re-run
tools/discover.sh.

## File map
| path | what it is | status |
|---|---|---|
| CLAUDE.md | this file | NORMATIVE process |
| HANDOFF.md | project truth: status, architecture, constraints | NORMATIVE |
| TASKS.md | ordered tasks + acceptance criteria | NORMATIVE |
| docs/DECISIONS.md | decision register | NORMATIVE; highest authority |
| docs/SCHEMA.md | snapshot.json wire contract "marvin/v1" | NORMATIVE |
| docs/GEOMETRY.md | coordinates, fonts, color rules, budgets | NORMATIVE |
| docs/DATA.md | metric -> source map + handling rules | NORMATIVE |
| docs/MARVIN.md | voice, mood triggers, seed lines, speakability | NORMATIVE for voice only (D-004) |
| docs/PANEL_BUYING.md | display purchase + install constraints | reference |
| docs/DISCOVERY.md | read-only system survey (2026-09-25) | reference; do not re-run |
| mockup.svg | the panel as drawn | NORMATIVE geometry; edit only under an owner-approved decision naming the change (D-022, D-032); its numbers are illustrative and are never copied into fixtures |
| README.md | human quick start | reference |
| Makefile | build + verify gate | NORMATIVE gate |
| go.mod | module hog.local/marvin-panel | zero dependencies; keep it that way |
| .golangci.yml, .gitleaks.toml | gate configuration | NORMATIVE gate |
| cmd/marvind/ | daemon | code |
| cmd/genfixtures/ | sole author of fixtures/*.json | code |
| internal/server/ | HTTP handlers, fixture replay | code |
| internal/collect/, internal/model/, internal/mood/ | T3-T9 | planned; do not create early |
| fixtures/ | calm busy hot dying | generated; regenerate, never hand-edit |
| web/ | index.html arrives at T10 (D-011) | code |
| deploy/ | systemd units | STALE until the deploy task (D-031) |
| tools/ | discover.sh, discover-sudo.sh, hdmi-probe.sh | owner-run tooling |

## Hard constraints
- Portrait 1080x1920. No keyboard, mouse or touch input; no interaction, ever. Always on.
- Never endanger the host. Panel services run as the dedicated account heartofgold
  (D-023), never uid 1000. CPU/RAM capped; restart always.
- An unavailable metric renders visible text ("NO TELEMETRY"). null is never replaced by 0.
- marvind never opens /dev/nvidia*. Its only Nvidia access is one nvidia-smi child per
  tick with a 750 ms timeout (D-008, D-024). The kiosk sees only the iGPU
  (PCI 0000:11:00.0) card + render nodes; DRM numbers are not stable (D-041).
- marvind binds 127.0.0.1:8042 only (D-002). Never bind, proxy or poll a reserved port
  (D-015). Never talk to marvinweb (D-004).
- No outbound network from marvind (D-013, enforced by lint).
- Go standard library only; go.mod stays dependency-free. No third-party package without
  owner approval.
- All logging to STDERR.
- Never write the NVMe serial number, WWN or UUID into any file in this repo or any log.
- Real GPU UUIDs may appear only where .gitleaks.toml allowlists them.
- American English spelling everywhere (D-033).

## Workflow rules (every task)
1. Scope: do exactly what the prompt says. Anything else you notice goes in the report,
   not in the diff.
2. Snapshot before modify: before editing or deleting any existing file, copy it with
   cp -p to /srv/hogdata/marvin/archive/<name>.<YYYYmmdd-HHMMSS>. If a snapshot fails,
   change nothing and report BLOCKED. New files need no snapshot.
3. Gate: stage the commit's files first (rule 5), then run make verify
   (golangci-lint, go test -race, govulncheck, gitleaks over history AND the staged
   changes, D-039). It must exit 0 before git commit; on failure fix, re-stage,
   re-run. Code changes also run make build. Docs-only and external-tool-only commits
   skip make build.
4. Deploy (once the targets exist, D-031): commit-completing prompts end with
   make deploy; commits that change runtime behavior follow it with make restart.
   Docs-only and external-tool-only commits skip both. make install-service-prereqs and
   make install-service are owner-run (password-prompted sudo).
5. Staging: git add / git rm only the files the prompt names. Never git add -A or
   git commit -a. Never push, amend or rewrite history unless the prompt says so.
6. No sudo. If a step needs root, stop and give the owner the exact commands to run.
7. govulncheck needs network. On failure: paste the error verbatim, retry once, then
   report BLOCKED rather than commit.
8. Report: write the report file the prompt names. Its last lines are
   "OUTPUT: c2c <report path>" (the owner runs it) and then READY or BLOCKED alone on the
   final line. READY only if every command passed.

## Escalation — stop and report BLOCKED rather than attempt a 4th time
1. The same step fails 3 times.
2. A multi-file data-model refactor is needed.
3. Display, kiosk, rotation or EDID plumbing misbehaves.
4. Code works but you cannot explain why.
5. Anything touching mockup.svg without a decision naming the change, or a PENDING OWNER
   item.
6. You believe an ADOPTED decision is wrong: write the argument; the owner rules.

## Definition of done
See HANDOFF.md "Definition of done".
