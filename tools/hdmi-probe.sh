#!/usr/bin/env bash
# Proves the motherboard HDMI is enabled WITH the Nvidia cards installed.
# Start it, then plug a monitor into the motherboard port within 45s.
set -u
before=$(mktemp)
for c in /sys/class/drm/card*-*/status; do printf '%s %s\n' "$c" "$(cat "$c" 2>/dev/null)" >> "$before"; done
echo "watching for a connector state change for 45s — plug into the MOTHERBOARD hdmi now"
for i in $(seq 45); do
  sleep 1
  for c in /sys/class/drm/card*-*/status; do
    now=$(cat "$c" 2>/dev/null)
    old=$(grep -F "$c " "$before" | cut -d' ' -f2)
    [ "$now" != "$old" ] && echo "[${i}s] $(basename "$(dirname "$c")") -> ${now:-unknown}"
  done
done
echo; echo "connector state now:"
for c in /sys/class/drm/card*-*/status; do printf '  %-34s %s\n' "$(basename "$(dirname "$c")")" "$(cat "$c" 2>/dev/null)"; done
echo; echo "modes advertised by connected connectors:"
for c in /sys/class/drm/card*-*/modes; do
  d=$(dirname "$c"); [ "$(cat "$d/status" 2>/dev/null)" = "connected" ] || continue
  echo "  $(basename "$d")"; head -8 "$c" | sed 's/^/    /'
done
rm -f "$before"
