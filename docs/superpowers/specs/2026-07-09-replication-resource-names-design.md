# Replication session resource names — design

**Date:** 2026-07-09
**Status:** Approved

## Problem

The `PowerStore — Replication / DR` dashboard identifies replicated devices by UUID.
The Sessions table leads with the session UUID; the RPO, Replication Backlog, and
Transfer Rate panels legend on the resource UUID. An operator reading the panel cannot
tell which volume is affected without a separate lookup.

Two contributing causes:

1. `powerstore_replication_session_state` carries `local_resource_id` (the device UUID)
   but no name. `powerstore_replication_transfer_rate_bytes_per_second` and
   `powerstore_replication_data_remaining_bytes` carry `resource_id`, likewise unnamed.
2. The dashboard's `organize` transform excludes both `local_resource_id` and `array`,
   so the two rows of a Metro pair — same logical session, reported once by each array,
   differing only in `role` — render as indistinguishable duplicates.

## Non-goals

`remote_system_name` is **out of scope**. `remote_system_id` refers to an object on the
peer array; resolving it requires adding a `GetAllRemoteSystems` fetch to the collection
cycle plus a new `Topology` index, mock, and failure path. That remains the deferred
data-protection roadmap item and gets its own spec.

The `Session` column keeps no friendlier form: a session UUID is the session's own
identity, not a device. It is dropped from the table rather than resolved.

## Design

### Topology

`Topology` gains an `fsName map[string]string` index, populated in `NewTopology` from the
`FileSystems` slice it already receives — a loop alongside the existing `nasName` one.
No new inventory fetch: every map this design reads is built from data the collector
pulls each cycle regardless.

A single resolver dispatches on the session's resource type:

```go
// ResourceName resolves a replication session's local resource id to its name,
// dispatching on the session's resource_type. Unknown ids and unknown types fall
// back to the id so the label is never empty.
func (t *Topology) ResourceName(resourceType, id string) string
```

It switches over `"volume"`, `"volume_group"`, `"file_system"`, and `"nas_server"`
against `volumeName`, `vgName`, `fsName`, and `nasName` respectively.

**Fallback:** on an unknown id, an unknown resource type, or an empty id, return `id`
unchanged. This follows the precedent set by `Topology.VolumeInfo` and its callers in
`derive_bulk.go`, where an unresolvable volume degrades to its id. The panel therefore
always shows something, and the worst case is exactly today's behavior.

### Labels

In `metrics.go`:

- `replicationSessionLabels` takes an additional `localResourceName` parameter and emits
  `local_resource_name` immediately after `local_resource_id`.
- `replicationResourceLabels` takes an additional `resourceName` parameter and emits
  `resource_name` immediately after `resource_id`.

Both follow the id/name adjacency convention already used by `volumeLabels`,
`applianceLabels`, `fileSystemLabels`, and `driveLabels`.

### Derive

In `derive_replication.go`:

- `deriveReplicationSessions` calls `topo.ResourceName(s.ResourceType, s.LocalResourceID)`
  and threads the result into `replicationSessionLabels`.
- `deriveReplicationTransfer` already receives `resourceType`; it resolves the same way
  and threads the result into `replicationResourceLabels`. Note that
  `replicatedVolumeResources` filters to `resource_type == "volume"`, so this path only
  ever passes `"volume"` today. It calls the same generic resolver anyway rather than
  indexing `volumeName` directly, so the code stays correct if the transfer path is later
  widened to other resource types.

No change to `replicatedVolumeResources` or its dedup behavior.

### Dashboard

`grafana/dashboards/protection/01-replication.json`, panel `Replication Sessions`:

- Remove `local_resource_name` and `array` from the `organize` transform's
  `excludeByName`. Keep `local_resource_id`, `cluster_id`, `Time`, `Value`, and
  `__name__` excluded.
- Add `session_id` to `excludeByName`.
- `indexByName` orders: `array`, `local_resource_name`, `resource_type`, `role`, `type`,
  `remote_system_id`, `state`.
- `renameByName` maps `array` → `Array` and `local_resource_name` → `Resource`; existing
  renames are retained.

Panels `RPO`, `Replication Backlog`, and `Transfer Rate`: switch the series legend format
from the resource UUID to `{{resource_name}}`.

The `Sessions in Bad State` stat panel is unaffected — its
`count(powerstore_replication_session_state{...state=~"..."})` selector does not reference
the changed labels.

## Compatibility

This widens the label set on three existing metric families. Exact-match `on(...)` joins
against `powerstore_replication_session_state`,
`powerstore_replication_transfer_rate_bytes_per_second`, or
`powerstore_replication_data_remaining_bytes` in downstream recording rules or alerts
could be affected. Label *selectors* (including every selector in the bundled dashboards
and `alerting.md`) are unaffected, since Prometheus selectors ignore labels they do not
name.

Per repo convention, `docs/metrics.md` and the Grafana JSON move in the same commit as
the metric change.

## Testing

TDD, at the existing seams:

- `topology_test.go`: `ResourceName` across all four resource types, an unknown id, an
  unknown resource type, and an empty id.
- `derive_replication_test.go`: table cases asserting `local_resource_name` on sessions of
  each resource type and the id-fallback case; `resource_name` on the transfer and
  backlog samples.
- `pipeline_integration_test.go`: widen the expected replication label set.

The replication path is not covered by the bulk/per-entity parity test, so that invariant
is untouched.

## Documentation

- `docs/metrics.md`: add the `local_resource_name` and `resource_name` label rows to the
  three replication metrics.
- `docs/changelog.md`: entry under the unreleased heading.
