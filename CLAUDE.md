# CLAUDE.md

Dell PowerStore Prometheus + OTLP exporter (Go). Binary `pstore_exporter`, metrics
port 9101, metric prefix `powerstore_`.

## Commands
```bash
make cli          # build bin/pstore_exporter
make test         # go test ./...
make test-race    # go test -race + coverage
make ci           # full gate: fmt-check, vet, lint, test -race, vuln (run before pushing)
make sure         # fmt + vet + test + build + lint (local dev loop)
PSTORE1_PASSWORD=x ./bin/pstore_exporter --config config.yaml --once   # one cycle, no server
docker compose up # exporter:9101 + Prometheus:9090 + Grafana:3000 + otel-collector
```

## Architecture
Background collection loop → immutable `Snapshot` in `SnapshotStore` → dual export
(Prometheus `/metrics` + optional OTLP push). One process, many arrays (each metric
carries an `array` label); per-array failures degrade gracefully (`powerstore_up=0`).
- `main.go` — CLI (cobra), HTTP server, wiring, SIGHUP/file-watch reload.
- `internal/powerstore/` — `client.go` (gopowerstore wrapper), `collector.go` (loop),
  `topology.go` (inventory + label-resolution indices), `perentity.go`/`derive_perentity.go`
  and `bulk.go`/`derive_bulk.go` (the two metric paths), `snapshot.go`, `prometheus.go`, `otlp.go`.
- `internal/models` config, `internal/{utils,logging,telemetry,config}` support.
Decisions are recorded in `docs/adr/`; metric catalog in `docs/metrics.md`.

## Non-obvious constraints (READ BEFORE EDITING)
- **Metric parity invariant:** the bulk and per-entity paths MUST emit identical metric
  names and label KEYS (a test enforces volume parity). Edit both `derive_*.go` together;
  use the shared label builders in `metrics.go`.
- **Values are gauges, not counters:** IOPS are per-second, bandwidth bytes/sec, latency µs.
  Aggregate with `sum`/`avg` in PromQL — NEVER `rate()`.
- **gopowerstore v1.22 limits:** no list-appliances method (enumerate distinct IDs from
  volumes+ports → `GetAppliance`); no version field on appliances (use
  `GetSoftwareMajorMinorVersion` for bulk capability ≥4.1); no `PerformanceMetricsByFileSystem`
  (FS capacity comes from inventory). See `docs/adr/0003`.
- **Serve before collect:** the HTTP server must start before the first collection cycle —
  gopowerstore's login isn't bounded by the collection timeout, so blocking startup on it
  stalls `/metrics`. See `docs/adr/0007`.
- **Semgrep policy:** no inline `//nolint` or semgrep suppressions; fix the finding.

## Testing
TDD. Offline coverage is at the `Client`-interface mock (`collector_test.go`) + per-derive
unit tests + the pipeline integration test; the live bulk HTTP download is not offline-testable.

## CI
`.github/workflows/ci.yml` runs lint/vet/test-race/govulncheck + SBOM + Semgrep on PRs.
Default branch is `main`. Docs deploy to GitHub Pages on push to `main` (`docs/**`).
