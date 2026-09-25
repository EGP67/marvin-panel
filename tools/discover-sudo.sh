#!/usr/bin/env bash
# Privileged read-only probes. Output goes to docs/DISCOVERY.sudo.md so the
# read-only docs/DISCOVERY.md stays exactly what discover.sh produced.
set -u
SUDO=$(command -v sudo || echo "")
r(){ printf '\n### `%s`\n\n```text\n' "$*"; bash -c "$*" 2>&1 | sed 's/^/  /' || true; printf '```\n'; }
say(){ printf '\n## %s\n' "$*"; }

printf '# DISCOVERY (privileged) — %s\n\nGenerated %s by tools/discover-sudo.sh\n' "$(hostname -f 2>/dev/null || hostname)" "$(date -Is)"

say "1. FAN VISIBILITY — the decisive probe"
r "$SUDO dmesg | grep -iE 'it87|nct6775|nct6796|w836|f7188|coretemp|k10temp|ec_|asus' | tail -25"
r "lsmod | grep -iE 'it87|nct|w836|coretemp|k10temp' || echo 'no sensor kernel modules loaded'"
r "$SUDO sensors-detect --auto 2>&1 | tail -45"
echo "   If --auto identified a chip, run: sudo sensors-detect   (interactive) and answer NO to"
echo "   any embedded-controller/BMC probe, then reboot and re-run tools/discover.sh section 9."

say "2. SMART health (STORAGE panel: status, reallocated sectors, power-on hours)"
for d in $(lsblk -dno NAME,TYPE | awk '$2=="disk"{print "/dev/"$1}'); do
  r "$SUDO smartctl -H -A $d 2>&1 | grep -iE 'Smart status|Reallocated|Power On Hours|Temperature|Percentage Used|Critical Warning|Media and Data|Unavailable Error' | head -12"
done

say "3. NVMe composite temperature + lifespan (THERMALS panel)"
r "$SUDO nvme list 2>&1 | head -8"
for c in $(ls /dev/nvme[0-9]* 2>/dev/null | grep -E 'nvme[0-9]+$'); do
  r "$SUDO nvme smart-log $c 2>&1 | grep -iE 'Composite Temperature|Percentage Used|Available Spare|Media and Data Error|Power On' | head -8"
done

say "4. hwmon tree as root (some chips never appear unprivileged)"
r "$SUDO sensors 2>&1 | head -60"
r "$SUDO find /sys/class/hwmon -maxdepth 2 \( -name 'temp*_input' -o -name 'fan*_input' -o -name '*_label' \) | sort | head -80"

say "5. GPU FANS — the reliable fallback if chassis fans stay invisible"
r "nvidia-smi --query-gpu=index,name,fan.speed,temperature.gpu,temperature.memory,power.draw --format=csv,noheader"
r "nvidia-smi -q -d FAN_SPEED 2>&1 | grep -A2 -iE 'Fan Speed|GPU' | head -14"

say "6. iGPU / HDMI plumbing and EDID history"
r "$SUDO dmesg | grep -iE 'amdgpu|drm|hdmi|connector|edid|dp[0-9]' | tail -35"
r "for c in /sys/class/drm/card0-*; do printf '%-32s %s\n' \"\$(basename \"\$c\")\" \"\$(cat \"\$c/status\" 2>/dev/null || echo n/a)\"; done"

say "7. Authoritative link speed (if /sys reported -1)"
for i in $(ls /sys/class/net | grep -vE 'lo|docker|veth|br-'); do r "$SUDO ethtool $i 2>&1 | grep -iE 'Speed|Duplex|Link detected'"; done

say "8. Service account prerequisites"
r "id marvin 2>&1; getent group render video docker 2>/dev/null"
r "$SUDO systemctl --version | head -1"
