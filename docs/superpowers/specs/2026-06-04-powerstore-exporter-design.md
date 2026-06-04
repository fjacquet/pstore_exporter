# PowerStore Exporter (`pstore_exporter`) — Design Spec

**Date:** 2026-06-04
**Status:** Approved — ready for implementation planning

## Context

`pflex_exporter` (`/Users/fjacquet/Projects/pflex_exporter`) is a polished Go Prometheus
exporter for Dell PowerFlex with a clean architecture: a single background **collection
loop → immutable snapshot store → dual export (Prometheus `/metrics` + optional OTLP push)**,
multi-cluster support, hot config reload, generation auto-detection, Grafana dashboards,
docker-compose, Kubernetes manifests, MkDocs docs, and a full CI/release pipeline.

The goal is to build the equivalent for **Dell PowerStore** in the same mold. Dell already
ships `powerstore-metrics-exporter`, but it uses a weaker design (Gin, per-IP routes, separate
registries, no snapshot model, no OTLP, no hot reload, no compose). We instead replicate the
`pflex_exporter` architecture and reuse Dell's maintained typed client `gopowerstore` v1.22.0
(used by `csi-powerstore` and `csm-metrics-powerstore`).

Target directory: `/Users/fjacquet/Projects/pstore_exporter`.

## Decisions

- **API client:** Dell `github.com/dell/gopowerstore` v1.22.0 (typed, maintained).
- **Metrics path:** Both, auto-detected — bulk CSV API on PowerStoreOS ≥ 4.1, per-entity
  `/metrics/generate` fallback on older arrays.
- **Entity scope:** Full — block **and** file (cluster, appliance, volume, volume group,
  drives, eth/fc ports, capacity/space, NAS servers, file systems, replication).
- **Variant model:** Keep snapshot/collector/dual-export verbatim; replace Gen1/Gen2
  *generation* detection with **API-capability detection** (bulk-capable vs not). Multi-array
  stays; every metric carries an `array` label.
- **Feature parity:** Full pflex parity (OTLP, hot reload, k8s manifests, MkDocs, CI/release/SBOM/Semgrep).
- **Naming:** binary `pstore_exporter`; metric prefix `powerstore_`. Default metrics port **9101**.

## Key technical finding

`gopowerstore` covers auth (`login_session` token/cookie, auto-refresh on 403), all topology
(`GetCluster`, `GetAppliance`, `GetVolumes`, `GetVolumeGroups`, `GetNASServers`, `ListFS`,
`GetFCPorts`, `GetEthPorts`, replication) and **per-entity** metrics
(`PerformanceMetricsByVolume/Appliance/Vg/FeEthPort/FeFcPort/FileSystem/Node`,
`SpaceMetricsBy…`, `WearMetricsByDrive`) with intervals `Twenty_Sec | Five_Mins | One_Hour | One_Day`.
It does **NOT** wrap the bulk CSV API, but exposes `client.APIClient()` for raw authenticated
requests — so the bulk path reuses the same authenticated session.

## Architecture (mirrors pflex_exporter, package renamed `powerstore`)

```
pstore_exporter/
├── main.go                      # Cobra CLI, HTTP server (/metrics,/health), collection loop, hot reload (port 9101)
├── config.yaml                  # arrays[], server, collection, opentelemetry; ${ENV} + passwordFile
├── go.mod                       # + github.com/dell/gopowerstore v1.22.0
├── Dockerfile / docker-compose.yml / docker-compose.ghcr.yml
├── prometheus.yml / otel-collector-config.yaml / mkdocs.yml / Makefile
├── internal/
│   ├── models/      config.go, safe_config.go (from pflex, s/cluster/array/, s/gateway/endpoint/)
│   ├── powerstore/  (was internal/powerflex/)
│   │   ├── interface.go   # Client: Name, GetTopology, GetMetricsBulk, GetMetricsPerEntity, Close
│   │   ├── client.go      # ArrayClient wrapping gopowerstore.Client + raw APIClient for bulk
│   │   ├── auth.go        # delegated to gopowerstore; thin wrapper for bulk session reuse
│   │   ├── capability.go  # replaces gen.go: detect bulk-API support via cluster/appliance OS version
│   │   ├── collector.go   # background loop, errgroup per-array, graceful degradation, snapshot publish
│   │   ├── snapshot.go    # SnapshotStore (RWMutex, atomic swap) — verbatim
│   │   ├── metrics.go     # Sample{Name,Labels,Value}, label builders (array/appliance/volume/...)
│   │   ├── prometheus.go  # unchecked PromCollector emitting from snapshot — verbatim pattern
│   │   ├── otlp.go        # OTLP observable gauges + periodic push — verbatim pattern
│   │   ├── bulk.go        # NEW: bulk CSV path (/latest_five_min_metrics enable + gz/tar/CSV parse)
│   │   ├── perentity.go   # per-entity path via gopowerstore PerformanceMetricsBy*/SpaceMetricsBy*
│   │   ├── derive_bulk.go / derive_perentity.go  # map responses → []Sample (powerstore_ names)
│   │   ├── state.go       # health/info metrics from operational/lifecycle states
│   │   └── *_test.go      # mock PowerStore gateway (httptest TLS): login_session, topology, metrics, bulk
│   ├── config/  watcher.go (SIGHUP + fsnotify) — verbatim
│   ├── logging/ logging.go — verbatim
│   ├── telemetry/ manager.go — verbatim
│   └── utils/   env.go, file.go — verbatim
├── grafana/  block/ + file/ dashboards + provisioning/   (replaces gen1/gen2)
├── deploy/   kubernetes/ (deployment, service, servicemonitor, configmap, secret.example, kustomization),
│             prometheus/pstore.rules.yml, pstore_exporter.service, .env.example
├── docs/     MkDocs Material (index, metrics, dashboards, opentelemetry, alerting, cicd, getting-started)
└── .github/workflows/  ci.yml, release.yml, docs.yml  (adapted from pflex)
```

### Collection flow (per array, per cycle)

1. `GetTopology(ctx)` via gopowerstore: cluster, appliances, volumes, volume groups, drives
   (hardware), eth/fc ports, NAS servers, file systems, replication sessions. Build an id→entity
   index + parent/child relations for label resolution (array, appliance, volume_group, nas_server).
2. `detectCapability()` — inspect cluster/appliance software version (≥ 4.1 ⇒ bulk-capable).
   Result stored on the array snapshot and exposed as `powerstore_array_bulk_api{...}` info metric.
3. Metrics:
   - **Bulk-capable:** `bulk.go` enables `/latest_five_min_metrics`, downloads the gzipped tar of
     CSVs via raw `APIClient()`, parses each known CSV (`performance_metrics_by_*`,
     `space_metrics_by_*`, `wear_metrics_by_drive`) → `[]Sample`.
   - **Fallback:** `perentity.go` calls gopowerstore `PerformanceMetricsBy*` / `SpaceMetricsBy*`
     / `WearMetricsByDrive` with interval `Five_Mins`, taking the **latest** sample per entity.
4. Derivations map each response → `Sample{Name:"powerstore_<entity>_<metric>", Labels, Value}`,
   resolving identity + parent labels. Both paths emit the **same metric names and canonical label
   sets** (empty values where inapplicable) so dashboards work regardless of path — this is the
   pflex Device/Volume label-union discipline applied to PowerStore.
5. `collectArray` returns `ArraySnapshot{Array, Up, BulkCapable, Samples, LastScrape, ScrapeError}`;
   `collectAll` runs arrays in parallel (errgroup, never fails the group) and atomically publishes
   the new `Snapshot` to the store. Prometheus `/metrics` and OTLP both read from the store.

### Metric surface (prefix `powerstore_`)

- Health/info: `powerstore_up{array}`, `powerstore_last_scrape_timestamp_seconds{array}`,
  `powerstore_array_bulk_api{array}`, `powerstore_scrape_error{array}`.
- Performance (volume/appliance/vg/eth_port/fc_port/file_system/node): `_read_iops`, `_write_iops`,
  `_total_iops`, `_read_bandwidth_bytes_per_second`, `_write_bandwidth_bytes_per_second`,
  `_read_latency_microseconds`, `_write_latency_microseconds`, `_avg_io_size_bytes`,
  plus appliance `_io_workload_cpu_utilization`.
- Space (appliance/volume/vg/cluster): `_logical_provisioned_bytes`, `_logical_used_bytes`,
  `_physical_total_bytes`, `_physical_used_bytes`, `_data_reduction_ratio`,
  `_efficiency_ratio`, `_snapshot_savings_ratio`, `_thin_savings_ratio`.
- Drive wear: `powerstore_drive_wear_*`. Replication: transfer-rate / lag gauges.
- gopowerstore returns latency in **microseconds**, bandwidth in **bytes/sec**, IOPS already
  per-second — emit as gauges; aggregate with `sum`/`avg` in PromQL, **never `rate()`**.

### Config shape (`config.yaml`)

```yaml
server:      { host: "0.0.0.0", port: "9101", uri: "/metrics", logName: "" }
collection:  { interval: "30s", timeout: "20s" }           # PowerStore 5-min data → poll modestly
opentelemetry:
  metrics: { enabled: false, endpoint: "localhost:4317", insecure: true, interval: "30s" }
  tracing: { enabled: false, endpoint: "localhost:4317", insecure: true, samplingRate: 0.1 }
arrays:
  - name: pstore-1
    endpoint: "https://10.0.0.1/api/rest"
    username: admin
    password: "${PSTORE1_PASSWORD}"        # or passwordFile: /etc/pstore_exporter/p1.pass
    insecureSkipVerify: true
    # interval: Five_Mins   # optional per-entity metrics interval override
```

## Build sequence (TDD throughout; mock PowerStore gateway like pflex's mockGateway)

1. **Scaffold + config** — `go mod init`, add gopowerstore, port `internal/models`, `utils`,
   `logging`, `telemetry`, `config/watcher.go` (rename cluster→array, gateway→endpoint). Tests for
   config validation + `${ENV}`/passwordFile.
2. **Client + topology** — `ArrayClient` over `gopowerstore.NewClientWithArgs`; `GetTopology` +
   relations index. Mock gateway serving `login_session` + topology endpoints. Tests: auth reuse,
   topology parse, graceful array-down.
3. **Capability detection** — `capability.go` from cluster/appliance version; `powerstore_array_bulk_api`.
4. **Per-entity metrics path** — `perentity.go` + `derive_perentity.go` → samples. Tests assert
   sample names/labels/values against fixtures.
5. **Bulk CSV path** — `bulk.go` (enable, raw `APIClient()` download, gz/tar/CSV parse) +
   `derive_bulk.go`. Tests: parse a fixture tar; assert identical sample names/labels as per-entity.
6. **Snapshot + Prometheus collector** — port `snapshot.go`, `prometheus.go`, `state.go`. Tests via
   registry gather; verify label-union consistency across both paths.
7. **Collector loop** — port `collector.go` (errgroup, timeout, atomic publish) + `main.go`
   (Cobra `--config/--debug/--once`, HTTP `/metrics`+`/health`, SIGHUP/file-watch reload).
8. **OTLP export** — port `otlp.go`; test with ManualReader.
9. **Grafana dashboards** — adapt pflex dashboards into `grafana/block/` + `grafana/file/`
   (cluster overview, appliances, volumes, volume groups, ports, capacity, NAS, file systems,
   drives/wear). Datasource + dashboard provisioning.
10. **Docker + compose** — multi-stage Dockerfile (golang:1.26 → alpine, non-root, `EXPOSE 9101`);
    `docker-compose.yml` (exporter + prometheus + grafana + otel-collector) and `.ghcr.yml` variant;
    `prometheus.yml`, `otel-collector-config.yaml`.
11. **Deploy + docs + CI** — k8s manifests + ServiceMonitor + `pstore.rules.yml`; systemd unit;
    MkDocs site; `.github/workflows` ci/release/docs adapted (Semgrep gate, SBOM, multi-arch GHCR).

## Reuse map (copy-and-adapt from pflex, do not reinvent)

- `internal/models/{config,safe_config}.go`, `internal/utils/{env,file}.go`,
  `internal/config/watcher.go`, `internal/logging/logging.go`, `internal/telemetry/manager.go`,
  `internal/powerflex/{snapshot,prometheus,otlp,collector,metrics,tracing}.go`,
  `main.go`, `Dockerfile`, `docker-compose*.yml`, `Makefile`, `.github/workflows/*`, `mkdocs.yml`,
  `deploy/*`, `grafana/provisioning/*` — all near-verbatim with cluster→array / pflex→powerstore renames.
- **New / substantially rewritten:** `client.go` (gopowerstore wrapper), `capability.go`,
  `bulk.go`, `perentity.go`, `derive_bulk.go`, `derive_perentity.go`, dashboards, README, docs content.

## Verification (end-to-end)

- `make ci` (fmt-check, vet, lint, `go test -race`, govulncheck) passes; Semgrep clean (no inline suppressions).
- `go run . --config config.yaml --once` against the mock gateway prints metrics; unit tests assert
  `powerstore_*` names, labels, and that **bulk and per-entity paths produce identical sample sets**.
- `docker compose up` → exporter `:9101/metrics` scraped by Prometheus; Grafana (`:3000`, admin/admin)
  auto-provisions block + file dashboards and panels populate.
- (If a real array is reachable) point `config.yaml` at it, confirm `powerstore_up=1`, capability
  metric reflects OS version, and volume/appliance/NAS/file-system panels render.
- OTLP: enable in config, run otel-collector, confirm metrics arrive on the collector's `:8889`.

## Notes / risks

- **Bulk tar parsing** is the main net-new code (gopowerstore doesn't wrap it); fixture-driven tests
  de-risk it. If the raw `APIClient().Query()` can't stream a gzipped tar cleanly, fall back to a thin
  `net/http`/`resty` client that reuses the array credentials for `/latest_five_min_metrics` only.
- Keep **label sets identical** across bulk and per-entity paths (pflex's hardest constraint) — enforce
  via shared label-builder functions and a test that diffs the two paths' sample signatures.
- PowerStore data granularity is ~5 min (or 20 s); set `collection.interval` to ~30 s and dedupe by
  latest timestamp — no value in sub-5-min polling for `Five_Mins` data.
