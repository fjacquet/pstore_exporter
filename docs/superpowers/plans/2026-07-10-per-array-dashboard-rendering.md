# Per-Array Dashboard Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every Grafana panel attribute its value to a named array, and give each array
and appliance a stable colour, so an operator running two PowerStores can read the dashboards.

**Architecture:** Pure JSON edits to `grafana/dashboards/**/*.json` plus a Go test that walks
those files and fails CI on the two defect classes. No collector change: every metric and label
required already exists. The test is written first and must fail on today's JSON.

**Tech Stack:** Grafana 10+ dashboard JSON, PromQL, Go 1.26.5 standard library (`encoding/json`,
`testing`), existing `make ci`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-10-per-array-dashboard-rendering-design.md`.
- Branch `fix/per-array-dashboard-rendering` already exists with the spec committed.
- Go toolchain floor is **1.26.5** (`go.mod`). Module path `github.com/fjacquet/pstore_exporter`.
- **No inline `//nolint` or semgrep suppressions.** Fix the finding (project policy, `CLAUDE.md`).
- Dashboards are hand-authored JSON. No jsonnet, no generator (ADR-0014).
- Grid geometry (`gridPos`) is never modified. No panel is added, removed, or repeated.
- Only overrides whose matcher is exactly `/ read$/` or `/ write$/` may be touched. The
  `/ used$/` and `/ total$/` overrides on capacity panels express a different distinction and
  must be left byte-identical.
- Adding or renaming a metric requires updating dashboards + `docs/metrics.md` +
  `docs/dashboards.md` in lockstep. This change renames no metric, but does touch dashboards.
- Prefix every shell command with `rtk` per `CLAUDE.md`, including inside `&&` chains.
- Run `make ci` before pushing.

---

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/dashboards/doc.go` | **Create.** Package clause so the directory builds. |
| `internal/dashboards/lint_test.go` | **Create.** Walks `../../grafana/dashboards`, enforces rules R1–R3. |
| `grafana/dashboards/overview/00-fleet-health.json` | **Modify.** 2 bare aggregations, 1 vanishing tile, 2 stat colour overrides. |
| `grafana/dashboards/overview/01-cluster-overview.json` | **Modify.** 3 bare aggregations, 2 timeseries colour panels. |
| `grafana/dashboards/block/05-capacity.json` | **Modify.** 4 bare aggregations. |
| `grafana/dashboards/block/06-ports.json` | **Modify.** 2 bare aggregations. |
| `grafana/dashboards/hardware/01-drives.json` | **Modify.** 2 bare aggregations, 1 needs zero-fill. |
| `grafana/dashboards/protection/01-replication.json` | **Modify.** 1 bare aggregation, needs zero-fill. |
| `grafana/dashboards/block/02-appliances.json` | **Modify.** 5 timeseries colour panels. |
| `grafana/dashboards/block/04-volume-groups.json` | **Modify.** 2 timeseries colour panels. |
| `grafana/dashboards/file/01-file-systems.json` | **Modify.** 2 timeseries colour panels. |
| `docs/adr/0016-per-array-rendering-and-series-colour.md` | **Create.** Supersedes ADR-0014's colour clause. |
| `docs/dashboards.md` | **Modify.** Restate colour convention + zero-fill idiom. |

Why `internal/dashboards/` and not `grafana/`: a directory holding only `_test.go` files makes
`go build ./...` report *"no non-test Go files"*. The one-line `doc.go` avoids that entirely.

---

## Task 1: Dashboard linter (R1) and the fourteen bare aggregations

The linter is written first and **must fail**, naming all fourteen panels. Fixing the JSON is
what makes it pass. Test and fix commit together — a red test on `main` breaks CI for everyone.

**Files:**
- Create: `internal/dashboards/doc.go`
- Create: `internal/dashboards/lint_test.go`
- Modify: `grafana/dashboards/overview/00-fleet-health.json`
- Modify: `grafana/dashboards/overview/01-cluster-overview.json`
- Modify: `grafana/dashboards/block/05-capacity.json`
- Modify: `grafana/dashboards/block/06-ports.json`
- Modify: `grafana/dashboards/hardware/01-drives.json`
- Modify: `grafana/dashboards/protection/01-replication.json`

**Interfaces:**
- Produces: `internal/dashboards.loadPanels(t *testing.T) []panelRef` — used by Task 2's rules.
  `panelRef` is `struct { File, Title, Type string; Targets []target; FieldConfig fieldConfig }`.
- Consumes: nothing.

- [ ] **Step 1: Create the package clause**

`internal/dashboards/doc.go`:

```go
// Package dashboards holds no production code. It exists so the bundled Grafana
// dashboard JSON can be linted by `go test ./...` without adding a toolchain.
//
// See docs/adr/0016-per-array-rendering-and-series-colour.md.
package dashboards
```

- [ ] **Step 2: Write the failing linter**

`internal/dashboards/lint_test.go`:

```go
package dashboards

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// dashboardRoot is relative to this package directory.
const dashboardRoot = "../../grafana/dashboards"

type target struct {
	Expr string `json:"expr"`
}

type colorSpec struct {
	Mode string `json:"mode"`
}

type fieldDefaults struct {
	Color *colorSpec `json:"color"`
}

type property struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

type matcher struct {
	ID      string `json:"id"`
	Options any    `json:"options"`
}

type override struct {
	Matcher    matcher    `json:"matcher"`
	Properties []property `json:"properties"`
}

type fieldConfig struct {
	Defaults  fieldDefaults `json:"defaults"`
	Overrides []override    `json:"overrides"`
}

type panel struct {
	Title       string      `json:"title"`
	Type        string      `json:"type"`
	Targets     []target    `json:"targets"`
	FieldConfig fieldConfig `json:"fieldConfig"`
}

type dashboard struct {
	Panels []panel `json:"panels"`
}

// panelRef is a panel plus the file it came from, for legible failure messages.
type panelRef struct {
	File string
	panel
}

// loadPanels decodes every bundled dashboard and returns its non-row panels.
func loadPanels(t *testing.T) []panelRef {
	t.Helper()

	var out []panelRef
	err := filepath.WalkDir(dashboardRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var dash dashboard
		if err := json.Unmarshal(raw, &dash); err != nil {
			t.Fatalf("%s: invalid dashboard JSON: %v", path, err)
		}
		rel, err := filepath.Rel(dashboardRoot, path)
		if err != nil {
			return err
		}
		for _, p := range dash.Panels {
			if p.Type == "row" {
				continue
			}
			out = append(out, panelRef{File: rel, panel: p})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dashboardRoot, err)
	}
	if len(out) == 0 {
		t.Fatalf("no panels found under %s", dashboardRoot)
	}
	return out
}

// bareAggregation matches an aggregation operator applied directly to a
// parenthesised expression, i.e. one with no `by (...)` clause.
var bareAggregation = regexp.MustCompile(`\b(sum|avg|count|min|max)\s*\(`)

// TestNoBareAggregation enforces R1: a panel expression must never collapse every
// dimension to a single value. With more than one array selected such a panel
// silently blends arrays, and the reader cannot tell which array it describes.
//
// `sum by (appliance_name) (...)` is deliberately allowed: it groups by *something*.
// That it omits `array` is a separate, tracked concern (see ADR-0016).
func TestNoBareAggregation(t *testing.T) {
	for _, p := range loadPanels(t) {
		for _, tgt := range p.Targets {
			if tgt.Expr == "" {
				continue
			}
			if !bareAggregation.MatchString(tgt.Expr) {
				continue
			}
			if strings.Contains(tgt.Expr, "by (") {
				continue
			}
			t.Errorf("%s: panel %q aggregates with no `by (...)` clause, so it "+
				"collapses all arrays into one value:\n  %s", p.File, p.Title, tgt.Expr)
		}
	}
}
```

- [ ] **Step 3: Run the linter and confirm it fails, naming fourteen panels**

```bash
rtk go test ./internal/dashboards/ -run TestNoBareAggregation -v
```

Expected: `FAIL`, with one `panel "..."` error line per panel below. Count them — there must be
exactly **14**. If the count differs, stop: the dashboards have drifted from the spec's audit.

```
00-fleet-health.json      Cluster Capacity Used %
00-fleet-health.json      Data Reduction Ratio
01-cluster-overview.json  Capacity Used %
01-cluster-overview.json  Data Reduction Ratio
01-cluster-overview.json  Efficiency Ratio
05-capacity.json          Data Reduction Ratio
05-capacity.json          Efficiency Ratio
05-capacity.json          Snapshot Savings Ratio
05-capacity.json          Thin Savings Ratio
06-ports.json             Ports Up
06-ports.json             Ports Down
01-drives.json            Unhealthy Drives
01-drives.json            Worn Drives > 80%
01-replication.json       Sessions in Bad State
```

- [ ] **Step 4: Fix the plain ratios and rollups**

In each panel below, replace the target's `expr` and `legendFormat`. Change nothing else.

`grafana/dashboards/overview/00-fleet-health.json` — panel `Cluster Capacity Used %`:

```json
"expr": "100 * sum by (array) (powerstore_cluster_physical_used_bytes{array=~\"$array\"}) / sum by (array) (powerstore_cluster_physical_total_bytes{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/overview/00-fleet-health.json` — panel `Data Reduction Ratio`:

```json
"expr": "avg by (array) (powerstore_cluster_data_reduction_ratio{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/overview/01-cluster-overview.json` — panel `Capacity Used %`:

```json
"expr": "100 * sum by (array) (powerstore_cluster_physical_used_bytes{array=~\"$array\"}) / sum by (array) (powerstore_cluster_physical_total_bytes{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/overview/01-cluster-overview.json` — panel `Data Reduction Ratio`:

```json
"expr": "avg by (array) (powerstore_cluster_data_reduction_ratio{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/overview/01-cluster-overview.json` — panel `Efficiency Ratio`:

```json
"expr": "avg by (array) (powerstore_cluster_efficiency_ratio{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/block/05-capacity.json` — panel `Data Reduction Ratio`:

```json
"expr": "avg by (array) (powerstore_cluster_data_reduction_ratio{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/block/05-capacity.json` — panel `Efficiency Ratio`:

```json
"expr": "avg by (array) (powerstore_cluster_efficiency_ratio{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/block/05-capacity.json` — panel `Snapshot Savings Ratio`:

```json
"expr": "avg by (array) (powerstore_appliance_snapshot_savings_ratio{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

`grafana/dashboards/block/05-capacity.json` — panel `Thin Savings Ratio`:

```json
"expr": "avg by (array) (powerstore_appliance_thin_savings_ratio{array=~\"$array\"})",
"legendFormat": "{{array}}"
```

- [ ] **Step 5: Fix the boolean counts**

`powerstore_port_link_up` is emitted for every port with value `1` or `0`. The `bool` modifier
turns the comparison into a per-port `1`/`0` instead of dropping non-matching series, so an
array whose ports are all up reports `0` rather than disappearing.

Delete the trailing `or vector(0)` — it yields a series with **no `array` label**, which under
`by (array)` renders as a blank-titled tile beside the real ones.

`grafana/dashboards/block/06-ports.json` — panel `Ports Up`:

```json
"expr": "sum by (array) (powerstore_port_link_up{array=~\"$array\"} == bool 1)",
"legendFormat": "{{array}}"
```

`grafana/dashboards/block/06-ports.json` — panel `Ports Down`:

```json
"expr": "sum by (array) (powerstore_port_link_up{array=~\"$array\"} == bool 0)",
"legendFormat": "{{array}}"
```

`grafana/dashboards/hardware/01-drives.json` — panel `Worn Drives > 80%`. Note this needs **no**
zero-fill: `powerstore_drive_wear_level_ratio` is emitted once per drive regardless of value.

```json
"expr": "sum by (array) (powerstore_drive_wear_level_ratio{array=~\"$array\"} > bool 0.8)",
"legendFormat": "{{array}}"
```

- [ ] **Step 6: Fix the info-series counts with a zero-fill**

`powerstore_drive_state` and `powerstore_replication_session_state` are info series whose value
is **always `1`**; the state lives in a label. A healthy array matches no series at all, so
`count by (array)` returns nothing and Grafana draws no tile — indistinguishable from a dead
exporter. `powerstore_up{array}` carries exactly one label and is the zero-filler.

The right-hand `sum by (array)` is **not** redundant: it pins the filler's label set to exactly
`{array}` so the `or` union is label-compatible with the left side.

`grafana/dashboards/hardware/01-drives.json` — panel `Unhealthy Drives`:

```json
"expr": "sum by (array) (powerstore_drive_state{array=~\"$array\",state!=\"Healthy\"}) or sum by (array) (powerstore_up{array=~\"$array\"} * 0)",
"legendFormat": "{{array}}"
```

`grafana/dashboards/protection/01-replication.json` — panel `Sessions in Bad State`:

```json
"expr": "sum by (array) (powerstore_replication_session_state{array=~\"$array\",state=~\"Error|Fractured|Paused|System_Paused\"}) or sum by (array) (powerstore_up{array=~\"$array\"} * 0)",
"legendFormat": "{{array}}"
```

- [ ] **Step 7: Fix the vanishing tile that R1 does not catch**

`overview/00-fleet-health.json` panel `Unhealthy Drives` already has `by (array)`, so R1 passes
it, but it has the same vanishing-tile defect. Replace its `expr`; the `legendFormat` is already
`{{array}}` and stays:

```json
"expr": "sum by (array) (powerstore_drive_state{state!=\"Healthy\",array=~\"$array\"}) or sum by (array) (powerstore_up{array=~\"$array\"} * 0)"
```

`overview/00-fleet-health.json` panel `Ports Down` likewise:

```json
"expr": "sum by (array) (powerstore_port_link_up{array=~\"$array\"} == bool 0)"
```

Leave `overview/00-fleet-health.json` panel `Active Critical Alerts` **exactly as it is**.
`derive_alerts.go` already emits a stable zero-valued series for every known severity, so it
never vanishes. Adding a filler there would be cargo-culting.

- [ ] **Step 8: Verify every dashboard is still valid JSON and the linter passes**

```bash
rtk go test ./internal/dashboards/ -run TestNoBareAggregation -v
```

Expected: `PASS`. A decode failure here means a trailing comma was introduced — `loadPanels`
calls `t.Fatalf` on invalid JSON, naming the file.

- [ ] **Step 9: Commit**

```bash
rtk git add internal/dashboards/ grafana/dashboards/ && \
rtk git commit -m "fix(grafana): render every aggregated panel per array

Fourteen panels across six dashboards aggregated with no by (...) clause,
collapsing all selected arrays into one unattributable value. Two more
(Unhealthy Drives, Ports Down on Fleet Health) counted info series that are
always 1, so a healthy array rendered no tile at all -- indistinguishable
from a dead exporter.

Adds internal/dashboards, a stdlib-only test that walks the bundled JSON and
fails CI on bare aggregations. ADR-0014 deferred this linter until drift
became a problem; it has.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01K4gasjsHDhM17ZxfurigPS"
```

---

## Task 2: Colour by entity, direction by line style

**Files:**
- Modify: `internal/dashboards/lint_test.go` (append R2 + R3)
- Modify: `grafana/dashboards/block/02-appliances.json`
- Modify: `grafana/dashboards/block/04-volume-groups.json`
- Modify: `grafana/dashboards/file/01-file-systems.json`
- Modify: `grafana/dashboards/overview/01-cluster-overview.json`
- Modify: `grafana/dashboards/overview/00-fleet-health.json`

**Interfaces:**
- Consumes: `loadPanels(t) []panelRef`, `override`, `property`, `matcher` from Task 1.
- Produces: nothing consumed downstream.

- [ ] **Step 1: Append the failing colour rules**

Append to `internal/dashboards/lint_test.go`:

```go
// directionMatcher matches the byRegexp options used for the read/write series
// convention, e.g. "/ read$/" and "/ write$/".
var directionMatcher = regexp.MustCompile(`(read|write)\$/$`)

// isDirectionOverride reports whether o selects series by read/write direction.
func isDirectionOverride(o override) bool {
	if o.Matcher.ID != "byRegexp" {
		return false
	}
	opts, ok := o.Matcher.Options.(string)
	return ok && directionMatcher.MatchString(opts)
}

// TestNoFixedDirectionColour enforces R2. Colouring by direction with a fixed
// colour erases the entity dimension: on a single-direction panel every appliance
// matches `/ read$/` and every appliance renders blue.
func TestNoFixedDirectionColour(t *testing.T) {
	for _, p := range loadPanels(t) {
		for _, o := range p.FieldConfig.Overrides {
			if !isDirectionOverride(o) {
				continue
			}
			for _, prop := range o.Properties {
				if prop.ID != "color" {
					continue
				}
				var c colorSpec
				if err := json.Unmarshal(prop.Value, &c); err != nil {
					t.Fatalf("%s: panel %q: bad color value: %v", p.File, p.Title, err)
				}
				if c.Mode == "fixed" {
					t.Errorf("%s: panel %q pins a fixed colour on a read/write matcher, "+
						"collapsing every entity into one colour", p.File, p.Title)
				}
			}
		}
	}
}

// TestDirectionPanelsColourByName enforces R3. A timeseries panel that still
// distinguishes direction must derive hue from the series name, so an array or
// appliance keeps one colour across every dashboard.
func TestDirectionPanelsColourByName(t *testing.T) {
	for _, p := range loadPanels(t) {
		if p.Type != "timeseries" {
			continue
		}
		var hasDirection bool
		for _, o := range p.FieldConfig.Overrides {
			if isDirectionOverride(o) {
				hasDirection = true
				break
			}
		}
		if !hasDirection {
			continue
		}
		got := ""
		if c := p.FieldConfig.Defaults.Color; c != nil {
			got = c.Mode
		}
		if got != "palette-classic-by-name" {
			t.Errorf("%s: panel %q distinguishes direction but has color.mode=%q, "+
				"want %q", p.File, p.Title, got, "palette-classic-by-name")
		}
	}
}
```

- [ ] **Step 2: Run and confirm both fail**

```bash
rtk go test ./internal/dashboards/ -run 'TestNoFixedDirectionColour|TestDirectionPanelsColourByName' -v
```

Expected: `FAIL`. `TestNoFixedDirectionColour` names **12** panels; `TestDirectionPanelsColourByName`
names **10** (the same 12 minus Fleet Health's two `stat` panels).

- [ ] **Step 3: Convert the ten timeseries panels**

For each panel listed below: in `fieldConfig.defaults`, add the `color` key; in
`fieldConfig.overrides`, **delete** the `/ read$/` override entirely (solid is the default) and
**replace** the `/ write$/` override's `color` property with a `custom.lineStyle` property.

The `defaults` addition, identical in all ten:

```json
"color": { "mode": "palette-classic-by-name" }
```

The replacement override, identical in all ten:

```json
{
  "matcher": { "id": "byRegexp", "options": "/ write$/" },
  "properties": [
    {
      "id": "custom.lineStyle",
      "value": { "fill": "dash", "dash": [10, 10] }
    }
  ]
}
```

Panels to convert:

| File | Panel |
| --- | --- |
| `block/02-appliances.json` | `Read Bandwidth per Appliance` |
| `block/02-appliances.json` | `Write Bandwidth per Appliance` |
| `block/02-appliances.json` | `Read Latency per Appliance` |
| `block/02-appliances.json` | `Write Latency per Appliance` |
| `block/04-volume-groups.json` | `Bandwidth (read / write) per Volume Group` |
| `block/04-volume-groups.json` | `Latency (read / write) per Volume Group` |
| `file/01-file-systems.json` | `Read / Write Latency` |
| `file/01-file-systems.json` | `Read / Write Bandwidth` |
| `overview/01-cluster-overview.json` | `Bandwidth (read / write)` |
| `overview/01-cluster-overview.json` | `Avg Latency (read / write)` |

On the four `02-appliances` panels every series is one direction, so `Write Bandwidth per
Appliance` ends up entirely dashed. That is intentional: dashed means write on every panel of
every dashboard, and the reader learns one rule.

- [ ] **Step 4: Add colour-by-name to `IOPS per Appliance`**

`block/02-appliances.json` panel `IOPS per Appliance` has no direction override, so R3 ignores
it — but it must share the palette or an appliance changes hue between the IOPS panel and the
bandwidth panels directly beneath it. Add to its `fieldConfig.defaults`:

```json
"color": { "mode": "palette-classic-by-name" }
```

Leave `IO Workload CPU Utilization per Appliance` on `"color": {"mode": "thresholds"}`. Its hue
encodes utilisation against the 0.70/0.90 threshold palette from ADR-0014, not identity.

- [ ] **Step 5: Strip the fixed colours from the two stat panels**

A `stat` panel has no line to style, so `custom.lineStyle` is meaningless there. Delete both the
`/ read$/` and `/ write$/` overrides outright, leaving `"overrides": []`. Each tile is already
labelled `{{array}} read` / `{{array}} write`, and the `"color": {"mode": "thresholds"}` already
in `defaults` stands.

- `overview/00-fleet-health.json` panel `Total Bandwidth (read / write)`
- `overview/00-fleet-health.json` panel `Avg Latency (read / write)`

- [ ] **Step 6: Confirm the untouched overrides really are untouched**

The capacity panels carry `/ used$/` and `/ total$/` overrides that look similar and mean
something different. Prove none changed:

```bash
rtk git diff -- grafana/dashboards | rtk grep -c 'used\$\|total\$'
```

Expected: `0`.

- [ ] **Step 7: Run the full linter**

```bash
rtk go test ./internal/dashboards/ -v
```

Expected: `PASS` — all three tests.

- [ ] **Step 8: Commit**

```bash
rtk git add internal/dashboards/ grafana/dashboards/ && \
rtk git commit -m "fix(grafana): colour series by entity, mark writes with a dashed line

ADR-0014 pinned read=blue/write=orange as fixed colours. On single-direction
panels every appliance matched the same regexp and rendered the same colour,
so 'IOPS per Appliance' was the only Appliances panel where two appliances
were distinguishable.

Hue now derives from the series name (palette-classic-by-name), so an array or
appliance keeps one colour across every dashboard; direction moves to line
style. Stat panels, which have no line, drop the overrides entirely.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01K4gasjsHDhM17ZxfurigPS"
```

---

## Task 3: ADR-0016 and dashboard documentation

**Files:**
- Create: `docs/adr/0016-per-array-rendering-and-series-colour.md`
- Modify: `docs/dashboards.md`
- Modify: `docs/adr/README.md` (index)

**Interfaces:**
- Consumes: nothing.
- Produces: nothing.

- [ ] **Step 1: Read the two files you are about to extend**

```bash
rtk read docs/adr/0014-grafana-dashboard-taxonomy.md && rtk read docs/adr/README.md
```

ADR-0014 is **not edited**. ADR-0009 superseded ADR-0003 by adding a new record rather than
rewriting history; follow that precedent.

- [ ] **Step 2: Write ADR-0016**

`docs/adr/0016-per-array-rendering-and-series-colour.md`:

```markdown
# 16. Per-Array Rendering and Series Colour by Entity

## Status

Accepted. Supersedes the fixed read/write series-colour clause of
[ADR-0014](0014-grafana-dashboard-taxonomy.md); the rest of ADR-0014 stands.

## Context

Every bundled dashboard declares `array` as `multi: true, includeAll: true`, so multi-array
selection is offered on every screen. Two defect classes then discarded it, both reported by
an operator running two arrays against one exporter:

- Fourteen panels across six dashboards aggregated with no `by (...)` clause, collapsing all
  selected arrays into a single unattributable value. `avg()` of a data reduction ratio across
  arrays is also arithmetically meaningless — a ratio of ratios is not a ratio.
- ADR-0014's `read = blue, write = orange`, implemented as `byRegexp` overrides pinning
  `color.mode: fixed`, erased the entity dimension. On `Read Bandwidth per Appliance` the query
  returns only read series, so every appliance matched `/ read$/` and rendered blue.

A third defect had not yet been reported. `powerstore_drive_state` and
`powerstore_replication_session_state` are info series that are always `1`, with the state in a
label. A healthy array matches no series, so `count by (array)` returned nothing and Grafana drew
no tile — indistinguishable from an array that is not being scraped.

## Decision

- **Every aggregating panel groups by `array`** and legends as `{{array}}`. `stat` and `gauge`
  panels already render one tile per series, so no panel is repeated and no `gridPos` changes.
- **Info-series counts are zero-filled** with `or sum by (array) (powerstore_up{...} * 0)`.
  `powerstore_up` carries exactly the `array` label; the `sum by (array)` on the filler pins its
  label set so the `or` union is label-compatible. Boolean comparisons use the `bool` modifier
  (`== bool 0`) instead, which yields `1`/`0` per series rather than filtering series away.
- **Hue encodes identity, line style encodes direction.** Timeseries panels set
  `color.mode: palette-classic-by-name`, so an array or appliance keeps one colour across every
  dashboard; `/ write$/` sets `custom.lineStyle` to a dash. `stat` panels have no line and simply
  drop the overrides. Threshold-coloured panels (CPU, drive wear, capacity gauges) are unaffected:
  their hue encodes a value against a threshold, not an identity.
- **A CI guard.** ADR-0014 deferred a consistency linter until "drift becomes a problem".
  Fourteen panels found by a customer is that problem. `internal/dashboards` walks the bundled
  JSON and fails `make ci` on bare aggregations and on fixed direction colours. It is stdlib-only
  and adds no toolchain, so the ADR-0014 rejection of a generator still holds.

## Consequences

- No collector change: every metric and label required already existed.
- Per-array tiles stay legible to roughly six arrays. Past that, Fleet Health's stat rows crowd,
  and the follow-up is a weighted rollup (`sum(logical_used) / sum(physical_used)` for DRR) shown
  beside the per-array breakdown — not a return to `avg()`.
- The linter is deliberately syntactic; it greps expression strings rather than parsing PromQL.
  It permits `sum by (appliance_name) (...)`, which omits `array` while grouping by something
  else. Two arrays sharing an appliance, volume or volume-group name therefore still merge on
  `block/02-appliances`, `block/03-volumes`, `block/04-volume-groups` and `file/01-file-systems`.
  PowerStore derives default appliance names from the serial number, so collision is unlikely;
  widening those groupings would change legends on every single-array install. Tracked, not fixed.
- Existing users see colour changes on ten panels. Dashboards are provisioned read-only from this
  repo, so there is nothing to migrate. Screenshots under `docs/` showing the old colours are stale.
```

- [ ] **Step 3: Add ADR-0015 *and* ADR-0016 to the index**

The table in `docs/adr/README.md` currently stops at `0014`. `docs/adr/0015-metro-witness-via-generic-query.md`
exists on disk but was never indexed — fix that drive-by while you are here. Append both rows
after the `0014` row:

```markdown
| [0015](0015-metro-witness-via-generic-query.md) | Metro Witness via Generic Query | Accepted |
| [0016](0016-per-array-rendering-and-series-colour.md) | Per-Array Rendering and Series Colour by Entity | Accepted |
```

Open `0015-metro-witness-via-generic-query.md` and copy its `# 15. ...` heading text verbatim
into the title cell if it differs from "Metro Witness via Generic Query".

- [ ] **Step 4: Document the conventions in `docs/dashboards.md`**

The file already has a `## PromQL conventions` section (around line 79) covering how to *query*
the metrics. Add a sibling `## Panel conventions` section covering how to *render* them, placed
immediately after `## PromQL conventions` ends and before `## Building more`:

```markdown
## Panel conventions

These rules are enforced by `go test ./internal/dashboards/` and therefore by `make ci`.
See [ADR-0016](adr/0016-per-array-rendering-and-series-colour.md).

**Group by `array`.** Every aggregating expression carries a `by (...)` clause including
`array`, and legends as `{{array}}`. A bare `avg(...)` or `sum(...)` blends every selected
array into one unattributable number.

**Never let a healthy array vanish.** `powerstore_drive_state` and
`powerstore_replication_session_state` are info series whose value is always `1`; a healthy
array matches no series and its tile disappears. Zero-fill with `powerstore_up`, whose only
label is `array`:

    sum by (array) (powerstore_drive_state{array=~"$array",state!="Healthy"})
      or sum by (array) (powerstore_up{array=~"$array"} * 0)

For numeric comparisons use the `bool` modifier instead, which keeps the series:

    sum by (array) (powerstore_port_link_up{array=~"$array"} == bool 0)

`powerstore_alert_active` already emits a stable zero series per known severity and needs
neither treatment.

**Hue is identity, line style is direction.** Timeseries panels set
`"color": {"mode": "palette-classic-by-name"}`, so an array or appliance keeps the same colour
on every dashboard. Write series are dashed via a `/ write$/` override setting
`custom.lineStyle`. Panels whose colour encodes a threshold rather than an identity — CPU
utilisation, drive wear, the capacity gauges — keep `"color": {"mode": "thresholds"}`.
```

- [ ] **Step 5: Verify the docs build and links resolve**

```bash
rtk grep -n "0016" docs/adr/README.md docs/dashboards.md
```

Expected: at least one hit in each file, and the ADR filename spelled identically in both.

- [ ] **Step 6: Commit**

```bash
rtk git add docs/ && \
rtk git commit -m "docs: ADR-0016 per-array rendering and series colour

Supersedes ADR-0014's fixed read/write colour clause; the rest of 0014 stands.
Records the zero-fill idiom and the linter that ADR-0014 deferred.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01K4gasjsHDhM17ZxfurigPS"
```

---

## Task 4: End-to-end verification against a live Grafana

The linter proves the JSON says what we intend. It cannot prove Grafana renders it, that the
PromQL parses, or that `or sum by (array) (... * 0)` actually produces a zero tile. Only a real
stack can. **Do not skip this task**: `== bool 0` and the `or` label-matching are the two places
this change is most likely to be subtly wrong.

**Files:**
- Modify: none (verification only). Any defect found sends you back to Task 1 or 2.

**Interfaces:**
- Consumes: the committed dashboards.
- Produces: nothing.

- [ ] **Step 1: Run the full CI gate**

```bash
rtk make ci
```

Expected: `PASS` throughout — `fmt-check`, `vet`, `lint`, `test -race`, `govulncheck`. The new
`internal/dashboards` package must appear in the test output.

- [ ] **Step 2: Bring up the stack**

```bash
rtk docker compose up -d && rtk docker compose ps
```

Expected: `exporter` (9446), `prometheus` (9090), `grafana` (3000), `otel-collector` all `Up`.

- [ ] **Step 3: Confirm the zero-fill genuinely yields a zero, not an empty result**

This is the assertion that matters. A healthy array must return exactly one sample valued `0`.

```bash
rtk curl -sG 'http://localhost:9090/api/v1/query' \
  --data-urlencode 'query=sum by (array) (powerstore_drive_state{state!="Healthy"}) or sum by (array) (powerstore_up * 0)'
```

Expected: `"status":"success"` and a `result` array with **one entry per configured array**, each
`"value"` ending in `"0"`, each `"metric"` containing exactly `{"array":"..."}` and no other
label. An empty `result` array means the `or` label sets did not match — the filler's
`sum by (array)` was probably dropped.

- [ ] **Step 4: Confirm the `bool` modifier reports zero for healthy ports**

```bash
rtk curl -sG 'http://localhost:9090/api/v1/query' \
  --data-urlencode 'query=sum by (array) (powerstore_port_link_up == bool 0)'
```

Expected: one entry per array, valued `"0"`. An empty result means `bool` was dropped and the
comparison filtered rather than scored.

- [ ] **Step 5: Confirm no blank-titled tile survives**

```bash
rtk grep -rn 'or vector(0)' grafana/dashboards/
```

Expected: no output. `vector(0)` yields a label-less series that renders as an unnamed tile.

- [ ] **Step 6: Look at the dashboards**

Open `http://localhost:3000`, set the `array` variable to **All**, and check each:

- **Fleet Health** — `Cluster Capacity Used %` and `Data Reduction Ratio` show one tile per
  array, each labelled with the array name. `Unhealthy Drives` and `Ports Down` show a `0` tile
  per array rather than nothing.
- **Cluster Overview** — `Capacity Used %`, `Data Reduction Ratio`, `Efficiency Ratio` each show
  one tile per array. `Bandwidth (read / write)` draws each array in its own hue, writes dashed.
- **Appliances** — every panel gives each appliance its own colour. An appliance has the *same*
  colour on `IOPS`, `Read Bandwidth` and `Read Latency`.

If only one array is configured locally, add a second target to `config.yaml` pointing at the
same array under a different `name:` — the `array` label is the config name, so this exercises
the multi-array path without a second physical array.

- [ ] **Step 7: Refresh the stale screenshots**

`docs/` carries a dashboard screenshot gallery (commit `3da0abc`) showing the old colours.
Recapture any image whose panels changed hue, and commit alongside.

- [ ] **Step 8: Tear down and commit any screenshot updates**

```bash
rtk docker compose down && rtk git status
```

If screenshots changed:

```bash
rtk git add docs/ && \
rtk git commit -m "docs: refresh dashboard screenshots for per-array colours

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01K4gasjsHDhM17ZxfurigPS"
```

- [ ] **Step 9: Push and open the PR**

Squash merges are disabled on this repo — the branch history is what lands, so the four commits
above are the merge commit's contents.

```bash
rtk git push -u origin fix/per-array-dashboard-rendering && \
rtk gh pr create --title "fix(grafana): render every panel per array, colour by entity" --body "$(cat <<'BODY'
Reported by a customer running two PowerStore arrays against one exporter.

- 14 panels across 6 dashboards aggregated with no `by (...)` clause, blending
  every selected array into one unattributable value.
- ADR-0014's fixed read/write colours erased the entity dimension: on
  single-direction panels every appliance matched `/ read$/` and rendered blue.
- Unreported: `Unhealthy Drives` / `Ports Down` counted always-`1` info series,
  so a healthy array rendered no tile at all — indistinguishable from a dead
  exporter.

Adds `internal/dashboards`, the stdlib-only CI guard ADR-0014 deferred.
ADR-0016 supersedes ADR-0014's colour clause; the rest of 0014 stands.

Design: `docs/superpowers/specs/2026-07-10-per-array-dashboard-rendering-design.md`

🤖 Generated with [Claude Code](https://claude.com/claude-code)

https://claude.ai/code/session_01K4gasjsHDhM17ZxfurigPS
BODY
)"
```

---

## Follow-ups (out of scope)

- `sum by (appliance_name)`, `by (volume_name)`, `by (volume_group_name)` and
  `by (file_system_name)` omit `array`. Two arrays sharing an entity name silently merge on
  `block/02-appliances`, `block/03-volumes`, `block/04-volume-groups`, `file/01-file-systems`.
- A weighted fleet rollup (`sum(logical_used) / sum(physical_used)`) beside the per-array
  breakdown, if a fleet larger than ~6 arrays appears.
- `snapshot_savings_ratio` / `thin_savings_ratio` use `avg by (array)`, an unweighted mean across
  appliances. Correct weighting needs a per-appliance capacity metric the exporter does not emit.
