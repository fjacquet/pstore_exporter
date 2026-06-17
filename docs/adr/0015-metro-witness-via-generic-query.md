# 15. Metro witness observability via the generic Query escape hatch

Date: 2026-06-17

## Status

Accepted

## Context

The exporter covers replication sessions, RPO, and transfer health, but not the
Metro witness — the third-site arbitrator that decides which side keeps serving
I/O when a Metro session fractures. A degraded witness silently removes the
automatic-failover guarantee, and the exporter could not see it.

PowerStore exposes the witness as a first-class resource (`GET /witness`,
PowerStoreOS 3.6+; schema confirmed in `docs/swagger/5504-4.4.0.json`). The
`gopowerstore` v1.22 SDK has no typed witness method.

## Decision

Read `/witness` through the generic `APIClient().Query` escape hatch, the same
sanctioned fallback used for drive enumeration (ADR-0009), decoding into
repo-local `witnessInfo`/`witnessConnection` structs.

Emit two info-style gauges (value `1`, state in a label):
`powerstore_metro_witness_state` (overall service state) and
`powerstore_metro_witness_connection_state` (per node).

Gate on capability by tolerating the benign `404` (`isNotFound`) rather than a
software-version check — simpler and more robust. A 404 or empty list yields no
samples; `powerstore_up` stays `1`.

Per-session witness engagement (`replication_session.witness_details`), remote
systems, and protection policies are out of scope for this increment.

## Consequences

- No new SDK dependency; consistent with the drives pattern.
- The fetch is not offline-unit-testable (generic Query needs a live array);
  `deriveWitness` is unit-tested, the fetch is validated via `--once --debug --trace`.
- New label keys `witness_id`, `witness_name`, `node_id` enter the metric set.
