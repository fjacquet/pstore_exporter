# /health always-200 (JSON) + /livez /readyz (pstore_exporter) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `/livez`/`/readyz` (always-200, no state). Convert `/health`
from plain text to a JSON body matching the rest of the family, always 200.

**Architecture:** New `staticOKHandler` registered at `/livez`/`/readyz`.
`healthHandler` (`main.go:386-403`, a method on `*Server`) rewritten to
build a JSON body from `s.store.Load().PerArray` instead of writing plain
text and a 503.

**Tech Stack:** Go, `net/http`, `net/http/httptest`.

## Global Constraints

- Repo: `/Users/fjacquet/Projects/pstore_exporter`.
- Spec: `/Users/fjacquet/Projects/obs_exporter/docs/superpowers/specs/2026-08-01-family-health-endpoint-design.md` (bucket C).
- **Breaking change**: `/health`'s body format changes from plain text to JSON, and the status code is always 200 (was 503 when every array was down). Call this out explicitly in CHANGELOG `### Changed`.
- `/livez`/`/readyz` are net-new — `### Added` in CHANGELOG.
- ADR index: `docs/adr/README.md`, 3-column format (`ADR | Title | Status`). Next ADR: 0017.
- No `main_test.go` exists in this repo's root package today — this plan creates it.
- `powerstore.ArraySnapshot.ScrapeError` is a `string`, not an `error` — no `.Error()` call needed. This repo's snapshot also carries `BulkCapable bool` — irrelevant to health, not included in the JSON body.

---

### Task 1: `/livez` `/readyz` + rewrite `/health` to JSON, always 200

**Files:**
- Modify: `main.go:119` (add two `mux.HandleFunc` lines after the existing `/health` registration)
- Modify: `main.go:386-403` (method `(s *Server) healthHandler`, full rewrite)
- Create: `main.go` — add `staticOKHandler` function after `healthHandler`'s closing brace
- Create: `main_test.go`

**Interfaces:**
- Consumes: `powerstore.SnapshotStore` (`internal/powerstore/snapshot.go`) — `Load() *Snapshot`, `NewSnapshotStore() *SnapshotStore` (seeds an empty, non-nil snapshot). `powerstore.Snapshot` (`internal/powerstore/snapshot.go:18-22`): `PerArray map[string]*ArraySnapshot`. `powerstore.ArraySnapshot` (`internal/powerstore/snapshot.go:8-15`): `Array string`, `Up bool`, `BulkCapable bool`, `ScrapeError string`, `LastScrape time.Time`, `Samples []Sample`.
- Produces: `staticOKHandler(w http.ResponseWriter, _ *http.Request)`. `(s *Server) healthHandler(w http.ResponseWriter, _ *http.Request)` — signature unchanged, body behavior changes.

- [ ] **Step 1: Write failing tests**

Create `main_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fjacquet/pstore_exporter/internal/powerstore"
)

func TestLivezReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	staticOKHandler(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyzReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	staticOKHandler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthReturns200WhenAllArraysDown(t *testing.T) {
	store := powerstore.NewSnapshotStore()
	store.Store(powerstore.BuildSnapshot([]*powerstore.ArraySnapshot{
		{Array: "pstore-01", Up: false, ScrapeError: "login POST: status 401", LastScrape: time.Now()},
	}))
	server := &Server{store: store}

	rec := httptest.NewRecorder()
	server.healthHandler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Arrays []struct {
			Array string `json:"array"`
			OK    bool   `json:"ok"`
			Err   string `json:"err"`
		} `json:"arrays"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Arrays) != 1 || body.Arrays[0].OK {
		t.Fatalf("arrays = %+v, want one array with ok=false", body.Arrays)
	}
	if body.Arrays[0].Err == "" {
		t.Fatalf("err field empty, want the scrape failure message")
	}
}

func TestHealthReturns200BeforeFirstCycle(t *testing.T) {
	server := &Server{store: powerstore.NewSnapshotStore()}

	rec := httptest.NewRecorder()
	server.healthHandler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
```

Check `main.go`'s own import path for `internal/powerstore` (e.g.
`"github.com/fjacquet/pstore_exporter/internal/powerstore"`) and use the
same string. `&Server{store: store}` must compile with just `store` set —
if the compiler demands more fields, add only the minimum it requires.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test . -run 'TestLivezReturnsOK|TestReadyzReturnsOK|TestHealthReturns200' -v`
Expected: `TestLivezReturnsOK`/`TestReadyzReturnsOK` FAIL with `undefined: staticOKHandler`. `TestHealthReturns200*` FAIL to decode JSON (old handler writes plain text).

- [ ] **Step 3: Add `staticOKHandler` and register `/livez` `/readyz`**

In `main.go`, change line 119 from:

```go
	mux.HandleFunc("/health", s.healthHandler)
```

to:

```go
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/livez", staticOKHandler)
	mux.HandleFunc("/readyz", staticOKHandler)
```

After `healthHandler`'s closing brace (currently line 403), add:

```go

// staticOKHandler always answers 200 — no array state, no collection
// state, nothing that can make it fail. /livez and /readyz both use it: a
// probe wired here can never be the reason a healthy process gets restarted
// or pulled from rotation.
func staticOKHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: Rewrite `healthHandler`**

Replace the full function (`main.go:386-403`, including its doc comment) with:

```go
// healthHandler always answers 200. The JSON body reports every configured
// array's cached status from the last collection cycle.
func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	type arrayHealth struct {
		Array      string `json:"array"`
		OK         bool   `json:"ok"`
		LastScrape string `json:"last_scrape"`
		Err        string `json:"err,omitempty"`
	}
	snap := s.store.Load()
	out := struct {
		Arrays []arrayHealth `json:"arrays"`
	}{}
	for _, as := range snap.PerArray {
		out.Arrays = append(out.Arrays, arrayHealth{
			Array:      as.Array,
			OK:         as.Up,
			LastScrape: as.LastScrape.Format(time.RFC3339),
			Err:        as.ScrapeError,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
```

Add `"encoding/json"` to `main.go`'s import block if not already present
(check with `go build ./...` in Step 6 — remove `fmt` if it becomes unused).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test . -run 'TestLivezReturnsOK|TestReadyzReturnsOK|TestHealthReturns200' -v`
Expected: all PASS.

- [ ] **Step 6: Run full test suite and build**

Run: `go build ./... && go test ./...`
Expected: builds clean (fix any now-unused imports), all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add main.go main_test.go
git commit -m "feat: /health returns JSON, always 200; add /livez /readyz

/health previously returned plain text and 503 when every array was
down. It now emits JSON (arrays: [{array, ok, last_scrape, err}]) and
always answers 200 — an array being unreachable is data the exporter
reports, not a failure of the exporter itself. Matches obs_exporter's
ADR-0013/ADR-0014 pattern.

BREAKING CHANGE: /health's body format changes from plain text to
JSON; its status code is always 200 (previously 503 when every array
was unreachable)."
```

---

### Task 2: Chart, ADR, docs, CHANGELOG

**Files:**
- Modify: `charts/pstore-exporter/values.yaml:50-57`
- Create: `docs/adr/0017-health-always-200-and-static-probes.md`
- Modify: `docs/adr/README.md` (append row after 0016, 3-column format `| ADR | Title | Status |`)
- Modify: `CHANGELOG.md` (under existing `## [Unreleased]`)
- Modify: any deployment/monitoring docs mentioning `/health`'s body format or probe wiring — grep first (see Step 1)

**Interfaces:**
- Consumes: nothing (docs-only task).
- Produces: nothing.

- [ ] **Step 1: Find every doc mentioning `/health` as a probe target or describing its body/status**

Run: `grep -rln '/health\|livenessProbe\|readinessProbe' docs/ README.md 2>/dev/null`

Update every hit: probes now use `/livez`/`/readyz` (always 200, no array
state); `/health` is JSON now (`arrays: [{array, ok, last_scrape, err}]`),
always 200 — not plain text, not ever 503.

- [ ] **Step 2: Update the chart**

In `charts/pstore-exporter/values.yaml:50-57`, change:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: http
readinessProbe:
  httpGet:
    path: /health
    port: http
```

to:

```yaml
livenessProbe:
  httpGet:
    path: /livez
    port: http
readinessProbe:
  httpGet:
    path: /readyz
    port: http
```

- [ ] **Step 3: Write ADR-0017**

Create `docs/adr/0017-health-always-200-and-static-probes.md`:

```markdown
# `/livez` `/readyz`, and `/health` always answering 200

## Status

Accepted (2026-08-01)

## Context

Same argument as obs_exporter's ADR-0013 and ADR-0014, applied here in one
pass: an exporter is a probe. "Array unreachable" is data it reports, not a
failure of the exporter process. Coupling that fact to an HTTP status code
on any endpoint — the chart's `livenessProbe`/`readinessProbe`, or the
informational `/health` — risks something downstream (kubelet, a dashboard,
a script) treating a healthy, correctly-reporting exporter as down.

`charts/pstore-exporter/values.yaml` wired both `livenessProbe` and
`readinessProbe` to `/health`, which answered 503 when every configured
array was unreachable. As a *liveness* check this was always wrong: no
restart makes an unreachable array reachable.

## Decision

Two new endpoints, `/livez` and `/readyz`, both `staticOKHandler` — always
`200 OK`, no `SnapshotStore` read, nothing that can make either fail once
the process is running. The chart's default probes now point at them.

`/health`'s `healthHandler` no longer writes plain text or a 503. It always
answers 200 with a JSON body: `arrays: [{array, ok, last_scrape, err}]`,
built from the same `SnapshotStore` `/metrics` reads.

## Consequences

- **Breaking**: `/health`'s response body changes from plain text
  (`"OK"`/`"OK (starting)"`/`"UNHEALTHY: ..."`) to JSON, and its status code
  is always 200 (previously 503 when every array was down). Anything
  parsing the old text format or gating on the old status code needs
  updating.
- Chart default probe wiring changes; a fresh `helm install` or an upgrade
  without pinned probe overrides gets the fix automatically.
- Alert on a per-array `_up` metric (or `/health`'s body), never on any
  probe's HTTP status.
```

- [ ] **Step 4: Add the ADR to the index**

In `docs/adr/README.md`, after the `0016` row, add:

```markdown
| [0017](0017-health-always-200-and-static-probes.md) | `/livez`/`/readyz` static probes; `/health` always answers 200 | Accepted |
```

- [ ] **Step 5: CHANGELOG entry**

In `CHANGELOG.md`, under the existing `## [Unreleased]` heading, add:

```markdown
### Added

- `/livez` and `/readyz`: probe endpoints that always answer 200, with no
  dependency on array reachability or the collection cycle. See ADR-0017.

### Changed

- `/health` always answers 200, never 503, and its body is now JSON
  (`arrays: [{array, ok, last_scrape, err}]`) instead of plain text. See
  ADR-0017. **Breaking**: anything parsing the old plain-text body or
  gating on the old 503 status needs updating.
- The chart's default `livenessProbe`/`readinessProbe` now point at
  `/livez`/`/readyz` instead of `/health`.
```

- [ ] **Step 6: Lint chart + build docs**

Run: `helm lint charts/pstore-exporter` (or the exact CI invocation from `.github/workflows/` if different)
Expected: exits 0.

Run: `mkdocs build --strict` (if `mkdocs.yml` present)
Expected: exits 0.

- [ ] **Step 7: Commit**

```bash
git add charts/pstore-exporter/values.yaml docs/adr/0017-health-always-200-and-static-probes.md \
  docs/adr/README.md CHANGELOG.md
git commit -m "docs+chart: record ADR-0017, repoint chart probes to /livez /readyz"
```
