# Geometry contract — mockup.svg is the source of truth
Canvas 1080x1920. Content margins x=64..1016 (inner width 952).

## Colour tokens (these only)
bg radial #0d2331 -> #04070b | panel fill #08131c | panel stroke #153a48
inner panel #0b1a24 | bar track #12303c | dim dot #15323d
cyan ok #5fd8ef | gold warn #f0b429 | red danger #ff6a3d
text primary #cdeef7 | text secondary #6b9dad | labels #4d7f8f
phrase box fill #0c1620 stroke #3a2f14

## Font scale (chosen to read across a room — never shrink)
wordmark 78 ls14 | headline 132 | gpu total 104 | fahrenheit 42 | body 28 | label 23 ls3
dim 24 | small 19 | celsius 25 | core label 18
family: ui-monospace,"DejaVu Sans Mono",monospace

## Vertical map (top -> bottom)
title baseline 126 | rule y=170 | phrase 190-318 | process 338-808 | graphics 828-1178
memory|network 1198-1368 | storage I/O|space 1388-1558 | thermals 1578-1856
footer baseline 1900

## Processor box internals
title baseline 382 | "LAST 120 s" right aligned baseline 386
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

Columns are always 6. Row count and pitch:
  threads <= 6   -> 1 row, p=14
  threads <= 12  -> 2 rows, p=11   (current machine: 12 threads on a 6-core Ryzen 5 9600X)
  threads <= 18  -> 3 rows, p=7
  threads >  18  -> drop dots, render one thin horizontal bar per thread instead
Labels "C0 78%", font 18, centered at x = 155 + 154*i, baseline in the 24px gap below
each row (665 row A, 752 row B).

## Colour rules (implemented once in internal/model, never inline in templates)
per-thread util: <40 cyan | 40..70 gold | >70 red
big CPU number fill = CPU TEMP BAND, not utilisation:
  <60C cyan #5fd8ef | 60..90C gold #f0b429 | >=90C red #ff6a3d
thermal bars: width = C/100 * bar width; tick marks the device limit; amber over 70C
throughput bars MUST print their ceiling ("SCALE 200 MiB/s", "47% OF LINK").
Never render a normalised bar without its stated scale.

## Reviewer checklist — every diff
- tokens, coordinates, font sizes and lattice divisibility above still intact?
- does the headline number agree with the graph's "now" dot and the mean of the matrix?
- same device reported with the same F/C pair everywhere it appears (GPU in two boxes)?
- Fahrenheit primary (larger), Celsius secondary, everywhere.
- any invented value, zero-fill, or swallowed error? reject.
- new box never resized to fit new content; the content changes instead.
