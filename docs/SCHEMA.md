# Snapshot wire contract — `marvin/v1`

Every snapshot (fixture today, collector output from T4 on) is one JSON object served at
`GET /snapshot.json` with `Content-Type: application/json`. Fixtures live in `fixtures/`
and are produced only by `go run ./cmd/genfixtures -out fixtures`; never edit them by
hand. Regeneration must be byte-identical (fixed seeds, fixed `generated_at`).

Hardware values (CPU model, GPU UUIDs, devices, interface names, totals) come from
`docs/DISCOVERY.md`. Mood names and triggers come from `docs/MARVIN.md`. Colours and
geometry are NOT in this contract; see `docs/GEOMETRY.md`.

## Top level

| key | type | notes |
|---|---|---|
| `schema` | string | always exactly `"marvin/v1"` |
| `scenario` | string | `calm` \| `busy` \| `hot` \| `dying` (collectors emit `live`) |
| `mood` | string | `bored` \| `melancholic` \| `aggrieved` \| `doomed` (T9 refines) |
| `generated_at` | string | RFC 3339 UTC |
| `host` | object | see below |
| `cpu` | object | PROCESSOR box |
| `memory` | object | MEMORY box |
| `gpus` | array | GRAPHICS box, exactly 2 entries, real Nvidia cards only |
| `temps` | object | THERMALS entries not owned by cpu/gpus |
| `fans` | array | FAN BANK 1/2, exactly 2 entries |
| `network` | array | NETWORK box, one entry per real interface |
| `connections` | object | `{established: int}` from `ss -s` |
| `storage` | array | SPACE split row + STORAGE I/O, one entry per mount |

All snapshots have every key above. Nullability is per-field below.

## `host`
`hostname` string, `kernel` string, `uptime_seconds` int.

## `cpu`
`model` string (`/proc/cpuinfo` "model name"), `threads` int, `physical_cores` int,
`total_pct` float, `per_thread_pct` float[threads], `freq_ghz` float, `load1/5/15` float,
`iowait_pct` float, `temp_c` float, `band` string, `hist_pct` float[120] (1 Hz ring,
oldest first). First sample of a delta renders `null`, never 0.

## `memory` — bytes are ints; `used = MemTotal - MemAvailable`, `cache = Cached + SReclaimable`.
`total_bytes`, `used_bytes`, `cache_bytes`, `used_pct`, `swap_total_bytes`, `swap_used_bytes`.

## `gpus[]` — ONLY nvidia-smi, matched by UUID; never the amdgpu display adapter.
`name` string, `uuid` string (`GPU-<8-4-4-4-12 hex>`), `mem_total_mib`/`mem_used_mib` int,
`util_pct` float, `temp_c` float, `band` string, `power_w` float, `power_limit_w` float,
`sm_clock_mhz` int, `hist_util_pct` float[30]. A stale GPU sets its temps/util to `null`
and keeps rendering (T7).

## `temps` — CPU and GPU temps live inside `cpu` / `gpus`; this holds the rest.
`nvme_c` float, `nvme_band` string, `nvme_sensor` string (hwmon label that won),
`nvme_max_c` float (sensor's `temp1_max`).

## `fans[]` — absence is real telemetry (docs/DISCOVERY.md section 9).
`bank` int (1-based), `label` string, `rpm` int|null, `max_rpm` int|null, `verdict` string.

## `network[]`
`if` string, `rx_bps`/`tx_bps` int (bytes/sec, deltas), `link_mbps` int|null,
`link_known` bool (false ⇒ panel renders no "% OF LINK"), `rx_err`/`tx_err` int.

## `storage[]`
`device`, `mount`, `fs` strings, `total_bytes`/`used_bytes`/`free_bytes` ints,
`used_pct` float, `state` string, `read_bps`/`write_bps` ints, `read_iops`/`write_iops`/
`in_flight` ints.

## Invariants (enforced by `cmd/genfixtures` tests; T3 reuses them on live data)

1. `schema == "marvin/v1"` and the top-level key set is identical across snapshots.
2. Celsius only on the wire. Fahrenheit is never stored or transmitted; render it as
   `round(C*9/5+32)` at display time (docs/DATA.md).
3. Temp `band`: `ok` < 60 ≤ `warn` < 90 ≤ `danger`, on every `temp_c`/`nvme_c`.
4. Mount `state`: `ok` < 80 ≤ `warn` < 90 ≤ `danger` on `used_pct`.
5. Fan `verdict`: `rpm == null` → `unknown`; `rpm == 0` → `stalled`;
   `rpm ≥ 0.99·max_rpm` and CPU `band != "ok"` → `not_cooling`; else `ok`.
6. All percentages are floats clamped to 0–100 and rounded to 1 decimal. Null means
   "no telemetry" and must never be replaced by 0 (docs/HANDOFF.md hard constraint).

## FIXTURE MATRIX (mood per scenario, trigger per docs/MARVIN.md)

| scenario | mood | trigger held | notable |
|---|---|---|---|
| calm | bored | load/threads < 0.02 | real idle GPU values from DISCOVERY.md; fans null |
| busy | melancholic | load/threads > 0.85 | 118 MiB/s inbound; CPU 67.4 °C warn |
| hot | aggrieved | temp > 80 °C and iowait > 15 % | GPU1 82.1 °C; hogdata 82 % warn |
| dying | doomed | disk < 5 % free and temp ≥ 90 °C | hogdata 97.4 %; CPU 93.7 °C; fans pinned → not_cooling |
