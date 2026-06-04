# Dashboards

Importable Grafana dashboards (PromQL) live in `grafana/`, split by focus area:

- `grafana/block/` — block storage dashboards: volume performance (IOPS, bandwidth,
  latency, avg I/O size), appliance overview (performance + capacity + efficiency ratios),
  port link state.
- `grafana/file/` — file storage dashboards: file system capacity utilization per NAS
  server, per array summary.

Dashboards include an `array` template variable populated via `label_values(powerstore_up, array)`
so you can filter to a single array or view all arrays stacked.

## Import

In Grafana: **Dashboards → New → Import**, upload the JSON, and select your Prometheus
data source. Each dashboard uses a `$array` template variable (and the appliance dashboard
also an `$appliance` variable).

## PromQL conventions

- Metrics are gauges; `array`, `appliance_name`, `volume_name`, etc. let you filter per object.
- `iops` and `bandwidth_bytes_per_second` are **pre-derived per-second** values — aggregate
  with `sum`/`avg by (...)`, **never** `rate()`.

Examples:

```promql
# Total read IOPS across all volumes for the selected array
sum by (volume_name) (powerstore_volume_read_iops{array=~"$array"})

# Appliance efficiency over time
powerstore_appliance_efficiency_ratio{array=~"$array", appliance_name=~"$appliance"}

# File system fill rate
100 * powerstore_file_system_size_used_bytes{array=~"$array"}
    / powerstore_file_system_size_total_bytes{array=~"$array"}

# Busiest volumes by total IOPS
topk(10, sum by (volume_name) (powerstore_volume_total_iops{array=~"$array"}))
```

## Building more

The dashboards follow the [metric naming scheme](metrics.md); new panels can be built
mechanically from it. Remember that all values use explicit units:
`_bandwidth_bytes_per_second`, `_latency_microseconds`, `_bytes`.
