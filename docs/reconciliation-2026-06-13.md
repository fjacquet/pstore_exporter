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

_(Task 2)_

## Pass 3 — Capability-gate audit

_(Task 3)_

## Pass 4 — Coverage map (all 55 entity types)

_(Task 4)_

## Fix list

_(Task 5)_
