# PowerStore Exporter

A Go exporter for **Dell PowerStore** arrays. It authenticates to the PowerStore REST API,
collects the full performance and capacity statistic set across all object types, and
exposes the metrics via **both** a Prometheus `/metrics` endpoint **and** an OTLP
metric push.

## Features

- **Dual export** — Prometheus pull (`/metrics`) and OTLP metric push, fed from one shared snapshot.
- **Two collection paths, auto-detected** — bulk CSV (PowerStoreOS 4.1+) via the compressed
  stats archive endpoint; per-entity REST fallback for older firmware — chosen automatically
  per array.
- **Multi-array** — one process monitors many arrays; every metric carries an `array` label.
- **Operational** — bearer auth with automatic token refresh, graceful per-array degradation,
  hot config reload (SIGHUP + file watch), snapshot-based health endpoint, optional OTLP
  tracing.

## Architecture

A single background **collection loop** polls every configured array on `collection.interval`,
derives metrics, and publishes an immutable **snapshot** (RWMutex pointer-swap). Both the
Prometheus collector (`Collect()` reads the snapshot) and the OTLP exporter (observable
gauges read the snapshot) serve from that snapshot, so API load is independent of the
number of scrapers and the push cadence.

```
PowerStore arrays ──► ArrayClient (auth + REST) ──► collection loop ──► SnapshotStore
                                                                           │      │
                                                               Prometheus ◄┘      └► OTLP push
```

The cost of the snapshot model is up to one `collection.interval` of staleness on a
scrape (acceptable — Dell's own pipeline polls every 30s); it is surfaced via
`powerstore_last_scrape_timestamp_seconds`.

## Scope

Supports PowerStore arrays running PowerStoreOS 3.x and later. The exporter auto-detects
the best collection path per array:

- **Bulk CSV** path (PowerStoreOS 4.1+): downloads the compressed stats archive in one
  API call and parses all entity types from it — the preferred path.
- **Per-entity REST** path (fallback): issues one REST query per entity type per cycle.

A single binary can simultaneously monitor arrays on different firmware versions.

See [Installation](getting-started/installation.md) to get started.
