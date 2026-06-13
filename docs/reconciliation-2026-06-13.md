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

Intro: some exporter "limits" are **gopowerstore-SDK constraints** (the v1.22 client
lacks a method), **not 4.4.0 REST API constraints** (the underlying path exists). This
distinction matters: SDK limits are worked around in-process (enumerate / generic
`Query`), whereas a true API gap would require a spec/firmware change. Each row below
marks which kind it is. Note also that the OpenAPI document is a **single 4.4.0.0
snapshot** (`info.version` = `4.4.0.0`); it advertises *presence* of paths but contains
no per-version capability matrix, so it cannot prove a 4.0-vs-4.1 boundary — only that
4.4.0 has the feature.

| Assumption (source) | Still true at 4.4.0? | Evidence |
| --- | --- | --- |
| Bulk CSV path gated on PowerStoreOS ≥4.1 via `GetSoftwareMajorMinorVersion` (`internal/powerstore/capability.go:10-16`; CLAUDE.md) | ✅ present at 4.4.0 — gate **satisfied** here, but boundary **not provable** from this spec | `paths` lists `/latest_five_min_metrics/enable [post]` and `/latest_five_min_metrics/download [post]`; `info.version` = `4.4.0.0` (≥4.1, so the gate passes). The 4.0/4.1 boundary is a **runtime software-version check**, not modeled in the OpenAPI doc — this 4.4.0-only spec confirms presence, **not** the introduction version. |
| ADR-0003 "no file-system performance available" (gopowerstore lacks FS perf) | ❌ **STALE — superseded by ADR-0009** | `entities` advertises FS perf entity types: `performance_metrics_by_file_system`, `space_metrics_by_file_system`, `performance_metrics_file_by_node`, `performance_metrics_file_by_appliance`, `performance_metrics_file_by_cluster`, `performance_metrics_by_nas_server`, `copy_metrics_by_file_system`, `copy_metrics_by_nas_server`. FS perf is now collected (`derive_filesystem_perf.go`); v1.22 `PerformanceMetricsByFileSystem` exists. |
| No list-appliances method → enumerate IDs from volumes+ports, then `GetAppliance` (CLAUDE.md; gopowerstore v1.22) | ✅ true, but **SDK limit, not API limit** | The REST `/appliance` collection **exists** in 4.4.0 (confirmed in Pass 1). The workaround exists only because gopowerstore v1.22 exposes no list method, not because the API lacks one. |
| No drive-list method → enumerate via generic `APIClient().Query` over `hardware` (CLAUDE.md; gopowerstore v1.22) | ✅ true, but **SDK limit, not API limit** | The REST `/hardware` path (type=Drive) **exists** in 4.4.0 (confirmed in Pass 1). Generic `Query` is an SDK-side workaround for the missing typed method. |
| No list-replication-sessions typed enumeration helper → SDK reads `replication_session` inventory directly (CLAUDE.md; gopowerstore v1.22) | ✅ true, but **SDK limit, not API limit** | The REST `/replication_session` path **exists** in 4.4.0 (confirmed in Pass 1). Any enumeration shortfall is SDK-driven, not an API gap. |
| No appliance version field → use `GetSoftwareMajorMinorVersion` (CLAUDE.md; gopowerstore v1.22) | ✅ true; software version sourced from a dedicated path, **SDK-shaped** | `appliance_instance` carries no software-version field, but `paths` exposes `/software_installed [get]` and `/software_installed/{id} [get]` — the array-wide software version the gate reads. |

## Pass 4 — Coverage map (all 55 entity types)

Legend — **Status**: `emitted` = a powerstore_ metric family already covers it; `not collected` = no metric emitted today. **Priority**: `—` (emitted, n/a); `high` = fits the block+file ops-monitoring purpose, clear value, collect next; `medium` = useful but niche / high-cardinality, collect later; `skip` = outside purpose (vSphere/VM analytics, per-protocol dialect deep-dive, or deprecated/redundant). All 55 `MetricsEntityEnum` values appear once, in spec order.

| Entity type | Status | Priority | Rationale |
|---|---|---|---|
| `performance_metrics_by_appliance` | emitted | — | Appliance perf family (`powerstore_appliance_*_iops/bandwidth/latency`, CPU util) already collected. |
| `performance_metrics_by_node` | not collected | medium | Per-node perf adds intra-appliance balance insight but doubles cardinality; appliance roll-up suffices for most ops dashboards. |
| `performance_metrics_by_volume` | emitted | — | Volume perf family (`powerstore_volume_*_iops/bandwidth/latency`) already collected. |
| `performance_metrics_by_cluster` | not collected | medium | Cluster-wide perf is derivable by `sum`/`avg` over appliance series in PromQL; native entity is convenience, not new signal. |
| `performance_metrics_by_vm` | not collected | skip | VM-level analytics is outside the block+file storage monitoring purpose. |
| `performance_metrics_by_vg` | emitted | — | Volume-group perf family (`powerstore_volume_group_*`) already collected. |
| `performance_metrics_by_fe_fc_port` | not collected | high | Front-end FC port perf is core SAN ops signal (host-facing throughput/latency, congestion); fits purpose, not yet collected. |
| `performance_metrics_by_fe_eth_port` | not collected | high | Front-end Ethernet port perf is core iSCSI/NVMe-TCP host-facing signal; fits purpose, not yet collected. |
| `performance_metrics_by_fe_eth_node` | not collected | high | Per-node FE Ethernet aggregate complements per-port view for node-level front-end load; clear ops value. |
| `performance_metrics_by_fe_fc_node` | not collected | high | Per-node FE FC aggregate complements per-port view for node-level front-end load; clear ops value. |
| `wear_metrics_by_drive` | not collected | skip | Deprecated since 2.0.0.0 (`x-deprecated_value`), superseded by `wear_metrics_by_drive_daily`; drive wear is already derived from inventory (`hardware_extra_details_instance.drive_wear_level` → `powerstore_drive_wear_level_ratio`). |
| `wear_metrics_by_drive_daily` | not collected | skip | Drive wear already covered via inventory (`powerstore_drive_wear_level_ratio`); this daily-cadence metrics entity is redundant with the inventory-derived gauge. |
| `space_metrics_by_cluster` | emitted | — | Cluster space family (`powerstore_cluster_*_bytes`, reduction/efficiency ratios) already collected. |
| `space_metrics_by_appliance` | emitted | — | Appliance space family (`powerstore_appliance_*_bytes`, savings/efficiency ratios) already collected. |
| `space_metrics_by_volume` | not collected | high | Per-volume capacity/efficiency is a primary block-storage capacity-planning signal; fits purpose, not yet collected. |
| `space_metrics_by_volume_family` | not collected | high | Volume-family space rolls snapshots+clones to the source volume — true consumed capacity for thin/snap planning; fits purpose. |
| `space_metrics_by_vm` | not collected | skip | VM-level space is VM analytics, outside the block+file storage purpose. |
| `space_metrics_by_storage_container` | not collected | medium | Storage-container (vVol datastore) space is useful where vVols are used, but niche for a block+file ops exporter. |
| `space_metrics_by_vg` | not collected | high | Per-volume-group capacity complements emitted vg perf; clear consistency-group capacity-planning value. |
| `copy_metrics_by_appliance` | not collected | medium | Replication/copy throughput per appliance is useful but coarse; session-level health (already via SDK cma_view) is the priority slice. |
| `copy_metrics_by_cluster` | not collected | medium | Cluster-wide copy roll-up derivable from finer copy series; convenience, not new signal. |
| `copy_metrics_by_vg` | not collected | medium | Per-vg copy stats niche; relevant only for group-replicated consistency groups. |
| `copy_metrics_by_rg` | not collected | medium | Per-replication-group copy stats niche; applies to NAS/group replication topologies only. |
| `copy_metrics_by_remote_system` | not collected | high | Copy throughput per remote system is a key DR-link health signal (per-peer transfer); fits ops/replication monitoring. |
| `copy_metrics_by_replication_session` | not collected | high | Per-session copy metrics are the core replication-health signal (RPO compliance, transfer progress); complements emitted SDK-derived `powerstore_replication_*`. |
| `copy_metrics_by_volume` | not collected | medium | Per-volume copy stats overlap with session-level health; useful but secondary. |
| `performance_metrics_by_file_system` | emitted | — | File-system perf family (`powerstore_file_system_*_iops/bandwidth/latency`) already collected (`derive_filesystem_perf.go`). |
| `performance_metrics_smb_by_node` | not collected | skip | Per-protocol SMB dialect deep-dive is outside the exporter's purpose. |
| `performance_metrics_smb_builtinclient_by_node` | not collected | skip | SMB built-in-client protocol detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_smb_branch_cache_by_node` | not collected | skip | SMB BranchCache protocol detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_smb1_by_node` | not collected | skip | SMB1 dialect detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_smb1_builtinclient_by_node` | not collected | skip | SMB1 built-in-client detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_smb2_by_node` | not collected | skip | SMB2 dialect detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_smb2_builtinclient_by_node` | not collected | skip | SMB2 built-in-client detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_nfs_by_node` | not collected | skip | Per-protocol NFS deep-dive is outside the exporter's purpose. |
| `performance_metrics_nfsv3_by_node` | not collected | skip | NFSv3 dialect detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_nfsv4_by_node` | not collected | skip | NFSv4 dialect detail; per-protocol deep-dive, out of scope. |
| `performance_metrics_file_by_node` | not collected | skip | Per-node aggregate file protocol perf; node-granularity dialect detail, out of scope (file perf covered per-filesystem). |
| `performance_metrics_file_by_appliance` | not collected | medium | Appliance-level aggregate file perf could complement per-filesystem view, but is reconstructable by summing FS series in PromQL. |
| `performance_metrics_file_by_cluster` | not collected | medium | Cluster-level aggregate file perf is derivable from per-filesystem series; convenience, not new signal. |
| `performance_metrics_by_ip_port` | not collected | medium | IP-port perf overlaps with FE Ethernet port metrics; useful but high-cardinality and partly redundant. |
| `performance_metrics_by_ip_port_iscsi` | not collected | medium | iSCSI IP-port perf is niche, high-cardinality; relevant only on iSCSI-heavy deployments. |
| `performance_metrics_by_nas_server` | not collected | high | NAS-server perf is the top-level file-serving entity (per-tenant throughput/latency); core file-ops signal, fits purpose. |
| `space_metrics_by_file_system` | emitted | — | File-system capacity (`powerstore_file_system_size_total/used_bytes`) already collected (inventory-derived). |
| `performance_metrics_by_initiator` | not collected | high | Per-initiator perf gives host-port-level SAN visibility (noisy-neighbour, path imbalance); clear block-ops value. |
| `performance_metrics_by_host` | not collected | high | Per-host perf is a primary block-storage ops signal (per-consumer IOPS/latency for dashboards); fits purpose. |
| `performance_metrics_by_hg` | not collected | high | Per-host-group perf aggregates clustered hosts (e.g. ESXi clusters consuming block); clear ops value. |
| `vsphere_metrics_by_vm` | not collected | skip | vSphere VM analytics, explicitly outside the exporter's purpose. |
| `vsphere_appson_metrics_by_node` | not collected | skip | vSphere AppsON node metrics; vSphere/VM analytics, out of scope. |
| `vsphere_appson_metrics_by_appliance` | not collected | skip | vSphere AppsON appliance metrics; vSphere/VM analytics, out of scope. |
| `space_metrics_by_remote_system` | not collected | high | Capacity per remote system supports DR-target capacity planning; fits replication/ops monitoring purpose. |
| `copy_metrics_by_file_system` | not collected | medium | Per-filesystem copy stats are useful for NAS replication but niche; session-level health is the priority. |
| `copy_metrics_by_nas_server` | not collected | medium | Per-NAS-server copy stats niche; relevant to NAS-replication topologies only. |
| `performance_headroom_by_appliance` | not collected | high | Appliance performance headroom (saturation/load-vs-capacity) is a high-value capacity-planning signal for ops; fits purpose. |
| `performance_metrics_by_appliance_resource_util` | not collected | high | Appliance resource-utilization (CPU/cache/back-end saturation) underpins bottleneck dashboards; clear ops value, fits purpose. |

## Fix list

_(Task 5)_
