# DECISIONS — register

One row per decision: id, date, status (ADOPTED / PENDING OWNER / REJECTED), decision, why.
**Future sessions must not re-open an ADOPTED entry.** If an entry is wrong, argue in
writing: add a dated entry proposing a replacement and let the owner rule. Silent
behaviour changes that contradict an ADOPTED row are a review rejection.

| id | date | status | decision | why |
|----|------|--------|----------|-----|
| D-001 | 2026-09-25 | ADOPTED | Kiosk engine = Brave Origin 154.1.96.59 (apt, Brave repo; brave-keyring 1.20, fonts-liberation; 451 MB; no desktop packages installed; no new listening sockets). Chromium is unavailable on noble (apt Candidate: none); Google Chrome declined by the owner; WebKitGTK is plan C only if measured idle CPU fails the 2%-of-a-core budget. Engine must be locked before T10 screenshots. | Screenshots taken in a different engine than the one that ships approve metrics we do not render. |
| D-002 | 2026-09-25 | ADOPTED | marvind listens on 127.0.0.1:8042 as a named constant. Fail loudly on bind failure. NEVER auto-increment. | 8080 rejected: the host already runs a service family that increments upward through 809x. |
| D-003 | 2026-09-25 | ADOPTED | Reserved ports — never bind, never proxy, never poll: 22 53 68/udp 546/udp 631 3350 3389 5353/udp 8091 8092 8093 8094 8095 8096 8097 8098 8099 8188 8189 8190 8888 8890 20241 36727 11434. | Owners: 22 sshd; 53 systemd-resolved; 631 CUPS; 3350/3389 xrdp (now disabled+masked); 8091 marvinweb main, 8092 coding, 8093 pic1, 8094 pic2, 8095 video, 8098 music, 8099 coder (all LAN-bound); 8096 llama-server architect (loopback by default, currently observed LAN-bound); 8097 llama-server coder (loopback, on-demand); 8188/8189 ComfyUI GPU0/GPU1; 8190 Wan2GP Gradio; 8888 SearXNG; 8890 searxng-mcp; 20241 and 36727 unidentified (loopback); 11434 retired Ollama. |
| D-004 | 2026-09-25 | ADOPTED | TWO MARVINS. marvinweb.service (port 8091, LAN-exposed) is the owner's existing AI web server and is unrelated to this project. marvind never talks to it, never proxies it, never becomes another one of its proxies. The marvind unit Description must read "HEART OF GOLD telemetry daemon (NOT marvinweb)". docs/MARVIN.md governs the panel's voice only. | Name collision; an agent confusing the two would wire the panel into the wrong service. |
| D-005 | 2026-09-25 | ADOPTED | Temperature colour unified: ONE function, band ok<60C / warn 60..90 / danger >=90, used for every temperature on the panel. 70C and 80C survive only as mood thresholds. The THERMALS header text change is recorded as PENDING (P-001), not applied. | Two competing colour rules (70C amber vs the 60/90 band) rendered the same temperature two ways in two boxes. |
| D-006 | 2026-09-25 | ADOPTED | Core dot matrix hard-scoped to 2 rows x 6 at p=11 for 12 threads. The >12-thread tiers in docs/GEOMETRY.md are DELETED as unbuildable. marvind refuses to start above 12 threads with a visible message. Matrix-mean agreement tolerance: +/- 12 points. | Three 64px rows need 192px; the band is 152px and the PROCESSOR box closes at y=808. p=11 in a 56px region quantises fills to 20% steps, so tighter agreement is unmeasurable. |
| D-007 | 2026-09-25 | ADOPTED | Wire schema field is the string "marvin/v1", not an integer. | A string self-documents and cannot be silently re-interpreted by a client doing numeric compare. |
| D-008 | 2026-09-25 | ADOPTED | NVML is struck. GPU telemetry is one nvidia-smi subprocess per tick with a 750ms timeout. | Opening a driver context on inference cards for no benefit. |
| D-009 | 2026-09-25 | ADOPTED | T11 acceptance replaced. New acceptance: marvind never opens /dev/nvidia\* and never appears in `nvidia-smi --query-compute-apps`. | "utilization.gpu reads 0% on both cards" is unachievable — llama-server is resident with 8.1 and 11.2 GiB. |
| D-010 | 2026-09-25 | ADOPTED | T11 split: T11a display plumbing, T11b 24h soak with flat RSS. | Soak failure and display failure have different owners and clocks; one task hid which. |
| D-011 | 2026-09-25 | ADOPTED | The front end is served BY marvind from the same loopback origin (GET / -> web/), not opened as file://. Requires a path-traversal test (GET /../../etc/passwd must not escape web/). | A file:// page fetching http://127.0.0.1:8042 is cross-origin and would force a permissive CORS header to satisfy a self-inflicted problem. |
| D-012 | 2026-09-25 | ADOPTED | SMART via root-owned file handoff, never from marvind directly. | marvind runs unprivileged; smartctl needs root. Handoff keeps root out of marvind. See docs/DATA.md SMART handoff. |
| D-013 | 2026-09-25 | ADOPTED | Gate addition: forbidigo in .golangci.yml bans outbound HTTP client calls (http.Get, http.Post, http.Client, http.DefaultClient) and net.Dial outside internal/server. | "Nothing phones home" is enforced by make verify instead of asserted in prose. |

## PENDING OWNER — mockup.svg edits (mockup.svg is NOT touched until each is approved)

| id | date | status | decision | why |
|----|------|--------|----------|-----|
| P-001 | 2026-09-25 | PENDING OWNER | THERMALS header text change (per D-005; exact replacement text not yet given by the owner). | Header wording must not imply a colour rule that no longer exists. Recorded, not applied. |
| P-002 | 2026-09-25 | PENDING OWNER | Replace mockup header `NETWORK — eth0` with `NET enp13s0`. | "NETWORK - eth0" overran the ESTAB/ERR text by 19 px and named a nonexistent interface (docs/GEOMETRY.md). |
| P-003 | 2026-09-25 | PENDING OWNER | Remove "PANIC COUNT" from the mockup footer. | It was never defined; Marvin says doomed in words, which is the same information with better delivery (docs/GEOMETRY.md footer budget). |
