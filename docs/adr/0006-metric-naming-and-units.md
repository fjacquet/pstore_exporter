# 6. Metric Naming and Units

## Status

Accepted

## Context

Prometheus metric naming conventions require explicit unit suffixes and
consistent prefixes to avoid collisions and to communicate semantics clearly.
Dell's own PowerStore exporter uses the `powerstore_` prefix, so adopting the
same convention eases migration for users switching between exporters. The
units of the raw values returned by the PowerStore API are not always obvious
and must be documented in metric names to prevent misuse (particularly the risk
of applying `rate()` to values that are already per-second gauges).

## Decision

- All metrics are prefixed `powerstore_` (matching Dell's own exporter for
  migration ease).
- Metric names include explicit unit suffixes:
  - `_bytes` — absolute byte counts (capacity, space used)
  - `_bytes_per_second` — bandwidth (already per-second from the array)
  - `_microseconds` — latency values
  - `_iops` — I/O operations per second (already per-second from the array)
  - Ratios and status flags are unitless (no suffix)
- IOPS and bandwidth values are emitted as **Gauges**, not Counters, because
  the PowerStore API delivers them as pre-computed per-second averages over the
  collection interval.

## Consequences

- Aggregation in PromQL must use `sum`/`avg`, **never `rate()`**: wrapping these
  Gauges in `rate()` double-rates the values and produces incorrect results.
  This is documented prominently in the README and the Metrics Reference.
- The `_bytes`, `_bytes_per_second`, `_microseconds` suffixes make the unit
  machine-readable for Prometheus/Grafana unit auto-detection.
- New metrics added to the exporter must follow the same naming discipline;
  deviations should require a new ADR.
- Users migrating from Dell's native exporter will find familiar metric names,
  reducing dashboard porting effort.
