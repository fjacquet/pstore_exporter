# Metrics Reference

All metrics are gauges prefixed `powerstore_` and carry an `array` label (the configured
array name) plus `cluster_id` (the PowerStore system id).

## Meta & health metrics

| Metric | Labels | Meaning |
|---|---|---|
| `powerstore_up` | `array` | `1` if the array was scraped successfully this cycle, else `0`. |
| `powerstore_last_scrape_timestamp_seconds` | `array` | Unix time of the last successful collection. |
| `powerstore_array_bulk_api` | `array` | `1` if bulk CSV collection is active for this array, `0` if per-entity REST fallback is used. |

## Volume metrics

Each volume metric carries: `array`, `cluster_id`, `volume_name`, `volume_id`,
`appliance_id`, `appliance_name`, `volume_group_name`, `volume_group_id`.

| Metric | Unit | Description |
|---|---|---|
| `powerstore_volume_read_iops` | ops/s | Read I/O operations per second |
| `powerstore_volume_write_iops` | ops/s | Write I/O operations per second |
| `powerstore_volume_total_iops` | ops/s | Total (read + write) I/O operations per second |
| `powerstore_volume_read_bandwidth_bytes_per_second` | bytes/s | Read throughput |
| `powerstore_volume_write_bandwidth_bytes_per_second` | bytes/s | Write throughput |
| `powerstore_volume_read_latency_microseconds` | µs | Average read latency |
| `powerstore_volume_write_latency_microseconds` | µs | Average write latency |
| `powerstore_volume_avg_io_size_bytes` | bytes | Average I/O size (read + write combined) |

## Appliance metrics

Each appliance metric carries: `array`, `cluster_id`, `appliance_name`, `appliance_id`,
`service_tag`.

| Metric | Unit | Description |
|---|---|---|
| `powerstore_appliance_read_iops` | ops/s | Appliance read I/O operations per second |
| `powerstore_appliance_write_iops` | ops/s | Appliance write I/O operations per second |
| `powerstore_appliance_total_iops` | ops/s | Appliance total I/O operations per second |
| `powerstore_appliance_read_bandwidth_bytes_per_second` | bytes/s | Appliance read throughput |
| `powerstore_appliance_write_bandwidth_bytes_per_second` | bytes/s | Appliance write throughput |
| `powerstore_appliance_read_latency_microseconds` | µs | Appliance average read latency |
| `powerstore_appliance_write_latency_microseconds` | µs | Appliance average write latency |
| `powerstore_appliance_io_workload_cpu_utilization` | ratio (0–1) | CPU utilization fraction for I/O workload |
| `powerstore_appliance_physical_total_bytes` | bytes | Raw physical capacity |
| `powerstore_appliance_physical_used_bytes` | bytes | Physical capacity in use |
| `powerstore_appliance_logical_provisioned_bytes` | bytes | Total logically provisioned capacity |
| `powerstore_appliance_logical_used_bytes` | bytes | Logical capacity consumed by data |
| `powerstore_appliance_data_reduction_ratio` | ratio | Data reduction ratio (compression + dedup) |
| `powerstore_appliance_efficiency_ratio` | ratio | Overall storage efficiency ratio |
| `powerstore_appliance_snapshot_savings_ratio` | ratio | Savings attributable to snapshots |
| `powerstore_appliance_thin_savings_ratio` | ratio | Savings attributable to thin provisioning |

## Cluster capacity metrics

Cluster-wide space rollup for capacity forecasting (`SpaceMetricsByCluster`). Carries only
`array`, `cluster_id`.

| Metric | Unit | Description |
|---|---|---|
| `powerstore_cluster_physical_total_bytes` | bytes | Total physical capacity of the cluster |
| `powerstore_cluster_physical_used_bytes` | bytes | Physical space consumed |
| `powerstore_cluster_logical_provisioned_bytes` | bytes | Total provisioned logical capacity |
| `powerstore_cluster_logical_used_bytes` | bytes | Logical space written |
| `powerstore_cluster_data_reduction_ratio` | ratio | Cluster data-reduction ratio (N:1) |
| `powerstore_cluster_efficiency_ratio` | ratio | Overall cluster efficiency ratio (N:1) |

## Volume group metrics

Each volume-group metric carries: `array`, `cluster_id`, `volume_group_name`,
`volume_group_id`. Performance comes from `PerformanceMetricsByVg`.

| Metric | Unit | Description |
|---|---|---|
| `powerstore_volume_group_read_iops` | ops/s | Read I/O operations per second |
| `powerstore_volume_group_write_iops` | ops/s | Write I/O operations per second |
| `powerstore_volume_group_total_iops` | ops/s | Total I/O operations per second |
| `powerstore_volume_group_read_bandwidth_bytes_per_second` | bytes/s | Read throughput |
| `powerstore_volume_group_write_bandwidth_bytes_per_second` | bytes/s | Write throughput |
| `powerstore_volume_group_read_latency_microseconds` | µs | Average read latency |
| `powerstore_volume_group_write_latency_microseconds` | µs | Average write latency |
| `powerstore_volume_group_avg_io_size_bytes` | bytes | Average I/O size |

## File system metrics

Each file system metric carries: `array`, `cluster_id`, `file_system_name`, `file_system_id`,
`nas_server_name`, `nas_server_id`.

Capacity is inventory-derived; performance comes from the typed
`PerformanceMetricsByFileSystem` method (averaged counters), parallel to the volume
performance metrics. Both are emitted on the bulk and per-entity paths.

| Metric | Unit | Description |
|---|---|---|
| `powerstore_file_system_size_total_bytes` | bytes | Provisioned file system capacity |
| `powerstore_file_system_size_used_bytes` | bytes | Used file system capacity |
| `powerstore_file_system_read_iops` | ops/s | Average read I/O operations per second |
| `powerstore_file_system_write_iops` | ops/s | Average write I/O operations per second |
| `powerstore_file_system_total_iops` | ops/s | Average total I/O operations per second |
| `powerstore_file_system_read_bandwidth_bytes_per_second` | bytes/s | Average read throughput |
| `powerstore_file_system_write_bandwidth_bytes_per_second` | bytes/s | Average write throughput |
| `powerstore_file_system_read_latency_microseconds` | µs | Average read latency |
| `powerstore_file_system_write_latency_microseconds` | µs | Average write latency |
| `powerstore_file_system_avg_io_size_bytes` | bytes | Average I/O size |

## Port metrics

Each port metric carries: `array`, `cluster_id`, `port_name`, `port_id`, `port_type`
(`eth` or `fc`), `appliance_id`.

| Metric | Labels | Description |
|---|---|---|
| `powerstore_port_link_up` | (port labels) | `1` if the port link is up, `0` if down. |

## Drive metrics

Each drive metric carries: `array`, `cluster_id`, `drive_id`, `drive_name`, `appliance_id`.
Drives are enumerated via a single generic GET on the `hardware` resource (PowerStore exposes
no typed list-drives method; see [ADR-0009](adr/0009-expand-metric-coverage-library-first.md)).

| Metric | Labels | Description |
|---|---|---|
| `powerstore_drive_state` | `state` | Info series, always `1`; the drive's lifecycle state is the `state` label (`Healthy`, `Failed`, …). |
| `powerstore_drive_wear_level_ratio` | (drive labels) | SSD wear consumed, `0.0` (new) → `1.0` (worn out). Only emitted on PowerStoreOS ≥ 4.3.0.0, which reports `drive_wear_level`. |

## Alert metrics

Each alert metric carries: `array`, `cluster_id`, `severity`.

| Metric | Labels | Description |
|---|---|---|
| `powerstore_alert_active` | `severity` | Count of **active** (uncleared) alerts at this severity. |

Alerts are aggregated **by severity** (not one series per alert) to keep cardinality
bounded. The standard PowerStore severities — `Critical`, `Major`, `Minor`, `Info`, `None` —
are always emitted, with value `0` when no active alert matches, so alerting rules can rely
on a stable series (e.g. `powerstore_alert_active{severity="Critical"} > 0`) rather than one
that disappears when the array is healthy. Source: `gopowerstore` `GetAlerts`. Emitted on
both the bulk and per-entity paths (see [ADR-0009](adr/0009-expand-metric-coverage-library-first.md)).

## Replication metrics

Source: typed `gopowerstore` methods (`GetReplicationRules`,
`GetReplicationSessionByLocalResourceID`, `VolumeMirrorTransferRate`). Sessions and transfer
metrics are enumerated from volumes carrying a protection policy. Emitted on both export
paths (see [ADR-0009](adr/0009-expand-metric-coverage-library-first.md)).

| Metric | Labels | Description |
|---|---|---|
| `powerstore_replication_session_state` | `session_id`, `local_resource_id`, `resource_type`, `role`, `type`, `remote_system_id`, `state` | Info series, always `1`; the session's current state is the `state` label (`OK`, `Synchronizing`, `Error`, `Fractured`, `System_Paused`, …). |
| `powerstore_replication_rpo_seconds` | `rule_id`, `remote_system_id` | Configured RPO of a replication rule, in seconds (`0` = synchronous). |
| `powerstore_replication_transfer_rate_bytes_per_second` | `resource_id`, `resource_type` | Current mirror replication throughput for the resource. |
| `powerstore_replication_data_remaining_bytes` | `resource_id`, `resource_type` | Outstanding data still to be replicated (backlog / RPO-risk indicator). |
| `powerstore_metro_witness_state` | `witness_id`, `witness_name`, `state` | Info series, always `1`; the Metro witness service's overall state is the `state` label (`OK`, `Partially_Connected`, `Disconnected`, `Initializing`, `Deleting`). |
| `powerstore_metro_witness_connection_state` | `witness_id`, `appliance_id`, `node_id`, `state` | Info series, always `1`; one per node, with the node's connection to the witness in the `state` label (`OK`, `Disconnected`, `Initializing`). |

The `state` metric follows the enum/info idiom: alert on undesirable states with a label
matcher rather than a numeric threshold — `powerstore_replication_session_state` carries the
value `1` regardless of state.

## PromQL guidance

!!! warning "Do not use `rate()`"
    `powerstore_*` IOPS and bandwidth values are **already per-second gauges** computed by
    PowerStore. In PromQL aggregate them with `sum` / `avg` by labels — **never** wrap them
    in `rate()` or you will double-rate the values.

```promql
# Total write IOPS across all volumes on one array
sum by (array) (powerstore_volume_write_iops{array="pstore-prod"})

# Average appliance read latency
avg by (appliance_name) (powerstore_appliance_read_latency_microseconds)

# File-system capacity used %
100 * powerstore_file_system_size_used_bytes / powerstore_file_system_size_total_bytes

# Physical capacity utilization per appliance
100 * powerstore_appliance_physical_used_bytes / powerstore_appliance_physical_total_bytes

# Cluster physical capacity utilization %
100 * powerstore_cluster_physical_used_bytes / powerstore_cluster_physical_total_bytes

# Ports that are down
powerstore_port_link_up == 0

# Any active critical alerts, per array
sum by (array) (powerstore_alert_active{severity="Critical"}) > 0

# Replication sessions in a bad state
powerstore_replication_session_state{state=~"Error|Fractured|System_Paused|Paused"}

# Replication backlog exceeding 1 GiB
powerstore_replication_data_remaining_bytes > 1073741824

# Metro witness degraded or unreachable
powerstore_metro_witness_state{state=~"Disconnected|Partially_Connected"}
# A specific node cannot reach the witness
powerstore_metro_witness_connection_state{state="Disconnected"}

# Drives that are not healthy
powerstore_drive_state{state!="Healthy"}

# SSDs past 80% wear
powerstore_drive_wear_level_ratio > 0.8
```

## Deferred metrics (available in the API, not yet wired)

The following are exposed by `gopowerstore` v1.22.0 as typed methods and are **available**,
not unavailable — they are simply not collected yet. Implementation order and field mappings
are in [`reconciliation-2026-06-05.md`](reconciliation-2026-06-05.md) and
[ADR-0009](adr/0009-expand-metric-coverage-library-first.md).

- **Per-volume space** (`powerstore_volume_logical_used_bytes`) — per-volume growth via
  `SpaceMetricsByVolume`. Deferred: it adds one API call per volume (doubling the per-volume
  call volume); cluster-level forecasting via `powerstore_cluster_*` covers the common case
  first. *(P3)*
- **NAS/SMB/NFS per-protocol & per-node performance** — `PerformanceMetricsNfs*ByNode`,
  `PerformanceMetricsSmb*ByNode`, `PerformanceMetricsByNode`. Deferred under YAGNI. *(P3)*
