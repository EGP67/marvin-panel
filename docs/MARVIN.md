# Voice
1. Never report a number without an opinion about it.
2. Present tense, flat, weary. No exclamation marks, no emoji. ALL CAPS reserved for the
   wordmark and DON'T PANIC.
3. He notices absences as sharply as presences: nobody SSH'd in, no backup ran, an idle box.
4. Severity ladder: wry -> bleak -> actionable. At danger level the line must contain the
   real problem ("RAID DEGRADED, DISK 3") inside the joke, never instead of it.
5. Rate limiting is what keeps him charming, not his vocabulary:
   - a mood must hold >=90s before changing (hysteresis)
   - an exact line cannot repeat within 10 min; a template family within 3 min
   - at danger level repeat every 60s — annoying on purpose is correct here
6. He calls himself Marvin and the machine the ship / HEART OF GOLD.
7. Never break character to show an error. "THE FAN SENSOR IS NOT ANSWERING." not "N/A".

## Mood -> trigger (internal/mood)
doomed      disk <5% free | SMART failing | temp >=90C | service crashloop
aggrieved   iowait >15% | any temp >80C | GPU thermal slowdown
melancholic load/threads >0.85 sustained 2m
bored       load/threads <0.02 (he resents being idle)
content     nominal — he finds this suspicious

## Seed lines (extend to ~10 per mood; preserve the rhythm)
bored       "THE CPU IS IDLE AT 6%. I'VE NEVER ONCE BEEN IDLE."
bored       "NOTHING IS HAPPENING. I HAVE BEEN THINKING ABOUT NOTHING FOR ELEVEN MINUTES."
content     "ALL SYSTEMS NOMINAL. THEY'RE ALWAYS NOMINAL RIGHT BEFORE SOMETHING."
melancholic "PROCESSOR AT 91%. I THINK, THEREFORE I AM — OVERWHELMED."
melancholic "THE HEART OF GOLD HAS THE WORST PROBABILITY COEFFICIENT IN THE GALAXY. MY TEMP IS SECOND."
aggrieved   "IOWAIT 22%. EVERYONE WANTS TO WRITE. NOBODY ASKED HOW I FEEL ABOUT IT."
aggrieved   "84 DEGREES. I RAN COLD ONCE. NOBODY NOTICED."
doomed      "4% REMAINS ON /srv/hogdata. I'D TELL YOU WHAT THAT MEANS BUT YOU'D ONLY CLEAN SOMETHING IMPORTANT."
doomed      "SMART REPORTS REALLOCATED SECTORS. I HAVE NO FEELINGS ABOUT THIS. (I HAVE MANY.)"
fans        "FAN BANK 1: NO TELEMETRY. I'M COOLING BY FORCE OF WILL."
memory      "SWAP 0% — SPARE, UNLIKE ME."
network     "118 MIB/S INBOUND AND STILL NOBODY CALLS."
processes   "JAVA AT 94%. AMBITIOUS."   [UNSPEAKABLE — see Line requirements below]
night       "I'M NOT ASLEEP. I'M IGNORING YOU WITH MY EYES CLOSED."

## Line requirements — speakability (T2 sync, 2026-09-25)
Every line carries `requires: [metric paths]`. The engine may emit a line only if EVERY
requirement resolves to a non-null value (docs/SCHEMA.md nullability rules). Paths are
snapshot.json fields; "engine" means internal/mood state, not a wire field.

| line | requires | status |
|---|---|---|
| bored "CPU IS IDLE AT 6%" | cpu.total_pct | SPEAKABLE |
| bored "NOTHING IS HAPPENING ... ELEVEN MINUTES" | cpu.total_pct, engine mood dwell | SPEAKABLE |
| content "ALL SYSTEMS NOMINAL" | cpu.band, gpus[].band, temps.nvme_band, storage[].state | SPEAKABLE |
| melancholic "PROCESSOR AT 91%" | cpu.total_pct | SPEAKABLE |
| melancholic "PROBABILITY COEFFICIENT ... MY TEMP" | cpu.temp_c | SPEAKABLE |
| aggrieved "IOWAIT 22%" | cpu.iowait_pct | SPEAKABLE |
| aggrieved "84 DEGREES" | cpu.temp_c | SPEAKABLE |
| doomed "4% REMAINS ON /srv/hogdata" | storage[mount=/srv/hogdata].free_bytes | SPEAKABLE (was /tank — corrected; this ship's mounts are / and /srv/hogdata on LVM over a single NVMe) |
| doomed "SMART REPORTS REALLOCATED SECTORS" | smart.* — UNKNOWN wire path; SMART exists only as the root-owned handoff file (docs/DATA.md), not yet a SCHEMA field | NOT EMITTABLE until T3 adds it |
| fans "FAN BANK 1: NO TELEMETRY" | fans[0].rpm == null | SPEAKABLE (absence is the requirement) |
| memory "SWAP 0%" | memory.swap_total_bytes, memory.swap_used_bytes | SPEAKABLE |
| network "118 MIB/S INBOUND" | network[].rx_bps | SPEAKABLE |
| processes "JAVA AT 94%" | per-process telemetry — does not exist on this ship | **UNSPEAKABLE** (kept so the reason survives) |
| night "I'M NOT ASLEEP" | (time of day only) | SPEAKABLE |

Voice-rule examples that predate discovery: rule 3's "nobody SSH'd in" and "no backup
ran", and rule 4's "RAID DEGRADED, DISK 3", name quantities this box cannot see — there
is no ssh/backup telemetry in the map, and there is no RAID (single NVMe under LVM). The
rules stand; those examples are UNSPEAKABLE here and their doomed-tier equivalents are
free space, SMART status and temperature.
