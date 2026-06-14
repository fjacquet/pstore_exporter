# Grafana Dashboards Rework — Design

**Date:** 2026-06-14
**Status:** Approved (design); pending implementation
**Approach:** Hand-authored JSON (Option A) — restructured folders + one shared design
system applied across all dashboards. No new toolchain. Plain JSON preserves the
Grafana UI export/import round-trip.

## Goal

Make the bundled PowerStore Grafana dashboards **crispy, pro, focused, and logical**:
consistent visual language, cross-dashboard navigation, a health-first landing page,
and alert-ready thresholds — while filling the gaps where emitted metrics have no
visualization.

## Scope

- Restyle the existing 6 dashboards for consistency and correct metric usage.
- Add 4 new dashboards covering metric families with no current visualization:
  fleet health, volume groups, replication/DR, and drives/hardware health.
- Add file-system **performance** panels (metrics emitted, currently unused).
- Reorganize folders and reduce docker-compose mounts to one.
- Update `docs/metrics.md` PromQL examples and add an ADR recording the decision.

Out of scope: jsonnet/grafonnet generation (Option B), a CI validation script
(Option C) — both noted as possible later follow-ups; no new exporter metrics.

## Constraints (verified against the repo)

- Provisioning uses `foldersFromFilesStructure: true`
  (`grafana/provisioning/dashboards/dashboards.yml`) → each subdirectory under the
  mounted dashboards path becomes a Grafana folder. `allowUiUpdates: true`.
- `docker-compose.yml` and `docker-compose.ghcr.yml` currently mount
  `./grafana/block` and `./grafana/file` separately into
  `/var/lib/grafana/dashboards/...`. Grafana image is `grafana/grafana:latest`;
  dashboards are `schemaVersion: 39` (dashboard links + data links available).
- All metrics are **gauges** prefixed `powerstore_`, carry `array` + `cluster_id`.
  Values are per-second / instantaneous — aggregate with `sum`/`avg`, **never**
  `rate()` (per `CLAUDE.md` and ADR set).

## Folder taxonomy

Move all dashboards under a single `grafana/dashboards/` parent so compose needs one
mount and new folders are free:

```
grafana/dashboards/
  overview/    00-fleet-health.json      (NEW — landing page / hub)
               01-cluster-overview.json  (refined)
  block/       02-appliances.json        (refined)
               03-volumes.json           (refined)
               04-volume-groups.json     (NEW)
               05-capacity.json          (refined — cluster_* rollup)
               06-ports.json             (refined)
  file/        01-file-systems.json      (refined — capacity + performance rows)
  protection/  01-replication.json       (NEW — DR/replication)
  hardware/    01-drives.json            (NEW — drive health + wear + alerts)
```

**Compose change** (both `docker-compose.yml` and `docker-compose.ghcr.yml`): replace
the two `block`/`file` mounts with a single read-only mount:

```yaml
- ./grafana/dashboards:/var/lib/grafana/dashboards:ro
```

`dashboards.yml` is unchanged (still `foldersFromFilesStructure: true`).

## Shared design system

Applied identically to every dashboard.

**Variables (standard block):**
- `datasource` — type `datasource`, query `prometheus`.
- `array` — type `query`, `label_values(powerstore_up, array)`, `multi`,
  `includeAll`, current = All.
- Detail dashboards add a chained, multi/All variable filtered by `$array`:
  - block appliance/volume/VG dashboards → `appliance`
    (`label_values(powerstore_appliance_total_iops{array=~"$array"}, appliance_name)`).
  - file → `nas_server`
    (`label_values(powerstore_file_system_size_total_bytes{array=~"$array"}, nas_server_name)`).
  - replication → `state`
    (`label_values(powerstore_replication_session_state{array=~"$array"}, state)`).

**Layout:** collapsible **rows** group panels by theme (Status / Performance /
Capacity / etc.); fixed 24-column grid with no overlaps; `time: now-6h`;
`refresh: 30s`; browser timezone.

**Navigation:**
- Every dashboard: header `links` of type `dashboards`, tag-filtered on `powerstore`,
  with `includeVars: true` and `keepTime: true` so `$array` and time carry across.
- Detail drill via panel/field **data links** carrying `${array}` (and
  `${__field.labels.appliance_name}` etc. where it narrows the target board).
- Fleet Health status tiles link to the matching detail dashboard.

**Units:** IOPS → `iops`; bandwidth → `Bps`; latency → `µs`; capacity → `bytes`
(binary, IEC); percentages → `percent` (0–100); ratios → `none` (e.g. DRR shown as
N:1 via display); RPO/staleness → `s`.

**Threshold palette (identical across dashboards):**

| Signal | green | yellow | red |
|---|---|---|---|
| Capacity used % | < 70 | 70 | 85 |
| Latency (µs) | < 1000 | 1000 | 3000 |
| CPU I/O util (ratio 0–1) | < 0.70 | 0.70 | 0.90 |
| Drive wear (ratio 0–1) | < 0.70 | 0.70 | 0.80 |
| Scrape staleness (s) | < 90 | 90 | 300 |

**Color & series rules:**
- Stats/gauges: `color.mode = thresholds`; status tiles use `colorMode: background`.
- Timeseries: classic palette; **read = blue, write = orange** consistently on every
  performance panel; `legendFormat` uses entity label (`{{array}}`, `{{appliance_name}}`,
  `{{volume_name}}`, …) plus ` read`/` write` suffix.
- Tables: cell/background color + value mappings for state columns.

**Health value mappings:**
- `powerstore_up`: 0→`DOWN` (red), 1→`UP` (green).
- `powerstore_array_bulk_api`: 0→`Disabled` (orange), 1→`Enabled` (green).
- `powerstore_port_link_up`: 0→`Down` (red), 1→`Up` (green).
- `powerstore_drive_state{state}`: `Healthy`→green; any other state→red/orange.
- `powerstore_replication_session_state{state}`: `OK`/`Synchronized`-type→green;
  `Error`/`Fractured`/`Paused`/`System_Paused`→red/orange.
- `powerstore_alert_active{severity}`: severity-colored (Critical red, Major orange).

## Per-dashboard panel plans

Queries use verified metric/label names. `$array` filter
(`{array=~"$array"}`) applies on every target.

### overview/00-fleet-health.json (NEW — landing/hub)
- **Status row** (stat tiles, `colorMode: background`):
  - Arrays Up — `powerstore_up`, value-mapped.
  - Scrape Staleness — `time() - powerstore_last_scrape_timestamp_seconds`, unit `s`,
    staleness thresholds; per `{{array}}`.
  - Active Critical Alerts — `sum by (array) (powerstore_alert_active{severity="Critical"})`.
  - Unhealthy Drives — `count by (array) (powerstore_drive_state{state!="Healthy"})`
    (0 → green).
  - Ports Down — `count by (array) (powerstore_port_link_up == 0)`.
  - Bulk API — `powerstore_array_bulk_api`, value-mapped.
- **Performance glance row** (stat w/ sparkline): fleet total IOPS
  `sum by (array) (powerstore_appliance_total_iops)`; total bandwidth
  `sum by (array) (powerstore_appliance_read_bandwidth_bytes_per_second + on(...) ...)`
  shown as read+write series; avg latency
  `avg by (array) (powerstore_appliance_read_latency_microseconds)` (read/write).
- **Capacity glance row:** cluster capacity used % gauge
  `100 * sum(powerstore_cluster_physical_used_bytes) / sum(powerstore_cluster_physical_total_bytes)`;
  DRR stat `avg(powerstore_cluster_data_reduction_ratio)`.
- Tiles link to the relevant detail dashboard (drives→hardware, alerts→hardware,
  ports→ports, capacity→capacity, perf→cluster-overview).

### overview/01-cluster-overview.json (refined)
- **Performance row:** Total IOPS, Bandwidth (read/write), Avg Latency (read/write) —
  as today but with the shared palette, fixed grid, and row grouping.
- **Capacity row:** replace appliance-summed gauge with the **cluster rollup** —
  physical used vs total (`powerstore_cluster_physical_*`), logical provisioned vs used
  (`powerstore_cluster_logical_*`), DRR + efficiency stats (`powerstore_cluster_*_ratio`).

### block/02-appliances.json (refined)
- Add `$appliance` filter. Rows: **Throughput** (IOPS + read/write bandwidth per
  appliance), **Latency** (read/write per appliance), **CPU**
  (`powerstore_appliance_io_workload_cpu_utilization`, ratio unit, CPU thresholds).
- Data link on series → block/03-volumes carrying `${array}` +
  `${__field.labels.appliance_name}`.

### block/03-volumes.json (refined)
- `topk(10, …)` driven by `$array`/`$appliance`. Panels: Top-10 Total IOPS, Read
  Latency, Write Latency (existing) + Top-10 Read/Write Bandwidth (new), and the
  Avg IO Size table. Leaf dashboard (no further drill).

### block/04-volume-groups.json (NEW)
- Timeseries: VG total IOPS `sum by (volume_group_name) (powerstore_volume_group_total_iops)`,
  read/write bandwidth, read/write latency. Summary table of current values per VG.

### block/05-capacity.json (refined)
- **Cluster row (lead):** physical used vs total (stacked), logical provisioned vs used,
  and DRR / efficiency / snapshot-savings / thin-savings stats — from
  `powerstore_cluster_*`.
- **Appliance row (secondary):** per-appliance physical used vs total + a capacity table
  with used % (`100 * powerstore_appliance_physical_used_bytes / powerstore_appliance_physical_total_bytes`)
  colored by capacity thresholds.

### block/06-ports.json (refined)
- Ports Up / Ports Down stats + status table (`powerstore_port_link_up`) with cell
  coloring and Up/Down mapping; row grouping; `port_type` column.

### file/01-file-systems.json (refined)
- Add `$nas_server` filter.
- **Capacity row:** size total vs used, used %
  (`100 * powerstore_file_system_size_used_bytes / powerstore_file_system_size_total_bytes`,
  capacity thresholds), capacity summary table.
- **Performance row (NEW):** FS total IOPS, read/write latency, read/write bandwidth
  from `powerstore_file_system_*` perf metrics, shared palette.

### protection/01-replication.json (NEW)
- **Sessions row:** table of `powerstore_replication_session_state` (columns from labels:
  `session_id`, `resource_type`, `role`, `type`, `remote_system_id`, `state`) with
  state color-mapping; `$state` filter. A stat counting sessions in a bad state
  (`count(powerstore_replication_session_state{state=~"Error|Fractured|Paused|System_Paused"})`).
- **Health row:** RPO `powerstore_replication_rpo_seconds` (unit `s`), backlog
  `powerstore_replication_data_remaining_bytes` (unit `bytes`), transfer rate
  `powerstore_replication_transfer_rate_bytes_per_second` (unit `Bps`) timeseries.

### hardware/01-drives.json (NEW)
- **Drives row:** drive-state table (`powerstore_drive_state`, columns `drive_name`,
  `appliance_id`, `state`) with Healthy=green / other=red mapping; worn-drive count stat
  (`count(powerstore_drive_wear_level_ratio > 0.8)`); wear-level bargauge per drive
  (`powerstore_drive_wear_level_ratio`, wear thresholds 0.70/0.80).
- **Alerts row:** active-alerts table from `powerstore_alert_active` grouped by
  `severity` (+ `array`), severity-colored.

## Documentation changes

- `docs/metrics.md`: align the PromQL examples with the panel queries above (cluster
  capacity %, latency, alerts, replication, drive wear).
- `docs/dashboards.md`: update the folder-layout description (5 folders under
  `grafana/dashboards/`, the 10 dashboards, single compose mount).
- New `docs/adr/0014-grafana-dashboard-taxonomy.md`: record the folder taxonomy, the
  single-mount decision, the shared design system, and why Option A over B/C.

## Testing / verification

No Go code changes, so `make test` is unaffected. Verification is by stack run:

1. `docker compose up`, open Grafana (`localhost:3000`).
2. Confirm 5 folders provision (overview, block, file, protection, hardware) and all
   10 dashboards load with no "Datasource not found" / migration errors.
3. With live or replayed metrics, confirm each panel renders (no empty/`No data` where
   data exists), thresholds color correctly, the `array`/`appliance`/`nas_server`/`state`
   variables populate, and dashboard links + data links navigate carrying `$array`.
4. JSON sanity: every dashboard is valid JSON, `schemaVersion: 39`, unique `uid`,
   `tags` include `powerstore`, no overlapping `gridPos`.

## Risks / notes

- `grafana/grafana:latest` could ship a schema migration; keep `schemaVersion: 39` and
  rely on Grafana's on-load migration (current behavior).
- `powerstore_alert_active` is aggregated **by severity** (bounded series), not
  per-alert — the alerts table shows severity counts, not individual alert rows.
- Some metric families (replication, drives, alerts) only appear when the array has
  that feature/data; panels must degrade to a clean empty state, not error.
- Moving files changes paths in both compose files and the provisioning comment; grep
  for `grafana/block` / `grafana/file` references across the repo before finalizing.
