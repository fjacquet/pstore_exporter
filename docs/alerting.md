# Alerting

`pstore_exporter` ships a starter Prometheus alert rule set at
`deploy/prometheus/pstore.rules.yml`, wired into the bundled Compose stack
(`prometheus.yml` references it; the file is mounted into the Prometheus container).

## Shipped alerts

| Alert | Trigger | Severity |
|---|---|---|
| `PowerStoreArrayDown` | `powerstore_up == 0` | critical |
| `PowerStoreHighVolumeReadLatency` | `powerstore_volume_read_latency_microseconds > 5000` for 5m | warning |
| `PowerStoreHighVolumeWriteLatency` | `powerstore_volume_write_latency_microseconds > 5000` for 5m | warning |
| `PowerStoreApplianceHighCpuUtilization` | `powerstore_appliance_io_workload_cpu_utilization > 0.85` for 10m | warning |
| `PowerStoreApplianceCapacityHigh` | `powerstore_appliance_physical_used_bytes / powerstore_appliance_physical_total_bytes > 0.85` | warning |
| `PowerStoreApplianceCapacityCritical` | same ratio `> 0.95` | critical |
| `PowerStoreFileSystemCapacityHigh` | `powerstore_file_system_size_used_bytes / powerstore_file_system_size_total_bytes > 0.85` | warning |
| `PowerStorePortDown` | `powerstore_port_link_up == 0` | warning |

Thresholds are tunable defaults — copy the file and adjust `expr`/`for` to your SLOs.

!!! warning "Do not use `rate()`"
    `powerstore_*_iops` and bandwidth metrics are already per-second gauges; aggregate
    with `sum`/`avg`, never `rate()`.

## Using the rules outside Compose

Point any Prometheus at the file via `rule_files:`, or import it into Grafana
(Alerting → Alert rules → import a Prometheus rule group). It depends only on metrics
exposed on `/metrics`.

```yaml
# prometheus.yml fragment
rule_files:
  - /etc/prometheus/pstore.rules.yml
```
