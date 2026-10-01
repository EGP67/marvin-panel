# Geometry contract — mockup.svg is the source of truth
Canvas 1080x1920. Content margins x=64..1016 (inner width 952).
Changes to the picture, mockup.svg or this file require an owner-approved decision.

## Color tokens (these only)
bg radial #0d2331 -> #04070b | panel fill #08131c | panel stroke #153a48
inner panel #0b1a24 | bar track #12303c | dim dot #15323d
cyan ok #5fd8ef | gold/amber warn #f0b429 | red danger #ff6a3d
bar gradient #1c6f83 -> #5fd8ef | memory cache segment #1c6f83 (D-034)
text primary #cdeef7 | text secondary #6b9dad | labels #4d7f8f
phrase box fill #0c1620 stroke #3a2f14

## Font scale (never shrink)
wordmark 78 ls14 | headline 132 | gpu total 104 | fahrenheit 42 | body 28 | label 23 ls3
dim 24 | small 19 | celsius 25 | core label 18
family: ui-monospace,"DejaVu Sans Mono",monospace
Advance 0.602 em (DejaVu Sans Mono, resolved on hog). Every px/char budget in this file
assumes it (D-049).

## Vertical map (top -> bottom)
title baseline 126 | rule y=170 | phrase 190-318 | processor 338-808 | graphics 828-1178
memory|network 1198-1368 | storage I/O|space 1388-1558 | thermals 1578-1856
footer baseline 1900

## Processor box internals
title "PROCESSOR: " + cpu.model_display (D-020), baseline 382 | "LAST 120 s" right
aligned baseline 386
big number baseline 500 at x=96 | "n GHZ | TEMP n°F / m°C" baseline 548
graph plot area x=356..986, y=398 (=100%) .. 494 (=0%); axis labels anchor=end x=344
core boxes: 6 columns, x = 88 + 154*i, w=134; row A y=584 h=64, row B y=672 h=64
dot region inside a box: x=box+5 w=124, y=box+4 h=56

## CPU CORE DOT MATRIX — DO NOT SIMPLIFY OR RESTRUCTURE
One global SVG pattern dot lattice plus one mask. Not per-core geometry.
Dot pitch p must satisfy BOTH divisibility constraints:
  horizontal core pitch 154 % p == 0
  vertical row pitch     88 % p == 0        (2-row case -> p=11)
Fill heights are integer multiples of p measured up from the region bottom
(row A bottom 644, row B bottom 732). This is what guarantees whole unclipped dots.

Columns are always 6. There is exactly ONE layout:
  12 threads -> 2 rows x 6, p=11   (this ship: 12 threads on a 6-core Ryzen 5 9600X)
Above 12 threads marvind refuses to start, with a visible message naming the thread
count (D-006). Headline/graph/matrix agreement tolerance: +/- 12 points, because p=11 in
a 56px region quantizes fills to 20% steps.
Labels "C0 78%", font 18, centered at x = 155 + 154*i, baseline in the 24px gap below
each row (665 row A, 752 row B).

## Color rules (implemented once in internal/model, never inline in templates)
Two temperature functions (D-016):
  CPU band — PROCESSOR big number fill only:
    <60C cyan #5fd8ef | 60..<90C gold #f0b429 | >=90C red #ff6a3d
  Thermal band — THERMALS values and bars:
    <70C cyan #5fd8ef | 70..<90C amber #f0b429 | >=90C red #ff6a3d
  The THERMALS header "| = LIMIT · AMBER OVER 70°C" stays as drawn.
  The same temperature may color differently in PROCESSOR and THERMALS; this is accepted.
per-thread utilization: <40 cyan | 40..70 gold | >70 red. The colors come from
cpu.per_thread_sev and the GPU card percentages from gpus[].util_sev (ok, warn, danger;
D-050); the browser never computes severity.
Processor graph: null hist_pct points are not drawn.
thermal bars: width = C/100 * bar width; the tick marks the device limit
(NVMe: temp1_max 83.85 C, x=947).
Throughput bars always print their scale: STORAGE "SCALE 200 MiB/s" (D-021); NETWORK
"<n>% OF <S> MiB/s" (D-018; S set from T5 measurements). Never "% OF LINK".
Never render a normalized bar without its stated scale.

## GRAPHICS box
Big number = mean of the two cards' util_pct. "VRAM used / total GiB" and
"POWER draw / limit W" sum both cards. The third text line (x=340, y=994) is gpu_line:
class quip at 28px (italic gold #f0b429), <= 38 characters (D-037).
Card headers "GPU0 · RTX 3060" / "GPU1 · RTX 3060" from gpus[].display_name (D-020).

## MEMORY cell (D-034)
Track x=96 y=1250 w=408 h=22 rx=11 fill #12303c.
Cache segment (underneath): x=96, width = (used+cache)/total * 408, rx=11, fill #1c6f83.
Used segment (on top): x=96, width = used/total * 408, rx=11, fill url(#bar).
Percentage text (right, "34%") = used_pct only. Detail line format as drawn:
"<used> / <total> GiB · CACHE <cache>", total as a whole number
("42.0 / 123 GiB · CACHE 62.0", D-037). SWAP line as drawn.

## NETWORK cell (D-018, D-046, D-049)
Header "WIFI". Counters "CON n · ERR n", right-aligned at x=976 (wire field
connections.established). RX/TX right labels "<n>% OF <S> MiB/s". S = 40 MiB/s until T5
measurements (D-037). Label fit is confirmed in the T10 Brave screenshot.

## STORAGE · I/O cell (D-021, D-049)
Header "STORAGE · I/O" and "SCALE 200 MiB/s" as drawn; their ~2 px clearance is accepted by
the owner (D-049).

## SPACE cell (D-019, D-049)
Rows "/", "/srv/hogdata", "/boot" at baselines 1466, 1506, 1546; bars (track and fill)
x=732 w=180 h=16 rx=8; right text "<pct>%" only, right-aligned at x=976. Fit at 22px:
"/srv/hogdata" ends at x≈719, 13 px before the bar; "100%" starts at x≈926. Used/total sizes
are not shown (Phase 2 Storage view). Header right text "IOWAIT n% · SMART <s>", where <s> is
OK, FAIL or -- (unknown).

## Footer budget (D-017)
37 characters maximum at 23px ls3 (16.85 px/char); must end before x=727
("DON'T PANIC" occupies 751..992).
Format: "UPTIME 41d 07:12:03 · PANIC COUNT 0" (35 characters).
panic_count null renders "--". A count above 99 renders "99+".

## Model-string shortening (D-020)
CPU: uppercase, strip the trailing " 6-Core Processor" -> "AMD RYZEN 5 9600X".
GPU: strip "NVIDIA GeForce " -> "RTX 3060", shown as "GPU0 · RTX 3060".

## Phrase box budget
<=100 characters, wrapped to 2 lines of <=52 at 30px (18.06 px/char). The phrase
engine rejects an over-budget line; the browser never truncates.

## Reviewer checklist — every diff
- Tokens, coordinates, font sizes and lattice divisibility above still intact?
- Does the headline number agree with the graph's "now" dot and the mean of the matrix?
- Same device reported with the same F/C pair everywhere it appears (GPU in two boxes)?
- Fahrenheit primary (larger), Celsius secondary, everywhere.
- Any invented value, zero-fill, or swallowed error? Reject.
- A box is never resized to fit new content; the content changes instead.
