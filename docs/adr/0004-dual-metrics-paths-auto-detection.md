# 4. Dual Metrics Paths with Auto-Detection

## Status

Accepted

## Context

PowerStore arrays running firmware ≥ 4.1 expose a bulk compressed-CSV endpoint
that returns all performance statistics for all entity types in a single API
call, downloadable once per collection cycle. Older firmware lacks this endpoint
and requires a separate REST call per entity type (`metrics/generate`). The bulk
path is dramatically more efficient on large arrays with many volumes, but we
cannot require ≥ 4.1 firmware and must support older arrays in production.

## Decision

The collector supports two collection paths, automatically selected per array:

1. **Bulk CSV** (PowerStoreOS ≥ 4.1): downloads the compressed stats archive
   once per cycle and parses all entity types from a single API response. The
   firmware version is detected at startup via `GetSoftwareMajorMinorVersion`.
2. **Per-entity REST** (fallback): issues one `metrics/generate` REST call per
   entity type per cycle. Used when bulk export is unavailable or when a bulk
   fetch fails at runtime (the collector falls back transparently).

**Invariant:** both paths emit *identical metric names and label keys* for every
entity type. This invariant is enforced by a parity test in the test suite, so
dashboards and alerts work regardless of which path is active on a given array.

The detected path is published as the gauge `powerstore_array_bulk_api`
(1 = bulk, 0 = per-entity).

## Consequences

- Efficient operation on modern firmware; graceful degradation on older firmware.
- Shared label-builder functions are required to ensure name/label parity across
  both derive families — any new metric must be added to both paths.
- The bulk download path is not directly offline-testable (it requires a live
  array or a faithful HTTP mock); the per-entity path is fully mockable.
- The parity test is a hard gate in CI: breaking metric-name or label-key
  consistency between paths fails the build.
