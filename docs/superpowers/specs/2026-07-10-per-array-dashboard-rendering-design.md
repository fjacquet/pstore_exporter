# Per-Array Dashboard Rendering and Series Colour

Date: 2026-07-10
Status: Approved

## Context

A customer running two PowerStore arrays against one exporter reported three problems:

1. On **PowerStore — Appliances**, only `IOPS per Appliance` gives each appliance its own
   colour; the bandwidth and latency panels draw both appliances in the same colour.
2. On **PowerStore — Cluster Overview**, `Capacity Used %`, `Data Reduction Ratio` and
   `Efficiency Ratio` show a single number with two arrays selected — you cannot tell which
   array it describes, or whether it blends both.
3. On **PowerStore — Fleet Health**, the two Capacity-glance panels have the same problem.

An audit of `grafana/dashboards/**/*.json` shows the reported panels are instances of two
bug classes that are wider than what the customer saw, plus a third bug he has not yet hit.

Every dashboard declares `array` as `multi: true, includeAll: true`. Multi-array selection
is therefore offered on every screen and then silently discarded by the affected panels.

## Problem 1 — aggregation drops the `array` dimension

Fourteen panels across six dashboards aggregate without `by (array)`:

| Dashboard | Panel | Current expression |
| --- | --- | --- |
| `overview/00-fleet-health` | Cluster Capacity Used % | `100 * sum(used) / sum(total)` |
| `overview/00-fleet-health` | Data Reduction Ratio | `avg(cluster_data_reduction_ratio)` |
| `overview/01-cluster-overview` | Capacity Used % | `100 * sum(used) / sum(total)` |
| `overview/01-cluster-overview` | Data Reduction Ratio | `avg(cluster_data_reduction_ratio)` |
| `overview/01-cluster-overview` | Efficiency Ratio | `avg(cluster_efficiency_ratio)` |
| `block/05-capacity` | Data Reduction Ratio | `avg(cluster_data_reduction_ratio)` |
| `block/05-capacity` | Efficiency Ratio | `avg(cluster_efficiency_ratio)` |
| `block/05-capacity` | Snapshot Savings Ratio | `avg(appliance_snapshot_savings_ratio)` |
| `block/05-capacity` | Thin Savings Ratio | `avg(appliance_thin_savings_ratio)` |
| `block/06-ports` | Ports Up | `count(port_link_up == 1) or vector(0)` |
| `block/06-ports` | Ports Down | `count(port_link_up == 0) or vector(0)` |
| `hardware/01-drives` | Unhealthy Drives | `count(drive_state{state!="Healthy"})` |
| `hardware/01-drives` | Worn Drives > 80% | `count(drive_wear_level_ratio > 0.8)` |
| `protection/01-replication` | Sessions in Bad State | `count(replication_session_state{state=~"..."})` |

Beyond the ambiguity, `avg()` of a data reduction ratio across arrays is arithmetically
meaningless — a ratio of ratios is not a ratio. The panel is wrong even when read as a
deliberate fleet rollup.

## Problem 2 — fixed direction colours erase the entity dimension

ADR-0014 mandates `read = blue, write = orange`, implemented as `byRegexp` overrides
setting `color.mode: fixed`. Twelve panels carry these overrides.

On the four `02-appliances` panels the query returns a **single direction only**
(`Read Bandwidth per Appliance` queries only read bandwidth). Every appliance's series
therefore matches `/ read$/` and every appliance renders blue. The override conveys no
information on these panels and destroys the appliance dimension. `IOPS per Appliance` has
no override, falls through to the default palette, and is the sole panel where the two
appliances are distinguishable — exactly as reported.

On genuinely mixed panels (`Bandwidth (read / write)`, `Latency (read / write)`,
`Read / Write Bandwidth`) the same overrides collapse *N* entities into two colours.

## Problem 3 — zero-valued tiles disappear (not reported; pre-existing)

`powerstore_drive_state` and `powerstore_replication_session_state` are info series that are
**always `1`**; the state lives in a `state` label. A healthy array emits no series matching
`{state!="Healthy"}`. `powerstore_port_link_up == 0` likewise matches nothing when all ports
are up.

Consequently `count by (array)(…)` returns no series for a healthy array and Grafana renders
**no tile at all** — visually identical to "this array is not being scraped". Fleet Health's
`Unhealthy Drives`, `Ports Down` and `Active Critical Alerts` already carry `by (array)` and
already exhibit this today. A tile that vanishes when everything is fine is a worse failure
than the one that prompted this work: it is indistinguishable from a dead exporter.

## Decision

### D1 — Per-array rendering

Every aggregating panel gains `by (array)` and a `{{array}}` legend. Grafana's `stat` and
`gauge` panels already render one tile per returned series, so no panel is repeated and no
grid geometry changes. This is the idiom Fleet Health's `Arrays Up` already uses.

Three expression sub-cases:

**Plain ratios and rollups.** One series per array already exists, so `by (array)` both
disambiguates and repairs the arithmetic:

```promql
avg by (array) (powerstore_cluster_data_reduction_ratio{array=~"$array"})
100 * sum by (array) (powerstore_cluster_physical_used_bytes{array=~"$array"})
    / sum by (array) (powerstore_cluster_physical_total_bytes{array=~"$array"})
```

`snapshot_savings_ratio` and `thin_savings_ratio` are appliance-scoped; `avg by (array)` is
an unweighted mean of appliances within one array. Accepted — appliances within an array are
of comparable capacity, and the alternative needs a weighting metric the exporter does not
emit.

**Boolean counts.** The `bool` modifier yields `1`/`0` per port rather than filtering series
away, so a fully healthy array reports `0` instead of vanishing:

```promql
sum by (array) (powerstore_port_link_up{array=~"$array"} == bool 0)
```

The existing `or vector(0)` is **removed**, not adapted: it produces a series carrying no
`array` label, which under `by (array)` would render as a blank-titled tile beside the real
ones.

**Info-series counts.** These have no zero-valued form and need an explicit zero-fill:

```promql
sum by (array) (powerstore_drive_state{array=~"$array",state!="Healthy"})
  or sum by (array) (powerstore_up{array=~"$array"} * 0)
```

`powerstore_up` carries exactly one label, `array`. The right-hand `sum by (array)` is not
redundant: it pins the filler's label set to `{array}` so the `or` union is label-compatible
with the left side. `Worn Drives > 80%` uses the same shape with `> bool 0.8`.

This applies to `Unhealthy Drives`, `Worn Drives > 80%`, `Active Critical Alerts` and
`Sessions in Bad State`, closing Problem 3 in the same pass.

### D2 — Colour by entity, direction by line style

Hue derives from the series name so an array or appliance keeps one colour across every
panel and every dashboard:

```json
"defaults": { "color": { "mode": "palette-classic-by-name" } }
```

Direction moves from hue to line style:

```json
{ "matcher":    { "id": "byRegexp", "options": "/ write$/" },
  "properties": [{ "id": "custom.lineStyle",
                   "value": { "fill": "dash", "dash": [10, 10] } }] }
```

The rule is **conditional on panel type**:

- **`timeseries` panels** (10 of the 12) take `palette-classic-by-name` plus the dashed
  `/ write$/` override. The `/ read$/` override is deleted; solid is the default.
- **`stat` panels** (Fleet Health's `Total Bandwidth` and `Avg Latency`) have no line to
  style. Their fixed-colour overrides are deleted outright and the existing
  `color.mode: thresholds` in `defaults` stands. Direction is already unambiguous from each
  tile's `{{array}} read` / `{{array}} write` label.

Only overrides whose matcher is `/ read$/` or `/ write$/` are touched. The visually similar
`/ used$/` and `/ total$/` overrides on the capacity panels express a different distinction
and are left exactly as they are.

### D3 — CI guard

ADR-0014 deferred a consistency linter: *"remains a possible later follow-up if drift becomes
a problem."* Drift is now a problem — fourteen panels across six dashboards, surfaced by a
customer rather than by CI.

Add `grafana/dashboards_lint_test.go` (package `grafana_test`, test files only). It walks
`dashboards/**/*.json`, decodes each into `map[string]any`, and asserts per panel target:

1. Any target whose `expr` contains an aggregation operator (`sum`, `avg`, `count`, `min`,
   `max`) applied across a metric carrying the `array` label must include `by (array)`.
2. No override whose matcher is `/ read$/` or `/ write$/` may set `color.mode: fixed`.
3. Every `timeseries` panel with more than one series-producing target, or a `by (…)`
   grouping, declares `color.mode: palette-classic-by-name`.

Table-driven over the parsed JSON, standard library only, no new dependency, runs inside the
existing `make ci`. Rule 1 is deliberately syntactic — it greps the expression string rather
than parsing PromQL. It will not catch every case and is not meant to; it catches the class
that just shipped.

### D4 — Documentation

- **ADR-0016** records the per-array rendering rule and supersedes ADR-0014's fixed
  read/write colour clause. ADR-0014 is not edited, following the precedent of ADR-0009
  superseding ADR-0003.
- `docs/dashboards.md` restates the colour convention and the zero-fill idiom.

## Consequences

- No Go collector change. Every metric and label required already exists.
- Per-array tiles are legible up to roughly six arrays; past that, Fleet Health's stat rows
  become cramped. If a larger fleet appears, the follow-up is a weighted rollup
  (`sum(logical_used) / sum(physical_used)` for DRR) beside the per-array breakdown, not a
  return to `avg()`.
- Existing Grafana users see colour changes on ten panels. Dashboards are provisioned
  read-only from this repo, so there is nothing to migrate.
- The screenshots in `docs/` showing the old colours become stale and are regenerated.

## Rejected alternatives

**Panel `repeat: array`.** Clones each panel per array, titled with the array name. Maximally
explicit, but every capacity row doubles in size with two arrays, each dashboard's grid must
be re-laid-out by hand, and four arrays make the page unusable.

**Keep fixed colours; split every mixed panel by direction.** Removes the conflict by never
drawing read and write together. Costs the side-by-side read/write comparison that makes the
latency panels useful, and grows the panel count on four dashboards.

**Fix only the five panels the customer named.** Leaves the identical defect in `05-capacity`,
`06-ports`, `01-drives` and `01-replication`, and leaves Problem 3 entirely unaddressed.
