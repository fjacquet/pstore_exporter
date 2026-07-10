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
  `color.mode: palette-classic-by-name`, which derives hue from each series' full display name.
  Within any one panel every array or appliance is therefore distinguishable, and its colour is
  stable across refreshes; `/ write$/` sets `custom.lineStyle` to a dash. Because the hash includes
  the ` read`/` write` suffix and legends differ between panels, the same entity is not guaranteed
  the same hue on a different panel or dashboard — only line style (solid vs. dashed) carries
  direction consistently. `stat` panels have no line and simply drop the overrides.
  Threshold-coloured panels (CPU, drive wear, capacity gauges) are unaffected: their hue encodes a
  value against a threshold, not an identity.
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
