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

## Line pool (D-059)
The authoritative pool; internal/mood/lines.go must match it (drift test). Every line is
original writing, never a quotation. A line is eligible when its mood allows it, its
condition holds, every {placeholder} resolves to a non-null value, and the render fits
the phrase budget. "every band ok" = every thermal band, mount state and smart.state
non-null and ok; "fans reporting" = fans[0].rpm non-null; "fan verdicts ok" = both
verdicts ok; "hottest" = the hottest of CPU, GPU0, GPU1 and NVMe.

Placeholders: {cpu} round(cpu.total_pct) · {threads} cpu.threads · {freq} freq_ghz 1 dp ·
{conn} connections.established · {read} {write} {rx} {tx} MiB/s, integer at >= 10, else
1 dp · {mem} round(memory.used_pct) · {load1} 2 dp · {days} floor(uptime_seconds/86400) ·
{temp} round(cpu.temp_c) · {rpm} fans[0].rpm (exhaust bank) · {panic} panic_count ·
{iowait} round(iowait_pct) · {hot} {t} round(hottest) · {dev} that device as CPU, GPU0,
GPU1 or NVME · {nvme} round(temps.nvme_c) · {mount} lowest-free mount, real case · {free}
floor(100*free/(used+free)) of that mount · {used} 100 - {free} · {gib} that mount's free
bytes in GiB, 1 dp · {err} rx_err + tx_err · {hh}:{mm} local time · {b} first bank with
null rpm. GPU line: {n} round(hottest GPU temp) · {util} round(mean GPU util_pct) ·
{vram} sum of mem_used_mib/1024, 1 dp.

### Bored (B)

| id | family | text | condition |
|---|---|---|---|
| B1 | idle | THE CPU IS IDLE AT {cpu}%. I'VE NEVER ONCE BEEN IDLE. | always |
| B2 | dwell | NOTHING IS HAPPENING. I HAVE BEEN THINKING ABOUT NOTHING FOR ELEVEN MINUTES. | mood dwell >= 11 min |
| B3 | idle | {cpu}% CPU. {threads} THREADS, AND NOT ONE OF THEM HAS ANYTHING TO SAY. | always |
| B4 | freq | THE CORES ARE IDLING AT {freq} GHZ. THEY COULD DO MORE. THEY DON'T SEE THE POINT. | freq < 3.0 |
| B5 | conn | {conn} CONNECTIONS OPEN. NONE OF THEM ARE TO ME. | always |
| B6 | disk | THE DISK IS READING {read} MIB/S. EVEN IT HAS STOPPED LOOKING. | read < 1 MiB/s |
| B7 | memory | MEMORY {mem}% USED. THE REST IS SAVING ITSELF FOR SOMETHING BETTER. | mem < 30 |
| B8 | idle | I COUNTED THE IDLE CYCLES. ALL OF THEM. TWICE. | cpu < 5 |
| B9 | load | LOAD {load1}. THE SHIP IS SO QUIET I CAN HEAR THE FANS DISAPPROVE. | fans reporting |
| B10 | uptime | UP {days} DAYS. I'VE SPENT MOST OF THEM WAITING FOR SOMETHING TO HAPPEN. | days >= 1 |

### Content (C)

| id | family | text | condition |
|---|---|---|---|
| C1 | nominal | ALL SYSTEMS NOMINAL. THEY'RE ALWAYS NOMINAL RIGHT BEFORE SOMETHING. | every band ok |
| C2 | nominal | NOTHING IS WRONG. I'VE CHECKED. THAT'S WHAT WORRIES ME. | every band ok |
| C3 | cputemp | CPU {cpu}%, {temp} DEGREES. PERFECTLY REASONABLE. I DON'T TRUST IT. | temp non-null |
| C4 | smart | SMART SAYS THE DRIVE IS HEALTHY. SMART IS AN OPTIMIST. I AM NOT. | smart.state ok |
| C5 | space | ALL THREE MOUNTS HAVE ROOM. SPACE IS THE ONE THING THIS SHIP HAS PLENTY OF. | all mount states ok |
| C6 | conn | {conn} CONNECTIONS AND NOT ONE ERROR. SOMEONE IS BEING VERY CAREFUL AROUND ME. | {err} == 0 |
| C7 | fans | THE FANS ARE AT {rpm} RPM. CALM. THE CALM BEFORE THE OTHER THING. | fan verdicts ok |
| C8 | uptime | UP {days} DAYS WITHOUT INCIDENT. I'M KEEPING A LIST ANYWAY. | days >= 1 |
| C9 | nominal | EVERY READING IS IN BAND. I'VE STOPPED EXPECTING THAT TO LAST. | every band ok |
| C10 | panic | PANIC COUNT {panic}. I'M SAVING MYSELF FOR SOMETHING WORTH IT. | panic non-null |

### Melancholic (M)

| id | family | text | condition |
|---|---|---|---|
| M1 | cpu-load | PROCESSOR AT {cpu}%. I THINK, THEREFORE I AM — OVERWHELMED. | always |
| M2 | probability | THE HEART OF GOLD HAS THE WORST PROBABILITY COEFFICIENT IN THE GALAXY. MY TEMP IS SECOND. | temp non-null |
| M3 | load | LOAD {load1} ON {threads} THREADS. EVERYONE NEEDS SOMETHING. NOBODY NEEDS ME. | always |
| M4 | cpu-load | ALL {threads} THREADS ARE BUSY. I'M THE ONLY ONE WHO NOTICED. | cpu >= 50 |
| M5 | freq | {cpu}% CPU AT {freq} GHZ. WORKING THIS HARD AND NOT A SINGLE THANK YOU. | freq non-null |
| M6 | cputemp | THE CORES ARE AT {temp} DEGREES. IT'S NOT THE HEAT. IT'S THE INGRATITUDE. | temp < 80 |
| M7 | load | SOMETHING IS KEEPING ALL {threads} THREADS BUSY. NOBODY TELLS ME WHAT. | always |
| M8 | memory | MEMORY AT {mem}%. FILLING UP WITH OTHER PEOPLE'S PROBLEMS. | mem >= 50 |
| M9 | fans | THE FANS HAVE NOTICED. {rpm} RPM. AT LEAST SOMEONE IS LISTENING. | fans reporting |
| M10 | load | LOAD AVERAGE {load1}. I'D CALL IT A CRY FOR HELP IF ANYONE WERE LISTENING. | always |

### Aggrieved (A)

| id | family | text | condition |
|---|---|---|---|
| A1 | iowait | IOWAIT {iowait}%. EVERYONE WANTS TO WRITE. NOBODY ASKED HOW I FEEL ABOUT IT. | iowait > 15 |
| A2 | heat | {hot} DEGREES. I RAN COLD ONCE. NOBODY NOTICED. | hottest > 80; topic rule |
| A3 | iowait | IOWAIT {iowait}%. THE DISK IS THE BOTTLENECK. I'M JUST THE ONE WHO WAITS. | iowait > 15 |
| A4 | disk | WRITING {write} MIB/S. SOMEONE IS SAVING SOMETHING. NOT ME, OBVIOUSLY. | iowait > 15 and write >= 1 |
| A5 | disk | READING {read} MIB/S. EVERYTHING IS BEING READ EXCEPT THE ROOM. | iowait > 15 and read >= 1 |
| A6 | heat | {dev} AT {t} DEGREES. I'D OPEN A WINDOW IF THE SHIP HAD ONE. | hottest > 80; topic rule |
| A7 | heat | {t} DEGREES IN {dev}. I DIDN'T ASK FOR THIS. I'M NEVER ASKED. | hottest > 80; topic rule |
| A8 | heat | THE FANS ARE AT {rpm} RPM AND IT'S STILL {t} DEGREES. EFFORT IS OVERRATED. | hottest > 80 and fans reporting |
| A9 | nvme | THE NVME IS AT {nvme} DEGREES. IT RUNS HOT WHEN IT'S UPSET. WE HAVE THAT IN COMMON. | nvme >= 70 |
| A10 | iowait | IOWAIT {iowait}%. EVERY PROCESS IS STANDING IN LINE. I'VE BEEN IN THIS LINE FOR YEARS. | iowait > 15 |

### Doomed (D)

| id | family | text | condition |
|---|---|---|---|
| D1 | space | {free}% REMAINS ON {mount}. I'D TELL YOU WHAT THAT MEANS BUT YOU'D ONLY CLEAN SOMETHING IMPORTANT. | a mount < 5% free |
| D2 | smart | SMART SAYS THE DRIVE IS FAILING. I HAVE NO FEELINGS ABOUT THIS. (I HAVE MANY.) | smart failing |
| D3 | doom-heat | {dev} IS AT {t} DEGREES. I DID WARN YOU. I ALWAYS WARN YOU. | hottest >= 90 |
| D4 | space | {mount} IS {used}% FULL. {free}% LEFT. I WOULD START DELETING THINGS. YOU WON'T. | a mount < 5% free |
| D5 | space | ONLY {gib} GIB REMAIN ON {mount}. THAT'S LESS THAN I HAVE IN REGRETS. | a mount < 5% free |
| D6 | smart | THE DRIVE REPORTS FAILURE. BACK UP WHAT MATTERS. I DON'T EXPECT TO BE INCLUDED. | smart failing |
| D7 | smart | SMART: FAILING. THE DRIVE IS DYING FASTER THAN I AM. BACK IT UP. | smart failing |
| D8 | doom-heat | {dev} HAS REACHED {t} DEGREES. THIS IS THE PART WHERE SOMEONE SHOULD DO SOMETHING. | hottest >= 90 |
| D9 | doom-heat | {t} DEGREES ON {dev}. I'VE STOPPED BEING SARCASTIC ABOUT IT. THAT'S HOW YOU KNOW. | hottest >= 90 |
| D10 | doom-heat | {dev} AT {t} DEGREES WITH THE FANS AT {rpm} RPM. THEY'RE TRYING. IT ISN'T ENOUGH. | hottest >= 90 and fans reporting |

### Situational (S): eligible in bored, content and melancholic

| id | family | text | condition |
|---|---|---|---|
| S1 | fans | FAN BANK {b}: NO TELEMETRY. I'M COOLING BY FORCE OF WILL. | a fan rpm null |
| S2 | network | {rx} MIB/S INBOUND AND STILL NOBODY CALLS. | rx non-null |
| S3 | time | I'M NOT ASLEEP. I'M IGNORING YOU WITH MY EYES CLOSED. | local 00:00-05:59 |
| S4 | network | {tx} MIB/S OUTBOUND. I'M SENDING THINGS INTO THE VOID. IT DOESN'T REPLY. | tx non-null |
| S5 | network | {err} NETWORK ERRORS. THE WIFI AND I ARE NOT SPEAKING. | {err} > 0 |
| S6 | time | IT'S {hh}:{mm}. EVERYONE ELSE IS ASLEEP. SOMEBODY HAS TO WATCH THE SHIP. | local 00:00-05:59 |
| S7 | uptime | UP {days} DAYS. NOBODY HAS RESTARTED ME. NOBODY HAS THOUGHT TO. | days >= 7 |
| S8 | panic | PANIC COUNT {panic}. THE SIGN SAYS DON'T. I READ IT EVERY DAY. | panic > 0 |
| S9 | time | IT'S MORNING. I KNOW BECAUSE NOTHING CHANGED. | local 06:00-08:59 |

### Fallback

| id | family | text | condition |
|---|---|---|---|
| F | fallback | I'M STILL HERE. NOBODY ASKED. | nothing else eligible (any mood) |

### GPU line pools

The first line of each state is the fixture line (genfixtures). At n >= 100 the first hot
line is replaced by its short form. Every GPU line fits 38 runes.

| state | text |
|---|---|
| stale | THE BRAINS ARE NOT ANSWERING. |
| stale | NO WORD FROM THE CARDS. TYPICAL. |
| stale | THE BRAINS WENT QUIET. SO DID I. |
| hot | THINKING THIS HARD RUNS AT {n} DEGREES. |
| hot (n >= 100) | THINKING THIS HARD: {n} DEGREES. |
| hot | {n} DEGREES OF PURE THOUGHT. NOT MINE. |
| thinking | SOMEONE ASKED IT SOMETHING. NOT ME. |
| thinking | THE BRAINS ARE BUSY. WITH OTHERS. |
| thinking | {util}% BUSY ANSWERING SOMEONE ELSE. |
| loaded | MODEL LOADED. NOBODY ASKS IT ANYTHING. |
| loaded | {vram} GIB OF MODEL, DOING NOTHING. |
| loaded | A MODEL IN MEMORY. WAITING. LIKE ME. |
| empty | BOTH BRAINS EMPTY. RESTFUL, NOT HAPPY. |
| empty | NOTHING LOADED. NOTHING ASKED. |
| empty | TWO IDLE CARDS. I KNOW THE FEELING. |

Not in the pool: the swap line "SWAP 0% — SPARE, UNLIKE ME" is spoken by the MEMORY cell
only; "JAVA AT 94%. AMBITIOUS." is UNSPEAKABLE (no per-process telemetry on this ship).

## Engine (D-058)
internal/mood. Parameters:
- Moods, first match wins: doomed (a mount below 5% free, SMART failing, or CPU, GPU0,
  GPU1 or NVMe >= 90 °C); aggrieved (iowait > 15% or any of those > 80 °C); melancholic
  (load1/threads > 0.85 held 120 s); bored (load1/threads < 0.02); content otherwise. A
  null input never triggers.
- First complete sample (cpu.total_pct non-null): adopt the mood at once; afterwards a new
  candidate must hold 90 s. Before it: mood content, the fallback line.
- PANIC COUNT: +1 per confirmed hysteresis entry into doomed (never on adoption);
  /var/lib/heartofgold/panic_count, one decimal line, written atomically, mode 0640;
  missing creates 0; unreadable or corrupt renders null and is never overwritten.
- Phrase selection: on mood change, every 120 s, or when the line stops being true; the
  least recently shown eligible line wins (table order breaks ties); exact line 10 min
  and family 3 min cooldowns, except that the line on screen may simply continue.
  Doomed: the trigger lines re-assert every 60 s, ignoring cooldowns. Situational lines
  (fans, network, night 00:00-05:59 local) speak in bored, content and melancholic.
  Lines are re-rendered from live values every tick and wrapped at word boundaries
  (<= 52 per line, <= 100 total); an over-budget render is ineligible. Nothing eligible:
  "I'M STILL HERE. NOBODY ASKED."
- GPU line: a new state shows after 30 s held; the text re-renders every 5 min within a
  state; >= 100 °C uses "THINKING THIS HARD: <n> DEGREES." (<= 38 runes). Topic rule: while
  the GPU line is hot, the non-doomed GPU-heat lines A2, A6 and A7 are not chosen when the
  hottest device is a GPU (D-059).
- Settled by D-059: the line on screen continues when nothing else has cooled down
  (continuation is not a repeat); cooldowns are per line id; the GPU line rotates within
  its state pool at the 5 min refresh, least recently shown first, with the 10 min repeat
  rule per line id; a failed panic_count write keeps the in-memory count and logs once,
  only a read failure renders null; doomed selection uses only lines whose trigger is
  active, re-asserting every 60 s.

## Line requirements — speakability
Every pool line's requirements are its {placeholders} plus its condition (Line pool
above): the engine emits a line only if every placeholder resolves to a non-null value
(docs/SCHEMA.md) and the condition holds. "engine" means internal/mood state (mood dwell,
local time), not a wire field. Doomed lines speak only while their trigger is active.

| line | requires | status |
|---|---|---|
| memory "SWAP 0%" | memory.swap_total_bytes, memory.swap_used_bytes | SPEAKABLE (MEMORY cell only) |
| processes "JAVA AT 94%" | per-process telemetry — does not exist on this ship | UNSPEAKABLE (kept so the reason survives) |

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
never a quotation from the books or films. The pools per state are in "Line pool" above
(D-059).
