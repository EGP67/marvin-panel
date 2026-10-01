# Geometry contract — mockup.svg is the source of truth
Canvas 1080x1920. Content margins x=24..1056 (inner width 1032), Phase 1 Wide (D-060).
Horizontal map from Phase 1 Thin: x' = 24 + (x - 64) x 1032/952, both ends mapped, 0.1 px.
Changes to the picture, mockup.svg or this file require an owner-approved decision.

## Color tokens (these only)
bg radial #0d2331 -> #04070b | panel fill #08131c | panel stroke #153a48
inner panel #0b1a24 | bar track #12303c | dim dot #15323d
cyan ok #5fd8ef | gold/amber warn #f0b429 | red danger #ff6a3d
bar gradient #1c6f83 -> #5fd8ef | memory cache segment #1c6f83 (D-034)
text primary #cdeef7 | text secondary #6b9dad | labels #4d7f8f
section headers (class hdr) #9fd4e3, as bright as the PROCESSOR title (D-060)
phrase box fill #0c1620 stroke #3a2f14

## Font scale (never shrink)
wordmark 78 ls14 | headline 132 | gpu total 104 | fahrenheit 42 | body 28 | label 23 ls3
header (hdr) 23 ls3: GRAPHICS, MEMORY, WIFI, STORAGE · I/O, SPACE, THERMALS and the THERMALS
sub-headers CPU, GPU0, GPU1, NVME M.2, FAN BANK 1, FAN BANK 2 (D-060); TOTAL GPU
UTILIZATION, the limit legend and the footer stay label (lbl)
dim 24 | small 19 | celsius 25 | core label 18
family: ui-monospace,"DejaVu Sans Mono",monospace
Advance 0.602 em (DejaVu Sans Mono, resolved on hog). Every px/char budget in this file
assumes it (D-049).
A fill attribute always wins over the stylesheet (D-051).

## Display mapping (D-054)
Native 1024x600 rotated right (screen 600x1024). The page fits the design uniformly
through the SVG viewBox: the top 24 design px are cropped (D-054 note), the height fills, and the
background and glass rects widen to the window (no black bars, never stretched). All
coordinates in this file stay design units.

## Vertical map (top -> bottom)
title baseline 126 | rule y=170 | phrase 190-318 | processor 338-808 | graphics 828-1178
memory|network 1198-1368 | storage I/O|space 1388-1558 | thermals 1578-1856
footer baseline 1900

## Processor box internals
title "PROCESSOR: " + cpu.model_display (D-020), baseline 382 | "LAST 120 s" right
aligned baseline 386
big number baseline 500 at x=58.7 | "n GHZ | TEMP n°F / m°C" baseline 548
graph plot area x=340.5..1023.5, y=398 (=100%) .. 494 (=0%); axis labels anchor=end
x=327.5. app.js reads the plot range from the 100% gridline (D-060).
big number at three digits ("100"): the "%" tspan and the three axis labels are hidden,
restored below 100 (D-051)
PEAK label at (cx+14, cy-7), anchor end; below its point at cy+21 when cy-7 would be above
y=410, clear of "LAST 120 s" (D-051)
PEAK label right edge = max(cx+14, 464.1): the Thin threshold 470 carried through the
D-060 map (left-edge companion to D-051 (3)).
core boxes: 6 columns, x = 60 + 165*i, w=134 (D-060); row A y=584 h=64, row B y=672 h=64
dot region inside a box: x=box+5 w=124, y=box+4 h=56

## CPU CORE DOT MATRIX — DO NOT SIMPLIFY OR RESTRUCTURE
One global SVG pattern dot lattice plus one mask. Not per-core geometry.
Dot pitch p must satisfy BOTH divisibility constraints:
  horizontal core pitch 165 % p == 0      (D-060; was 154)
  vertical row pitch     88 % p == 0        (2-row case -> p=11)
Fill heights are integer multiples of p measured up from the region bottom
(row A bottom 644, row B bottom 732). This is what guarantees whole unclipped dots.
The lattice is phased so every dot region starts on a cell: pattern translate x =
(x0 + 5) mod 11 = 10 with x0 = 60; mask rect x=60 w=959 (5*165 + 134), centered on 540.

Columns are always 6. There is exactly ONE layout:
  12 threads -> 2 rows x 6, p=11   (this ship: 12 threads on a 6-core Ryzen 5 9600X)
Above 12 threads marvind refuses to start, with a visible message naming the thread
count (D-006). Headline/graph/matrix agreement tolerance: +/- 12 points, because p=11 in
a 56px region quantizes fills to 20% steps.
Labels "C0 78%", font 18, centered at x = 127 + 165*i, baseline in the 24px gap below
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
Processor graph: null hist_pct points are not drawn; the line, area, PEAK and "now"
markers draw the most recent contiguous non-null run only (hidden when there is none).
Fans: rpm null renders the label "FAN BANK n: NO TELEMETRY" and "--" in the RPM slot,
bar width 0 (a 28px "NO TELEMETRY" in the RPM slot would overlap bank 2 and pass the box
edge).
thermal bars: width = C/100 * bar width; the tick marks the device limit
(NVMe: temp1_max 83.85 C, x=981.2).
Throughput bars always print their scale: STORAGE "SCALE 200 MiB/s" (D-021); NETWORK
"<n>% OF <S> MiB/s" (D-018; S set from T5 measurements). Never "% OF LINK".
Never render a normalized bar without its stated scale.

## GRAPHICS box
Big number = mean of the two cards' util_pct. "VRAM used / total GiB" and
"POWER draw / limit W" sum both cards. The third text line (x=323.2, y=994) is gpu_line:
class quip at 28px (italic gold #f0b429), <= 38 characters (D-037).
Card headers "GPU0 · RTX 3060" / "GPU1 · RTX 3060" from gpus[].display_name (D-020).
Total bar track x=58.7 w=971.3; cards x=50 / 553 w=477; sparkline baselines x=67.4..509.6
and 570.4..1012.6 (app.js reads them, D-060).

## MEMORY cell (D-034)
Track x=58.7 y=1250 w=442.3 h=22 rx=11 fill #12303c (D-060).
Cache segment (underneath): x=58.7, width = (used+cache)/total * track width, rx=11, fill
#1c6f83.
Used segment (on top): x=58.7, width = used/total * track width, rx=11, fill url(#bar).
Percentage text (right, "34%") = used_pct only. Detail line format as drawn:
"<used> / <total> GiB · CACHE <cache>", total as a whole number
("42.0 / 123 GiB · CACHE 62.0", D-037). SWAP line as drawn; the quip
" — SPARE, UNLIKE ME" appears only at 0%.

## NETWORK cell (D-018, D-046, D-049)
Header "WIFI" at x=561.7. Counters "CON n · ERR n", right-aligned at x=1012.6 (wire field
connections.established). RX/TX right labels "<n>% OF <S> MiB/s". S = 40 MiB/s until T5
measurements (D-037). Label fit is confirmed in the T10 Brave screenshot. The "% OF S"
text stays unbounded while its bar clamps; revisit when T5 sets S (D-051). RX/TX tracks
x=561.7 w=450.9.

## STORAGE · I/O cell (D-021, D-049, D-060)
Header "STORAGE · I/O" (x=58.7, ends x≈277.7) and "SCALE 200 MiB/s" (right-aligned at
x=501, starts x≈314.4): about 37 px clear, the D-049 (3) crowding is resolved by D-060.
READ/WRITE tracks x=58.7 w=442.3. No IOPS/QUEUE line (D-056).

## SPACE cell (D-019, D-049)
Rows "/", "/srv/hogdata", "/boot" (labels x=561.7) at baselines 1466, 1506, 1546; bars
(track and fill) x=748.1 w=195.2 h=16 rx=8; right text "<pct>%" only, right-aligned at
x=1012.6. Fit at 22px: "/srv/hogdata" ends at x≈720.6, 27.5 px before the bar; "100%"
starts at x≈962.9, 19.6 px after the bar ends (943.3) (D-060). Used/total sizes
are not shown (Phase 2 Storage view). Header right text "IOWAIT n% · SMART <s>", where <s> is
OK, FAIL or -- (unknown).

## Footer budget (D-017)
42 characters maximum at 23px ls3 (16.85 px/char) from x=58.7: the 42nd glyph ends at
x≈763.4, before "DON'T PANIC" (765.3..1030.0) (D-060).
Format: "UPTIME 41d 07:12:03 · PANIC COUNT 0" (35 characters).
panic_count null renders "--". A count above 99 renders "99+".
"UPTIME 100d … · PANIC COUNT 99+" (38 characters, ends x≈699) fits with about 66 px to
spare (D-051 (4), D-060).

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
