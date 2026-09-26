# Snapshot wire contract — `marvin/v1`

Every snapshot (fixture today, collector output from T4 on) is one JSON object served at
`GET /snapshot.json` with `Content-Type: application/json`. Fixtures live in `fixtures/`
and are produced only by `go run ./cmd/genfixtures -out fixtures`; never edit them by
hand. Regeneration must be byte-identical (fixed seeds, fixed `generated_at`).

Amended 2026-09-26 by D-016..D-019, D-027, D-034, D-035 and D-037; T2b implements the
amendments in cmd/genfixtures. The version string stays "marvin/v1": nothing consumes the
contract yet.

Hardware values (CPU model, GPU UUIDs, devices, interface names, totals) come from
`docs/DISCOVERY.md` and the 2026-09-26 verification in HANDOFF.md. Mood names and
triggers come from `docs/MARVIN.md`. Colors and geometry are NOT in this contract; see
`docs/GEOMETRY.md`.

## Top level

| key | type | notes |
|---|---|---|
| `schema` | string | always exactly `"marvin/v1"` |
| `scenario` | string | `calm` \| `busy` \| `hot` \| `dying` (collectors emit `live`) |
| `mood` | string | `bored` \| `content` \| `melancholic` \| `aggrieved` \| `doomed` |
| `generated_at` | string | RFC 3339 UTC |
| `host` | object | see below |
| `phrase` | object | phrase box text |
| `gpu_line` | string | GRAPHICS box Marvin line (D-037) |
| `cpu` | object | PROCESSOR box |
| `memory` | object | MEMORY cell |
| `gpus` | array | GRAPHICS box; exactly 2 entries, real Nvidia cards only |
| `temps` | object | THERMALS entries not owned by cpu/gpus |
| `fans` | array | FAN BANK 1/2; exactly 2 entries |
| `network` | array | NETWORK cell; exactly 1 entry |
| `connections` | object | `{established: int}` from `ss -s` |
| `disk_io` | object | STORAGE · I/O cell (physical NVMe) |
| `storage` | array | SPACE cell; exactly 3 entries |
| `smart` | object | SPACE header SMART status |
| `panic_count` | int\|null | footer PANIC COUNT (D-017) |

All snapshots have every key above. Nullability is per field below. null always means
"no telemetry" and is never replaced by 0.

## `host`
`hostname` string, `kernel` string, `uptime_seconds` int.

## `phrase`
`lines` string[1..2]. Each line <= 52 characters; total <= 100 characters. marvind
chooses and wraps the line (it owns phrase choice); the browser never wraps or truncates.
The attribution line "— MARVIN, ON BOARD SINCE LAUNCH" is static in the SVG.

## `gpu_line` (D-037)
string, 1-38 characters, chosen by marvind from GPU state (docs/MARVIN.md "GPU
line"). Always present: when the GPUs are stale it is a stale-state line.

## `cpu`
`model` string (raw `/proc/cpuinfo` "model name"), `model_display` string (D-020, e.g.
"AMD RYZEN 5 9600X"), `threads` int, `physical_cores` int, `total_pct` float|null,
`per_thread_pct` (float|null)[threads], `freq_ghz` float, `load1`/`load5`/`load15` float
(mood input only; not displayed, D-017), `iowait_pct` float|null, `temp_c` float|null,
`band` string (PROCESSOR big-number band, 60/90), `thermal_band` string (THERMALS band,
70/90), `hist_pct` float[120] (1 Hz ring, oldest first). The first sample of any delta is
null, never 0.

## `memory`
Bytes are ints. `total_bytes`; `used_bytes` = MemTotal - MemAvailable; `cache_bytes` =
Cached + SReclaimable - Shmem, clamped to [0, total_bytes - used_bytes]; `used_pct` =
used/total; `swap_total_bytes`, `swap_used_bytes`. The panel draws used and cache as a
stacked bar (D-034); the percentage shown is `used_pct` only.

## `gpus[]` — ONLY nvidia-smi, matched by UUID; never the amdgpu display adapter
`name` string (raw), `display_name` string (D-020, e.g. "RTX 3060"), `uuid` string
(`GPU-<8-4-4-4-12 hex>`), `mem_total_mib`/`mem_used_mib` int, `util_pct` float|null,
`temp_c` float|null, `thermal_band` string, `power_w` float|null, `power_limit_w` float,
`hist_util_pct` float[30]. A stale GPU (T7) sets its temp, util and power to null and
keeps rendering.

## `temps` — CPU and GPU temps live inside `cpu` / `gpus`; this holds the rest
`nvme_c` float|null, `nvme_thermal_band` string, `nvme_sensor` string (hwmon label that
won), `nvme_max_c` float (the sensor's `temp1_max`).

## `fans[]` — absence is real telemetry (D-027)
`bank` int (1-based), `label` string, `rpm` int|null, `max_rpm` int|null, `verdict`
string. Until O2 decides, every fan has `rpm` null, `max_rpm` null, `verdict` "unknown".

## `network[]` — exactly one entry (D-018)
`if` string, always "wlp14s0"; `rx_bps`/`tx_bps` int|null (bytes/sec deltas);
`rx_err`/`tx_err` int. There are no link fields: Wi-Fi has no fixed ceiling. The bar
scale S is a panel constant in docs/GEOMETRY.md.

## `disk_io` — the physical NVMe (D-035)
`device` string, always "nvme0n1"; `read_bps`/`write_bps` int|null;
`read_iops`/`write_iops` int|null; `queue_avg` float|null (delta of /proc/diskstats
weighted time-in-queue ms / delta of elapsed ms); `in_flight` int. Per-mount I/O is not
reported: summing LVM volumes double-counts against the physical disk.

## `storage[]` — exactly three entries, in this order: "/", "/srv/hogdata", "/boot" (D-019)
`device`, `mount`, `fs` strings; `total_bytes`/`used_bytes`/`free_bytes` ints;
`used_pct` float; `state` string.

## `smart` — from the root-owned handoff /var/lib/marvin/smart.json (D-012)
`state` string: `ok` | `failing` | `unknown`. `unknown` when the file is missing,
unparsable, or older than 60 minutes; `failing` when `smart_status.passed` is false.
`percentage_used` int|null, `unsafe_shutdowns` int|null, `age_seconds` int|null.
serial_number, wwn and uuid are NEVER copied onto the wire, into fixtures or into logs.

## `panic_count` (D-017)
Lifetime count of confirmed entries into mood "doomed", read from /var/lib/heartofgold/.
null when the state file is missing or unreadable.

## Invariants (enforced by `cmd/genfixtures` tests; T3 reuses them on live data)
1. `schema == "marvin/v1"` and the top-level key set is identical across snapshots.
2. Celsius only on the wire. Fahrenheit is never stored or transmitted; it is rendered as
   `round(C*9/5+32)` at display time (docs/DATA.md).
3. `cpu.band`: `ok` < 60 <= `warn` < 90 <= `danger`.
4. `thermal_band` on every thermal reading (`cpu`, each `gpus[]`, `temps.nvme_*`):
   `ok` < 70 <= `warn` < 90 <= `danger`.
5. Mount `state`: `ok` < 80 <= `warn` < 90 <= `danger` on `used_pct`.
6. Fan `verdict`: `rpm == null` -> `unknown`; `rpm == 0` -> `stalled`;
   `rpm >= 0.99*max_rpm` and `cpu.thermal_band != "ok"` -> `not_cooling`; else `ok`.
7. All percentages are floats clamped to 0-100 and rounded to 1 decimal.
8. Cardinality: `gpus` 2, `fans` 2, `network` 1, `storage` 3 in the stated order.
9. `memory.used_bytes + memory.cache_bytes <= memory.total_bytes`.
10. `phrase.lines`: 1-2 lines, each <= 52 characters, total <= 100.
11. `panic_count` is null or >= 0. Null is never replaced by 0 anywhere.
12. `gpu_line` is 1-38 characters.

## Fixture matrix (mood per scenario, trigger per docs/MARVIN.md)

| scenario | mood | trigger held | notable |
|---|---|---|---|
| calm | bored | load/threads < 0.02 | real idle GPU values; fans null; panic_count 0 |
| busy | melancholic | load/threads > 0.85 | 118 MiB/s inbound on wlp14s0; CPU 67.4 °C (cpu.band warn, thermal_band ok) |
| hot | aggrieved | temp > 80 °C and iowait > 15 % | GPU1 82.1 °C; /srv/hogdata 82 % warn |
| dying | doomed | disk < 5 % free and temp >= 90 °C | /srv/hogdata 97.4 %; CPU 93.7 °C; panic_count > 0 |
Every fixture's `phrase.lines` is a docs/MARVIN.md seed line for its mood that is
speakable from that fixture's data.
