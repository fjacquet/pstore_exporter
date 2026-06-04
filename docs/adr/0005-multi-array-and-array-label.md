# 5. Multi-Array Support and Array Label

## Status

Accepted

## Context

Organizations typically operate more than one PowerStore array (production,
DR, dev/test). Running a separate exporter process per array multiplies
operational overhead and makes cross-array PromQL comparisons awkward. A single
exporter process that monitors all arrays is simpler to deploy and maintain.

## Decision

One `pstore_exporter` process monitors all configured arrays. Every metric
carries an `array` label set to the configured array name, enabling filtering
and aggregation by array in PromQL. Arrays are polled concurrently in the
background collection loop using Go goroutines. A failure on one array (network
unreachable, auth error, API timeout) does not affect collection for other
arrays; the failed array is surfaced as `powerstore_up{array="<name>"} 0` and
the snapshot for that array is marked stale rather than omitted.

Per-array credentials are specified in the config file using environment-variable
interpolation (`${ENV_VAR}`) or a `passwordFile` path, so secrets never appear
in the config file itself.

## Consequences

- Simple horizontal PromQL aggregation: `sum by (array)(powerstore_volume_read_iops)`.
- No leader election or HA coordination is needed; the process is stateless
  between collection cycles.
- Config is a list of arrays, each with its own `endpoint`, `username`,
  `password`/`passwordFile`, and optional `insecureSkipVerify`.
- A single process failure affects monitoring for all arrays simultaneously
  (mitigated by standard process supervision and the Kubernetes Deployment
  restart policy).
- Memory footprint scales with the number of arrays and their entity counts,
  but remains modest in practice (one snapshot per array in RAM).
