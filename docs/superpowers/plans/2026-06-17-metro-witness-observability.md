# Metro Witness Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the Dell PowerStore Metro witness service state and per-node connection state as Prometheus + OTLP gauges.

**Architecture:** Mirror the existing drive-enumeration path. A new `derive_witness.go` decodes the `/witness` REST resource (fetched through `gopowerstore`'s generic `APIClient().Query` escape hatch, since the SDK has no typed witness method) into info-style samples, wired into `ArrayClient.commonMetrics` so both the bulk and per-entity export paths emit it.

**Tech Stack:** Go, `github.com/dell/gopowerstore` v1.22.0, Prometheus client_golang, OTLP. Tests: standard `go test`.

## Global Constraints

- Metric prefix is `powerstore_`; metrics port 9446. (verbatim from CLAUDE.md)
- Every metric's first labels are `array` then `cluster_id` (via `baseLabels`).
- Info-style metrics: value is always `1`, the enum lives in a `state` label.
- "Absent, never zero": an unparseable/missing field yields **no sample**, never a fabricated `0`.
- Per-metric failure degrades gracefully — `LogWarn` and return `nil`; never fail the whole collection cycle (`powerstore_up` stays `1`).
- No inline `//nolint` and no semgrep suppressions — fix the finding. (CLAUDE.md Semgrep policy)
- TDD, frequent commits. `make ci` (fmt-check, vet, lint, test-race, govulncheck) must be green before pushing.
- Metric changes update `docs/metrics.md` + Grafana dashboards in lockstep, and record an ADR.
- Conventional-commit messages; end every commit message with the trailer:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- **Create** `internal/powerstore/derive_witness.go` — `witnessInfo`/`witnessConnection` structs + `deriveWitness`. One responsibility: map a decoded witness resource to samples.
- **Create** `internal/powerstore/derive_witness_test.go` — offline unit tests for `deriveWitness`.
- **Modify** `internal/powerstore/metrics.go` — add `witnessStateLabels` + `witnessConnectionLabels` builders.
- **Modify** `internal/powerstore/client.go` — add `enumerateWitnesses` + `witnessMetrics`; wire one line into `commonMetrics`.
- **Modify** `docs/metrics.md` — two metric rows + alert examples.
- **Modify** `grafana/dashboards/protection/01-replication.json` — a "Metro witness" status panel.
- **Create** `docs/adr/0015-metro-witness-via-generic-query.md` — the decision record.

---

## Task 1: Witness derive function + label builders

The pure, fully offline-testable core: decode → samples.

**Files:**
- Create: `internal/powerstore/derive_witness.go`
- Create: `internal/powerstore/derive_witness_test.go`
- Modify: `internal/powerstore/metrics.go` (add two builders after `driveStateLabels`, ~line 92)

**Interfaces:**
- Consumes: `Sample` (`{Name string; Labels []Label; Value float64}`), `Label`, `baseLabels(arrayName, clusterID string) []Label`, `Topology.ClusterID() string`, and the test helpers `NewTopology(gopowerstore.Cluster{...}, nil×7)` and `sampleByLabel(samples, metricName, labelName, labelValue) (float64, bool)` — all already in the package.
- Produces:
  - `witnessInfo struct { ID, Name, State string; Connections []witnessConnection }`
  - `witnessConnection struct { State, ApplianceID, NodeID string }`
  - `deriveWitness(array string, topo *Topology, witnesses []witnessInfo) []Sample`
  - `witnessStateLabels(arrayName, clusterID, witnessID, witnessName, state string) []Label`
  - `witnessConnectionLabels(arrayName, clusterID, witnessID, applianceID, nodeID, state string) []Label`

- [ ] **Step 1: Write the failing test**

Create `internal/powerstore/derive_witness_test.go`:

```go
package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestDeriveWitnessStateAndConnections(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	witnesses := []witnessInfo{
		{
			ID:    "w-1",
			Name:  "witness-east",
			State: "OK",
			Connections: []witnessConnection{
				{State: "OK", ApplianceID: "a-1", NodeID: "n-1"},
				{State: "Disconnected", ApplianceID: "a-1", NodeID: "n-2"},
			},
		},
	}

	got := deriveWitness("p1", topo, witnesses)

	// Overall service state: info series, value 1, state in a label.
	if v, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", "w-1"); !ok || v != 1 {
		t.Fatalf("witness state series: want 1, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "state", "OK"); !ok {
		t.Fatal("expected a state=OK witness series")
	}
	// One connection series per node, value 1.
	if v, ok := sampleByLabel(got, "powerstore_metro_witness_connection_state", "node_id", "n-1"); !ok || v != 1 {
		t.Fatalf("n-1 connection series: want 1, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_connection_state", "state", "Disconnected"); !ok {
		t.Fatal("expected a state=Disconnected connection series for n-2")
	}
}

func TestDeriveWitnessSkipsEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	witnesses := []witnessInfo{
		{ID: "", State: "OK"},                                  // no ID → skipped entirely
		{ID: "w-2", State: ""},                                 // no state → no state series
		{ID: "w-3", State: "OK", Connections: []witnessConnection{{State: "", NodeID: "n-9"}}}, // empty conn state → skipped
	}

	got := deriveWitness("p1", topo, witnesses)

	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", ""); ok {
		t.Fatal("witness with empty ID must emit nothing")
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", "w-2"); ok {
		t.Fatal("witness with empty state must not emit a state series")
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_connection_state", "node_id", "n-9"); ok {
		t.Fatal("connection with empty state must not emit a series")
	}
	// w-3 itself (state OK) should still emit its state series.
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", "w-3"); !ok {
		t.Fatal("w-3 should still emit its OK state series")
	}
}

func TestDeriveWitnessEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	if got := deriveWitness("p1", topo, nil); len(got) != 0 {
		t.Fatalf("no witnesses should emit nothing, got %+v", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/powerstore/ -run TestDeriveWitness -v`
Expected: FAIL — compile error, `undefined: witnessInfo`, `undefined: deriveWitness`.

- [ ] **Step 3: Add the label builders**

In `internal/powerstore/metrics.go`, immediately after `driveStateLabels` (ends ~line 92), add:

```go
// witnessStateLabels builds the label set for the Metro witness service info
// series. `state` is the WitnessStateEnum value as a string.
func witnessStateLabels(arrayName, clusterID, witnessID, witnessName, state string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"witness_id", witnessID},
		Label{"witness_name", witnessName},
		Label{"state", state},
	)
}

// witnessConnectionLabels builds the per-node connection label set for the Metro
// witness. `state` is the WitnessConnectionStateEnum value as a string.
func witnessConnectionLabels(arrayName, clusterID, witnessID, applianceID, nodeID, state string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"witness_id", witnessID},
		Label{"appliance_id", applianceID},
		Label{"node_id", nodeID},
		Label{"state", state},
	)
}
```

- [ ] **Step 4: Create the derive function**

Create `internal/powerstore/derive_witness.go`:

```go
package powerstore

// witnessConnection is the subset of a PowerStore witness_connection_instance we
// export: one node's connection to the witness service.
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

// deriveWitness emits the Metro witness service state and one connection-state
// series per node. Both are info-style: the value is always 1 and the current
// state is carried as a label (the kube-state-metrics enum idiom). Operators
// alert on undesirable states, e.g. {state=~"Disconnected|Partially_Connected"}.
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

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/powerstore/ -run TestDeriveWitness -v`
Expected: PASS (all three tests).

- [ ] **Step 6: Commit**

```bash
git add internal/powerstore/derive_witness.go internal/powerstore/derive_witness_test.go internal/powerstore/metrics.go
git commit -m "feat(powerstore): derive Metro witness state metrics

Add deriveWitness + witness label builders emitting
powerstore_metro_witness_state and powerstore_metro_witness_connection_state
as info series (value 1, state in a label).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Fetch the witness resource and wire it into the collector

Connects the derive to the live API. The generic-`Query` fetch is **not** offline-unit-testable (same as `enumerateDrives` — see CLAUDE.md "Testing"); verification here is a clean compile + no test regression + lint, with live validation documented for an operator.

**Files:**
- Modify: `internal/powerstore/client.go` — add `enumerateWitnesses` + `witnessMetrics` near `driveMetrics`/`enumerateDrives` (~lines 442–477); add one line to `commonMetrics` (~line 534, after `c.driveMetrics`).

**Interfaces:**
- Consumes: `c.gp.APIClient().Query`, `c.gp.APIClient().QueryParams()`, `api.RequestConfig` (`api "github.com/dell/gopowerstore/api"` already imported), `isNotFound(err error) bool` (client.go:131), `logging.LogWarn` (already imported), `fmt` (already imported), `deriveWitness` (Task 1), `c.name`, `topo`.
- Produces: `witnessMetrics(ctx context.Context, topo *Topology) []Sample`, called from `commonMetrics`.

- [ ] **Step 1: Add `enumerateWitnesses` and `witnessMetrics`**

In `internal/powerstore/client.go`, immediately after `enumerateDrives` (ends ~line 477), add:

```go
// witnessMetrics fetches the Metro witness service and derives its state series.
// A 404 means the array predates the witness feature (PowerStoreOS < 3.6) or has
// no witness configured — that is benign and silently yields no samples. Any
// other error is logged and degraded to no samples (powerstore_up stays 1).
func (c *ArrayClient) witnessMetrics(ctx context.Context, topo *Topology) []Sample {
	witnesses, err := c.enumerateWitnesses(ctx)
	if err != nil {
		if !isNotFound(err) {
			logging.LogWarn(fmt.Sprintf("array %q: enumerate witnesses: %v", c.name, err))
		}
		return nil
	}
	return deriveWitness(c.name, topo, witnesses)
}

// enumerateWitnesses lists Metro witness services via the generic API, paginating
// defensively. The witness list is tiny (typically one), but the pattern mirrors
// enumerateDrives for consistency. gopowerstore has no typed witness method, so
// the resource is read with the generic Query escape hatch (see ADR-0009/0015).
func (c *ArrayClient) enumerateWitnesses(ctx context.Context) ([]witnessInfo, error) {
	const pageSize = 2000
	var all []witnessInfo
	for offset := 0; ; offset += pageSize {
		qp := c.gp.APIClient().QueryParams().
			Select("id", "name", "state", "connections").
			Order("id").
			Limit(pageSize).
			Offset(offset)

		var page []witnessInfo
		_, err := c.gp.APIClient().Query(ctx, api.RequestConfig{
			Method:      "GET",
			Endpoint:    "witness",
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

- [ ] **Step 2: Wire it into `commonMetrics`**

In `internal/powerstore/client.go`, in `commonMetrics` (~line 534), add the witness line after the drive line:

```go
	samples = append(samples, c.driveMetrics(ctx, topo)...)
	samples = append(samples, c.witnessMetrics(ctx, topo)...)
	return samples
```

- [ ] **Step 3: Verify it compiles**

Run: `make cli`
Expected: builds `bin/pstore_exporter` with no errors.

- [ ] **Step 4: Verify no test regression + lint clean**

Run: `make sure`
Expected: fmt, vet, test, build, lint all pass (Task 1's tests still green; no new offline test for the fetch, by design).

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/client.go
git commit -m "feat(powerstore): fetch /witness and emit witness metrics

Enumerate the witness resource via the generic Query escape hatch and wire
witnessMetrics into commonMetrics so both export paths emit it. Benign 404s
(pre-3.6 arrays / no witness configured) are silenced.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 6: (Operator, against a live array — cannot run here) Live validation**

On a PowerStoreOS 3.6+ array with a configured witness:
```bash
PSTORE1_PASSWORD=… ./bin/pstore_exporter --config real.yaml --once --debug --trace 2>trace.log | sort > samples.txt
grep powerstore_metro_witness samples.txt
```
Expected: `powerstore_metro_witness_state{…,state="OK"} 1` and one `powerstore_metro_witness_connection_state{…} 1` per node. On an array without the feature: no witness lines, and no "enumerate witnesses" warning in the logs.

---

## Task 3: Lockstep docs, dashboard, and ADR

**Files:**
- Modify: `docs/metrics.md` — add two rows + alert examples in the protection/replication section.
- Modify: `grafana/dashboards/protection/01-replication.json` — add a witness status panel.
- Create: `docs/adr/0015-metro-witness-via-generic-query.md`.

**Interfaces:** Consumes the metric names/labels from Tasks 1–2. Produces no code.

- [ ] **Step 1: Document the metrics**

In `docs/metrics.md`, after the `powerstore_replication_data_remaining_bytes` row, add:

```markdown
| `powerstore_metro_witness_state` | `witness_id`, `witness_name`, `state` | Info series, always `1`; the Metro witness service's overall state is the `state` label (`OK`, `Partially_Connected`, `Disconnected`, `Initializing`, `Deleting`). |
| `powerstore_metro_witness_connection_state` | `witness_id`, `appliance_id`, `node_id`, `state` | Info series, always `1`; one per node, with the node's connection to the witness in the `state` label (`OK`, `Disconnected`, `Initializing`). |
```

Then, near the replication alert examples, add:

```markdown
# Metro witness degraded or unreachable
powerstore_metro_witness_state{state=~"Disconnected|Partially_Connected"}
# A specific node cannot reach the witness
powerstore_metro_witness_connection_state{state="Disconnected"}
```

- [ ] **Step 2: Add the Grafana panel**

In `grafana/dashboards/protection/01-replication.json`, append this object to the `panels` array (set `id` to one higher than the current max panel `id`, and set `gridPos.y` below the last row so it doesn't overlap):

```json
{
  "type": "state-timeline",
  "title": "Metro witness state",
  "description": "Overall Metro witness service state and per-node connectivity. Non-OK = automatic Metro failover at risk.",
  "datasource": { "type": "prometheus", "uid": "${datasource}" },
  "fieldConfig": {
    "defaults": {
      "custom": { "fillOpacity": 80, "lineWidth": 0 },
      "mappings": [
        { "type": "value", "options": { "OK": { "color": "green", "index": 0 } } },
        { "type": "value", "options": { "Initializing": { "color": "yellow", "index": 1 } } },
        { "type": "value", "options": { "Partially_Connected": { "color": "orange", "index": 2 } } },
        { "type": "value", "options": { "Disconnected": { "color": "red", "index": 3 } } }
      ]
    }
  },
  "targets": [
    {
      "datasource": { "type": "prometheus", "uid": "${datasource}" },
      "expr": "powerstore_metro_witness_state{array=~\"$array\"} * 0 + label_replace(powerstore_metro_witness_state{array=~\"$array\"}, \"witness\", \"$1\", \"witness_name\", \"(.*)\")",
      "legendFormat": "{{array}} / {{witness_name}} ({{state}})",
      "refId": "A"
    },
    {
      "datasource": { "type": "prometheus", "uid": "${datasource}" },
      "expr": "powerstore_metro_witness_connection_state{array=~\"$array\"}",
      "legendFormat": "{{array}} / node {{node_id}} ({{state}})",
      "refId": "B"
    }
  ],
  "gridPos": { "h": 8, "w": 24, "x": 0, "y": 999 }
}
```

Note: match the dashboard's existing template-variable names (`$array`, `${datasource}`) — open the file and confirm the variable names in `templating.list` before saving; adjust the panel's `$array`/`${datasource}` references if they differ. Set `gridPos.y` to the actual next free row, not `999`.

- [ ] **Step 3: Validate the dashboard JSON**

Run: `python3 -c "import json,sys; json.load(open('grafana/dashboards/protection/01-replication.json')); print('valid')"`
Expected: `valid`.

- [ ] **Step 4: Write the ADR**

Create `docs/adr/0015-metro-witness-via-generic-query.md`:

```markdown
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
```

- [ ] **Step 5: Commit**

```bash
git add docs/metrics.md grafana/dashboards/protection/01-replication.json docs/adr/0015-metro-witness-via-generic-query.md
git commit -m "docs(powerstore): document Metro witness metrics, dashboard, ADR-0015

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Final gate

- [ ] **Step 1: Run the full CI gate**

Run: `make ci`
Expected: fmt-check, vet, lint, test-race, govulncheck all green.

- [ ] **Step 2: Confirm the witness metrics are documented and emitted consistently**

Run: `grep -c metro_witness docs/metrics.md internal/powerstore/derive_witness.go`
Expected: both files report matches (names match between code and docs).

---

## Self-Review

**Spec coverage:**
- Witness service state metric → Task 1 (`powerstore_metro_witness_state`). ✓
- Per-node connection state metric → Task 1 (`powerstore_metro_witness_connection_state`). ✓
- Generic-`Query` fetch mirroring drives → Task 2 (`enumerateWitnesses`). ✓
- Wired on both export paths → Task 2 (`commonMetrics`). ✓
- Benign-404 / absent-never-zero error handling → Task 2 (`isNotFound`) + Task 1 (empty-skip tests). ✓
- Offline unit test + documented live validation → Task 1 + Task 2 Step 6. ✓
- `docs/metrics.md`, Grafana, ADR-0015 in lockstep → Task 3. ✓
- `address`/`last_updated_timestamp` excluded (YAGNI) → not in `witnessInfo`. ✓
- Out-of-scope items (per-session engagement, remote systems, policies) → ADR records them as deferred. ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete code. The Grafana `gridPos.y: 999` and `$array`/`${datasource}` references are explicitly flagged in Task 3 Step 2 as values to reconcile against the live dashboard, with how-to — not silent placeholders.

**Type consistency:** `witnessInfo`/`witnessConnection` field names, `deriveWitness` signature, and the two label builders are used identically across Tasks 1–2. Metric names `powerstore_metro_witness_state` / `powerstore_metro_witness_connection_state` match across code, tests, docs, dashboard, and ADR.
