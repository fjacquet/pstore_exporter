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

## Pass 4 — Coverage map (all 56 entity types)

_(Task 4)_

## Fix list

_(Task 5)_
