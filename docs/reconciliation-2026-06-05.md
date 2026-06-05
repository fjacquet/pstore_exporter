# API Reconciliation — pstore_exporter vs PowerStore REST API

**Date:** 2026-06-05
**Spec source:** *API Documentation — PowerStore REST API* (4,318 pp., generated 2026-05-29),
extracted to text and cross-referenced.
**Library reference:** `github.com/dell/gopowerstore` **v1.22.0** (pinned, and the latest
published tag — `v1.12.0 … v1.22.0`).
**Constraint:** library-first — every candidate maps to a typed gopowerstore method; the
raw/generic API (`APIClient().Query`) is a fallback only where no typed method exists, as
the bulk CSV path already does (ADR-0003).

This document has two halves: an **audit** of the metrics we already emit, and a
**prioritized gap backlog**. No collector behavior changes in this cycle — each backlog item
is its own follow-up.

---

## Part 1 — Correctness audit of existing metrics

Verified each emitted metric against the gopowerstore response struct field comments (which
are generated from the same API spec) and the extracted REST reference.

| Metric family | Source method / struct | Field & unit check | Verdict |
|---|---|---|---|
| `powerstore_volume_*_iops` | `PerformanceMetricsByVolume` → `CommonMaxAvgIopsBandwidthFields` | `read_iops`/`write_iops`/`total_iops` = "operations per second" | **PASS** |
| `powerstore_volume_*_bandwidth_bytes_per_second` | same | `read_bandwidth`/`write_bandwidth` = "bytes per second" | **PASS** |
| `powerstore_volume_*_latency_microseconds` | same → `CommonAvgFields` | `avg_read_latency`/`avg_write_latency` = "microseconds" | **PASS** |
| `powerstore_volume_avg_io_size_bytes` | `avg_io_size` = "bytes" | **PASS** |
| `powerstore_appliance_*_iops/bandwidth/latency` | `PerformanceMetricsByAppliance` | same common fields | **PASS** |
| `powerstore_appliance_io_workload_cpu_utilization` | `io_workload_cpu_utilization` = percentage | **PASS** (value is 0–100 %, not a ratio — name omits unit; acceptable, see note 1) |
| `powerstore_appliance_physical/logical_*_bytes` | `SpaceMetricsByAppliance` | `physical_total`/`physical_used`/`logical_provisioned`/`logical_used` = bytes | **PASS** |
| `powerstore_appliance_*_ratio` | same | `data_reduction`/`efficiency_ratio`/`snapshot_savings`/`thin_savings` are N:1 ratios (e.g. `10` = 10:1) | **PASS** (note 2) |
| `powerstore_file_system_size_*_bytes` | inventory (`ListFS`) | capacity in bytes | **PASS** |
| `powerstore_port_link_up` | inventory (`GetFCPorts`/`GetEthPorts`) | boolean→gauge | **PASS** |

**Notes**
1. CPU utilization is a 0–100 percentage, not a fraction. The metric name has no unit
   suffix; if we ever rename for consistency, `_ratio` would be wrong (it is a percent).
   Leave as-is to avoid breaking dashboards; document the range.
2. Savings/reduction values are N:1 ratios (`10` means 10:1), **not** fractions in [0,1].
   PromQL like `100 * (1 - 1/ratio)` gives the saved-percentage. Already named `_ratio`. OK.
3. **Per-entity vs bulk statistic mismatch (low severity):** the per-entity path emits the
   point-in-time `read_iops`/`write_iops` fields, while the bulk path emits the averaged
   `avg_read_iops`/`avg_write_iops` columns under the *same* metric name. Values differ
   slightly by construction (instant vs 5-min average). Metric/label parity holds; only the
   statistic differs. Document this; no code change required.

### Audit defects found (require doc fixes this cycle)

- **D1 — Stale `PerformanceMetricsByFileSystem` claim.** `ADR-0003` states *"There is no
  `PerformanceMetricsByFileSystem` method"* and `docs/metrics.md` lists NAS/FS performance
  as deferred-because-unavailable. **False in v1.22.0:** the method exists
  (`PerformanceMetricsByFileSystem(ctx, fsID, interval) → []PerformanceMetricsByFileSystemResponse`)
  and the endpoint `performance_metrics_by_file_system` appears 58× in the spec. The ADR note
  predates the version bump. → Fixed by ADR-0009; corrected in `docs/metrics.md` & `CLAUDE.md`.
- **D2 — Deferred list understates availability.** `docs/metrics.md` lists drive wear and
  volume-group performance as "planned, not yet collected." Both are typed methods in
  v1.22.0 (`WearMetricsByDrive`, `PerformanceMetricsByVg`) — they are *unimplemented*, not
  *unavailable*. Reword to "available, not yet wired."

---

## Part 2 — Gap backlog (library-first, prioritized)

Each row names the **typed method** that sources it. Domains not present in the bulk CSV set
(alerts, replication, drive wear) are derived via typed calls and appended to **both**
`BulkMetrics` and `PerEntityMetrics` outputs — exactly as `deriveFileSystemCapacity` and
`derivePortLinkStatus` already are — preserving the metric-parity invariant.

### P1 — Hardware health & faults

| Proposed metric | Labels | Source (typed) | Notes |
|---|---|---|---|
| `powerstore_alert_active` | `severity`, `resource_type`, `event_code` | `GetAlerts(state=ACTIVE)` (`AlertsClient`) | Count of active alerts grouped by severity → the alerting backbone. Replaces needing per-component (battery/PSU/fan) structs, which the typed client does not expose. |
| `powerstore_drive_endurance_remaining_ratio` | `drive_id`, `appliance_id` | `WearMetricsByDrive` → `PercentEnduranceRemaining` | Value 0–100 → divide by 100. **Drive enumeration has no typed list method** (like appliances) → enumerate drive IDs via generic `APIClient().Query` on `/hardware?type=Drive`, then call the typed wear method. |

### P1 — Replication & protection

| Proposed metric | Labels | Source (typed) | Notes |
|---|---|---|---|
| `powerstore_replication_session_state` | `session_id`, `role`, `type`, `resource_type`, `remote_system_id` | `GetReplicationSessionByLocalResourceID` / `GetReplicationSessionByID` | Map `RSStateEnum` → numeric (e.g. `OK`=1, degraded/failed=0). Enumerate replicated resources from volumes/VGs with a protection policy. |
| `powerstore_replication_transfer_rate_bytes_per_second` | `resource_id`, `resource_type` | `VolumeMirrorTransferRate` / `FileSystemMirrorTransferRate` → `MirrorBandwidth` | Already bytes/sec; gauge. |
| `powerstore_replication_data_remaining_bytes` | `resource_id` | same → `DataRemaining` | Backlog/lag indicator for RPO risk. |
| `powerstore_replication_rpo_seconds` | `rule_id`, `remote_system_id` | `GetReplicationRules` → `Rpo` (`RPOEnum`) | Map enum (Five_Minutes, …) → seconds. Static-ish; low cardinality. |

### P2 — Capacity forecasting

| Proposed metric | Labels | Source (typed) | Notes |
|---|---|---|---|
| `powerstore_cluster_physical_total_bytes` / `_used_bytes` | `cluster_id` | `SpaceMetricsByCluster` | Cluster-level rollup for full-by-date projection. |
| `powerstore_volume_logical_used_bytes` | volume labels | `SpaceMetricsByVolume` | Per-volume growth → forecasting; reuses existing volume label builder. |
| `powerstore_volume_group_logical_used_bytes` | VG labels | `SpaceMetricsByVolumeGroup` | Per-VG growth. |
| `powerstore_cluster_capacity_bytes` | `cluster_id` | `GetCapacity` (int64) | Single scalar; cheap. |

### P2 — File / NAS performance (resolves D1/D2)

| Proposed metric | Labels | Source (typed) | Notes |
|---|---|---|---|
| `powerstore_file_system_read_iops` / `write` / `total` | FS labels (existing) | `PerformanceMetricsByFileSystem` → `Avg*Iops` | **Upgrades** FS coverage from capacity-only to live performance. Reuses `fileSystemLabels`. |
| `powerstore_file_system_*_bandwidth_bytes_per_second` | same | `Avg*Bandwidth` | bytes/sec. |
| `powerstore_file_system_*_latency_microseconds` | same | `Avg*Latency` | µs. |
| `powerstore_volume_group_*_iops/bandwidth/latency` | VG labels | `PerformanceMetricsByVg` | The deferred VG-perf item; now available. |

### P3 — Deeper file protocol & node detail (defer unless requested — YAGNI)

`PerformanceMetricsNfsByNode` / `…Nfsv3/v4ByNode`, `PerformanceMetricsSmbByNode` (+ SMB1/2
variants), `PerformanceMetricsByNode`, `PerformanceMetricsByFe{Fc,Eth}{Port,Node}`,
`CopyMetricsBy*`, `SpaceMetricsByVolumeFamily/StorageContainer`. High cardinality / niche;
add only when a concrete dashboard needs them.

---

## Recommended implementation order

1. ~~**Hardware alerts** (`powerstore_alert_active`)~~ — **DONE.** Reference slice for the
   "typed-call → derive → append to both paths" pattern (`derive_alerts.go`).
2. ~~**Replication session state + transfer rate**~~ — **DONE.** `powerstore_replication_*`
   (session state info series, RPO, transfer rate, data-remaining) via `derive_replication.go`
   + `ArrayClient.replicationMetrics`; enumerated from policy-bearing volumes, typed-only.
3. ~~**File-system performance** (`PerformanceMetricsByFileSystem`)~~ — **DONE.** Live per-FS
   IOPS/bandwidth/latency via `derive_filesystem_perf.go` + `ArrayClient.fileSystemPerf`,
   parallel to volume metrics. Closes defect **D1** (the stale "method unavailable" claim).
4. **Volume-group performance** + **capacity (cluster/volume space)**.
5. Drive wear (needs drive enumeration); P3 protocol/node detail last.

Each step: add the derive function (mirroring `derive_perentity.go`), reuse shared label
builders in `metrics.go`, wire into both `PerEntityMetrics` and `BulkMetrics`, add a
parity/unit test, and extend `docs/metrics.md`. No new HTTP plumbing except drive
enumeration (P3) which uses the existing generic `APIClient().Query` escape hatch.
