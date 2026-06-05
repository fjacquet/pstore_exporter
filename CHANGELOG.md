# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/fjacquet/pstore_exporter/compare/v0.3.0...main
[0.3.0]: https://github.com/fjacquet/pstore_exporter/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/fjacquet/pstore_exporter/releases/tag/v0.2.0
[0.1.0]: https://github.com/fjacquet/pstore_exporter/releases/tag/v0.1.0
