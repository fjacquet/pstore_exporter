# Design: Reconcile `pstore_exporter` against PowerStore REST API 4.4.0

**Date:** 2026-06-13
**Status:** Approved (brainstorming) — next step: `writing-plans`
**Source of truth:** `docs/swagger/5504-4.4.0.json` (OpenAPI 3.1.0, PowerStore REST API
`4.4.0.0`, captured from a model 5504 array)

## Goal

Full reconciliation of the exporter's metric collection against the canonical 4.4.0 API
definition. Two outputs:

1. A **correctness audit** of every endpoint, response field/column, entity type, and
   capability gate the exporter currently *uses* — catching stale assumptions and
   missing-fallback risks.
2. A **coverage gap map** of everything 4.4.0 exposes that the exporter does *not* yet
   collect, prioritized.

The session produces an analysis report + a prioritized fix list. **No code changes during
this work** — the fix list feeds a separate implementation plan via `writing-plans`.

## Scope

- **Cover all 55 `/metrics/generate` entity types** (the full `MetricsEntityEnum`), not just
  the block/file/space families. Entities outside the exporter's purpose (import/migration,
  vSphere, SMB/NFS protocol-detail) are still catalogued in the coverage map, each with an
  explicit priority/skip tag and rationale — so the decision to skip is recorded, not implied.
- Inventory endpoints the exporter reads for topology/label resolution (volume, appliance,
  cluster, fc_port, eth_port, sas_port, file_system, volume_group, replication_session,
  drive via generic Query, alert).
- The two metric paths: per-entity SDK path (`perentity.go` / `derive_perentity.go` +
  `derive_*_perf.go`) and the raw bulk path (`bulk.go` / `derive_bulk.go`).

Out of scope: changing collection behavior, adding metrics, editing ADRs — those are
downstream of the fix plan.

## Key methodological subtlety

The OpenAPI spec describes the **REST `/metrics/generate` JSON response**, *not* the bulk CSV
tar archive returned by `/latest_five_min_metrics/download`. Consequences:

- **SDK / per-entity path → spec is authoritative.** Field-level checks are exact:
  gopowerstore struct JSON tags map 1:1 to `base_*_metrics_by_*` schema properties.
- **Bulk CSV path → spec is semantically indicative, not authoritative on exact column
  spelling.** Bulk "latest five minute" tables historically use `avg_`- and `last_`-prefixed
  column names that the REST JSON does not carry. We validate *entity + metric coverage*
  against the spec, and flag any primary CSV key with **no spec sibling and no fallback** as
  "verify against a live `--trace` capture" rather than asserting it is wrong.

## Validation passes

1. **Endpoint audit** — every path the exporter calls exists in 4.4.0 `paths` (452 total).
   Mostly pre-confirmed: `/latest_five_min_metrics/{enable,download}` and `/metrics/generate`
   both present.
2. **Field audit** — for each emitted metric, map its source field/column to a canonical
   4.4.0 property in the matching `base_*` schema. Flag:
   - primary key with no spec counterpart and no fallback (silent-zero risk), and
   - fields the spec defines that we read under a different name.
3. **Capability-gate audit** — the `≥4.1` bulk gate (`capability.go`) vs what 4.4.0
   advertises; confirm SDK methods assumed present/absent actually match (e.g.
   `PerformanceMetricsByFileSystem` exists — ADR-0003's "no FS perf" note is stale, already
   noted in CLAUDE.md/ADR-0009; record formally).
4. **Coverage map** — all 55 entity types × {emitted | not} with priority tag
   (high / medium / skip) and one-line rationale each. Same treatment for notable per-entity
   fields in the families we already cover.

## Seed findings (from the brainstorming spot-check)

Recorded so they survive into execution:

- **No `avg_`-prefixed IOPS/bandwidth in the REST schema.** Canonical volume/appliance fields
  are `read_iops`, `write_iops`, `total_iops`, `read_bandwidth`, … . `derive_bulk.go` reads
  `avg_read_iops` *first* with `read_iops` as fallback — fallback covers it; confirm the bulk
  CSV spelling.
- **`avg_io_workload_cpu_utilization` (appliance perf) has no fallback;** canonical field is
  `io_workload_cpu_utilization`. Silent-zero risk if the CSV matches the REST spelling.
- **Coverage gaps inside covered entities:** `total_bandwidth`, mirror/copy/unmap/zero
  IOPS+bandwidth, `system_free_space`, `data_physical_used`,
  `logical_used_{file_system,volume,vvol}` — present in 4.4.0, not emitted.
- **Unused entity types (sample):** `performance_headroom_by_appliance`,
  `space_metrics_by_storage_container`, `performance_metrics_by_{host,hg,initiator}`,
  `performance_metrics_by_nas_server`, `space_metrics_by_volume_family`, copy/replication
  variants, vSphere, SMB/NFS protocol detail.

## Deliverables

1. `docs/reconciliation-2026-06-13.md` — findings per pass, mirroring the style of
   `docs/reconciliation-2026-06-05.md`. Includes the full 55-entity coverage table.
2. ADR touch-ups where assumptions are provably stale (e.g. formalize the ADR-0003
   supersession; new ADR only if the coverage strategy itself changes).
3. A prioritized **fix list** feeding `writing-plans`, split into:
   - *Correctness fixes* — missing fallbacks, wrong/renamed keys, gate corrections.
   - *Coverage additions* — new entities/fields, each sized and parity-aware (bulk +
     per-entity paths must stay in lockstep per the metric-parity invariant).

## Method notes / how the audit is executed

- Parse `docs/swagger/5504-4.4.0.json` with a small throwaway Python script to extract:
  `MetricsEntityEnum`, each `base_*_metrics_by_*` property set (resolving `allOf`/`$ref`),
  and inventory schema property sets.
- Cross-reference against emitted metric names (`grep powerstore_*`) and the source
  fields/columns the derive functions read.
- Respect the **metric-parity invariant**: any coverage fix must land in both `derive_bulk.go`
  and the per-entity derive together, using shared label builders in `metrics.go`.
- Per project policy: no `//nolint`/semgrep suppressions; the throwaway parse script is not
  committed (analysis tooling, not product code).
