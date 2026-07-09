# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- GoReleaser-driven release pipeline (`.goreleaser.yaml`): cross-platform archives,
  `checksums.txt`, multi-arch GHCR image, and a Homebrew cask published to
  `fjacquet/homebrew-tap` (`brew install fjacquet/tap/pstore_exporter`).
- Keyless **cosign** signature of the release checksums (Sigstore bundle,
  `checksums.txt.sigstore.json`) via GitHub OIDC — no long-lived keys.
- `make release-snapshot` for a local dry-run of the full release pipeline.

### Changed

- Replication metrics now carry the replicated resource's **name**, not just its uuid:
  `powerstore_replication_session_state` gains `local_resource_name`, and
  `powerstore_replication_transfer_rate_bytes_per_second` /
  `powerstore_replication_data_remaining_bytes` gain `resource_name`. Names resolve for
  volume, volume_group, file_system, and nas_server sessions, falling back to the id when
  the resource is absent from the inventory. The Replication / DR dashboard now shows the
  resource name and the reporting `array` (which disambiguates the two rows of a Metro
  pair) in place of the session uuid.
- SBOM generation moved into GoReleaser (syft) as the single source of truth,
  replacing `cyclonedx-gomod`. Releases now ship a CycloneDX SBOM per archive
  (`*.cdx.json`); CI regenerates them in snapshot mode. `make tools` no longer
  installs `cyclonedx-gomod`.
- Release artifacts are now `.tar.gz` archives (was raw binaries).
- Dev `Dockerfile` build stage pinned to `golang:1.26.4` (matches `go.mod`).

### Security

- Bumped the Go toolchain floor to **1.26.5** (`go.mod`), patching two standard-library
  vulnerabilities reported by `govulncheck`: [GO-2026-5856](https://pkg.go.dev/vuln/GO-2026-5856)
  (reachable via `crypto/tls` from the exporter's HTTP transport) and
  [GO-2026-4970](https://pkg.go.dev/vuln/GO-2026-4970) (root escape via symlink in `os`,
  reachable through `gopowerstore.NewClientWithArgs`). Both are fixed only in `go1.26.5`.
- All third-party and first-party GitHub Actions are now pinned to full commit
  SHAs (with `# vX` comments) across `ci.yml`, `release.yml`, and `docs.yml`,
  hardening the CI/CD supply chain against mutable-tag attacks. See
  [ADR 0008](adr/0008-goreleaser-sha-pinning-and-signing.md).

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

[Unreleased]: https://github.com/fjacquet/pstore_exporter/compare/v0.1.0...main
[0.1.0]: https://github.com/fjacquet/pstore_exporter/releases/tag/v0.1.0
