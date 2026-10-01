# Snapshot wire contract — `marvin/v1`

Every snapshot (fixture today, collector output from T4 on) is one JSON object served at
`GET /snapshot.json` with `Content-Type: application/json`. Fixtures live in `fixtures/`
and are produced only by `go run ./cmd/genfixtures -out fixtures`; never edit them by
hand. Regeneration must be byte-identical (fixed seeds, fixed `generated_at`).

Amended 2026-09-26 by D-016..D-019, D-027, D-034, D-035 and D-037. Implemented in
cmd/genfixtures by T2b (155dabb; disk totals a4fac41). The version string stays
"marvin/v1": nothing consumes the contract yet.

Hardware values (CPU model, GPU UUIDs, devices, interface names, totals) come from
`docs/DISCOVERY.md` and the 2026-09-26 verification in HANDOFF.md. Mood names and
triggers come from `docs/MARVIN.md`. Colors and geometry are NOT in this contract; see
`docs/GEOMETRY.md`.

## Top level

| key | type | notes |
|---|---|---|
| `schema` | string | always exactly `"marvin/v1"` |
| `scenario` | string | `calm` \| `busy` \| `hot` \| `dying` \| `startup` (collectors emit `live`) |
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
| `connections` | object | `{established: int\|null}`: TCP state 01 (ESTABLISHED) in /proc/net/tcp and tcp6 (D-036); displayed as "CON n" (D-049) |
| `disk_io` | object | STORAGE · I/O cell (physical NVMe) |
| `storage` | array | SPACE cell; exactly 3 entries |
| `smart` | object | SPACE header SMART status |
| `panic_count` | int\|null | footer PANIC COUNT (D-017) |

All snapshots have every key above. null always means "no telemetry" and is never
replaced by 0.

## Nullability (D-050)
Every value measured at runtime, or derived from one, is nullable and is null when its
source fails; a derived field (band, state, severity, used_pct) is null when its input is
null. Identity and constants are non-null: `schema`, `scenario`, `generated_at`,
`host.hostname`, `host.kernel`, `cpu.model`, `cpu.model_display`, `cpu.threads`,
`cpu.physical_cores`, `gpus[].name`, `display_name`, `uuid`, `network[].if`,
`disk_io.device`, `storage[].device`, `mount`, `fs`, `fans[].bank`, `label`.
Engine-chosen fields always carry a value because each has an explicit unknown or stale
form: `mood`, `phrase.lines`, `gpu_line`, `fans[].verdict`, `smart.state`. Arrays are
never null; only their entries may be, where the type below says so.

## `host`
`hostname` string, `kernel` string, `uptime_seconds` int|null.

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
`per_thread_pct` (float|null)[threads], `per_thread_sev` (string|null)[threads] (`ok`,
`warn` or `danger` from model.Severity of the matching `per_thread_pct` entry, D-050),
`freq_ghz` float|null, `load1`/`load5`/`load15` float|null (mood input only; not
displayed, D-017), `iowait_pct` float|null, `temp_c` float|null, `band` string|null
(PROCESSOR big-number band, 60/90), `thermal_band` string|null (THERMALS band, 70/90),
`hist_pct` (float|null)[120] (1 Hz ring, oldest first; a null entry means no sample for
that second, so the ring has leading nulls until it fills, D-050). The first sample of any
delta is null, never 0.

## `memory`
Bytes are ints; every field is nullable. `total_bytes` int|null; `used_bytes` int|null =
MemTotal - MemAvailable; `cache_bytes` int|null = Cached + SReclaimable - Shmem, clamped to
[0, total_bytes - used_bytes]; `used_pct` float|null = used/total; `swap_total_bytes`,
`swap_used_bytes` int|null. The panel draws used and cache as a stacked bar (D-034); the
percentage shown is `used_pct` only.

## `gpus[]` — ONLY nvidia-smi, matched by UUID; never the amdgpu display adapter
`name` string (raw), `display_name` string (D-020, e.g. "RTX 3060"), `uuid` string
(`GPU-<8-4-4-4-12 hex>`), `mem_total_mib`/`mem_used_mib` int|null, `util_pct` float|null,
`util_sev` string|null (model.Severity of `util_pct`, D-050), `temp_c` float|null,
`thermal_band` string|null, `power_w` float|null, `power_limit_w` float|null,
`hist_util_pct` (float|null)[30] (oldest first; null entry = no sample, D-050). A stale GPU
(T7) sets `mem_used_mib`, temp, util and power (and their derived band and severity) to
null and keeps rendering; `mem_total_mib` and `power_limit_w` are null until the first
successful read.

## `temps` — CPU and GPU temps live inside `cpu` / `gpus`; this holds the rest
`nvme_c` float|null, `nvme_thermal_band` string|null, `nvme_sensor` string|null (hwmon
label that won), `nvme_max_c` float|null (the sensor's `temp1_max`).

## `fans[]` — absence is real telemetry (D-027)
`bank` int (1-based), `label` string, `rpm` int|null, `max_rpm` int|null, `verdict`
string. Until O2 decides, every fan has `rpm` null, `max_rpm` null, `verdict` "unknown".

## `network[]` — exactly one entry (D-018)
`if` string, always "wlp14s0"; `rx_bps`/`tx_bps` int|null (bytes/sec deltas);
`rx_err`/`tx_err` int|null. There are no link fields: Wi-Fi has no fixed ceiling. The bar
scale S is a panel constant in docs/GEOMETRY.md.

## `disk_io` — the physical NVMe (D-035, D-056)
`device` string, always "nvme0n1"; `read_bps`/`write_bps` int|null. IOPS, queue and
in-flight counts are not on the wire (D-056). Per-mount I/O is not reported: summing LVM
volumes double-counts against the physical disk.

## `storage[]` — exactly three entries, in this order: "/", "/srv/hogdata", "/boot" (D-019)
`device`, `mount`, `fs` strings; `total_bytes`/`used_bytes`/`free_bytes` int|null;
`used_pct` float|null; `state` string|null.

## `smart` — from the root-owned handoff /var/lib/marvin/smart.json (D-012)
`state` string: `ok` | `failing` | `unknown`. `unknown` when the file is missing,
unparsable, or older than 60 minutes; `failing` when `smart_status.passed` is false.
`percentage_used` int|null, `unsafe_shutdowns` int|null, `age_seconds` int|null.
serial_number, wwn and uuid are NEVER copied onto the wire, into fixtures or into logs.

## `panic_count` (D-017)
Lifetime count of confirmed entries into mood "doomed", read from /var/lib/heartofgold/.
null when the state file is missing or unreadable.

## Invariants (enforced by `cmd/genfixtures` and `internal/model` tests; T3 reuses them on live data)
1. `schema == "marvin/v1"` and the top-level key set is identical across snapshots.
2. Celsius only on the wire. Fahrenheit is never stored or transmitted; it is rendered as
   `round(C*9/5+32)` at display time (docs/DATA.md).
3. `cpu.band`: `ok` < 60 <= `warn` < 90 <= `danger` when `cpu.temp_c` is non-null; null
   when it is null.
4. `thermal_band` on every thermal reading (`cpu`, each `gpus[]`, `temps.nvme_*`):
   `ok` < 70 <= `warn` < 90 <= `danger` when the temperature is non-null; null when it is
   null.
5. Mount `state`: `ok` < 80 <= `warn` < 90 <= `danger` on `used_pct` when it is non-null;
   null when it is null.
6. Fan `verdict`: `rpm == null` -> `unknown`; `rpm == 0` -> `stalled`;
   `rpm >= 0.99*max_rpm` and `cpu.thermal_band` non-null and `!= "ok"` -> `not_cooling`;
   else `ok`.
7. All non-null percentages are floats clamped to 0-100 and rounded to 1 decimal.
8. Cardinality: `gpus` 2, `fans` 2, `network` 1, `storage` 3 in the stated order.
9. `memory.used_bytes + memory.cache_bytes <= memory.total_bytes` when all three are
   non-null.
10. `phrase.lines`: 1-2 lines, each <= 52 characters, total <= 100.
11. `panic_count` is null or >= 0. Null is never replaced by 0 anywhere.
12. `gpu_line` is 1-38 characters.
13. Severity fields (`cpu.per_thread_sev[i]`, `gpus[].util_sev`) equal model.Severity of
    their percentage (`ok` < 40 <= `warn` <= 70 < `danger`); null when it is null.
14. History lengths are fixed: `cpu.hist_pct` 120, `gpus[].hist_util_pct` 30.
15. Agreement (D-050, D-006): `per_thread_pct` and `per_thread_sev` have `threads`
    entries; when `cpu.total_pct` is non-null, the last `hist_pct` entry equals it and the
    mean of the non-null `per_thread_pct` entries is within 12 points of it.

## Fixture matrix (mood per scenario, trigger per docs/MARVIN.md)

| scenario | mood | trigger held | notable |
|---|---|---|---|
| calm | bored | load/threads < 0.02 | real idle GPU values; fans null; panic_count 0 |
| busy | melancholic | load/threads > 0.85 | 118 MiB/s inbound on wlp14s0; CPU 67.4 °C (cpu.band warn, thermal_band ok) |
| hot | aggrieved | temp > 80 °C and iowait > 15 % | GPU1 82.1 °C; /srv/hogdata 82 % warn |
| dying | doomed | disk < 5 % free and temp >= 90 °C | /srv/hogdata 97.4 %; CPU 93.7 °C; panic_count > 0 |
| startup | content | first tick, no trigger held | deltas, histories, CPU/NVMe temps, both GPUs null; SMART unknown; panic_count null |
Every fixture's `phrase.lines` is a docs/MARVIN.md seed line that is speakable from that
fixture's data: the seed line for its mood in calm, busy, hot and dying, and the fans
line ("FAN BANK 1: NO TELEMETRY", fans[0].rpm null) in startup.
