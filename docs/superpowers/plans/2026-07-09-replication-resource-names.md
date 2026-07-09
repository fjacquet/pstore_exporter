# Replication Resource Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the replicated device's *name* (not its UUID) on the PowerStore Replication / DR dashboard.

**Architecture:** Add a `Topology.ResourceName(resourceType, id)` resolver backed by the name indices `Topology` already builds from per-cycle inventory, plus one new `fsName` index. Thread its result into the replication label builders as `local_resource_name` (session series) and `resource_name` (transfer/backlog series). Then surface those labels in the Grafana panel. No new API calls.

**Tech Stack:** Go 1.x, `github.com/dell/gopowerstore` v1.22.0, Prometheus exposition, Grafana JSON dashboards.

## Global Constraints

- **Spec:** `docs/superpowers/specs/2026-07-09-replication-resource-names-design.md`
- **Branch:** `feat/replication-resource-names` (already created, off `main`; spec committed at `752fd1b`).
- **TDD:** write the failing test, watch it fail, implement minimally, watch it pass, commit.
- **Semgrep policy:** no inline `//nolint` or semgrep suppressions; fix the finding.
- **Metric parity invariant:** the bulk and per-entity paths must emit identical metric names and label keys. **This plan does not touch either path** — replication is a third path, so the parity test is unaffected. Do not edit `derive_bulk.go` or `derive_perentity.go`.
- **Values are gauges:** never wrap these metrics in `rate()`.
- **Lockstep rule:** a metric label change must land in the same commit as its `docs/metrics.md` row and its `grafana/*.json` panel.
- **Fallback rule:** an unresolvable resource id yields `local_resource_name` / `resource_name` equal to the **id itself**, never the empty string.
- **Gate before pushing:** `make ci`.

## Corrections to the spec

Three claims in the approved spec were checked against the code and are wrong. This plan supersedes them:

1. The spec says to "widen the expected replication label set" in `pipeline_integration_test.go`. That file has **no replication coverage whatsoever** (`grep -in replicat` returns nothing). There is no such assertion to widen. No task touches it.
2. The spec says the `RPO` panel's legend should switch to `{{resource_name}}`. `RPO` legends on `{{rule_id}}` and is driven by `powerstore_replication_rpo_seconds`, a **per-rule** metric with no local resource. RPO is left unchanged. Only `Replication Backlog` and `Transfer Rate` change.
3. The spec's dashboard section says to keep `local_resource_id` excluded while un-excluding `local_resource_name`. That is what Task 3 does — noted here only because the two label names differ by one word and are easy to transpose.

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `internal/powerstore/topology.go` | Inventory + label-resolution indices | Modify: add `fsName` index, `ResourceName` method |
| `internal/powerstore/topology_test.go` | Topology unit tests | Modify: add `TestResourceName` |
| `internal/powerstore/metrics.go` | Shared label builders | Modify: 2 label builders gain a name param |
| `internal/powerstore/derive_replication.go` | Replication metric derivation | Modify: resolve names, pass through |
| `internal/powerstore/derive_replication_test.go` | Replication derive unit tests | Modify: assert the new labels |
| `grafana/dashboards/protection/01-replication.json` | Replication dashboard | Modify: 3 panels |
| `docs/metrics.md` | Metric catalog | Modify: 3 label rows |
| `docs/changelog.md` | Changelog | Modify: Unreleased → Changed |

Three tasks. Task 1 (resolver) and Task 2 (labels) are each independently testable and independently rejectable. Task 3 bundles dashboard + docs because the lockstep rule forbids splitting a label change from its catalog row.

---

### Task 1: Topology resource-name resolver

**Files:**
- Modify: `internal/powerstore/topology.go` (struct fields ~23-29, `NewTopology` ~44-94, new method after `VolumeInfo`)
- Test: `internal/powerstore/topology_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `func (t *Topology) ResourceName(resourceType, id string) string` — Task 2 calls this. Returns the resolved name, or `id` verbatim on any miss (unknown id, unknown/empty `resourceType`, empty `id`).

Background: `NewTopology` already receives `fs []gopowerstore.FileSystem` and stores it as the `FileSystems` field, but builds no name index from it. `gopowerstore.FileSystem` has `ID string` and `Name string` (verified in `fs_types.go:183-187`). The `volumeName`, `vgName`, and `nasName` maps already exist and are already populated.

The four `resourceType` values come from the PowerStore `replication_session` API: `"volume"`, `"volume_group"`, `"file_system"`, `"nas_server"`.

- [ ] **Step 1: Write the failing test**

Append to `internal/powerstore/topology_test.go`:

```go
func TestResourceName(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1"},
		nil,
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1"}},
		[]gopowerstore.VolumeGroup{{ID: "vg-1", Name: "vgA"}},
		[]gopowerstore.NAS{{ID: "nas-1", Name: "NAS01"}},
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "FS01"}},
		nil, nil,
	)

	cases := []struct {
		name         string
		resourceType string
		id           string
		want         string
	}{
		{"volume resolves", "volume", "v-1", "vol1"},
		{"volume group resolves", "volume_group", "vg-1", "vgA"},
		{"file system resolves", "file_system", "fs-1", "FS01"},
		{"nas server resolves", "nas_server", "nas-1", "NAS01"},
		// Fallback: never empty, always degrades to the id.
		{"unknown id falls back to id", "volume", "v-missing", "v-missing"},
		{"unknown resource type falls back to id", "galaxy", "x-1", "x-1"},
		{"empty id falls back to empty", "volume", "", ""},
		{"right id wrong type falls back to id", "volume", "fs-1", "fs-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := topo.ResourceName(tc.resourceType, tc.id); got != tc.want {
				t.Fatalf("ResourceName(%q, %q) = %q, want %q",
					tc.resourceType, tc.id, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/ -run TestResourceName -v`
Expected: FAIL — compile error, `topo.ResourceName undefined (type *Topology has no field or method ResourceName)`.

- [ ] **Step 3: Add the `fsName` index**

In `internal/powerstore/topology.go`, add the field to the index block (after `nasName`):

```go
	nasName             map[string]string
	fsName              map[string]string
```

In `NewTopology`, add to the map-initialization block (after `nasName: make(...)`):

```go
		nasName:             make(map[string]string),
		fsName:              make(map[string]string),
```

And populate it next to the existing NAS loop:

```go
	for _, n := range nas {
		t.nasName[n.ID] = n.Name
	}
	for _, f := range fs {
		t.fsName[f.ID] = f.Name
	}
	return t
```

- [ ] **Step 4: Add the `ResourceName` method**

In `internal/powerstore/topology.go`, after `VolumeInfo`:

```go
// ResourceName resolves a replication session's local resource id to a display
// name, dispatching on the session's resource_type. Unknown ids, unknown resource
// types, and ids absent from the inventory all fall back to the id itself, so the
// label is never empty and the panel degrades to showing the uuid rather than a
// blank cell.
func (t *Topology) ResourceName(resourceType, id string) string {
	var name string
	switch resourceType {
	case "volume":
		name = t.volumeName[id]
	case "volume_group":
		name = t.vgName[id]
	case "file_system":
		name = t.fsName[id]
	case "nas_server":
		name = t.nasName[id]
	}
	if name == "" {
		return id
	}
	return name
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/powerstore/ -run TestResourceName -v`
Expected: PASS, all 8 subtests.

Then confirm nothing else regressed:

Run: `go test ./internal/powerstore/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/powerstore/topology.go internal/powerstore/topology_test.go
git commit -m "feat(powerstore): resolve replication resource ids to names in Topology"
```

---

### Task 2: Emit the name labels

**Files:**
- Modify: `internal/powerstore/metrics.go` (`replicationSessionLabels` ~125, `replicationResourceLabels` ~139)
- Modify: `internal/powerstore/derive_replication.go` (`deriveReplicationSessions` ~23, `deriveReplicationTransfer` ~59)
- Test: `internal/powerstore/derive_replication_test.go`

**Interfaces:**
- Consumes: `Topology.ResourceName(resourceType, id string) string` from Task 1.
- Produces: `powerstore_replication_session_state` gains label `local_resource_name`; `powerstore_replication_transfer_rate_bytes_per_second` and `powerstore_replication_data_remaining_bytes` gain label `resource_name`. Task 3 renders these.

Note: `deriveReplicationTransfer` is only ever called with `resourceType == "volume"` today — `client.go:339` passes the literal `"volume"`, because `replicatedVolumeResources` filters to volume-type sessions. Call the generic resolver anyway rather than indexing `volumeName` directly, so the code stays correct if that path is later widened.

The existing test helper `sampleByLabel(samples, metricName, labelName, labelValue) (float64, bool)` already lives in `derive_replication_test.go:10` — reuse it, don't redefine it.

- [ ] **Step 1: Write the failing tests**

Append to `internal/powerstore/derive_replication_test.go`:

```go
func TestDeriveReplicationSessionResourceNames(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "c1"},
		nil,
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1"}},
		[]gopowerstore.VolumeGroup{{ID: "vg-1", Name: "vgA"}},
		[]gopowerstore.NAS{{ID: "nas-1", Name: "NAS01"}},
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "FS01"}},
		nil, nil,
	)
	sessions := []gopowerstore.ReplicationSession{
		{ID: "rs-vol", ResourceType: "volume", LocalResourceID: "v-1", State: gopowerstore.RsStateOk},
		{ID: "rs-vg", ResourceType: "volume_group", LocalResourceID: "vg-1", State: gopowerstore.RsStateOk},
		{ID: "rs-fs", ResourceType: "file_system", LocalResourceID: "fs-1", State: gopowerstore.RsStateOk},
		{ID: "rs-nas", ResourceType: "nas_server", LocalResourceID: "nas-1", State: gopowerstore.RsStateOk},
		{ID: "rs-ghost", ResourceType: "volume", LocalResourceID: "v-missing", State: gopowerstore.RsStateOk},
	}

	got := deriveReplicationSessions("p1", topo, sessions)

	for _, want := range []string{"vol1", "vgA", "FS01", "NAS01"} {
		if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "local_resource_name", want); !ok {
			t.Fatalf("expected a session series with local_resource_name=%q", want)
		}
	}
	// An id absent from inventory degrades to the id, never to "".
	if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "local_resource_name", "v-missing"); !ok {
		t.Fatal("unresolvable resource must fall back to local_resource_name=<id>")
	}
	if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "local_resource_name", ""); ok {
		t.Fatal("local_resource_name must never be empty")
	}
}

func TestDeriveReplicationTransferResourceName(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "c1"}, nil,
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1"}},
		nil, nil, nil, nil, nil,
	)
	samples := []gopowerstore.VolumeMirrorTransferRateResponse{
		{ID: "v-1", MirrorBandwidth: 250, DataRemaining: 4000},
	}

	got := deriveReplicationTransfer("p1", topo, "v-1", "volume", samples)

	if v, ok := sampleByLabel(got, "powerstore_replication_transfer_rate_bytes_per_second", "resource_name", "vol1"); !ok || v != 250 {
		t.Fatalf("transfer rate by resource_name: want 250, got %v (present=%v)", v, ok)
	}
	if v, ok := sampleByLabel(got, "powerstore_replication_data_remaining_bytes", "resource_name", "vol1"); !ok || v != 4000 {
		t.Fatalf("data remaining by resource_name: want 4000, got %v (present=%v)", v, ok)
	}
}

func TestDeriveReplicationTransferUnknownResourceFallsBackToID(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	samples := []gopowerstore.VolumeMirrorTransferRateResponse{
		{ID: "v-9", MirrorBandwidth: 1, DataRemaining: 2},
	}

	got := deriveReplicationTransfer("p1", topo, "v-9", "volume", samples)

	if _, ok := sampleByLabel(got, "powerstore_replication_transfer_rate_bytes_per_second", "resource_name", "v-9"); !ok {
		t.Fatal("unresolvable resource must fall back to resource_name=<id>")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/powerstore/ -run 'TestDeriveReplication(SessionResourceNames|TransferResourceName|TransferUnknownResourceFallsBackToID)' -v`
Expected: FAIL — the assertions miss, e.g. `expected a session series with local_resource_name="vol1"`. (These compile, because they only read labels; they fail on absence.)

- [ ] **Step 3: Widen the label builders**

In `internal/powerstore/metrics.go`, replace `replicationSessionLabels`:

```go
// replicationSessionLabels builds the label set for a replication session's
// info series. `state` is the current RSStateEnum value as a string.
// localResourceName is the resolved display name of the replicated resource; it
// falls back to localResourceID when the resource is absent from the inventory.
func replicationSessionLabels(arrayName, clusterID, sessionID, localResourceID, localResourceName, resourceType, role, sessionType, remoteSystemID, state string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"session_id", sessionID},
		Label{"local_resource_id", localResourceID},
		Label{"local_resource_name", localResourceName},
		Label{"resource_type", resourceType},
		Label{"role", role},
		Label{"type", sessionType},
		Label{"remote_system_id", remoteSystemID},
		Label{"state", state},
	)
}
```

And replace `replicationResourceLabels`:

```go
// replicationResourceLabels builds the label set for per-resource replication
// transfer metrics (resourceType is e.g. "volume"). resourceName falls back to
// resourceID when the resource is absent from the inventory.
func replicationResourceLabels(arrayName, clusterID, resourceID, resourceName, resourceType string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"resource_id", resourceID},
		Label{"resource_name", resourceName},
		Label{"resource_type", resourceType},
	)
}
```

- [ ] **Step 4: Resolve the names at the derive sites**

In `internal/powerstore/derive_replication.go`, in `deriveReplicationSessions`, replace the `labels := ...` line inside the loop:

```go
		name := topo.ResourceName(s.ResourceType, s.LocalResourceID)
		labels := replicationSessionLabels(array, clusterID, s.ID, s.LocalResourceID, name,
			s.ResourceType, s.Role, s.Type, s.RemoteSystemID, string(s.State))
```

In the same file, in `deriveReplicationTransfer`, replace the `labels := ...` line:

```go
	name := topo.ResourceName(resourceType, resourceID)
	labels := replicationResourceLabels(array, topo.ClusterID(), resourceID, name, resourceType)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/powerstore/ -run TestDeriveReplication -v`
Expected: PASS — the three new tests plus the five pre-existing `TestDeriveReplication*` tests.

Then the whole package (the label builders are shared; `metrics_test.go` and `otlp_test.go` must still pass):

Run: `go test ./...`
Expected: `ok` for every package.

- [ ] **Step 6: Commit**

```bash
git add internal/powerstore/metrics.go internal/powerstore/derive_replication.go internal/powerstore/derive_replication_test.go
git commit -m "feat(powerstore): add local_resource_name and resource_name replication labels"
```

---

### Task 3: Surface names in the dashboard and catalog

**Files:**
- Modify: `grafana/dashboards/protection/01-replication.json` (panels `Replication Sessions`, `Replication Backlog`, `Transfer Rate`)
- Modify: `docs/metrics.md:160`, `:162`, `:163`
- Modify: `docs/changelog.md` (Unreleased → Changed)

**Interfaces:**
- Consumes: labels `local_resource_name`, `resource_name` from Task 2.
- Produces: nothing downstream.

Two independent fixes ride together here, both in the `Replication Sessions` panel's `organize` transform. First, un-exclude `local_resource_name` so the device name renders. Second, un-exclude `array` — without it the two rows of a Metro pair (same logical session, reported once by each array, differing only in `role`) render as indistinguishable duplicates. `session_id` becomes excluded: a session uuid is the session's own identity, not a device, and it is the thing currently crowding out the name.

Leave `local_resource_id`, `cluster_id`, `Time`, `Value`, and `__name__` excluded. Note `local_resource_id` and `local_resource_name` differ by one word — do not transpose them.

- [ ] **Step 1: Rewrite the `Replication Sessions` organize transform**

In `grafana/dashboards/protection/01-replication.json`, find the panel titled `Replication Sessions`. Replace its `organize` transformation's `options` object wholesale with:

```json
{
  "excludeByName": {
    "Time": true,
    "Value": true,
    "__name__": true,
    "cluster_id": true,
    "local_resource_id": true,
    "session_id": true
  },
  "indexByName": {
    "array": 0,
    "local_resource_name": 1,
    "resource_type": 2,
    "role": 3,
    "type": 4,
    "remote_system_id": 5,
    "state": 6
  },
  "renameByName": {
    "array": "Array",
    "local_resource_name": "Resource",
    "resource_type": "Resource Type",
    "role": "Role",
    "type": "Type",
    "remote_system_id": "Remote System",
    "state": "State"
  }
}
```

- [ ] **Step 2: Relegend the two resource-keyed timeseries panels**

In the same file, in the panel titled `Replication Backlog`, change its target's legend:

```json
"legendFormat": "{{resource_name}}"
```

In the panel titled `Transfer Rate`, make the identical change:

```json
"legendFormat": "{{resource_name}}"
```

**Do not touch the `RPO` panel.** It legends on `{{rule_id}}` and is driven by `powerstore_replication_rpo_seconds`, a per-rule metric that has no local resource. Leave it exactly as it is.

- [ ] **Step 3: Verify the dashboard JSON is still valid and the edits landed**

Run:

```bash
python3 -c "
import json
d = json.load(open('grafana/dashboards/protection/01-replication.json'))
def walk(ps):
    for p in ps:
        yield p
        yield from walk(p.get('panels', []))
for p in walk(d['panels']):
    t = p.get('title', '')
    if t == 'Replication Sessions':
        o = [x for x in p['transformations'] if x['id'] == 'organize'][0]['options']
        assert 'array' not in o['excludeByName'], 'array must be shown'
        assert o['excludeByName'].get('session_id'), 'session_id must be excluded'
        assert o['excludeByName'].get('local_resource_id'), 'local_resource_id must stay excluded'
        assert o['renameByName']['local_resource_name'] == 'Resource'
    if t in ('Replication Backlog', 'Transfer Rate'):
        assert p['targets'][0]['legendFormat'] == '{{resource_name}}', t
    if t == 'RPO':
        assert p['targets'][0]['legendFormat'] == '{{rule_id}}', 'RPO must be untouched'
print('dashboard OK')
"
```

Expected: `dashboard OK`.

- [ ] **Step 4: Update the metric catalog**

In `docs/metrics.md`, replace the three replication rows (lines 160, 162, 163) — add the new label to each `Labels` cell, keeping the descriptions:

Line 160 — add `local_resource_name` after `local_resource_id`:

```
| `powerstore_replication_session_state` | `session_id`, `local_resource_id`, `local_resource_name`, `resource_type`, `role`, `type`, `remote_system_id`, `state` | Info series, always `1`, one per replication session of any resource type (e.g. volume, volume_group, file_system) — including Metro sessions. The session's current state is the `state` label (`OK`, `Synchronizing`, `Error`, `Fractured`, `System_Paused`, `Paused`, …); `role`/`type` distinguish Metro (`role=Metro_Preferred|Metro_Non_Preferred`, `type=Metro_Active_Active`) from async/sync sessions. `local_resource_name` is the replicated resource's name, falling back to its id when the resource is absent from the inventory. |
```

Lines 162-163 — add `resource_name` after `resource_id`:

```
| `powerstore_replication_transfer_rate_bytes_per_second` | `resource_id`, `resource_name`, `resource_type` | Current mirror replication throughput for the resource. Emitted only for volumes with an active replication session. |
| `powerstore_replication_data_remaining_bytes` | `resource_id`, `resource_name`, `resource_type` | Outstanding data still to be replicated (backlog / RPO-risk indicator). Emitted only for volumes with an active replication session. |
```

Leave the `powerstore_replication_rpo_seconds` row (line 161) unchanged.

- [ ] **Step 5: Add the changelog entry**

In `docs/changelog.md`, under `## [Unreleased]` → `### Changed`, add:

```markdown
- Replication metrics now carry the replicated resource's **name**, not just its uuid:
  `powerstore_replication_session_state` gains `local_resource_name`, and
  `powerstore_replication_transfer_rate_bytes_per_second` /
  `powerstore_replication_data_remaining_bytes` gain `resource_name`. Names resolve for
  volume, volume_group, file_system, and nas_server sessions, falling back to the id when
  the resource is absent from the inventory. The Replication / DR dashboard now shows the
  resource name and the reporting `array` (which disambiguates the two rows of a Metro
  pair) in place of the session uuid.
```

If `### Changed` does not exist under `## [Unreleased]`, create it immediately after the `### Added` block.

- [ ] **Step 6: Run the full CI gate**

Run: `make ci`
Expected: PASS — `fmt-check`, `vet`, `lint`, `test -race`, `govulncheck` all green.

If `lint` flags `replicationSessionLabels` for having too many parameters, do **not** add a `//nolint` (repo policy forbids suppressions). Introduce a small `replicationSession` params struct in `metrics.go` and pass that instead.

- [ ] **Step 7: Commit**

```bash
git add grafana/dashboards/protection/01-replication.json docs/metrics.md docs/changelog.md
git commit -m "feat(grafana): show replication resource name and array, drop session uuid"
```

---

### Task 4: Verify against a live array, then open the PR

**Files:** none modified.

**Interfaces:** consumes the whole feature.

- [ ] **Step 1: Confirm the new labels appear in a real scrape**

Run (substitute the real password; `--once` runs a single cycle and exits):

```bash
PSTORE1_PASSWORD=x ./bin/pstore_exporter --config config.yaml --once --debug 2>&1 \
  | grep powerstore_replication_session_state
```

Expected: every line carries a non-empty `local_resource_name="..."`. If any line shows `local_resource_name=""`, the fallback is broken — stop and fix Task 1's `ResourceName`.

If no live array is reachable, skip this step and say so explicitly in the PR body rather than implying it was verified.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/replication-resource-names
gh pr create --title "feat: resolve replication resource names in metrics and dashboard" --body "$(cat <<'EOF'
Replication panels identified devices by uuid. The volume identity was already on the wire
(`local_resource_id` was emitted, then discarded by the dashboard's organize transform); this
resolves it to a name and surfaces it.

- `Topology.ResourceName(resourceType, id)` resolves volume / volume_group / file_system /
  nas_server ids against the inventory indices already built each cycle. New `fsName` index.
  No new API calls.
- `powerstore_replication_session_state` gains `local_resource_name`;
  `powerstore_replication_transfer_rate_bytes_per_second` and
  `powerstore_replication_data_remaining_bytes` gain `resource_name`. Unresolvable ids fall
  back to the id, never to "".
- The Sessions table now leads with Array + Resource and drops the session uuid. Un-excluding
  `array` also fixes the Metro pair rendering as two identical-looking rows.

Out of scope: `remote_system_name` (needs a `GetAllRemoteSystems` fetch — the deferred
data-protection roadmap item).

Spec: `docs/superpowers/specs/2026-07-09-replication-resource-names-design.md`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: a PR url.

---

## Self-Review

**Spec coverage.** `fsName` index → Task 1 Step 3. `ResourceName` + fallback → Task 1 Steps 4-5. `replicationSessionLabels` / `replicationResourceLabels` widening → Task 2 Step 3. Derive sites → Task 2 Step 4. Dashboard organize transform → Task 3 Step 1. Legend formats → Task 3 Step 2. `docs/metrics.md` → Task 3 Step 4. `docs/changelog.md` → Task 3 Step 5. Testing section → Tasks 1 and 2. Compatibility section → no task needed (verified: the only `on(...)` joins in the repo are in the vendored `node-exporter-full.json`).

Three spec requirements were **dropped as factually wrong**, documented under "Corrections to the spec" above: the `pipeline_integration_test.go` widening (no replication coverage exists there), the `RPO` panel relegend (per-rule metric, no resource), and the implied idea that `deriveReplicationTransfer` needs a real type switch (it only ever sees `"volume"`).

**Type consistency.** `ResourceName(resourceType, id string) string` — declared Task 1 Step 4, called Task 2 Step 4 with `(s.ResourceType, s.LocalResourceID)` and `(resourceType, resourceID)`. Both are `string`. `replicationSessionLabels` gains `localResourceName` as the 5th param, positioned after `localResourceID`; the Task 2 Step 4 call site passes `name` in that slot. `replicationResourceLabels` gains `resourceName` as the 4th param, after `resourceID`; the call site matches. Label keys `local_resource_name` and `resource_name` are spelled identically in metrics.go, the tests, the dashboard JSON, and `docs/metrics.md`. `sampleByLabel` is reused from `derive_replication_test.go:10`, not redefined.

**Placeholder scan.** No TBD/TODO. Every code step carries complete code. Every command carries expected output. The one conditional branch (lint parameter-count) states the concrete remedy rather than "handle appropriately".
