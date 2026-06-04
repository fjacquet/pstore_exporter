# 2. Snapshot Collection Model

## Status

Accepted

## Context

PowerStore performance data has approximately 5-minute granularity — the array
only refreshes its internal counters on that cadence. The array REST API is also
rate-limited. The naive Prometheus exporter pattern — polling the backend on
every scrape — would hammer the array API on every `/metrics` call and would
scale linearly with the number of scrapers (Prometheus, alerting replicas, etc.).
With multiple arrays and a busy Prometheus setup this becomes untenable.

The `pflex_exporter` project (the template for this exporter) proved this pattern
at scale with a similar API surface.

## Decision

A single background collection loop polls all arrays once per configured interval
and publishes an immutable `Snapshot` struct to a shared store via a pointer-swap
protected by an `RWMutex`. Both the Prometheus collector (`/metrics` handler) and
the OTLP exporter read from the current snapshot in the store. Neither reader
blocks or triggers a backend call; collection is entirely decoupled from scrape
frequency.

## Consequences

- **API load is constant**: one poll cycle per interval regardless of how many
  scrapers or OTLP consumers are attached.
- **Metrics are eventually consistent**: data can be up to one interval stale
  at the moment of a scrape. This is acceptable given the 5-minute native
  granularity.
- **Readers never block collection**: the RWMutex pointer-swap pattern lets
  readers hold the old snapshot while the writer atomically publishes a new one.
- **Failure isolation**: a collection error for one array produces a stale but
  valid snapshot for that array; other arrays are unaffected.
- The snapshot store becomes a critical internal interface; its shape must remain
  stable or be versioned carefully when the metric set changes.
