# 14. Grafana Dashboard Taxonomy and Shared Design System

## Status

Accepted

## Context

The bundled Grafana dashboards had grown organically into two folders (`grafana/block/`
and `grafana/file/`) of hand-authored JSON. Several problems had accumulated:

- **Coverage gaps.** Whole emitted metric families had no visualization: replication/DR
  (`powerstore_replication_*`), drive and hardware health (`powerstore_drive_state`,
  `powerstore_drive_wear_level_ratio`), active alerts (`powerstore_alert_active`), volume
  groups (`powerstore_volume_group_*`), and file-system performance.
- **Wrong source metric.** The capacity dashboard summed per-appliance physical/logical
  bytes rather than the purpose-built cluster rollup (`powerstore_cluster_*`).
- **No consistency.** Thresholds, units, series colors, and layout were defined ad hoc
  per panel; there was no health-first landing page and no navigation between dashboards.

The dashboards ship in a Go repository with strict CI and no front-end toolchain. The
Grafana provisioning provider already uses `foldersFromFilesStructure: true`, so each
subdirectory under the mounted dashboards path becomes a Grafana folder.

## Decision

Reorganize and restyle all dashboards as plain hand-authored JSON (no jsonnet/grafonnet
generation layer, no CI validation script) and fill the coverage gaps.

- **Taxonomy.** Move every dashboard under a single `grafana/dashboards/` parent with five
  logical folders: `overview/` (fleet health + cluster overview), `block/` (appliances,
  volumes, volume groups, capacity, ports), `file/` (file systems), `protection/`
  (replication), `hardware/` (drives + alerts). docker-compose mounts the tree with one
  read-only volume instead of one mount per folder, so new folders are free.
- **Shared design system, applied to every dashboard.** A standard `datasource` + `array`
  template block (`label_values(powerstore_up, array)`, multi/All), with chained
  `appliance` / `nas_server` / `state` variables only where a dashboard needs them;
  collapsible row grouping; a tag-based dashboard-links dropdown (`powerstore`) carrying
  variables and time; consistent unit ids; a fixed threshold palette (capacity 70/85 %,
  latency 1000/3000 µs, CPU and drive wear 0.70/0.90 and 0.70/0.80, scrape staleness
  90/300 s); and fixed series colors (read = blue, write = orange).
- **Correct sources.** Capacity leads with the `powerstore_cluster_*` rollup; the
  per-appliance breakdown is secondary. File systems show both capacity and the
  (previously unused) performance metrics.
- **Plain JSON over a generator.** Hand-authored JSON keeps the Grafana UI
  export/import round-trip intact and adds no toolchain to a repo that has none for
  dashboards. A generator (jsonnet) or a CI consistency linter were both considered and
  rejected as disproportionate for ~10 dashboards; the linter remains a possible later
  follow-up if drift becomes a problem.

## Consequences

- New entry point: `overview/00-fleet-health` surfaces up/down, scrape staleness, active
  critical alerts, unhealthy drives, and ports-down at a glance, and is the navigation hub.
- Every previously emitted-but-unvisualized metric family now has a home, so adding a
  metric and "where do I see it" have a clear answer.
- Consistency is maintained by convention and review, not mechanically — divergence is
  possible if future edits ignore the gold-standard `overview/01-cluster-overview.json`.
  Keep that file as the reference when adding panels.
- The single compose mount and `foldersFromFilesStructure` mean adding a folder needs no
  compose change; adding a dashboard is just a new JSON file in the right folder.
- Per the project invariant, adding or renaming a metric still requires updating the
  relevant dashboard(s) and `docs/metrics.md` / `docs/dashboards.md` in lockstep.
