# Metric -> source map (Ubuntu on hog.local)

## GPU binding rule (read before writing gpu.go)
Marvin's GPU0 and GPU1 are the two NVIDA cards and come ONLY from nvidia-smi, matched by
name/UUID, never by index order. The motherboard HDMI is driven by the CPU's integrated
amdgpu graphics; that adapter also appears under /sys/class/drm and may also appear in
hwmon with its own temperature. It is NOT GPU0 or GPU1. It may be reported once, small,
as the ship's display adapter, or not at all — but it must never displace or rename a real
GPU, and its idle temperature must never be presented as "GPU0".

CPU model string : /proc/cpuinfo "model name" (first), trim whitespace
Threads          : runtime.NumCPU()   Physical: lscpu -p=CPU,CORE, dedupe on CORE
Per-thread %     : TWO /proc/stat samples >=500ms apart. One sample is always wrong.
                   busy = user+nice+system+irq+sirq+softirq+iowait; total = all fields
Frequency        : /sys/devices/system/cpu/cpufreq/policy0/scaling_cur_freq (kHz)
Load             : /proc/loadavg ; 4th field = running/total processes
Memory           : /proc/meminfo -> used = MemTotal - MemAvailable (NOT MemFree)
                   cache = Cached + SReclaimable ; swap from SwapTotal/SwapFree
Disk space       : syscall.Statfs per configured mountpoint
Disk IO          : /proc/diskstats fields 6/10 (sectors read/written) * 512, delta/sec
                   IOPS fields 4/8 ; in flight field 12 ; iowait = 6th field of "cpu " line
Network          : /sys/class/net/<if>/statistics/{rx_bytes,tx_bytes} delta/sec
                   ceiling /sys/class/net/<if>/speed (Mb/s) -> "% OF LINK"
                   established via ss -s ; errors via rx/tx_errors
GPU              : nvidia-smi --query-gpu=index,name,uuid,memory.total,memory.used,
                     temperature.gpu,utilization.gpu,power.draw,clocks.sm
                     --format=csv,noheader,nounits
                   ONE subprocess per tick (~30ms), parse all fields from it. Never once
                   per field. NVML bindings are acceptable if vendored.
Temps            : /sys/class/hwmon/hwmon*/temp*_input matched by temp*_label substring
                   (Tctl|Tdie|Package|Pkg for CPU, edge|junction for NVMe). Names vary by
                   vendor: log which sensor won and its path, and fall back to the highest
                   value only when no label matches.
NVMe             : hwmon nvme* input, else smartctl -A -j /dev/nvme0
Fans             : /sys/class/hwmon/*/fan*_input. DISCOVERY.md decides whether this exists.
                   If absent: render "FAN BANK n: NO TELEMETRY" and stop guessing.

## Handling rules already paid for — keep them
- Every delta needs a previous sample; first tick renders "--", never 0.
- Counters wrap: negative delta means discard the sample, not display a spike.
- Fahrenheit is computed as round(C*9/5+32). Store Celsius only. Never store both.
- nvidia-smi can hang on a sick GPU: exec with a 750ms timeout, mark that GPU stale, keep
  the render loop alive. Assert tick interval stays under 1.2s in tests.
- Cadence: 1s instantaneous values; 5s temps, SMART, GPU names, fan RPM.
- Ring buffers: 120 samples at 1s for the CPU graph, 30 for GPU sparklines.
- No dependency may open a listening socket other than 127.0.0.1, and none may phone home.

## FAN TELEMETRY — DECIDED by discovery (docs/DISCOVERY.md section 9)
"No fan*_input anywhere — fans are not software-visible on this board" (as unprivileged user).
Resolution order, fixed here so it is not re-decided in code:
1. If tools/discover-sudo.sh binds a SuperIO chip and a reboot yields working fan*_input:
   use two channels, and relabel each bank with its discovered *_label so the panel names
   what it is actually reading.
2. Otherwise bind the two slots to GPU fans (nvidia-smi fan.speed) and change the labels to
   "FAN BANK 1 — GPU0" / "FAN BANK 2 — GPU1". Honest labelling is mandatory; Marvin reports
   a quantity he can see, never a guess.
3. Otherwise render "FAN BANK n: NO TELEMETRY" permanently and let Marvin complain about it.
Detect once at startup, cache the conclusion, log which branch was taken. Never poll sysfs
every second for a chip that does not exist.

## DISPLAY ADAPTER — CONFIRMED by discovery (docs/DISCOVERY.md section 5)
card0 is bound to amdgpu: the CPU integrated graphics owns the motherboard HDMI. No
connector reported connected only because nothing is attached yet.
- GPU0/GPU1 come from nvidia-smi ONLY, matched by name/UUID, never by index or /sys order.
- amdgpu may expose its own hwmon temperature and connectors. That is the ship's display
  adapter, not GPU0. Excluding it entirely is the default.
- Acceptance test for T11: with the panel running, nvidia-smi utilization.gpu must read 0%
  on both cards. If it does not, something is rendering on the wrong device.
