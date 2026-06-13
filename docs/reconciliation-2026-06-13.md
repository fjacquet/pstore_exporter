# PowerStore 4.4.0 Spec Reconciliation — 2026-06-13

Reconciles `pstore_exporter` metric collection against the canonical PowerStore REST API
`4.4.0.0` definition (`docs/swagger/5504-4.4.0.json`, model 5504). Companion to
`docs/reconciliation-2026-06-05.md`.

**Method:** spec entity types / per-entity field schemas / inventory schemas extracted and
cross-referenced against emitted metrics and the fields the derive functions read. The spec
describes the REST `/metrics/generate` JSON — authoritative for the SDK path, semantically
indicative for the bulk CSV path (which uses `avg_`/`last_` column prefixes the REST JSON lacks).

## Pass 1 — Endpoint audit

Every endpoint and SDK method the exporter invokes, mapped to its 4.4.0 REST path. Metrics
performance/space methods all `POST /metrics/generate` with an `entity` body param (the
`entity` value is checked against the spec's entity-type enum).

### Bulk HTTP path (`internal/powerstore/bulk.go`)

| Exporter call | REST path / entity | Present in 4.4.0? | Notes |
| --- | --- | --- | --- |
| POST enable | `/latest_five_min_metrics/enable` | ✅ | Repo-owned raw HTTP; arms 5-min aggregation. |
| POST download | `/latest_five_min_metrics/download` | ✅ | Repo-owned raw HTTP; CSV bulk dump (≥4.1 capability). |

### Per-entity SDK path (gopowerstore v1.22 — `client.go`, `perentity.go`)

| Exporter call | REST path / entity | Present in 4.4.0? | Notes |
| --- | --- | --- | --- |
| `GetCluster` | `GET /cluster` | ✅ | Cluster inventory + ID. |
| `GetVolumes` | `GET /volume` | ✅ | Volume inventory. |
| `GetVolumeGroups` | `GET /volume_group` | ✅ | VG inventory. |
| `GetNASServers` | `GET /nas_server` | ✅ | NAS server inventory. |
| `GetFCPorts` | `GET /fc_port` | ✅ | FC port inventory. |
| `GetEthPorts` | `GET /eth_port` | ✅ | Eth port inventory. |
| `GetAlerts` | `GET /alert` | ✅ | Active alerts. |
| `GetReplicationRules` | `GET /replication_rule` | ✅ | Replication rule inventory. |
| `GetReplicationSessionByLocalResourceID` | `GET /replication_session` (filtered) | ✅ | No list-sessions in SDK; enumerated from replicated volumes. |
| `GetAppliance` | `GET /appliance/{id}` | ✅ | No list-appliances in SDK; IDs enumerated from volumes+ports. |
| `GetSoftwareMajorMinorVersion` | `GET /software_installed` (derived) | ✅ | Bulk ≥4.1 capability gate; no appliance version field. |
| `enumerateDrives` (`APIClient().Query`) | `GET /hardware` (type=Drive) | ✅ | No list-drives in SDK; generic query, ADR-0009 fallback. |
| `PerformanceMetricsByAppliance` | `POST /metrics/generate` · `performance_metrics_by_appliance` | ✅ | Entity in 4.4.0 enum. |
| `PerformanceMetricsByVolume` | `POST /metrics/generate` · `performance_metrics_by_volume` | ✅ | Entity in 4.4.0 enum. |
| `PerformanceMetricsByVg` | `POST /metrics/generate` · `performance_metrics_by_vg` | ✅ | Entity in 4.4.0 enum. |
| `PerformanceMetricsByFileSystem` | `POST /metrics/generate` · `performance_metrics_by_file_system` | ✅ | Entity in 4.4.0 enum; FS perf is live (ADR-0009, supersedes ADR-0003). |
| `SpaceMetricsByAppliance` | `POST /metrics/generate` · `space_metrics_by_appliance` | ✅ | Entity in 4.4.0 enum. |
| `SpaceMetricsByCluster` | `POST /metrics/generate` · `space_metrics_by_cluster` | ✅ | Entity in 4.4.0 enum. |

No exporter call maps to a path absent from 4.4.0. `/sas_port` exists in the spec but the
exporter does not collect SAS ports (FC + Eth only) — not a gap, just unused surface.

## Pass 2 — Field audit

Each emitted metric is mapped to the source field/column the derive function reads and to
the canonical 4.4.0 field. **Read the verdicts through the REST-vs-bulk-CSV caveat:** the
OpenAPI schema models the REST `/metrics/generate` JSON, so it is authoritative for the
per-entity SDK path (gopowerstore JSON tags map 1:1 to the `base_*_metrics_by_*`
properties). It is only *semantically indicative* for the bulk CSV path, whose
"latest five minute" columns carry `avg_`/`last_` prefixes the REST JSON does not. Hence a
bulk primary key like `avg_read_iops` with **no** REST sibling is fine **iff** a REST-name
fallback (`read_iops`) exists in the schema — that is a ✅, not a gap. The real risk
(⚠️) is a read whose primary key has no spec sibling **and** no covering fallback →
silent-zero. A field read under a name the spec defines nowhere is ❌.

### Volume performance — bulk (`derive_bulk.go:43-52`) + SDK (`derive_perentity.go:24-33`)

Schema: `base_performance_metrics_by_volume`. SDK reads `PerformanceMetricsByVolumeResponse`
(embeds `CommonAvgFields` + `CommonMaxAvgIopsBandwidthFields`; `r.ReadIops` etc. are
json `read_iops`).

| Emitted metric | source field/column | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_volume_read_iops` | bulk `avg_read_iops`→`read_iops` · SDK `ReadIops` | `read_iops` | ✅ | bulk primary is avg-spelled; `read_iops` fallback covers it |
| `powerstore_volume_write_iops` | bulk `avg_write_iops`→`write_iops` · SDK `WriteIops` | `write_iops` | ✅ | fallback covers bulk spelling |
| `powerstore_volume_total_iops` | bulk `avg_total_iops`→`total_iops` · SDK `TotalIops` | `total_iops` | ✅ | fallback covers bulk spelling |
| `powerstore_volume_read_bandwidth_bytes_per_second` | bulk `avg_read_bandwidth`→`read_bandwidth` · SDK `ReadBandwidth` | `read_bandwidth` | ✅ | fallback covers bulk spelling |
| `powerstore_volume_write_bandwidth_bytes_per_second` | bulk `avg_write_bandwidth`→`write_bandwidth` · SDK `WriteBandwidth` | `write_bandwidth` | ✅ | fallback covers bulk spelling |
| `powerstore_volume_read_latency_microseconds` | bulk `avg_read_latency` · SDK `AvgReadLatency` | `avg_read_latency` | ✅ | spec field is itself `avg_`-prefixed; no fallback needed |
| `powerstore_volume_write_latency_microseconds` | bulk `avg_write_latency` · SDK `AvgWriteLatency` | `avg_write_latency` | ✅ | spec field is `avg_`-prefixed |
| `powerstore_volume_avg_io_size_bytes` | bulk `avg_io_size` · SDK `AvgIoSize` | `avg_io_size` | ✅ | spec field is `avg_`-prefixed |

### Appliance performance — bulk (`derive_bulk.go:65-74`) + SDK (`derive_perentity.go:45-54`)

Schema: `base_performance_metrics_by_appliance`.

| Emitted metric | source field/column | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_appliance_read_iops` | bulk `avg_read_iops`→`read_iops` · SDK `ReadIops` | `read_iops` | ✅ | fallback covers bulk spelling |
| `powerstore_appliance_write_iops` | bulk `avg_write_iops`→`write_iops` · SDK `WriteIops` | `write_iops` | ✅ | fallback covers bulk spelling |
| `powerstore_appliance_total_iops` | bulk `avg_total_iops`→`total_iops` · SDK `TotalIops` | `total_iops` | ✅ | fallback covers bulk spelling |
| `powerstore_appliance_read_bandwidth_bytes_per_second` | bulk `avg_read_bandwidth`→`read_bandwidth` · SDK `ReadBandwidth` | `read_bandwidth` | ✅ | fallback covers bulk spelling |
| `powerstore_appliance_write_bandwidth_bytes_per_second` | bulk `avg_write_bandwidth`→`write_bandwidth` · SDK `WriteBandwidth` | `write_bandwidth` | ✅ | fallback covers bulk spelling |
| `powerstore_appliance_read_latency_microseconds` | bulk `avg_read_latency` · SDK `AvgReadLatency` | `avg_read_latency` | ✅ | spec field is `avg_`-prefixed |
| `powerstore_appliance_write_latency_microseconds` | bulk `avg_write_latency` · SDK `AvgWriteLatency` | `avg_write_latency` | ✅ | spec field is `avg_`-prefixed |
| `powerstore_appliance_io_workload_cpu_utilization` | bulk `avg_io_workload_cpu_utilization` (**no fallback**) · SDK `IoWorkloadCPUUtilization` | `io_workload_cpu_utilization` | ⚠️ | **`derive_bulk.go:73`**: bulk primary `avg_io_workload_cpu_utilization` has no spec sibling (spec field is `io_workload_cpu_utilization`, no `avg_`) and **no fallback** → silent-zero on the bulk path if the CSV column is not avg-spelled. SDK path is fine (`IoWorkloadCPUUtilization` = json `io_workload_cpu_utilization`, matches spec). Verify the bulk column name vs a live `--trace` capture; add `"io_workload_cpu_utilization"` fallback. |

### Appliance space — bulk (`derive_bulk.go:87-96`) + SDK (`derive_perentity.go:66-75`)

Schema: `base_space_metrics_by_appliance`. Bulk primaries are already the bare (REST) names;
the `last_*` entries are extra fallbacks, not the primary.

| Emitted metric | source field/column | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_appliance_physical_total_bytes` | `physical_total`→`last_physical_total` · SDK `PhysicalTotal` | `physical_total` | ✅ | primary is the bare spec name |
| `powerstore_appliance_physical_used_bytes` | `physical_used`→`last_physical_used` · SDK `PhysicalUsed` | `physical_used` | ✅ | |
| `powerstore_appliance_logical_provisioned_bytes` | `logical_provisioned`→`last_logical_provisioned` · SDK `LogicalProvisioned` | `logical_provisioned` | ✅ | |
| `powerstore_appliance_logical_used_bytes` | `logical_used`→`last_logical_used` · SDK `LogicalUsed` | `logical_used` | ✅ | |
| `powerstore_appliance_data_reduction_ratio` | `data_reduction`→`last_data_reduction` · SDK `DataReduction` | `data_reduction` | ✅ | |
| `powerstore_appliance_efficiency_ratio` | `efficiency_ratio`→`last_efficiency_ratio` · SDK `EfficiencyRatio` | `efficiency_ratio` | ✅ | |
| `powerstore_appliance_snapshot_savings_ratio` | `snapshot_savings`→`last_snapshot_savings` · SDK `SnapshotSavings` | `snapshot_savings` | ✅ | |
| `powerstore_appliance_thin_savings_ratio` | `thin_savings`→`last_thin_savings` · SDK `ThinSavings` | `thin_savings` | ✅ | |

### Cluster space — SDK (`derive_cluster_space.go:16-23`)

Schema: `base_space_metrics_by_cluster`. SDK-only (no bulk path for cluster space).

| Emitted metric | source field | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_cluster_physical_total_bytes` | `PhysicalTotal` | `physical_total` | ✅ | |
| `powerstore_cluster_physical_used_bytes` | `PhysicalUsed` | `physical_used` | ✅ | |
| `powerstore_cluster_logical_provisioned_bytes` | `LogicalProvisioned` | `logical_provisioned` | ✅ | |
| `powerstore_cluster_logical_used_bytes` | `LogicalUsed` | `logical_used` | ✅ | |
| `powerstore_cluster_data_reduction_ratio` | `DataReduction` | `data_reduction` | ✅ | |
| `powerstore_cluster_efficiency_ratio` | `EfficiencyRatio` | `efficiency_ratio` | ✅ | |

### File-system performance — SDK (`derive_filesystem_perf.go:23-32`)

Schema: `base_performance_metrics_by_file_system`. **SDK path** — so the spec is
authoritative. The `PerformanceMetricsByFileSystemResponse` struct carries *both* the
spec-aligned `ReadIops`/`WriteIops`/`TotalIops`/`ReadBandwidth`/`WriteBandwidth` (json
`read_iops` …) **and** averaged `AvgReadIops`/`AvgWriteIops`/… (json `avg_read_iops` …)
fields. The derive reads the **`Avg*`** fields, whose JSON tags the 4.4.0 FS schema does
**not** define (it defines the bare `read_iops`/`write_iops`/`total_iops`/`read_bandwidth`/
`write_bandwidth`). The spec-aligned fields go unread.

| Emitted metric | source field | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_file_system_read_iops` | `AvgReadIops` (json `avg_read_iops`) | `read_iops` | ⚠️ | **`derive_filesystem_perf.go:24`**: reads `avg_read_iops`-tagged field; spec FS schema has only `read_iops`. If the live API populates `read_iops` (per spec) and not `avg_read_iops`, this is silent-zero. The unread `ReadIops` field (json `read_iops`) is the spec-aligned source. Verify vs a live `--trace` capture of the FS perf response. |
| `powerstore_file_system_write_iops` | `AvgWriteIops` (json `avg_write_iops`) | `write_iops` | ⚠️ | **`derive_filesystem_perf.go:25`**: same divergence; spec field is `write_iops`. Verify vs live `--trace`. |
| `powerstore_file_system_total_iops` | `AvgTotalIops` (json `avg_total_iops`) | `total_iops` | ⚠️ | **`derive_filesystem_perf.go:26`**: same; spec field is `total_iops`. Verify vs live `--trace`. |
| `powerstore_file_system_read_bandwidth_bytes_per_second` | `AvgReadBandwidth` (json `avg_read_bandwidth`) | `read_bandwidth` | ⚠️ | **`derive_filesystem_perf.go:27`**: same; spec field is `read_bandwidth`. Verify vs live `--trace`. |
| `powerstore_file_system_write_bandwidth_bytes_per_second` | `AvgWriteBandwidth` (json `avg_write_bandwidth`) | `write_bandwidth` | ⚠️ | **`derive_filesystem_perf.go:28`**: same; spec field is `write_bandwidth`. Verify vs live `--trace`. |
| `powerstore_file_system_read_latency_microseconds` | `AvgReadLatency` (json `avg_read_latency`) | `avg_read_latency` | ✅ | spec FS schema defines `avg_read_latency` |
| `powerstore_file_system_write_latency_microseconds` | `AvgWriteLatency` (json `avg_write_latency`) | `avg_write_latency` | ✅ | spec FS schema defines `avg_write_latency` |
| `powerstore_file_system_avg_io_size_bytes` | `AvgSize` (json `avg_size`) | `avg_size` | ✅ | spec FS schema defines `avg_size` |

### File-system capacity — inventory (`derive_perentity.go:82-92`)

Sourced from the `file_system` inventory object (not a metrics entity), so the canonical
reference is the `file_system_instance` schema, not `base_space_metrics_by_file_system`.

| Emitted metric | source field | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_file_system_size_total_bytes` | `fs.SizeTotal` (json `size_total`) | `size_total` | ✅ | inventory field, not metrics |
| `powerstore_file_system_size_used_bytes` | `fs.SizeUsed` (json `size_used`) | `size_used` | ✅ | inventory field, not metrics |

### Volume-group performance — SDK (`derive_volumegroup_perf.go:20-29`)

Schema: `base_performance_metrics_by_vg`. The `PerformanceMetricsByVgResponse` struct
exposes the bare `ReadIops`/`WriteIops`/`TotalIops`/`ReadBandwidth`/`WriteBandwidth` (json
`read_iops` …) — the derive reads those, matching the spec.

| Emitted metric | source field | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_volume_group_read_iops` | `ReadIops` (json `read_iops`) | `read_iops` | ✅ | bare field read, spec-aligned |
| `powerstore_volume_group_write_iops` | `WriteIops` (json `write_iops`) | `write_iops` | ✅ | |
| `powerstore_volume_group_total_iops` | `TotalIops` (json `total_iops`) | `total_iops` | ✅ | |
| `powerstore_volume_group_read_bandwidth_bytes_per_second` | `ReadBandwidth` (json `read_bandwidth`) | `read_bandwidth` | ✅ | |
| `powerstore_volume_group_write_bandwidth_bytes_per_second` | `WriteBandwidth` (json `write_bandwidth`) | `write_bandwidth` | ✅ | |
| `powerstore_volume_group_read_latency_microseconds` | `AvgReadLatency` (json `avg_read_latency`) | `avg_read_latency` | ✅ | spec field is `avg_`-prefixed |
| `powerstore_volume_group_write_latency_microseconds` | `AvgWriteLatency` (json `avg_write_latency`) | `avg_write_latency` | ✅ | spec field is `avg_`-prefixed |
| `powerstore_volume_group_avg_io_size_bytes` | `AvgIoSize` (json `avg_io_size`) | `avg_io_size` | ✅ | spec field is `avg_`-prefixed |

### Drive state + wear — inventory (`derive_drives.go:32-37`)

Sourced from the `hardware` (type=Drive) inventory enumeration, **not** the
`base_wear_metrics_by_drive_instance` metrics entity. Canonical references are the
`hardware_instance` / `hardware_extra_details_instance` schemas. The metrics entity's
`percent_endurance_remaining` is **not** read by the exporter (distinct from the inventory
`drive_wear_level` field below).

| Emitted metric | source field | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_drive_state` (info=1) | `d.LifeCycleState` (json `life_cycle_state`) | `life_cycle_state` (hardware_instance) | ✅ | inventory enum carried as a label |
| `powerstore_drive_wear_level_ratio` | `extra_details.drive_wear_level` ÷ 100 | `drive_wear_level` (hardware_extra_details_instance) | ✅ | spec defines `drive_wear_level`; the metrics-entity `percent_endurance_remaining` is a different, unused source |

### Replication / copy — SDK (`derive_replication.go`)

Session state and RPO come from the `replication_session` / `replication_rule` inventory
objects (✅ spec-aligned). Transfer rate + backlog come from the SDK
`VolumeMirrorTransferRate` / `FileSystemMirrorTransferRate` methods, which hit the
`volume_mirror_transfer_rate_cma_view` / `file_system_mirror_transfer_rate_cma_view`
endpoints — **internal `*_cma_view` views the 4.4.0 OpenAPI spec does not model at all**
(the `base_copy_metrics_by_*` schemas describe a *different* `/metrics/generate` copy entity
with `transfer_rate`/`data_transferred`, not these). So the transfer rows are
"endpoint not modeled in spec," not a field mismatch.

| Emitted metric | source field | canonical 4.4.0 field | match? | risk/note |
| --- | --- | --- | --- | --- |
| `powerstore_replication_session_state` (info=1) | `s.State` + `LocalResourceID`/`ResourceType`/`Role`/`Type`/`RemoteSystemID` | `replication_session_instance.{state,local_resource_id,resource_type,role,type,remote_system_id}` | ✅ | all label fields present in spec |
| `powerstore_replication_rpo_seconds` | `r.Rpo` (RPOEnum→seconds) + `RemoteSystemID` | `replication_rule_instance.{rpo,remote_system_id}` | ✅ | enum mapped to seconds |
| `powerstore_replication_transfer_rate_bytes_per_second` | `latest.MirrorBandwidth` (json `mirror_bandwidth`) | — (`*_cma_view`, unmodeled) | ✅ | spec does not model the `*_cma_view` endpoint; `mirror_bandwidth` exists nowhere in 4.4.0 — expected, not a gap. The `base_copy_metrics_by_volume` schema is a different entity (`transfer_rate`). |
| `powerstore_replication_data_remaining_bytes` | `latest.DataRemaining` (json `data_remaining`) | — (`*_cma_view`, unmodeled) | ✅ | same `*_cma_view` caveat; `data_remaining` does also appear in `base_copy_metrics_by_volume` but that is a different entity |

## Pass 3 — Capability-gate audit

_(Task 3)_

## Pass 4 — Coverage map (all 55 entity types)

_(Task 4)_

## Fix list

_(Task 5)_
