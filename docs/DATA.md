# Metric -> source map (Ubuntu 24.04 on hog.local)
Sources verified 2026-09-26. Wire shapes: docs/SCHEMA.md. Decisions: docs/DECISIONS.md.
Standard library only: read /proc and /sys directly. The only per-tick subprocess is
nvidia-smi.

## GPU binding rule (read before writing gpu.go)
GPU0 and GPU1 are the two NVIDIA GeForce RTX 3060 cards (PCI 0000:01:00.0 and
0000:06:00.0). They come ONLY from nvidia-smi, matched by UUID, never by index order or
/sys order. The motherboard HDMI is driven by the CPU's integrated graphics (amdgpu, PCI
0000:11:00.0; its DRM card number is not stable, D-041), which also appears under
/sys/class/drm and in hwmon ("amdgpu", label "edge"). It is NOT GPU0 or GPU1 and is
excluded from telemetry.

## Map
CPU model string : /proc/cpuinfo "model name" (first), trimmed. Display form per D-020.
Threads          : /sys/devices/system/cpu/online (NOT runtime.NumCPU(), which is
                   affinity-derived and would let a capped unit hide cores). Refuse to
                   start above 12 (D-006).
Physical cores   : sysfs topology, unique (physical_package_id, core_id) over online
                   CPUs (once at startup, D-055); no lscpu child.
Per-thread %     : TWO /proc/stat samples >= 500 ms apart. One sample is always wrong.
                   total = user+nice+system+idle+iowait+irq+softirq+steal (guest and
                   guest_nice are already counted inside user).
                   busy = total - idle - iowait (matches mpstat; D-036).
iowait           : the iowait field of the "cpu " line in /proc/stat, as a delta.
Frequency        : /sys/devices/system/cpu/cpufreq/policy0/scaling_cur_freq (kHz).
Load             : /proc/loadavg. Mood input only; not displayed (D-017).
Memory           : /proc/meminfo. used = MemTotal - MemAvailable (NOT MemFree).
                   cache = Cached + SReclaimable - Shmem, clamped to [0, total - used]
                   (D-034). swap used = SwapTotal - SwapFree.
Disk space       : syscall.Statfs on /, /srv/hogdata, /boot (D-019), df semantics (D-055):
                   used = (blocks - bfree) x bsize, free = bavail x bsize,
                   used_pct = used / (used + free).
Disk I/O         : /proc/diskstats line for nvme0n1 (D-035), fields counted with
                   major=1, minor=2, name=3: sectors read (6) and written (10) x 512 per
                   second. No IOPS, in_flight or queue (D-056).
Network          : /sys/class/net/wlp14s0/statistics/{rx_bytes,tx_bytes} deltas per
                   second; errors from rx_errors / tx_errors. No link ceiling (D-018).
Connections      : count entries with state 01 (ESTABLISHED) in /proc/net/tcp and
                   /proc/net/tcp6 (D-036).
GPU              : nvidia-smi --query-gpu=index,name,uuid,pci.bus_id,memory.total,
                   memory.used,temperature.gpu,utilization.gpu,power.draw,power.limit
                   --format=csv,noheader,nounits
                   ONE subprocess per tick (measured 27 ms), 750 ms timeout, all fields
                   parsed from it (D-008, D-024). Bind by uuid; log pci.bus_id.
Temps            : /sys/class/hwmon/hwmon*/ matched by NAME first, then label:
                   k10temp temp1 "Tctl" -> CPU; nvme temp1 "Composite" -> NVMe
                   (nvme_max_c from temp1_max, 83.85 C). Excluded: amdgpu ("edge", the
                   display adapter) and mt7921_phy0 (the Wi-Fi chip). hwmon indexes are
                   not stable across boots: match by name at startup and log the winning
                   path. GPU temperatures come from nvidia-smi.
SMART            : root-owned handoff file (see SMART handoff below).
Fans             : system chassis fans only; no source today (see Fans below).

## Handling rules already paid for — keep them
- Every delta needs a previous sample. The first tick is null and renders "--", never 0.
- Counters wrap: a negative delta discards the sample; never display a spike.
- Fahrenheit = round(C*9/5+32), computed at display time. Store Celsius only.
- nvidia-smi can hang on a sick GPU: 750 ms timeout, mark that GPU stale, keep the
  render loop alive. Tick interval stays under 1.2 s (T7 test).
- Cadence: 1 s for instantaneous values and nvidia-smi; 5 s for hwmon temperatures and
  the SMART file.
- Ring buffers: 120 samples at 1 s for the CPU graph, 30 for GPU sparklines.
- No listening socket other than 127.0.0.1:8042. Nothing phones home from marvind (D-013).

## Display adapter (confirmed 2026-09-26; numbering rule 2026-09-29, D-041)
The iGPU is amdgpu at PCI 0000:11:00.0, boot_vga=1; its connector HDMI-A-1 is the
motherboard port. The RTX 3060s are PCI 0000:01:00.0 and 0000:06:00.0. DRM card, renderD
and i2c-N numbers follow probe order and are not stable across boots: always resolve by
PCI address (/sys/bus/pci/devices/0000:11:00.0/drm/). The kiosk sees only the iGPU's card
and render nodes (D-023, D-041).

## Fans (D-027, D-036)
FAN BANK 1/2 are the SYSTEM CHASSIS FANS, never GPU fans. The box has eight chassis
fans: four daisy-chained on each of two motherboard fan headers, so the two banks map
1:1 onto the two headers. No hwmon fan*_input exists today (no SuperIO driver bound).
Owner task O2 tries the in-kernel nct6683 driver. Until it succeeds, fans are null and
render "FAN BANK n: NO TELEMETRY".
If O2 succeeds, design for these physical facts:
- One tach per header: only the first fan in each chain reports RPM, so a failed fan in
  positions 2-4 is invisible to RPM alone.
- All four fans on a chain share one PWM domain: bank RPM is a proxy for intent, not for
  the health of any single fan.
- Infer dead fans rather than trusting RPM: RPM pinned near its maximum while
  temperatures rise means "BANK n RUNNING BUT NOT COOLING" (verdict not_cooling, SCHEMA
  invariant 6).
- If the chains are Molex-powered rather than on PWM/tach splitters, no tach reaches the
  board and O2 cannot succeed.
- Per-fan health would need a hub with per-port tachs (optional hardware,
  docs/PANEL_BUYING.md).

## SMART handoff (D-012)
- /var/lib/marvin/smart.json and smart.rc, written by /usr/local/sbin/marvin-smart via
  marvin-smart.service, triggered by marvin-smart.timer every 15 min (OnBootSec=2min,
  RandomizedDelaySec=60). Copies of the script and units go into deploy/ in doc step 5.
- The files are root:root 0644 in a 0755 directory. heartofgold reads them and never
  writes them.
- Missing, unparsable, or older than 60 minutes => smart.state "unknown", rendered
  "SMART --".
- smartctl JSON paths for NVMe: counters under nvme_smart_health_information_log
  (available_spare, available_spare_threshold, percentage_used, data_units_read,
  data_units_written, power_on_hours, unsafe_shutdowns, num_err_log_entries).
  Temperature is temperature.current (NOT temperature_celsius, which is ATA-only).
  smart_status.passed false => state "failing".
- NVMe temperature for THERMALS still comes from hwmon; it agrees with SMART (31.85 C vs
  32 C on 2026-09-26).
- NEVER copy serial_number, wwn or uuid from that file onto the wire, into fixtures,
  logs, or any repo file.

## T5 verification traffic
- LAN peer: the owner's laptop, scp to and from hog over Wi-Fi (wlp14s0).
- WAN reference (2026-09-25): download 28,556,489 B/s (228 Mbit/s); upload 16,715,125 B/s
  (134 Mbit/s).
- Cloudflare's __down endpoint returns 1 byte and is broken; never cite it.
