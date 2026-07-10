# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-07-10

First stable release. Makes the bundled Grafana dashboards correct when one
exporter monitors multiple PowerStore arrays, and adds a CI guard so they stay
correct. See `docs/changelog.md` for the full entry.

### Changed

- Every aggregating dashboard panel now groups `by (array)`, rendering one labelled
  tile/series per array instead of a single blended value. Ratio rollups use
  `sum by (array)(…)/sum by (array)(…)` instead of a meaningless `avg()`-of-ratios.
- Series colour derives from the entity name (`palette-classic-by-name`), read solid /
  write dashed — supersedes the fixed `read=blue / write=orange` convention that collapsed
  entities into one colour (ADR-0016 supersedes ADR-0014's colour clause).

### Fixed

- Healthy arrays no longer render a missing tile on count panels (Unhealthy Drives, Ports
  Down, Sessions in Bad State) — those count always-`1` info series and are now zero-filled.
- "Top 10 Volumes" bandwidth panels no longer draw all ten volumes in one colour.

### Added

- CI linter (`internal/dashboards`, part of `make ci`) failing on panels that aggregate
  without `by (array)`, pin fixed read/write colours, or use catch-all fixed colours.

## [0.12.0] - 2026-07-09

### Added

- GoReleaser-driven release pipeline (`.goreleaser.yaml`): cross-platform archives, a
  multi-arch GHCR image, keyless cosign signing, and a Homebrew cask. `make release-snapshot`
  runs the pipeline as a local dry-run.

### Changed

- Replication metrics now carry the replicated resource's **name**, not just its uuid:
  `powerstore_replication_session_state` gains `local_resource_name`, and the transfer/backlog
  metrics gain `resource_name` (volume, volume_group, file_system, nas_server; falling back to
  the id when the resource is absent from inventory). The Replication / DR dashboard shows the
  resource name and reporting `array` in place of the session uuid.
- SBOM generation moved into GoReleaser (syft) as the single source of truth.

### Security

- Bumped the Go toolchain floor to **1.26.5** (`go.mod`), patching GO-2026-5856 and
  GO-2026-4970 (reachable via `crypto/tls` and `os` respectively). Dev `Dockerfile` pinned
  to `golang:1.26.5` to match. All GitHub Actions pinned to full commit SHAs.

## [0.11.0] - 2026-07-08

### Fixed

- `powerstore_drive_state` and `powerstore_drive_wear_level_ratio` were never
  emitted: the drive enumeration selected the non-existent
  `hardware.life_cycle_state` property, which returned an error and dropped all
  drive metrics. Corrected to the `lifecycle_state` property.
- Replication transfer and backlog metrics
  (`powerstore_replication_transfer_rate_bytes_per_second`,
  `powerstore_replication_data_remaining_bytes`) no longer emit phantom `0`/`0`
  series for protected volumes that have no active replication session.
- Inactive metro/replication-destination file systems (which report `size_total`
  as null, decoded to `0`) no longer emit misleading size-0 capacity and
  performance series.

### Added

- `powerstore_replication_session_state` now covers Metro sessions
  (`role=Metro_Preferred`): replication sessions are enumerated via the generic
  `replication_session` API instead of a protection-policy-gated probe, so Metro
  sessions — previously invisible to the exporter — are captured.
- `--debug` now logs a per-array count of bulk volume rows that could not be
  resolved to inventory (snapshots/clones). The emitted `volume_name` label is
  unchanged.

## [0.10.3] - 2026-07-03

### Added

- `pstore_exporter_build_info{version, goversion}` metric (constant `1`) exposing
  the running exporter version and Go version, following the standard Prometheus
  build-info pattern. Exporter-level (no `array` label).

## [0.10.2] - 2026-07-01

### Changed

- Documented handling of special characters in the monitoring password.
- Switched the MkDocs site to the brand icon for its favicon and logo.

## [0.10.1] - 2026-06-20

### Added

- Metro witness observability metrics.

### Changed

- Migrated CI to the `fjacquet/ci` reusable make-based workflows and made the
  `security` job advisory to match the fleet default.

## [0.10.0] - 2026-06-17

### Fixed

- PowerStore collector now paginates alerts, silences benign 404s, and caps
  fan-out concurrency.

## [0.9.1] - 2026-06-16

### Added

- Helm chart with lockstep publishing alongside the container image.

## [0.9.0] - 2026-06-14

### Changed

- **BREAKING:** canonical metrics port is now `9446`.

### Added

- Node Exporter Full (1860) Grafana dashboard.

## [0.8.0] - 2026-06-14

### Changed

- Reworked the Grafana dashboards: taxonomy, design system, and metric coverage.

## [0.7.1] - 2026-06-14

### Changed

- Added ADRs recording the config-reload, OTLP-push, credentials, and
  trace-scope decisions.

## [0.7.0] - 2026-06-14

### Changed

- Reconciled the exporter against the PowerStore REST API 4.4.0 reference.

### Fixed

- Spec-aligned the PowerStoreOS 4.4.0 performance fields and added Windows
  release builds.

## [0.6.1] - 2026-06-12

### Fixed

- Docker image copies the CA bundle from the builder stage instead of installing
  it via `apk`.

## [0.6.0] - 2026-06-12

### Added

- Native `.env` loading at startup (no-override semantics).

## [0.5.0] - 2026-06-11

### Added

- `${ENV}` expansion in the configured username and parameterization of the
  Compose stack via `PSTORE1_*` variables.

## [0.4.0] - 2026-06-11

### Added

- `--trace` flag and `--once --debug` sample dump for live-array validation.

## [0.3.1] - 2026-06-06

Internal code-quality cleanup of the metric-coverage collectors. No behavior
change — metric names, labels, and values are identical to 0.3.0.

### Changed

- Centralized the metrics shared by the bulk and per-entity export paths into a
  single `commonMetrics` helper so the metric-parity invariant is structural
  rather than maintained by convention.
- Extracted a generic concurrent fan-out helper (`parallelSamples`) for the
  per-file-system and per-volume-group performance collectors, removing
  duplicated errgroup boilerplate and unnecessary mutexes.
- Routed the drive lifecycle-state series through a shared label builder and made
  the alert series emission deterministic.

## [0.3.0] - 2026-06-05

Metric coverage expansion from reconciling the exporter against the full PowerStore
REST API reference and the pinned `gopowerstore` v1.22.0 surface (library-first; no
dependency bump). See `docs/reconciliation-2026-06-05.md` and ADR-0009.

### Added

- Hardware alert metrics (`powerstore_alert_active`): count of active alerts by
  severity, via `GetAlerts`, with stable zero series for alerting rules.
- Replication & protection metrics (`powerstore_replication_*`): session-state info
  series, RPO seconds, mirror transfer rate, and data-remaining backlog, via
  `GetReplicationRules` / `GetReplicationSessionByLocalResourceID` /
  `VolumeMirrorTransferRate`.
- File-system performance metrics (`powerstore_file_system_*` IOPS/bandwidth/latency)
  via `PerformanceMetricsByFileSystem`, parallel to the volume metrics.
- Volume-group performance metrics (`powerstore_volume_group_*`) via
  `PerformanceMetricsByVg`.
- Cluster capacity metrics (`powerstore_cluster_*`: physical/logical space,
  data-reduction and efficiency ratios) via `SpaceMetricsByCluster`, for capacity
  forecasting.
- Drive wear and lifecycle-state metrics (`powerstore_drive_wear_level_ratio`,
  `powerstore_drive_state`) from a single generic `hardware` enumeration.

### Fixed

- Corrected stale documentation (ADR-0003, `CLAUDE.md`, `docs/metrics.md`) that
  claimed `PerformanceMetricsByFileSystem` was unavailable in gopowerstore v1.22;
  the method exists and is now used. Recorded the decision in ADR-0009 and audited
  the existing metric mappings (field names, units, gauge semantics) against the
  REST API spec.

## [0.1.0] - 2026-06-04

### Added

- Snapshot-model collector: a single background loop polls all arrays per interval
  and publishes an immutable snapshot to a store; both the Prometheus `/metrics`
  endpoint and the optional OTLP push read from that store, decoupling API load
  from scrape frequency.
- Multi-array support: one exporter process monitors many arrays simultaneously;
  every metric carries an `array` label; arrays are polled in parallel with
  graceful per-array degradation (one array's failure does not affect others,
  surfaced as `powerstore_up{array} 0`).
- Dell `gopowerstore` v1.22.0 client for login-session token/cookie auth, topology
  enumeration, and per-entity performance metrics.
- Auto-detected metrics collection paths: bulk compressed-CSV API on PowerStoreOS
  ≥ 4.1 (detected via `GetSoftwareMajorMinorVersion`), with automatic per-entity
  `metrics/generate` fallback for older firmware or on bulk-fetch failure; both
  paths emit identical metric names and label keys.
- Metric coverage: appliance performance (IOPS, bandwidth, latency) and space;
  volume performance; file-system capacity; port link status.
- Hot config reload via SIGHUP and file-system watch (fsnotify).
- Grafana dashboards for block and file workloads with provisioning configuration.
- Docker Compose stack with Prometheus, Grafana, and optional OTEL Collector.
- Kubernetes manifests including Deployment, ServiceMonitor, and alert rules.
- systemd unit file for bare-metal deployment.
- MkDocs-Material documentation site.
- GitHub Actions workflows for CI, release, and docs publication.

[Unreleased]: https://github.com/fjacquet/pstore_exporter/compare/v0.11.0...main
[0.11.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.10.3...v0.11.0
[0.10.3]: https://github.com/fjacquet/pstore_exporter/compare/v0.10.2...v0.10.3
[0.10.2]: https://github.com/fjacquet/pstore_exporter/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/fjacquet/pstore_exporter/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/fjacquet/pstore_exporter/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/fjacquet/pstore_exporter/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/fjacquet/pstore_exporter/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/fjacquet/pstore_exporter/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/fjacquet/pstore_exporter/releases/tag/v0.2.0
[0.1.0]: https://github.com/fjacquet/pstore_exporter/releases/tag/v0.1.0
