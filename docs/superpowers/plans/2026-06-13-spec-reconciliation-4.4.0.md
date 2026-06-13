# 4.4.0 Spec Reconciliation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reconcile the exporter's metric collection against PowerStore REST API 4.4.0 and produce a reconciliation report + a prioritized fix list (no product code changes).

**Architecture:** Read the canonical spec at `docs/swagger/5504-4.4.0.json`, extract its metric entity types / per-entity field schemas / inventory schemas with a throwaway Python script, cross-reference against what the exporter emits and the fields its derive functions read, then write findings into `docs/reconciliation-2026-06-13.md` across four audit passes. ADR touch-ups and a prioritized fix list close it out.

**Tech Stack:** Go (read-only inspection), Python 3 (throwaway spec parsing — not committed), Markdown (deliverables). Spec: OpenAPI 3.1.0.

**Source spec:** `docs/swagger/5504-4.4.0.json` (PowerStore REST API `4.4.0.0`, model 5504).
**Design:** `docs/superpowers/specs/2026-06-13-spec-reconciliation-4.4.0-design.md`.

**Cross-cutting rules:**
- No product code changes in this plan. Deliverables are Markdown only.
- The throwaway Python parser is NOT committed (analysis tooling). Run it from `/tmp`.
- Spec describes the REST `/metrics/generate` JSON — authoritative for the SDK path; only
  semantically indicative (not column-spelling-authoritative) for the bulk CSV path.
- Respect the metric-parity invariant when writing the fix list: every coverage addition
  must be slated for BOTH `derive_bulk.go` and the per-entity derive together.

---

## Task 0: Scaffold the report and the throwaway parser

**Files:**
- Create: `docs/reconciliation-2026-06-13.md`
- Create (uncommitted, in /tmp): `/tmp/specparse.py`

- [ ] **Step 1: Write the throwaway spec parser**

Create `/tmp/specparse.py` — a reusable helper imported by later tasks (run as `python3 /tmp/specparse.py <cmd>`).

```python
import json, sys
SPEC = json.load(open('docs/swagger/5504-4.4.0.json'))
S = SPEC['components']['schemas']

def fields(name, _seen=None):
    """Resolve a schema's full property set, following allOf and $ref."""
    _seen = _seen or set()
    if name in _seen: return {}
    _seen.add(name)
    out = {}
    sc = S.get(name, {})
    for part in sc.get('allOf', []):
        if '$ref' in part:
            out.update(fields(part['$ref'].split('/')[-1], _seen))
        for k in part.get('properties', {}): out[k] = 1
    for k in sc.get('properties', {}): out[k] = 1
    return out

def entity_enum():
    gen = SPEC['paths']['/metrics/generate']['post']
    sch = gen['requestBody']['content']['application/json']['schema']
    if '$ref' in sch: sch = S[sch['$ref'].split('/')[-1]]
    return sch['properties']['entity']['enum']

if __name__ == '__main__':
    cmd = sys.argv[1] if len(sys.argv) > 1 else ''
    if cmd == 'entities':
        for e in entity_enum(): print(e)
    elif cmd == 'fields':
        for f in sorted(fields(sys.argv[2])): print(f)
    elif cmd == 'paths':
        for p in sorted(SPEC['paths']): print(p, list(SPEC['paths'][p].keys()))
```

- [ ] **Step 2: Verify the parser works**

Run: `cd /Users/fjacquet/Projects/pstore_exporter && python3 /tmp/specparse.py entities | wc -l`
Expected: `55`

Run: `python3 /tmp/specparse.py fields base_performance_metrics_by_volume`
Expected: includes `read_iops`, `write_iops`, `total_iops`, `avg_read_latency`, `volume_id`.

- [ ] **Step 3: Create the report skeleton**

Create `docs/reconciliation-2026-06-13.md`:

```markdown
# PowerStore 4.4.0 Spec Reconciliation — 2026-06-13

Reconciles `pstore_exporter` metric collection against the canonical PowerStore REST API
`4.4.0.0` definition (`docs/swagger/5504-4.4.0.json`, model 5504). Companion to
`docs/reconciliation-2026-06-05.md`.

**Method:** spec entity types / per-entity field schemas / inventory schemas extracted and
cross-referenced against emitted metrics and the fields the derive functions read. The spec
describes the REST `/metrics/generate` JSON — authoritative for the SDK path, semantically
indicative for the bulk CSV path (which uses `avg_`/`last_` column prefixes the REST JSON lacks).

## Pass 1 — Endpoint audit

_(Task 1)_

## Pass 2 — Field audit

_(Task 2)_

## Pass 3 — Capability-gate audit

_(Task 3)_

## Pass 4 — Coverage map (all 55 entity types)

_(Task 4)_

## Fix list

_(Task 5)_
```

- [ ] **Step 4: Commit**

```bash
rtk git add docs/reconciliation-2026-06-13.md
rtk git commit -m "docs(reconciliation): scaffold 4.4.0 reconciliation report"
```

---

## Task 1: Pass 1 — Endpoint audit

**Files:**
- Modify: `docs/reconciliation-2026-06-13.md` (Pass 1 section)

- [ ] **Step 1: Enumerate the endpoints the exporter calls**

Read the call sites. The repo-owned raw HTTP path lives in `bulk.go`; the SDK-driven calls
in `client.go`, `perentity.go`, `derive_*.go`.

Run: `cd /Users/fjacquet/Projects/pstore_exporter && grep -rnoE '(latest_five_min_metrics/[a-z]+|/metrics/generate)' internal/powerstore/*.go`
Expected: `bulk.go` references `latest_five_min_metrics/enable` and `latest_five_min_metrics/download`.

Run: `grep -rnoE 'Get[A-Z][A-Za-z]+|PerformanceMetricsBy[A-Za-z]+|SpaceMetricsBy[A-Za-z]+|APIClient\(\)\.Query' internal/powerstore/*.go | sort -u`
This lists the gopowerstore SDK methods (each maps to a REST path) plus the generic `Query` (drives).

- [ ] **Step 2: Confirm each path exists in 4.4.0**

Run: `python3 /tmp/specparse.py paths | grep -E 'latest_five_min_metrics|/metrics/generate|/volume|/appliance|/cluster|/fc_port|/eth_port|/sas_port|/file_system|/volume_group|/replication_session|/alert|/drive|/hardware'`

For each SDK method, map it to its REST path (e.g. `GetVolumes` → `/volume`,
`PerformanceMetricsByAppliance` → `/metrics/generate` with `entity=performance_metrics_by_appliance`,
drives → generic `Query` on `/hardware` or `/drive`). Note any method whose path is absent in 4.4.0.

- [ ] **Step 3: Write the Pass 1 table**

Replace the `_(Task 1)_` placeholder with a table: `Exporter call | REST path / entity | Present in 4.4.0? | Notes`.
One row per endpoint/SDK method. Mark every confirmed path ✅; flag any absent path ❌ with detail.

- [ ] **Step 4: Verify no row is unsubstantiated**

Re-run the Step 2 grep and confirm every ✅ row corresponds to a path printed by the script.

- [ ] **Step 5: Commit**

```bash
rtk git add docs/reconciliation-2026-06-13.md
rtk git commit -m "docs(reconciliation): pass 1 endpoint audit"
```

---

## Task 2: Pass 2 — Field audit

**Files:**
- Modify: `docs/reconciliation-2026-06-13.md` (Pass 2 section)

- [ ] **Step 1: Extract the fields each derive function reads**

Run: `cd /Users/fjacquet/Projects/pstore_exporter && grep -nE 'csvFloat\(r,' internal/powerstore/derive_bulk.go`
This lists every bulk CSV column (primary + fallback) the exporter reads.

Run: `grep -rnoE '\.[A-Z][A-Za-z]+' internal/powerstore/derive_perentity.go internal/powerstore/derive_*_perf.go | sort -u`
This lists the gopowerstore struct fields the SDK path reads.

- [ ] **Step 2: Extract canonical fields for every covered entity**

For each entity the exporter covers, dump the canonical field set:

```bash
for e in base_performance_metrics_by_volume base_performance_metrics_by_appliance \
         base_space_metrics_by_appliance base_performance_metrics_by_file_system \
         base_space_metrics_by_file_system base_copy_metrics_by_volume \
         base_performance_metrics_by_vg base_wear_metrics_by_drive base_space_metrics_by_cluster; do
  echo "=== $e ==="; python3 /tmp/specparse.py fields "$e"
done
```

(Confirm the exact `base_*` names exist first: `python3 -c "import json;print([k for k in json.load(open('docs/swagger/5504-4.4.0.json'))['components']['schemas'] if k.startswith('base_') and 'drive' in k])"` — adjust e.g. wear-metric schema name as needed.)

- [ ] **Step 3: Write the Pass 2 field-mapping table**

Replace `_(Task 2)_` with a table per covered entity: `Emitted metric | source field/column read | canonical 4.4.0 field | match? | risk`.

Classify each row:
- ✅ exact or fallback-covered match
- ⚠️ **missing-fallback risk** — primary key has no spec sibling AND no fallback (e.g.
  `avg_io_workload_cpu_utilization` vs canonical `io_workload_cpu_utilization`; flag
  "verify against live `--trace` capture")
- ❌ field read under a name the spec does not define at all

Seed findings to confirm and include (from the design's spot-check):
- `avg_read_iops`/`avg_write_iops`/`avg_total_iops` primaries have no REST sibling; `read_iops`
  etc. fallbacks cover them → ✅ (note the bulk-CSV-spelling caveat).
- `avg_io_workload_cpu_utilization` has no fallback → ⚠️.

- [ ] **Step 4: Verify each ⚠️/❌ against the code**

For every ⚠️/❌ row, open the cited line in `derive_bulk.go`/`derive_*_perf.go` and confirm the
read has no covering fallback. Remove any row that actually has a fallback.

- [ ] **Step 5: Commit**

```bash
rtk git add docs/reconciliation-2026-06-13.md
rtk git commit -m "docs(reconciliation): pass 2 field audit"
```

---

## Task 3: Pass 3 — Capability-gate audit

**Files:**
- Modify: `docs/reconciliation-2026-06-13.md` (Pass 3 section)

- [ ] **Step 1: Read the current capability gate**

Run: `cd /Users/fjacquet/Projects/pstore_exporter && cat internal/powerstore/capability.go`
Identify the bulk gate (software version `≥4.1` via `GetSoftwareMajorMinorVersion`).

- [ ] **Step 2: Confirm gated SDK methods exist in the SDK and the 4.4.0 API**

Cross-check the assumptions recorded in CLAUDE.md / ADR-0003 / ADR-0009 against the spec:
- `performance_metrics_by_file_system` IS in the entity enum → confirms FS perf exists (ADR-0003
  "no FS perf" note is stale; ADR-0009 supersedes). Verify:

Run: `python3 /tmp/specparse.py entities | grep -E 'file_system|file_by_|nas_server'`
Expected: `performance_metrics_by_file_system`, `space_metrics_by_file_system`,
`performance_metrics_file_by_*`, `performance_metrics_by_nas_server` all present.

- Confirm bulk path availability at 4.4.0 (`latest_five_min_metrics/*` present from Pass 1).

- [ ] **Step 3: Write the Pass 3 table**

Replace `_(Task 3)_` with: `Assumption (source: CLAUDE.md/ADR) | Still true at 4.4.0? | Evidence`.
Explicitly record the ADR-0003 "no FS perf" note as **stale — superseded by ADR-0009**, with the
entity-enum evidence. Note the `≥4.1` gate is satisfied by 4.4.0.

- [ ] **Step 4: Verify**

Re-run the Step 2 grep; confirm each "Still true" verdict cites a printed line.

- [ ] **Step 5: Commit**

```bash
rtk git add docs/reconciliation-2026-06-13.md
rtk git commit -m "docs(reconciliation): pass 3 capability-gate audit"
```

---

## Task 4: Pass 4 — Coverage map (all 55 entity types)

**Files:**
- Modify: `docs/reconciliation-2026-06-13.md` (Pass 4 section)

- [ ] **Step 1: List all 55 entity types and current emitted metrics**

Run: `cd /Users/fjacquet/Projects/pstore_exporter && python3 /tmp/specparse.py entities`
Run: `grep -rhoE 'powerstore_[a-z_]+' internal/powerstore/*.go | sort -u`

- [ ] **Step 2: Classify every entity type**

For each of the 55 entity types, decide emitted vs not, and assign a priority tag with a
one-line rationale. Use these tags:
- **emitted** — already collected (appliance/volume/cluster perf+space, FS perf+space, VG perf,
  drive wear, copy/replication).
- **high** — fits the exporter's purpose, not yet collected, clear operator value
  (e.g. `performance_headroom_by_appliance`, `space_metrics_by_storage_container`,
  `performance_metrics_by_{host,hg,initiator}`, `performance_metrics_by_nas_server`,
  `space_metrics_by_volume_family`, fe_fc/fe_eth port + node perf, `space_metrics_by_remote_system`).
- **medium** — useful but niche or higher-cardinality (`space_metrics_by_vm`,
  `performance_metrics_by_ip_port[_iscsi]`, `copy_metrics_by_{nas_server,file_system,remote_system}`).
- **skip** — outside this exporter's purpose; record the reason
  (vSphere `vsphere_*`, SMB/NFS protocol detail `performance_metrics_{smb*,nfs*}_by_node`,
  `wear_metrics_by_drive_daily` duplicate cadence, `performance_metrics_by_appliance_resource_util`
  if redundant).

- [ ] **Step 3: Write the Pass 4 coverage table**

Replace `_(Task 4)_` with a 55-row table: `Entity type | Status | Priority | Rationale`.
Every one of the 55 enum values MUST appear exactly once.

- [ ] **Step 4: Verify completeness**

Run: `python3 /tmp/specparse.py entities | sort > /tmp/all_entities.txt`
Extract the entity column from the table and compare:
`grep -oE 'performance_metrics_by_[a-z_]+|space_metrics_by_[a-z_]+|copy_metrics_by_[a-z_]+|wear_metrics_by_[a-z_]+|vsphere[a-z_]*|performance_headroom_by_[a-z_]+|performance_metrics_(smb|nfs|file)[a-z0-9_]*' docs/reconciliation-2026-06-13.md | sort -u > /tmp/table_entities.txt`
Run: `comm -23 /tmp/all_entities.txt /tmp/table_entities.txt`
Expected: empty (every enum value is in the table).

- [ ] **Step 5: Commit**

```bash
rtk git add docs/reconciliation-2026-06-13.md
rtk git commit -m "docs(reconciliation): pass 4 full coverage map (55 entity types)"
```

---

## Task 5: Fix list + ADR touch-ups

**Files:**
- Modify: `docs/reconciliation-2026-06-13.md` (Fix list section)
- Modify: `docs/adr/0003-use-gopowerstore-client.md` (stale-note supersession)

- [ ] **Step 1: Assemble the prioritized fix list**

From Pass 2 (⚠️/❌ rows) and Pass 4 (high/medium rows), write two subsections under the
`_(Task 5)_` placeholder:

**Correctness fixes** — table: `Issue | File:line | Fix | Effort`. Include every ⚠️ missing-fallback
(e.g. add `io_workload_cpu_utilization` fallback) and any ❌ wrong key. Each notes whether a live
`--trace` capture is needed to confirm the bulk CSV column spelling before changing code.

**Coverage additions** — table: `Entity/field | Priority | Both paths? | Effort | Notes`. Every row
MUST state the metric-parity obligation (land in `derive_bulk.go` AND the per-entity derive
together, shared label builders in `metrics.go`). Group high before medium.

- [ ] **Step 2: Formalize the stale ADR-0003 note**

Read `docs/adr/0003-use-gopowerstore-client.md`. Append a short superseded note at the relevant
"no FS perf" point:

```markdown
> **Superseded (2026-06-13):** PowerStore 4.4.0 exposes `performance_metrics_by_file_system`
> and `space_metrics_by_file_system`; gopowerstore v1.22 provides `PerformanceMetricsByFileSystem`.
> Live FS performance is collected (`derive_filesystem_perf.go`). See ADR-0009 and
> `docs/reconciliation-2026-06-13.md`.
```

(If an equivalent note already exists from prior reconciliation, leave it and reference the new report instead of duplicating.)

- [ ] **Step 3: Verify the fix list traces back to findings**

Confirm every correctness-fix row cites a Pass 2 ⚠️/❌ row, and every coverage row cites a Pass 4
high/medium entity. No fix should appear that has no upstream finding.

- [ ] **Step 4: Commit**

```bash
rtk git add docs/reconciliation-2026-06-13.md docs/adr/0003-use-gopowerstore-client.md
rtk git commit -m "docs(reconciliation): fix list + formalize ADR-0003 supersession"
```

---

## Task 6: Final review and index linkage

**Files:**
- Modify: `docs/reconciliation-2026-06-13.md`
- Modify: `docs/index.md` (link the new report, if it indexes docs)

- [ ] **Step 1: Read the whole report end to end**

Run: `cd /Users/fjacquet/Projects/pstore_exporter && cat docs/reconciliation-2026-06-13.md`
Check: no `_(Task N)_` placeholders remain; all four passes + fix list are populated; every
table verdict cites evidence.

- [ ] **Step 2: Confirm the 55-entity invariant once more**

Re-run Task 4 Step 4. Expected: empty diff.

- [ ] **Step 3: Link from the docs index if applicable**

Run: `grep -n 'reconciliation' docs/index.md`
If `docs/index.md` lists reconciliation docs, add a bullet linking `reconciliation-2026-06-13.md`
next to the 2026-06-05 entry. If it does not, skip this step.

- [ ] **Step 4: Commit**

```bash
rtk git add docs/reconciliation-2026-06-13.md docs/index.md
rtk git commit -m "docs(reconciliation): final review + index linkage"
```

---

## Done criteria

- `docs/reconciliation-2026-06-13.md` has all four passes + a two-part fix list, no placeholders.
- The Pass 4 table contains all 55 entity types exactly once (verified by `comm`).
- ADR-0003 stale note formalized.
- Every fix-list row traces to an upstream finding.
- No product code (`internal/`, `main.go`) modified by this plan.
