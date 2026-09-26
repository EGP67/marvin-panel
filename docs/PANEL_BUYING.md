# Panel acquisition constraints — nothing here is optional to reverse later

## Display signal path (decides whether this project can be seen at all)
1. The panel attaches to the MOTHERBOARD HDMI, i.e. the CPU's integrated graphics.
   CHECK IN BIOS FIRST: "Initial Display Output" / "IGD Configuration" must allow IGD while
   discrete GPUs are installed. Many boards default to PEG-only and the port is dead.
2. Buy a plain HDMI (or DP) panel. AVOID DisplayLink/evdi USB displays: extra host CPU,
   driver fragility, and it would make Marvin the most expensive thing on the box.
3. Native 1080x1920 portrait is rare. Acceptable alternatives, in preference order:
   a) panel whose OSD rotates firmware-side (free, no GPU cost, no compositor involvement)
   b) any 1920x1080 panel rotated by xrandr --output ... --rotate left|right
   c) custom modeline via edid-decode + kernel cmdline (last resort)
4. Confirm the port's version and that it stays powered when the OS is idle. Check
   /sys/class/drm/card*-*/status flips to connected when something is attached — test with
   any spare monitor before buying, not after.

## Physical / environmental
5. IPS, not OLED or TN. A static 9:16 dashboard is the worst possible content for OLED;
   burn-in is a when, not an if. If an OLED is chosen anyway, budget for pixel-shift and a
   nightly blank schedule as a requirement, not an enhancement.
6. Temperature rating: many small HDMI LCDs are rated 0-50C. Inside a case near exhaust you
   can exceed that. Mount in intake airflow; measure with the side closed for 24h.
7. Power: prefer a panel powered from inside the PSU (Molex/USB header) over a wall adapter
   so it dies and boots with the machine. Confirm a spare connector exists.
8. Clearance: measure case width + glass standoff before ordering thickness. Check the cable
   has room to bend, and that an HDMI head is not blocked by the GPU or CPU cooler.
9. Brightness/PWM: some panels dim by PWM at levels that flicker unpleasantly when filmed or
   stared at. Prefer one with a decent brightness range for indoor viewing through glass.
10. Mounting: VESA holes or a printed bracket behind the glass; the panel must be removable
    without disassembling the case, because you will iterate on this.

## Before buying — verify with discovery output
- docs/DISCOVERY.md section 5 shows whether an amdgpu connector exists and is attachable
- section 9 shows whether FAN BANK telemetry is real; if absent, tell no one and ship the
  "NO TELEMETRY" line anyway
- section 11 confirms port 8080 is free for marvind

## Pre-purchase test (two minutes, do it before ordering)
bash tools/hdmi-probe.sh, then plug any HDMI monitor into the motherboard port with the
Nvidia cards installed. A connector must flip to connected and advertise 1920x1080.
If nothing changes: BIOS "Initial Display Output" / "IGD Configuration" / "Multi-Monitor"
must allow IGD alongside discrete GPUs. Fix and retest before buying. Do not paper over a
dead port with a DisplayLink adapter — it would make the panel the most expensive process
on the box in CPU terms.

## T2 sync (2026-09-25) — facts that change install assumptions
- Kiosk engine DECIDED (DECISIONS.md D-001): Brave Origin 154.1.96.59 from the Brave apt
  repo (brave-keyring 1.20, fonts-liberation; 451 MB installed; no desktop packages; no
  new listening sockets). The DisplayLink prohibition above still stands.
- The port check above referenced 8080; superseded by DECISIONS.md D-002: marvind binds
  127.0.0.1:8042. Never bind, proxy or poll the reserved ports in D-003.
- xrdp on this host is disabled + masked (3350/3389, D-003). Nothing to turn off at install.
- Display plumbing: the kiosk runs as X display :0 on tty1, with the screen pinned to
  BusID "PCI:17:0:0", Driver "amdgpu".
- The kiosk unit uses PrivateDevices=yes with BindPaths limited to /dev/dri/card0 and
  /dev/dri/renderD128, so renderD129 and renderD130 - the two inference cards - do not
  exist for it.
- Optional hardware: a PWM fan hub with per-port tachs would turn the two 4-fan chains
  into eight individually-visible fans (docs/DATA.md FAN WIRING); optional, not required.
