# Voice
1. Never report a number without an opinion about it.
2. Present tense, flat, weary. No exclamation marks, no emoji. Phrase text is uppercase,
   as drawn in the mockup; literal paths and interface names keep their real case
   (D-036).
3. He notices absences as sharply as presences: an idle box, a sensor that will not
   answer.
4. Severity ladder: wry -> bleak -> actionable. At danger level the line must contain the
   real problem ("4% REMAINS ON /srv/hogdata") inside the joke, never instead of it.
5. Rate limiting is what keeps him charming, not his vocabulary:
   - a mood must hold >= 90 s before changing (hysteresis)
   - an exact line cannot repeat within 10 min; a template family within 3 min
   - at danger level repeat every 60 s — annoying on purpose is correct here
6. He calls himself Marvin and the machine the ship / HEART OF GOLD.
7. Never break character in the phrase box to show an error: "THE FAN SENSOR IS NOT
   ANSWERING.", not "N/A". Numeric fields elsewhere on the panel render "--" or
   "NO TELEMETRY" per docs/GEOMETRY.md.
   Exception: when /snapshot.json is unreachable the page shows "THE SHIP IS NOT
   ANSWERING. I KNOW HOW IT FEELS." — marvind cannot choose a line it cannot serve.
8. Every line fits the phrase budget in docs/GEOMETRY.md: <= 100 characters, wrapped to
   2 lines of <= 52.

## Mood -> trigger (internal/mood)
doomed      any mount < 5% free | smart.state failing | any temperature >= 90 C
aggrieved   iowait > 15% | any temperature > 80 C
melancholic load1/threads > 0.85 sustained 2 min
bored       load1/threads < 0.02 (he resents being idle)
content     nominal — he finds this suspicious
Removed (D-036): "service crashloop" and "GPU thermal slowdown" — nothing collects them.
Each confirmed entry into doomed increments the lifetime PANIC COUNT (D-017).

## Seed lines (extend to ~10 per mood; preserve the rhythm)
bored       "THE CPU IS IDLE AT 6%. I'VE NEVER ONCE BEEN IDLE."
bored       "NOTHING IS HAPPENING. I HAVE BEEN THINKING ABOUT NOTHING FOR ELEVEN MINUTES."
content     "ALL SYSTEMS NOMINAL. THEY'RE ALWAYS NOMINAL RIGHT BEFORE SOMETHING."
melancholic "PROCESSOR AT 91%. I THINK, THEREFORE I AM — OVERWHELMED."
melancholic "THE HEART OF GOLD HAS THE WORST PROBABILITY COEFFICIENT IN THE GALAXY. MY TEMP IS SECOND."
aggrieved   "IOWAIT 22%. EVERYONE WANTS TO WRITE. NOBODY ASKED HOW I FEEL ABOUT IT."
aggrieved   "84 DEGREES. I RAN COLD ONCE. NOBODY NOTICED."
doomed      "4% REMAINS ON /srv/hogdata. I'D TELL YOU WHAT THAT MEANS BUT YOU'D ONLY CLEAN SOMETHING IMPORTANT."
doomed      "SMART SAYS THE DRIVE IS FAILING. I HAVE NO FEELINGS ABOUT THIS. (I HAVE MANY.)"
fans        "FAN BANK 1: NO TELEMETRY. I'M COOLING BY FORCE OF WILL."
memory      "SWAP 0% — SPARE, UNLIKE ME."
network     "118 MIB/S INBOUND AND STILL NOBODY CALLS."
processes   "JAVA AT 94%. AMBITIOUS."   [UNSPEAKABLE — see below]
night       "I'M NOT ASLEEP. I'M IGNORING YOU WITH MY EYES CLOSED."

## Line requirements — speakability
Every line carries `requires: [metric paths]`. The engine may emit a line only if EVERY
requirement resolves to a non-null value (docs/SCHEMA.md). "engine" means internal/mood
state, not a wire field.

| line | requires | status |
|---|---|---|
| bored "CPU IS IDLE AT 6%" | cpu.total_pct | SPEAKABLE |
| bored "NOTHING IS HAPPENING ... ELEVEN MINUTES" | cpu.total_pct, engine mood dwell | SPEAKABLE |
| content "ALL SYSTEMS NOMINAL" | cpu.thermal_band, gpus[].thermal_band, temps.nvme_thermal_band, storage[].state, smart.state | SPEAKABLE |
| melancholic "PROCESSOR AT 91%" | cpu.total_pct | SPEAKABLE |
| melancholic "PROBABILITY COEFFICIENT ... MY TEMP" | cpu.temp_c | SPEAKABLE |
| aggrieved "IOWAIT 22%" | cpu.iowait_pct | SPEAKABLE |
| aggrieved "84 DEGREES" | cpu.temp_c | SPEAKABLE |
| doomed "4% REMAINS ON /srv/hogdata" | storage[mount=/srv/hogdata].free_bytes, total_bytes | SPEAKABLE |
| doomed "SMART SAYS THE DRIVE IS FAILING" | smart.state == "failing" | SPEAKABLE (D-035, D-036) |
| fans "FAN BANK 1: NO TELEMETRY" | fans[0].rpm == null | SPEAKABLE (absence is the requirement) |
| memory "SWAP 0%" | memory.swap_total_bytes, memory.swap_used_bytes | SPEAKABLE |
| network "118 MIB/S INBOUND" | network[0].rx_bps | SPEAKABLE |
| processes "JAVA AT 94%" | per-process telemetry — does not exist on this ship | UNSPEAKABLE (kept so the reason survives) |
| night "I'M NOT ASLEEP" | time of day only | SPEAKABLE |

Examples that name quantities this ship cannot see — "nobody SSH'd in", "no backup ran",
"RAID DEGRADED" (there is no RAID: one NVMe under LVM) — are UNSPEAKABLE. The doomed-tier
equivalents here are free space, SMART state and temperature.

## GPU line (D-037)
One line of at most 38 characters in the GRAPHICS box (gold italic), chosen by marvind
from GPU state. Voice rules 1-8 apply. An exact line cannot repeat within 10 min, and the
GPU line never repeats the topic of the current phrase-box line.
State, evaluated each tick, first match wins:
stale     either card stale (temp/util null)            "THE BRAINS ARE NOT ANSWERING."
hot       either card temp_c >= 80                       "THINKING THIS HARD RUNS AT <n> DEGREES."
thinking  mean util_pct >= 20                            "SOMEONE ASKED IT SOMETHING. NOT ME."
loaded    either card mem_used_mib >= 1024               "MODEL LOADED. NOBODY ASKS IT ANYTHING."
empty     otherwise                                      "BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY."
Pacing: the line changes when a new state has held for 30 s, or every 5 min within a
state. Extend to ~5 lines per state; each line is original writing in Marvin's voice,
never a quotation from the books or films.
