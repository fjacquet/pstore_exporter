# Exporter metric-gaps: design

- **Date:** 2026-07-08
- **Status:** Draft (awaiting review)
- **Author:** Frederic Jacquet (with Claude)
- **Scope:** `internal/powerstore` — four independent fixes surfaced by validating a live
  two-array metrics dump (`pstore2.log`) against the code, docs, and the PowerStore 4.4.0
  REST swagger (`docs/swagger/5504-4.4.0.json`).

## Context

A full validation of a live `--once --debug` scrape (arrays `pstore-1`/`pstore-2`) found the
output structurally sound: 50 metric families, all gauges, 100% label-key parity across
series and both arrays, and every arithmetic invariant (IOPS decomposition, `avg_io_size`,
capacity inequalities, cluster=appliance rollup, ratios) green. The metro-witness and
replication-asymmetry observations were confirmed correct-by-design.

The audit also surfaced four defects/gaps that value-sanity alone could not — three of them
"silently missing metric" issues. All four root causes are now confirmed against the code, a
targeted `--debug` run, and read-only REST queries. This spec covers fixing all four. They
are independent and should land as **separate PRs**.

## Findings and root causes (all confirmed)

| # | Symptom | Confirmed root cause | Evidence |
|---|---------|----------------------|----------|
| 1 | `powerstore_drive_state` and `powerstore_drive_wear_level_ratio` absent on both arrays | `enumerateDrives` selects property `life_cycle_state`, which does not exist on the `hardware` resource; the whole query 400s and drives never collect | `--debug` stderr: `enumerate drives: Unknown property hardware.life_cycle_state requested.` (both arrays); swagger `hardware_instance` exposes `lifecycle_state`, not `life_cycle_state` |
| 2 | `powerstore_replication_session_state` absent, while `transfer_rate`/`data_remaining` present with 0/0 | Sessions are probed only for volumes with `ProtectionPolicyID != ""`; the real session is a **Metro** session (`role=Metro_Preferred`) that is not driven by a protection policy, so it is never queried. Separately, `VolumeMirrorTransferRate` returns a phantom 0/0 row for a policy-bearing volume that has no session | REST `replication_session` on pstore-1 returns exactly one session, `local_resource_id=1b510307…(diab-admin)`, `state=Paused`, `role=Metro_Preferred`; the volume carrying transfer metrics (`7d1739f6`) has **no** session |
| 3 | pstore-1 `FS01` reports `size_total=0` and emits no perf | The FS is an inactive metro/replication-destination stub whose API `size_total` is **`null`**; gopowerstore's value-typed `FileSystem.SizeTotal` decodes `null`→`0` | REST `file_system` on pstore-1: `FS01` = `Primary`/`General`, `size_total:null`, `size_used:null`, `access_type:null`, `parent_id:null` |
| 4 | `volume_name` == `volume_id` (UUID) for most volumes | Bulk-path CSV rows carry volume IDs absent from the snapshot-filtered inventory index (snapshots/clones/transient); `VolumeInfo` discards the map comma-ok, so a true index miss is indistinguishable from a genuinely empty name and both fall back to the UUID | Per-entity run resolves every name (`diab-admin`, `sib-test-thin`, …); only the bulk path shows UUIDs |

## Non-goals

- No new metric **names**. #1 restores already-documented families; #2 populates the existing
  `replication_session_state` for Metro sessions; #3/#4 change emission/labels, not names.
- No change to the bulk-vs-per-entity path selection, session management, or the collection loop.
- No live-array integration test (the bulk download and typed SDK perf calls are not offline-testable).

## Design

All four changes preserve the **metric-parity invariant**: any family emitted on both the bulk
and per-entity paths must keep identical names and label KEYS. `driveMetrics`,
`replicationMetrics`, `fileSystemPerf`, and `deriveFileSystemCapacity` all run in
`commonMetrics` (shared by both paths), so #1/#2/#3 are single-path edits. #4 touches the two
volume fallback sites (`derive_bulk.go`, `derive_perentity.go`) and they MUST stay in lockstep.

Project policy reminders that bind the implementation: **TDD** (test first); **no inline
`//nolint` or semgrep suppressions** (fix findings); update `docs/metrics.md` and the Grafana
dashboards **in lockstep** with any metric behavior change.

### Fix #1 — Drive metrics: correct the `hardware` property name

**Root cause is a one-property typo.** The `hardware_instance` schema confirms `id`, `name`,
`appliance_id`, `extra_details`, `type`, and `lifecycle_state` are all valid; only
`life_cycle_state` is wrong.

Changes:
1. `internal/powerstore/client.go` (`enumerateDrives`, ~L458): `Select(…, "lifecycle_state", …)`
   (was `"life_cycle_state"`).
2. `internal/powerstore/derive_drives.go` (~L18): struct tag
   `LifeCycleState string \`json:"lifecycle_state"\`` (was `"life_cycle_state"`). Keep the Go
   field name or rename to `LifecycleState` for consistency — cosmetic.
3. **Observability:** `enumerateDrives`/`driveMetrics` currently return `nil` silently when the
   query succeeds but yields zero drives. Add a debug-level log of the drive count so a future
   empty result is visible without a code dive.

Tests (TDD):
- Unit test: decode a `hardware?type=eq.Drive` JSON fixture (id, name, appliance_id,
  `lifecycle_state`, `extra_details.drive_wear_level`) into `driveInfo`; assert the tag binds
  and `deriveDrives` emits `powerstore_drive_state{…,life_cycle_state-value…}=1` and, when
  `drive_wear_level` is present, `powerstore_drive_wear_level_ratio = wear/100`. This fixture is
  the regression guard: a future property rename fails the decode assertion loudly.

Expected result: both `drive_state` (always) and `drive_wear_level_ratio` (on PowerStoreOS ≥
4.3, which these arrays satisfy) begin emitting for every drive.

### Fix #2 — Replication sessions: enumerate generically; reconcile transfer

**Redesign `replicationMetrics` to discover sessions directly** instead of inferring them from
protection-policy volumes. This is the generic-API escape hatch already used for drives and
witnesses (ADR-0009), and it is the only way to capture Metro sessions.

Changes (`internal/powerstore/client.go`, `replicationMetrics` L295–366):
1. Add `enumerateReplicationSessions(ctx)` — a single generic
   `GET /api/rest/replication_session?select=id,local_resource_id,resource_type,role,type,remote_system_id,state`,
   paginated defensively (mirrors `enumerateDrives`/`enumerateWitnesses`). Decode into a small
   local struct.
2. Emit `powerstore_replication_session_state` for **every** returned session (state-as-label
   enum, unchanged shape) — this now includes Metro sessions (`role=Metro_Preferred`, etc.).
3. **Drive transfer/backlog from the session list, not the protection-policy list.** For each
   session with `resource_type == "volume"`, call `VolumeMirrorTransferRate(local_resource_id)`
   and emit `transfer_rate`/`data_remaining` only when the response is non-empty. This removes
   the phantom 0/0 series for policy-bearing volumes that have no session.
4. `replication_rpo_seconds` is unchanged — it is legitimately per-**rule** cluster config from
   `GetReplicationRules`, independent of sessions.
5. Remove the now-moot per-volume `GetReplicationSessionByLocalResourceID` probe and its
   silent-404 handling.

Open sub-decision for the plan (default chosen): keep emitting `transfer_rate`/`data_remaining`
labeled `resource_type="volume"` for volume sessions only; do not attempt transfer metrics for
`file_system`/`virtual_volume` sessions in this change (they can be a follow-up).

Tests (TDD):
- `deriveReplicationSessions` already emits one series per session; add a fixture with a
  `Metro_Preferred`/`Paused` session and assert `state="Paused"` is carried as a label.
- Table test for the reconcile logic: sessions=[Metro volume A], transfer available for A only,
  a policy-volume B with no session ⇒ expect `session_state{A}`, `transfer{A}`, and **no**
  `transfer{B}`.

Expected result: `replication_session_state` appears (including the Metro `Paused` session on
`diab-admin`); the phantom `transfer_rate`/`data_remaining` for `sib-test-thin` disappears.

Bonus: this advances the deferred data-protection roadmap item "per-session engagement".

### Fix #3 — File systems: skip the null/zero-size destination stub

**Skip file systems whose `size_total` is 0** (the SDK's decoding of an API `null`) when
emitting capacity, and correspondingly skip perf collection for them. Real PowerStore
filesystems always have a non-zero provisioned size, so this targets exactly the inactive
metro/replication-destination stubs. The API exposes no `is_replication_destination`/`state`
flag to filter on more precisely, so the `size_total==0` rule is the reliable signal available.

Changes:
1. `internal/powerstore/derive_perentity.go` (`deriveFileSystemCapacity`, L82–93): skip a
   filesystem when `fs.SizeTotal == 0` (continue).
2. `internal/powerstore/client.go` (`fileSystemPerf`, L373–382): skip the perf call for the
   same filesystems (they already emit nothing, but this avoids a needless per-FS request).
   Factor the "is a chartable filesystem" predicate so both sites share one definition.

Tests (TDD):
- `deriveFileSystemCapacity` with a mixed inventory (one `SizeTotal>0`, one `SizeTotal==0`)
  ⇒ only the non-zero FS emits `size_total`/`size_used`.

Expected result: pstore-1 `FS01`'s size-0 series disappear; pstore-2 `FS01` (100 GiB, active)
is unaffected.

### Fix #4 — Volume name: distinguish a true miss from an empty name

**Make `VolumeInfo` return the comma-ok**, fall back to the UUID only on a genuine index miss,
and count misses so unresolved bulk rows are observable. Behavior for operators is unchanged
(unresolved rows still carry the UUID as `volume_name` — the "keep + count" decision), but the
exporter can now report how many rows didn't resolve.

Changes:
1. `internal/powerstore/topology.go` (`VolumeInfo`, L116–118): return
   `(name string, applianceID string, known bool)` — `known` is the comma-ok of the
   `volumeName` lookup. (Or add `VolumeName(id) (string, bool)`; keep the signature minimal.)
2. `internal/powerstore/derive_bulk.go` (L34–37) and `internal/powerstore/derive_perentity.go`
   (L18–21): **keep the existing behavior** — fall back to `volID` whenever the resolved name is
   empty (covers both a true index miss and a genuinely empty `Name`), so operator-visible output
   is unchanged (the "keep + count" decision). Use `known` **solely** to increment the miss
   counter for true index misses (`!known`). Edit both sites together (parity).
3. Add a per-cycle miss count logged at debug (e.g. `array %q: N bulk volume rows unresolved to
   inventory`). A dedicated exporter metric for the miss count is **out of scope** here (keep the
   change small); revisit if operators want to alert on it.

Tests (TDD):
- `topology_test.go`: `VolumeInfo` returns `known=false` for an unknown id and `known=true`
  (empty name) for an indexed volume with an empty `Name`.
- `derive_bulk` test: a row whose `volume_id` is not in the index ⇒ `volume_name == volume_id`
  and the miss is counted.

## Cross-cutting

- **Parity test:** the existing volume parity test must still pass; #4 edits both derive paths.
- **Docs:** update `docs/metrics.md` notes for drives (now emitted) and replication session
  (now includes Metro). No new metric names, so the catalog gains clarifications, not rows.
- **Grafana:** any drive-health or replication-session panels that were empty will now populate;
  verify dashboards reference the restored series and add a Metro-session state panel if useful
  (lockstep with the metric behavior change).
- **CI:** `make ci` (fmt/vet/lint/test-race/govulncheck) + Semgrep must pass; no suppressions.

## Suggested PR sequence

1. **#1 drives** — smallest, highest value, a clear bug. Land first.
2. **#3 file-system skip** — small, self-contained.
3. **#4 volume-name comma-ok** — small, touches both derive paths (+ parity test).
4. **#2 replication-session enumeration** — largest; a focused redesign of `replicationMetrics`
   that also captures Metro session state.

## Follow-ups (out of scope)

- Transfer/backlog metrics for `file_system`/`virtual_volume` replication sessions.
- An exporter metric for unresolved-volume-row count (vs. the debug log added in #4).
- Investigate the intermittent bulk-download 404 that forced the per-entity fallback in the
  debug run (handled gracefully today; not a regression).
