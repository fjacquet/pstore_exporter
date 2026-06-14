# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the
`pstore_exporter` project. ADRs capture significant architectural choices,
the context that drove them, and their consequences — providing a durable
record of *why*, not just *what*.

Records are numbered sequentially and use the MADR-lite format.

| # | Title | Status |
|---|-------|--------|
| [0001](0001-record-architecture-decisions.md) | Record Architecture Decisions | Accepted |
| [0002](0002-snapshot-collection-model.md) | Snapshot Collection Model | Accepted |
| [0003](0003-use-gopowerstore-client.md) | Use gopowerstore Client | Accepted |
| [0004](0004-dual-metrics-paths-auto-detection.md) | Dual Metrics Paths with Auto-Detection | Accepted |
| [0005](0005-multi-array-and-array-label.md) | Multi-Array Support and Array Label | Accepted |
| [0006](0006-metric-naming-and-units.md) | Metric Naming and Units | Accepted |
| [0007](0007-serve-http-before-first-collection.md) | Serve HTTP Before First Collection | Accepted |
| [0008](0008-goreleaser-sha-pinning-and-signing.md) | GoReleaser, SHA-Pinned Actions, and Signed Artifacts | Accepted |
| [0009](0009-expand-metric-coverage-library-first.md) | Expand Metric Coverage, Library-First | Accepted |
| [0010](0010-config-hot-reload.md) | Configuration Hot-Reload via SIGHUP and File-Watch | Accepted |
| [0011](0011-otlp-push-export.md) | OTLP Push Export via Observable Gauges | Accepted |
| [0012](0012-credentials-via-env-and-file.md) | Credentials via Environment, `${ENV}` Interpolation, and Password Files | Accepted |
| [0013](0013-trace-transport-scope.md) | `--trace` HTTP Transport and Its Scope Limitation | Accepted |
