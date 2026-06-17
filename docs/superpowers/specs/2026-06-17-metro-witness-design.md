# Metro witness observability — design

**Date:** 2026-06-17
**Status:** Approved (design); pending implementation plan
**Scope:** First increment of the data-protection observability effort.

## Problem

`pstore_exporter` already exposes replication observability — session state
(`powerstore_replication_session_state`), configured RPO
(`powerstore_replication_rpo_seconds`), and transfer health
(`powerstore_replication_transfer_rate_bytes_per_second`,
`powerstore_replication_data_remaining_bytes`), with docs and a Grafana dashboard
(`grafana/dashboards/protection/01-replication.json`).

The **Metro witness** — the third-site arbitrator that prevents split-brain by
deciding which side keeps serving I/O when a Metro session fractures — has **no
coverage**: no metric, no doc, no panel. A degraded or disconnected witness silently
removes the automatic-failover guarantee for every Metro volume, and today the
exporter cannot see it. This increment closes that gap.

PowerStore exposes the witness as a first-class resource (`GET /witness`,
PowerStoreOS 3.6+; schema confirmed against `docs/swagger/5504-4.4.0.json`, API
4.4.0.0). A single witness instance protects all Metro sessions on the cluster and is
registered on both peers.

## Goal

Emit the witness service's overall state and its per-node connection state as
Prometheus + OTLP gauges, degrading gracefully on arrays where the feature is absent
or no witness is configured.

## Non-goals (deferred to later increments)

- Per-session witness engagement (`replication_session.witness_details.state`).
- Remote systems (`remote_system`).
- Protection policies (the remainder of the rules/policies inventory).

## Source of truth (PowerStoreOS 4.4 swagger)

`witness_instance` (`GET /witness` collection, `/witness/{id}` instance):

| Field | Type | Notes |
|---|---|---|
| `id` | string | Unique identifier |
| `name` | string | User-provided name |
| `address` | string | IPv4/IPv6/FQDN (not exported — see below) |
| `state` | `WitnessStateEnum` | `OK` / `Partially_Connected` / `Disconnected` / `Deleting` / `Initializing` |
| `connections[]` | `witness_connection_instance` | Per node on each appliance |

`witness_connection_instance`:

| Field | Type | Notes |
|---|---|---|
| `state` | `WitnessConnectionStateEnum` | `OK` / `Disconnected` / `Initializing` |
| `appliance_id` | string | Appliance hosting the node |
| `node_id` | string | Node whose connection is described |
| `last_updated_timestamp` | string | Not exported (YAGNI) |

## Approach

Mirror the existing **drive enumeration** path (`derive_drives.go` +
`ArrayClient.enumerateDrives` + `driveMetrics`, wired in `commonMetrics`). The
`gopowerstore` SDK has no typed witness method, so the resource is fetched through the
generic `APIClient().Query` escape hatch — the same sanctioned fallback used for
drives (ADR-0009).

```
commonMetrics(ctx, topo)                       // emitted on BOTH bulk & per-entity paths
  └─ c.witnessMetrics(ctx, topo)
       ├─ c.enumerateWitnesses(ctx) []witnessInfo   // GET /witness, generic Query, paginated
       └─ deriveWitness(c.name, topo, witnesses) []Sample
```

Wiring it into `commonMetrics` (client.go:525) gives structural metric parity across
the two export paths for free, exactly as drives do.

### Rejected alternatives

- **Typed SDK per-resource calls** — no typed witness method exists; non-starter.
- **Version-gate on `GetSoftwareMajorMinorVersion` ≥ 3.6** — more code and more
  brittle than simply tolerating the benign `404` on unsupported arrays.
- **Per-session witness engagement now** — valuable but couples to the existing
  replication fetch (needs a raw `witness_details` field query the SDK struct omits);
  deferred to keep this increment standalone.

## Components

### New: `internal/powerstore/derive_witness.go`

```go
// witnessConnection is the subset of a witness_connection_instance we export.
type witnessConnection struct {
    State       string `json:"state"`
    ApplianceID string `json:"appliance_id"`
    NodeID      string `json:"node_id"`
}

// witnessInfo is the subset of a PowerStore witness_instance we map to metrics.
// PowerStore (and gopowerstore) expose no typed witness method, so these are
// decoded from a generic GET on the /witness resource (see ADR-0009, ADR-0015).
type witnessInfo struct {
    ID          string              `json:"id"`
    Name        string              `json:"name"`
    State       string              `json:"state"`
    Connections []witnessConnection `json:"connections"`
}

// deriveWitness emits the witness service state (info series) and one connection
// state series per node. Both are info-style: value is always 1, the state is a
// label (the kube-state-metrics enum idiom). Operators alert on undesirable
// states, e.g. {state=~"Disconnected|Partially_Connected"}.
func deriveWitness(array string, topo *Topology, witnesses []witnessInfo) []Sample {
    clusterID := topo.ClusterID()
    var out []Sample
    for _, w := range witnesses {
        if w.ID == "" {
            continue
        }
        if w.State != "" {
            out = append(out, Sample{"powerstore_metro_witness_state",
                witnessStateLabels(array, clusterID, w.ID, w.Name, w.State), 1})
        }
        for _, conn := range w.Connections {
            if conn.State == "" {
                continue
            }
            out = append(out, Sample{"powerstore_metro_witness_connection_state",
                witnessConnectionLabels(array, clusterID, w.ID, conn.ApplianceID, conn.NodeID, conn.State), 1})
        }
    }
    return out
}
```

### Changed: `internal/powerstore/client.go`

- `witnessMetrics(ctx, topo) []Sample` — mirror of `driveMetrics`: call
  `enumerateWitnesses`; on a benign `404` return `nil` silently; on any other error
  `LogWarn` and return `nil`.
- `enumerateWitnesses(ctx) ([]witnessInfo, error)` — mirror of `enumerateDrives`:
  `Query` with `Endpoint:"witness"`, `Select("id","name","state","connections")`,
  paginated. Detect the benign `404` with the helper introduced in `be65bfe` and
  return it as a sentinel the caller treats as "no witness".
- One line in `commonMetrics`: `samples = append(samples, c.witnessMetrics(ctx, topo)...)`.

### Changed: `internal/powerstore/metrics.go`

- `witnessStateLabels(array, cluster, id, name, state string) []Label`
- `witnessConnectionLabels(array, cluster, witnessID, applianceID, nodeID, state string) []Label`

Following the `driveStateLabels` convention (identity label `array` + `cluster` +
resource identifiers + `state`).

## Metrics

| Metric | Labels | Value | Meaning |
|---|---|---|---|
| `powerstore_metro_witness_state` | `array, cluster, witness_id, witness_name, state` | `1` | Overall witness-service state |
| `powerstore_metro_witness_connection_state` | `array, cluster, witness_id, appliance_id, node_id, state` | `1` | Per-node connection to the witness |

`address` and `last_updated_timestamp` are intentionally not exported (`witness_name`
identifies the service; the connection metric already conveys liveness).

Alert idiom (mirrors replication):

```promql
# Witness service degraded or unreachable
powerstore_metro_witness_state{state=~"Disconnected|Partially_Connected"}
# A specific node cannot reach the witness
powerstore_metro_witness_connection_state{state="Disconnected"}
```

## Error handling — "absent, never zero"

- **`GET /witness` → 404** (pre-3.6 arrays, feature absent): benign, **silenced** via
  the 404 helper from `be65bfe`. No version gate.
- **Empty list** (3.6+, witness not configured): naturally absent samples.
- **Any other error**: `LogWarn` + return `nil`. `powerstore_up` stays `1` — failure
  is per-metric, never fails the collection cycle.
- Records with empty `id`, and connections with empty `state`, are skipped — never a
  fabricated `0`.

## Testing (TDD)

- **`internal/powerstore/derive_witness_test.go`** (offline, mirrors the drives derive
  test): table-driven. A `witnessInfo{State:"OK"}` with two connections (one `OK`, one
  `Disconnected`) asserts exact metric names, label sets, and values; cases cover the
  empty-`id` and empty-`state` skips and the empty-input case.
- The generic-`Query` fetch is not offline-testable (same as `enumerateDrives`):
  validated live with
  `./bin/pstore_exporter --config real.yaml --once --debug --trace` against a 3.6+
  array that has a witness configured, diffing the dumped samples against
  `docs/metrics.md`.

## Lockstep deliverables (project rules)

- **`docs/metrics.md`** — add both metric rows and the alert examples in the
  protection section, next to the replication metrics.
- **`grafana/dashboards/protection/01-replication.json`** — add a "Metro witness"
  status panel (witness is part of the protection story; same dashboard). Update in
  lockstep with the metric change.
- **`docs/adr/0015-metro-witness-via-generic-query.md`** — record: witness
  observability added through the generic-`Query` escape hatch (extends ADR-0009);
  benign-404 handling as the capability gate instead of a version check; metrics and
  labels chosen.

## Acceptance criteria

1. On a 3.6+ array with a configured, healthy witness, `/metrics` exposes
   `powerstore_metro_witness_state{...,state="OK"} 1` and one
   `powerstore_metro_witness_connection_state{...} 1` per node.
2. On an array without the witness feature, neither metric appears and no warning is
   logged (benign 404 silenced); `powerstore_up` is `1`.
3. `make ci` is green (fmt, vet, lint, test-race, govulncheck), no `//nolint` or
   semgrep suppressions.
4. `docs/metrics.md`, the Grafana protection dashboard, and ADR-0015 are updated in
   the same change.
