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

## File system metrics

Each file system metric carries: `array`, `cluster_id`, `file_system_name`, `file_system_id`,
`nas_server_name`, `nas_server_id`.

| Metric | Unit | Description |
|---|---|---|
| `powerstore_file_system_size_total_bytes` | bytes | Provisioned file system capacity |
| `powerstore_file_system_size_used_bytes` | bytes | Used file system capacity |

## Port metrics

Each port metric carries: `array`, `cluster_id`, `port_name`, `port_id`, `port_type`
(`eth` or `fc`), `appliance_id`.

| Metric | Labels | Description |
|---|---|---|
| `powerstore_port_link_up` | (port labels) | `1` if the port link is up, `0` if down. |

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

# Ports that are down
powerstore_port_link_up == 0
```

## Deferred metrics (planned for a future release)

The following object types are not yet collected in v1 but are planned:

- **Drive wear** (`powerstore_drive_*`) — SSD endurance metrics per drive.
- **Volume group performance** (`powerstore_volume_group_*`) — aggregate IOPS/bandwidth
  per volume group.
- **NAS server performance** (`powerstore_nas_server_*`) — file protocol throughput and
  latency per NAS server.
