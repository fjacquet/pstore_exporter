# 9. Expand Metric Coverage, Library-First

## Status

Accepted

## Context

The exporter emits six metric families (volume/appliance performance, appliance space,
file-system capacity, port link status, meta). A reconciliation of the application against
the full PowerStore REST API reference (4,318 pp., 2026-05-29) and the pinned
`gopowerstore` v1.22.0 surface — recorded in `docs/reconciliation-2026-06-05.md` — found
that the library already exposes typed methods for nearly every gap an operator cares about:
replication, hardware alerts, drive wear, file-system and volume-group performance, and
cluster/volume space.

The reconciliation also surfaced a stale claim: **ADR-0003 states "There is no
`PerformanceMetricsByFileSystem` method."** That was true for the earlier client version it
described, but is **false in v1.22.0** — the method exists and the
`performance_metrics_by_file_system` endpoint appears 58× in the spec. Drive wear
(`WearMetricsByDrive`) and volume-group performance (`PerformanceMetricsByVg`), both listed
as "deferred / not yet available" in `docs/metrics.md`, are likewise present as typed
methods. They are *unimplemented*, not *unavailable*.

## Decision

1. **Expand coverage library-first.** New metrics map to existing `gopowerstore` typed
   methods. The raw/generic `APIClient().Query` path is used only where the library has no
   typed method (e.g. enumerating drive IDs, as we already do for appliances). No
   hand-rolled REST/auth clients beyond the existing bulk-CSV path.
2. **No dependency bump is required.** v1.22.0 is the latest published tag and already
   carries the needed methods.
3. **Adopt the prioritized backlog** in `docs/reconciliation-2026-06-05.md`: P1 hardware
   alerts and replication; P2 file-system & volume-group performance and capacity; P3
   per-protocol/node detail deferred under YAGNI until a dashboard needs it.
4. **Preserve the parity invariant.** Domains absent from the bulk CSV set (alerts,
   replication, drive wear, and the typed FS/VG performance calls) are derived via typed
   calls and appended to **both** `BulkMetrics` and `PerEntityMetrics`, exactly as
   `deriveFileSystemCapacity` and `derivePortLinkStatus` are today.
5. **Supersede the stale portion of ADR-0003.** The "no `PerformanceMetricsByFileSystem`"
   workaround no longer applies; file-system capacity-from-inventory remains valid as a
   fallback but live FS performance is now the intended path.

This ADR records the decision and order only; each backlog item ships in its own change with
its own derive function, shared label builders (`metrics.go`), parity/unit tests, and
`docs/metrics.md` update.

## Consequences

**Benefits**
- Replication, hardware-fault, file-performance, and capacity visibility — the operator
  priorities — without new auth/HTTP code.
- Honest docs: the "deferred because unavailable" list is corrected to "available, not yet
  wired."

**Costs / risks**
- More per-entity API calls per cycle (more sessions, more latency); each new domain must
  degrade gracefully per-entity like the existing paths and respect the collection timeout.
- Enum-valued fields (replication state, RPO) require explicit numeric mappings that must be
  kept in sync with the API; document the mapping next to the derive function.
- Drive-wear enumeration depends on the generic `Query` escape hatch and is therefore the
  most fragile item — scheduled last (P3).

**Supersedes:** the `PerformanceMetricsByFileSystem` workaround note in ADR-0003.
