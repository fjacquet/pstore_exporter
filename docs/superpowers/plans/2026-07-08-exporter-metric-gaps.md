# Exporter Metric Gaps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix four metric gaps found by validating a live two-array scrape: restore drive metrics, capture Metro replication sessions (and stop phantom transfer series), skip null-size file systems, and make volume-name resolution observable.

**Architecture:** All four are edits under `internal/powerstore`. Three are one-line-to-one-function corrections; one (#2) is a focused rewrite of `replicationMetrics` to enumerate replication sessions via the generic API (the escape-hatch pattern already used for drives and witnesses), then drive transfer metrics from the real session list. Each task is an independent, separately committable PR.

**Tech Stack:** Go, `github.com/dell/gopowerstore@v1.22.0` (generic `APIClient().Query` for resources with no typed method), logrus logging, Prometheus/OTLP export.

**Spec:** `docs/superpowers/specs/2026-07-08-exporter-metric-gaps-design.md`

## Global Constraints

- **Metric parity invariant:** the bulk (`derive_bulk.go`) and per-entity (`derive_perentity.go`) paths MUST emit identical metric names and label KEYS; when a change touches volume/file-system emission, edit BOTH derive sites together and keep the shared label builders in `metrics.go` the single source of truth.
- **Values are gauges** (IOPS/sec, bytes/sec, latency µs) — never counters.
- **No inline `//nolint` or semgrep suppressions** — fix the finding.
- **TDD**: write the failing test first; commit frequently.
- **Module path:** `github.com/fjacquet/pstore_exporter`; logging import `github.com/fjacquet/pstore_exporter/internal/logging`.
- **Gate before pushing:** `make ci` (fmt-check, vet, lint, test-race, govulncheck) + Semgrep must pass.
- **Docs/Grafana lockstep:** any metric behavior change updates `docs/metrics.md` and the affected Grafana dashboards in the same PR.
- **Suggested PR order:** Task 1 → Task 2 → Task 3 → Task 4 (smallest/highest-value first; Task 4 is the only redesign). Task 3 consumes `logging.LogDebug`, which Task 1 adds — land Task 1 first, or add `LogDebug` in whichever lands first.

---

### Task 1: Fix drive metrics — correct the `hardware` property name (#1)

Drives never collect because `enumerateDrives` selects `life_cycle_state`, which does not exist on the `hardware` resource (the whole query 400s). The PowerStore 4.4.0 swagger confirms the property is `lifecycle_state`. Fixing the JSON tag + the `Select` restores both `powerstore_drive_state` and `powerstore_drive_wear_level_ratio`. Also add a `LogDebug` helper and log the drive count so a future empty enumeration is visible under `--debug`.

**Files:**
- Modify: `internal/powerstore/derive_drives.go:18` (struct tag)
- Modify: `internal/powerstore/client.go:458` (`Select`) and `client.go:442-449` (`driveMetrics` debug log)
- Modify: `internal/logging/logging.go` (add `LogDebug`)
- Test: `internal/powerstore/derive_drives_test.go` (add a JSON-decode regression test)

**Interfaces:**
- Produces: `logging.LogDebug(msg string)` — a debug-level structured log wrapper, consumed by Task 4.
- Produces: `driveInfo` decodes the `hardware` payload's `lifecycle_state` into its `LifeCycleState` field.

- [ ] **Step 1: Write the failing decode test**

Add to `internal/powerstore/derive_drives_test.go` (add `"encoding/json"` to its import block):

```go
func TestDriveInfoDecodesLifecycleState(t *testing.T) {
	// Shape of a hardware?type=eq.Drive row. The PowerStore property is
	// "lifecycle_state" (not "life_cycle_state"); a mismatched struct tag
	// silently decodes it to "" — which is the bug this guards against.
	const payload = `[{"id":"d-1","name":"Drive-0","appliance_id":"a-1",` +
		`"lifecycle_state":"Healthy","extra_details":{"drive_wear_level":30}}]`

	var got []driveInfo
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 drive, got %d", len(got))
	}
	if got[0].LifeCycleState != "Healthy" {
		t.Fatalf("lifecycle_state must decode into LifeCycleState; got %q", got[0].LifeCycleState)
	}
	if got[0].ExtraDetails.DriveWearLevel == nil || *got[0].ExtraDetails.DriveWearLevel != 30 {
		t.Fatalf("drive_wear_level must decode; got %v", got[0].ExtraDetails.DriveWearLevel)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/powerstore/ -run TestDriveInfoDecodesLifecycleState -v`
Expected: FAIL — `lifecycle_state must decode into LifeCycleState; got ""` (the current tag is `life_cycle_state`).

- [ ] **Step 3: Fix the struct tag**

In `internal/powerstore/derive_drives.go`, line 18, change the tag only (keep the Go field name `LifeCycleState`):

```go
	LifeCycleState string            `json:"lifecycle_state"`
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/powerstore/ -run TestDriveInfoDecodesLifecycleState -v`
Expected: PASS.

- [ ] **Step 5: Fix the live query Select and add the debug count log**

In `internal/powerstore/client.go`, `enumerateDrives` (~line 458), change the selected property:

```go
			Select("id", "name", "appliance_id", "lifecycle_state", "extra_details").
```

Add `LogDebug` to `internal/logging/logging.go` (mirrors `LogInfo`; emits only when the logrus level is Debug, i.e. under `--debug`):

```go
// LogDebug logs a debug message tagged with the program name. It emits only when
// the log level is Debug (the --debug flag), so it is safe for per-cycle detail.
func LogDebug(msg string) {
	log.WithFields(log.Fields{"job": programName}).Debug(msg)
}
```

In `internal/powerstore/client.go`, `driveMetrics` (~line 442-449), log the count after a successful enumeration:

```go
func (c *ArrayClient) driveMetrics(ctx context.Context, topo *Topology) []Sample {
	drives, err := c.enumerateDrives(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: enumerate drives: %v", c.name, err))
		return nil
	}
	logging.LogDebug(fmt.Sprintf("array %q: enumerated %d drives", c.name, len(drives)))
	return deriveDrives(c.name, topo, drives)
}
```

- [ ] **Step 6: Verify the package builds and all tests pass**

Run: `go build ./... && go test ./internal/powerstore/ ./internal/logging/`
Expected: PASS. (`enumerateDrives`/`driveMetrics` are validated live — see Step 8.)

- [ ] **Step 7: Update docs**

In `docs/metrics.md`, confirm the `powerstore_drive_state` / `powerstore_drive_wear_level_ratio` entries note they are emitted per drive (they were silently absent before this fix). No new metric names.

- [ ] **Step 8: Commit**

```bash
git add internal/powerstore/derive_drives.go internal/powerstore/derive_drives_test.go internal/powerstore/client.go internal/logging/logging.go docs/metrics.md
git commit -m "fix(powerstore): correct drive hardware property to lifecycle_state

The drive enumeration selected life_cycle_state, which does not exist on
the hardware resource, so the query 400'd and drive_state /
drive_wear_level_ratio never emitted. Use the swagger-confirmed
lifecycle_state property; add LogDebug + a drive-count log so an empty
enumeration is visible under --debug."
```

**Live validation (after deploy):** `--once --debug` stderr should no longer show `enumerate drives: Unknown property …`; `/metrics` should show `powerstore_drive_state` for every drive.

---

### Task 2: Skip null/zero-size file systems (#3)

pstore-1's `FS01` is an inactive metro/replication-destination stub whose API `size_total` is `null`; gopowerstore's value-typed `FileSystem.SizeTotal` decodes `null`→`0`, so the exporter emits a meaningless size-0 series (and attempts perf that returns nothing). Real filesystems always have a non-zero provisioned size, so skip `SizeTotal == 0` file systems.

**Files:**
- Modify: `internal/powerstore/derive_perentity.go:82-93` (`deriveFileSystemCapacity`) + add a shared predicate
- Modify: `internal/powerstore/client.go:373-382` (`fileSystemPerf`)
- Test: `internal/powerstore/derive_perentity_test.go` (add a capacity-skip test)

**Interfaces:**
- Produces: `chartableFileSystem(fs gopowerstore.FileSystem) bool` — true when the FS has a non-zero provisioned size; used by both the capacity and perf paths.

- [ ] **Step 1: Write the failing test**

Add to `internal/powerstore/derive_perentity_test.go` (it is in `package powerstore`, so `sampleByLabel` and `NewTopology` are available):

```go
func TestDeriveFileSystemCapacitySkipsZeroSize(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil,
		[]gopowerstore.FileSystem{
			{ID: "fs-real", Name: "FS01", NasServerID: "nas-1", SizeTotal: 107374182400, SizeUsed: 1623195648},
			{ID: "fs-stub", Name: "FS01", NasServerID: "nas-2", SizeTotal: 0, SizeUsed: 0},
		},
		nil, nil)

	got := deriveFileSystemCapacity("p1", topo)

	if _, ok := sampleByLabel(got, "powerstore_file_system_size_total_bytes", "file_system_id", "fs-real"); !ok {
		t.Fatal("real FS must emit size_total")
	}
	if _, ok := sampleByLabel(got, "powerstore_file_system_size_total_bytes", "file_system_id", "fs-stub"); ok {
		t.Fatal("size-0 FS (null size stub) must not emit a capacity series")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/powerstore/ -run TestDeriveFileSystemCapacitySkipsZeroSize -v`
Expected: FAIL — `size-0 FS … must not emit a capacity series` (currently both are emitted).

- [ ] **Step 3: Add the predicate and skip in the capacity path**

In `internal/powerstore/derive_perentity.go`, add near `deriveFileSystemCapacity`:

```go
// chartableFileSystem reports whether a file system is a real, provisioned data
// filesystem worth charting. Inactive metro/replication-destination stubs report
// size_total as null, which the SDK decodes to 0; PowerStore never provisions a
// real filesystem at 0 bytes, so a 0 total is the reliable "skip me" signal (the
// REST API exposes no is_replication_destination/state flag to filter on).
func chartableFileSystem(fs gopowerstore.FileSystem) bool {
	return fs.SizeTotal > 0
}
```

Then guard the loop body in `deriveFileSystemCapacity`:

```go
	for _, fs := range topo.FileSystems {
		if !chartableFileSystem(fs) {
			continue
		}
		labels := fileSystemLabels(array, clusterID, fs.Name, fs.ID, topo.NASName(fs.NasServerID), fs.NasServerID)
		out = append(out,
			Sample{"powerstore_file_system_size_total_bytes", labels, float64(fs.SizeTotal)},
			Sample{"powerstore_file_system_size_used_bytes", labels, float64(fs.SizeUsed)},
		)
	}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/powerstore/ -run TestDeriveFileSystemCapacitySkipsZeroSize -v`
Expected: PASS.

- [ ] **Step 5: Skip the same file systems in the perf path**

In `internal/powerstore/client.go`, `fileSystemPerf` (~line 373-382), avoid a needless per-FS perf request for skipped file systems by filtering the input slice:

```go
func (c *ArrayClient) fileSystemPerf(ctx context.Context, topo *Topology) []Sample {
	chartable := make([]gopowerstore.FileSystem, 0, len(topo.FileSystems))
	for _, fs := range topo.FileSystems {
		if chartableFileSystem(fs) {
			chartable = append(chartable, fs)
		}
	}
	return parallelSamples(ctx, chartable, c.maxConcurrency, func(ctx context.Context, fs gopowerstore.FileSystem) []Sample {
		resp, err := c.gp.PerformanceMetricsByFileSystem(ctx, fs.ID, c.interval)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: file system %s perf failed: %v", c.name, fs.ID, err))
			return nil
		}
		return deriveFileSystemPerf(c.name, topo, fs, resp)
	})
}
```

- [ ] **Step 6: Verify build and tests**

Run: `go build ./... && go test ./internal/powerstore/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/powerstore/derive_perentity.go internal/powerstore/derive_perentity_test.go internal/powerstore/client.go
git commit -m "fix(powerstore): skip null/zero-size file systems

An inactive metro/replication-destination filesystem reports size_total
as null, which the SDK decodes to 0, producing a meaningless size-0
series and a perf request that returns nothing. Skip SizeTotal==0 file
systems in both the capacity and perf paths."
```

---

### Task 3: Volume-name resolution — distinguish a true miss (#4)

`VolumeInfo` discards the map comma-ok, so a bulk-CSV row whose `volume_id` is absent from the (snapshot-filtered) inventory is indistinguishable from a genuinely empty name — both fall back to the UUID. Keep that operator-visible behavior (the "keep + count" decision), but return the comma-ok so the bulk path can count and debug-log unresolved rows.

**Files:**
- Modify: `internal/powerstore/topology.go:116-118` (`VolumeInfo` signature)
- Modify: `internal/powerstore/derive_bulk.go:34-37` (count misses, debug log)
- Modify: `internal/powerstore/derive_perentity.go:18` (adapt to new signature)
- Test: `internal/powerstore/topology_test.go` (comma-ok) and `internal/powerstore/derive_bulk_test.go` (UUID fallback)

**Interfaces:**
- Consumes: `logging.LogDebug` (from Task 1).
- Produces: `VolumeInfo(id string) (name, applianceID string, known bool)` — `known` is false when `id` is not in the volume index.

- [ ] **Step 1: Write the failing topology test**

Add to `internal/powerstore/topology_test.go`:

```go
func TestVolumeInfoKnown(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1"}, nil,
		[]gopowerstore.Volume{
			{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"},
			{ID: "v-empty", Name: "", ApplianceID: "appl-1"},
		},
		nil, nil, nil, nil, nil,
	)

	if name, _, known := topo.VolumeInfo("v-1"); !known || name != "vol1" {
		t.Fatalf("known volume: want (vol1,true), got (%q,%v)", name, known)
	}
	// Present but empty name → known=true (a genuine empty name, not a miss).
	if name, _, known := topo.VolumeInfo("v-empty"); !known || name != "" {
		t.Fatalf("empty-name volume: want (\"\",true), got (%q,%v)", name, known)
	}
	// Absent id → known=false.
	if _, _, known := topo.VolumeInfo("nope"); known {
		t.Fatal("unknown id must report known=false")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/powerstore/ -run TestVolumeInfoKnown -v`
Expected: FAIL to compile — `VolumeInfo` returns 2 values, not 3.

- [ ] **Step 3: Change the `VolumeInfo` signature**

In `internal/powerstore/topology.go`, lines 115-118:

```go
// VolumeInfo returns the name and appliance ID for a volume id, plus whether the
// id was found in the inventory index. A false `known` means the id was absent
// (e.g. a snapshot/clone present in the bulk CSV but filtered out of the volume
// inventory); callers fall back to the id for the name and may count the miss.
func (t *Topology) VolumeInfo(id string) (name, applianceID string, known bool) {
	name, known = t.volumeName[id]
	return name, t.volumeApplianceID[id], known
}
```

- [ ] **Step 4: Adapt the per-entity caller (no behavior change)**

In `internal/powerstore/derive_perentity.go`, line 18:

```go
		volName, applID, _ := topo.VolumeInfo(volID)
```

- [ ] **Step 5: Run the topology test to verify it passes**

Run: `go test ./internal/powerstore/ -run TestVolumeInfoKnown -v`
Expected: PASS.

- [ ] **Step 6: Write the failing bulk fallback test**

Add to `internal/powerstore/derive_bulk_test.go`:

```go
func TestDeriveBulkVolumePerfNameFallback(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "c1"}, nil,
		[]gopowerstore.Volume{{ID: "v-known", Name: "prod-db", ApplianceID: "a-1"}},
		nil, nil, nil, nil, nil,
	)
	rows := []map[string]string{
		{"volume_id": "v-known", "appliance_id": "a-1", "avg_read_iops": "1"},
		{"volume_id": "v-snap", "appliance_id": "a-1", "avg_read_iops": "2"}, // not in inventory
	}

	got := deriveBulkVolumePerf("p1", topo, rows)

	if _, ok := sampleByLabel(got, "powerstore_volume_read_iops", "volume_name", "prod-db"); !ok {
		t.Fatal("known volume must resolve to its name")
	}
	// Unresolved row keeps the UUID as the name (keep + count decision).
	if _, ok := sampleByLabel(got, "powerstore_volume_read_iops", "volume_name", "v-snap"); !ok {
		t.Fatal("unresolved row must fall back to volume_id as volume_name")
	}
}
```

- [ ] **Step 7: Run it to verify it fails**

Run: `go test ./internal/powerstore/ -run TestDeriveBulkVolumePerfNameFallback -v`
Expected: FAIL to compile — `derive_bulk.go` still calls the 2-value `VolumeInfo`.

- [ ] **Step 8: Update the bulk caller — fall back, count misses, debug-log**

In `internal/powerstore/derive_bulk.go`: add `"fmt"` and `"github.com/fjacquet/pstore_exporter/internal/logging"` to the imports, and rewrite `deriveBulkVolumePerf`'s loop head + add a post-loop log:

```go
func deriveBulkVolumePerf(array string, topo *Topology, rows []map[string]string) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	misses := 0
	for _, r := range rows {
		volID := r["volume_id"]
		volName, applID, known := topo.VolumeInfo(volID)
		if volName == "" {
			volName = volID // keep the UUID for both a true miss and an empty name
		}
		if !known {
			misses++
		}
		if applID == "" {
			applID = r["appliance_id"]
		}
		vgID, vgName := topo.VolumeGroupOf(volID)
		labels := volumeLabels(array, clusterID, volName, volID, applID, topo.ApplianceName(applID), vgName, vgID)
		out = append(out,
			Sample{"powerstore_volume_read_iops", labels, csvFloat(r, "avg_read_iops", "read_iops")},
			Sample{"powerstore_volume_write_iops", labels, csvFloat(r, "avg_write_iops", "write_iops")},
			Sample{"powerstore_volume_total_iops", labels, csvFloat(r, "avg_total_iops", "total_iops")},
			Sample{"powerstore_volume_read_bandwidth_bytes_per_second", labels, csvFloat(r, "avg_read_bandwidth", "read_bandwidth")},
			Sample{"powerstore_volume_write_bandwidth_bytes_per_second", labels, csvFloat(r, "avg_write_bandwidth", "write_bandwidth")},
			Sample{"powerstore_volume_read_latency_microseconds", labels, csvFloat(r, "avg_read_latency")},
			Sample{"powerstore_volume_write_latency_microseconds", labels, csvFloat(r, "avg_write_latency")},
			Sample{"powerstore_volume_avg_io_size_bytes", labels, csvFloat(r, "avg_io_size")},
		)
	}
	if misses > 0 {
		logging.LogDebug(fmt.Sprintf("array %q: %d bulk volume rows unresolved to inventory (snapshots/clones)", array, misses))
	}
	return out
}
```

- [ ] **Step 9: Run the bulk test and the full package**

Run: `go test ./internal/powerstore/ -run TestDeriveBulkVolumePerfNameFallback -v && go test ./internal/powerstore/`
Expected: PASS (including the existing parity test).

- [ ] **Step 10: Commit**

```bash
git add internal/powerstore/topology.go internal/powerstore/topology_test.go internal/powerstore/derive_bulk.go internal/powerstore/derive_bulk_test.go internal/powerstore/derive_perentity.go
git commit -m "feat(powerstore): make bulk volume-name misses observable

VolumeInfo now returns comma-ok so the bulk path can tell a true
inventory miss (snapshot/clone in the CSV) from an empty name. Output is
unchanged (unresolved rows keep the UUID as volume_name); a --debug log
reports how many rows were unresolved."
```

---

### Task 4: Enumerate replication sessions generically; reconcile transfer (#2)

`replicationMetrics` only probes volumes carrying a `ProtectionPolicyID`, so the real session — a **Metro** session (`role=Metro_Preferred`, `state=Paused`) not driven by a protection policy — is never queried, and `replication_session_state` is missing. Separately, `VolumeMirrorTransferRate` returns a phantom 0/0 row for a policy-bearing volume with no session. Fix: enumerate `replication_session` via the generic API (the drives/witnesses pattern), emit state for every session (incl. Metro), and drive transfer metrics from the volume-type sessions so phantoms disappear. `replication_rpo_seconds` (per-rule) is unchanged.

**Files:**
- Modify: `internal/powerstore/client.go:295-366` (rewrite `replicationMetrics`; add `enumerateReplicationSessions` and `replicationTransfers`)
- Modify: `internal/powerstore/derive_replication.go` (add pure `replicatedVolumeResources`)
- Test: `internal/powerstore/derive_replication_test.go` (reconcile helper + a Metro guard)

**Interfaces:**
- Consumes: `deriveReplicationSessions(array string, topo *Topology, sessions []gopowerstore.ReplicationSession) []Sample` (unchanged); `deriveReplicationTransfer(...)`, `deriveReplicationRules(...)`, `parallelSamples[T]`, `isNotFound`, `logging.LogWarn`.
- Produces: `replicatedVolumeResources(sessions []gopowerstore.ReplicationSession) []string` — the `LocalResourceID`s of volume-type sessions to query transfer for.

- [ ] **Step 1: Write the failing reconcile-helper test**

Add to `internal/powerstore/derive_replication_test.go` (add `"reflect"` to imports):

```go
func TestReplicatedVolumeResources(t *testing.T) {
	sessions := []gopowerstore.ReplicationSession{
		{ID: "rs-1", ResourceType: "volume", LocalResourceID: "v-1"},
		{ID: "rs-2", ResourceType: "file_system", LocalResourceID: "fs-1"}, // not a volume → skip
		{ID: "rs-3", ResourceType: "volume", LocalResourceID: ""},          // no id → skip
		{ID: "rs-4", ResourceType: "volume", LocalResourceID: "v-2"},
	}
	got := replicatedVolumeResources(sessions)
	want := []string{"v-1", "v-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/powerstore/ -run TestReplicatedVolumeResources -v`
Expected: FAIL to compile — `replicatedVolumeResources` undefined.

- [ ] **Step 3: Add the pure helper**

In `internal/powerstore/derive_replication.go`, append:

```go
// replicatedVolumeResources returns the local resource IDs of volume-type
// replication sessions — the resources whose live mirror transfer rate is worth
// querying. Non-volume sessions (file_system, virtual_volume) and sessions with
// no local resource id are skipped. Driving transfer queries from real sessions
// (rather than from every protection-policy volume) avoids phantom 0/0 series.
func replicatedVolumeResources(sessions []gopowerstore.ReplicationSession) []string {
	var ids []string
	for _, s := range sessions {
		if s.ResourceType == "volume" && s.LocalResourceID != "" {
			ids = append(ids, s.LocalResourceID)
		}
	}
	return ids
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/powerstore/ -run TestReplicatedVolumeResources -v`
Expected: PASS.

- [ ] **Step 5: Add a Metro-session guard test**

Add to `internal/powerstore/derive_replication_test.go` (guards that a Metro session, once fetched, produces a state series — the behavior that was previously never exercised because Metro sessions were never queried):

```go
func TestDeriveReplicationSessionMetro(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	sessions := []gopowerstore.ReplicationSession{
		{ID: "rs-metro", State: gopowerstore.RsStatePaused, Role: "Metro_Preferred",
			Type: "Metro_Active_Active", ResourceType: "volume",
			RemoteSystemID: "remote-9", LocalResourceID: "v-9"},
	}
	got := deriveReplicationSessions("p1", topo, sessions)

	if v, ok := sampleByLabel(got, "powerstore_replication_session_state", "state", "Paused"); !ok || v != 1 {
		t.Fatalf("metro session state=Paused: want 1, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "role", "Metro_Preferred"); !ok {
		t.Fatal("expected a role=Metro_Preferred label")
	}
}
```

Run: `go test ./internal/powerstore/ -run TestDeriveReplicationSessionMetro -v`
Expected: PASS (guards existing derive behavior).

- [ ] **Step 6: Add the generic session enumeration**

In `internal/powerstore/client.go`, add alongside `enumerateWitnesses` (the `api` package is already imported):

```go
// enumerateReplicationSessions lists replication sessions via the generic API,
// paginating defensively. gopowerstore exposes no list-sessions method (only
// by-id/by-local-resource lookups), so the resource is read with the generic
// Query escape hatch (see ADR-0009), which also captures Metro sessions that a
// protection-policy-driven probe would miss.
func (c *ArrayClient) enumerateReplicationSessions(ctx context.Context) ([]gopowerstore.ReplicationSession, error) {
	const pageSize = 2000
	var all []gopowerstore.ReplicationSession
	for offset := 0; ; offset += pageSize {
		qp := c.gp.APIClient().QueryParams().
			Select("id", "state", "role", "type", "resource_type", "local_resource_id", "remote_system_id").
			Order("id").
			Limit(pageSize).
			Offset(offset)

		var page []gopowerstore.ReplicationSession
		_, err := c.gp.APIClient().Query(ctx, api.RequestConfig{
			Method:      "GET",
			Endpoint:    "replication_session",
			QueryParams: qp,
		}, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
	}
}
```

- [ ] **Step 7: Rewrite `replicationMetrics` and add the transfer fan-out**

Replace the body of `replicationMetrics` (`client.go:295-366`) with:

```go
func (c *ArrayClient) replicationMetrics(ctx context.Context, topo *Topology) []Sample {
	var samples []Sample

	// RPO is per-rule cluster config, independent of active sessions.
	if rules, err := c.gp.GetReplicationRules(ctx); err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: get replication rules: %v", c.name, err))
	} else {
		samples = append(samples, deriveReplicationRules(c.name, topo, rules)...)
	}

	// Enumerate all sessions (captures async, sync, AND Metro sessions that a
	// protection-policy probe cannot see).
	sessions, err := c.enumerateReplicationSessions(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: enumerate replication sessions: %v", c.name, err))
		return samples
	}
	samples = append(samples, deriveReplicationSessions(c.name, topo, sessions)...)

	// Transfer/backlog only for volumes that actually have a session — no phantom
	// 0/0 series for policy volumes with no session.
	samples = append(samples, c.replicationTransfers(ctx, topo, replicatedVolumeResources(sessions))...)
	return samples
}

// replicationTransfers fans out the mirror transfer rate query across the given
// volume ids, emitting transfer-rate and data-remaining samples. A not-found is
// skipped silently (no active transfer); other errors are logged.
func (c *ArrayClient) replicationTransfers(ctx context.Context, topo *Topology, volumeIDs []string) []Sample {
	return parallelSamples(ctx, volumeIDs, c.maxConcurrency, func(ctx context.Context, volID string) []Sample {
		rate, err := c.gp.VolumeMirrorTransferRate(ctx, volID)
		if err != nil {
			if !isNotFound(err) {
				logging.LogWarn(fmt.Sprintf("array %q: mirror transfer rate for volume %s: %v", c.name, volID, err))
			}
			return nil
		}
		return deriveReplicationTransfer(c.name, topo, volID, "volume", rate)
	})
}
```

This removes the old per-volume `GetReplicationSessionByLocalResourceID` probe, the `ProtectionPolicyID` candidate loop, and the `resReplication` struct.

- [ ] **Step 8: Verify build, then run package + integration tests with the race detector**

Run: `go build ./... && go test -race ./internal/powerstore/`
Expected: PASS. Pay attention to `collector_test.go` and `pipeline_integration_test.go` — the mock's generic `APIClient().Query` (already used by drives/witnesses) now also serves `replication_session`; confirm they still pass (an empty session list yields no session/transfer samples).

- [ ] **Step 9: Update docs/Grafana**

In `docs/metrics.md`, note that `powerstore_replication_session_state` now covers Metro sessions (`role=Metro_Preferred`/`Metro_Non_Preferred`, `type=Metro_Active_Active`). If a Grafana dashboard has a replication-session panel, verify it renders the newly-populated series and consider a Metro-session state row (lockstep).

- [ ] **Step 10: Commit**

```bash
git add internal/powerstore/client.go internal/powerstore/derive_replication.go internal/powerstore/derive_replication_test.go docs/metrics.md
git commit -m "fix(powerstore): enumerate replication sessions generically

Probe was gated on ProtectionPolicyID, so Metro sessions were never seen
and policy volumes with no session emitted phantom 0/0 transfer series.
Enumerate replication_session via the generic API (the drives/witnesses
pattern) to capture Metro sessions, and drive transfer metrics from the
real session list. RPO (per-rule) is unchanged."
```

**Live validation (after deploy):** `powerstore_replication_session_state{role="Metro_Preferred",state="Paused"}` appears; the phantom `powerstore_replication_transfer_rate_bytes_per_second` / `_data_remaining_bytes` for the session-less volume disappears.

---

## Self-Review

**1. Spec coverage:**
- #1 drives → Task 1 (tag + Select + LogDebug + count log). ✓
- #2 replication → Task 4 (generic enumeration, transfer reconcile, RPO unchanged, Metro captured). ✓
- #3 file systems → Task 2 (`chartableFileSystem`, both paths). ✓
- #4 volume name → Task 3 (`VolumeInfo` comma-ok, keep+count, debug log). ✓
- Cross-cutting: parity (Task 3 edits both derive paths; Task 4 stays in shared `commonMetrics`); docs updates in Tasks 1/2/4; TDD throughout. ✓

**2. Placeholder scan:** No TBD/TODO; every code step shows complete code; every run step states the exact command and expected result. ✓

**3. Type consistency:** `VolumeInfo` 3-return used consistently in Task 3 (topology.go def, both callers). `chartableFileSystem(gopowerstore.FileSystem) bool` used identically in capacity and perf paths. `replicatedVolumeResources([]gopowerstore.ReplicationSession) []string` and `deriveReplicationSessions([]gopowerstore.ReplicationSession)` match the SDK type enumerated by `enumerateReplicationSessions`. `logging.LogDebug(string)` defined in Task 1, consumed in Task 3. ✓

## Out of Scope (tracked, not built here)

- Transfer/backlog metrics for `file_system`/`virtual_volume` replication sessions.
- A dedicated exporter metric for the unresolved-row count (vs. the Task 3 debug log).
- The intermittent bulk-download 404 that triggers the per-entity fallback (handled gracefully today).
