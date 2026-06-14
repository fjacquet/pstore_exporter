# 11. OTLP Push Export via Observable Gauges

## Status

Accepted

## Context

ADR-0002 established the snapshot model and named both readers — the Prometheus
`/metrics` handler and the OTLP exporter — but recorded only the *pull* path's
behavior. The OTLP *push* path has its own design decisions worth capturing: how
instruments map to the snapshot, how new metric names are handled, and how push
cadence relates to collection cadence.

OTLP is optional: many deployments scrape Prometheus directly and never enable
it. When enabled, the exporter pushes to an OTLP gRPC endpoint (typically an
OpenTelemetry Collector). The metric set is not static — the bulk/per-entity
paths (ADR-0004) and library-first expansion (ADR-0009) mean new metric names
can appear, and a config reload (ADR-0010) can introduce arrays that surface
additional names.

## Decision

OTLP export is an optional, push-based reader over the same immutable snapshot:

- **Observable (asynchronous) gauges.** Each metric name is registered as a
  `Float64ObservableGauge`. A callback reads the *current* snapshot at collection
  time and emits one observation per sample, with the sample's labels mapped to
  OTLP attributes. The exporter holds no metric state of its own — the snapshot
  remains the single source of truth, consistent with ADR-0002. Gauges (not
  counters) match the value semantics fixed in ADR-0006.
- **Periodic reader drives push cadence.** A `PeriodicReader` exports on its own
  interval, decoupled from the background collection interval. Each push reflects
  whatever snapshot is current; the reader never triggers a backend call.
- **`EnsureInstruments` is idempotent and reload-aware.** It registers an
  observable gauge for every metric name present in the current snapshot. It is
  called at startup and again after a reload that changes the array set, so
  newly appearing metric names get instruments without restarting the process.
- **Test seam.** `newOTLPExporter` accepts an injected `Reader` (a
  `ManualReader` in tests) so export can be driven deterministically offline.

## Consequences

- Push and pull export identical values from the same snapshot; OTLP adds no
  parallel collection path and cannot diverge from `/metrics`.
- Observable gauges mean the exporter carries no mutable accumulator — there is
  nothing to reset, and a missed push simply skips one interval rather than
  losing a counter delta.
- Push cadence and collection cadence are independent; an aggressive OTLP
  interval re-pushes the same snapshot rather than hammering the array.
- New metric names require an `EnsureInstruments` call to become visible over
  OTLP. This is wired into startup and reload; any future code path that can
  introduce metric names must call it too, or those series will be absent from
  the push stream while still present on `/metrics`.
- Downstream consumers must treat every PowerStore metric as a gauge and
  aggregate with `sum`/`avg`, never `rate()` (ADR-0006).
