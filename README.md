# pstore_exporter

[![CI](https://github.com/fjacquet/pstore_exporter/actions/workflows/ci.yml/badge.svg)](https://github.com/fjacquet/pstore_exporter/actions/workflows/ci.yml)
[![Release](https://github.com/fjacquet/pstore_exporter/actions/workflows/release.yml/badge.svg)](https://github.com/fjacquet/pstore_exporter/actions/workflows/release.yml)
[![Docs](https://github.com/fjacquet/pstore_exporter/actions/workflows/docs.yml/badge.svg)](https://fjacquet.github.io/pstore_exporter/)
[![Go Report Card](https://goreportcard.com/badge/github.com/fjacquet/pstore_exporter)](https://goreportcard.com/report/github.com/fjacquet/pstore_exporter)
[![Go Version](https://img.shields.io/github/go-mod/go-version/fjacquet/pstore_exporter)](go.mod)
[![Latest Release](https://img.shields.io/github/v/release/fjacquet/pstore_exporter?sort=semver)](https://github.com/fjacquet/pstore_exporter/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

A Go exporter for **Dell PowerStore** arrays. It authenticates to the PowerStore REST API,
collects the full performance and capacity statistic set, and exposes the metrics via
**both** a Prometheus `/metrics` endpoint **and** an OTLP metric push. It follows the
architecture of [`pflex_exporter`](https://github.com/fjacquet/pflex_exporter).

## Features

- **Dual export** — Prometheus pull (`/metrics`) and OTLP metric push, fed from one shared snapshot.
- **Two collection paths, auto-detected** — bulk CSV (PowerStoreOS 4.1+) via the compressed
  stats archive endpoint; per-entity REST fallback for older firmware — chosen automatically
  per array.
- **Multi-array** — one process monitors many arrays; every metric carries an `array` label.
- **Operational** — bearer auth with automatic token refresh, graceful per-array degradation,
  hot config reload (SIGHUP + file watch), snapshot-based health endpoint, optional OTLP
  tracing.

## Quick start

Install with Homebrew (macOS / Linuxbrew):

```bash
brew install fjacquet/tap/pstore_exporter
export PSTORE1_PASSWORD='your-monitor-password'
pstore_exporter --config config.yaml   # bring your own config.yaml
# metrics: http://localhost:9446/metrics   health: http://localhost:9446/health
```

Or build from source:

```bash
make cli
export PSTORE1_PASSWORD='your-monitor-password'
./bin/pstore_exporter --config config.yaml
```

Or with Docker Compose:

```bash
PSTORE1_PASSWORD='your-monitor-password' docker compose up --build
```

Or with the published image:

```bash
docker pull ghcr.io/fjacquet/pstore_exporter:latest
```

## Collection paths

The exporter auto-detects the right path per array at startup:

- **Bulk CSV** (PowerStoreOS 4.1+): downloads the compressed stats archive once per cycle
  and parses all entity types from a single API call. This is the preferred path — low API
  load, high fidelity.
- **Per-entity REST** (fallback): issues one REST query per entity type per cycle. Used
  when bulk export is unavailable.

The detected path is published as `powerstore_array_bulk_api` (`1` = bulk, `0` = per-entity).

## Multi-array

Each metric carries an `array` label (the configured array name) and a `cluster_id`
label. A single exporter binary can monitor several PowerStore arrays simultaneously:

```yaml
arrays:
  - name: pstore-prod
    endpoint: "https://10.0.0.1/api/rest"
    username: admin
    password: "${PSTORE1_PASSWORD}"
    insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}
  - name: pstore-dr
    endpoint: "https://10.0.0.2/api/rest"
    username: admin
    password: "${PSTORE2_PASSWORD}"
    insecureSkipVerify: ${PSTORE2_SKIP_CERTIFICATE}
```

`insecureSkipVerify` accepts either a native boolean (`true`/`false`) or a `${VAR}`
environment reference resolved at startup, same as `endpoint`/`username`/`password`.

## PromQL guidance

`powerstore_*` IOPS and bandwidth metrics are **already per-second gauges** computed by
PowerStore. Aggregate them with `sum`/`avg` in PromQL — **never wrap them in `rate()`** or
you will double-rate the values.

```promql
# Total read IOPS across all volumes on a specific array
sum by (array) (powerstore_volume_read_iops{array="pstore-prod"})

# Average appliance write latency
avg by (appliance_name) (powerstore_appliance_write_latency_microseconds)

# File-system capacity used %
100 * powerstore_file_system_size_used_bytes / powerstore_file_system_size_total_bytes
```

## Documentation

Full docs at **<https://fjacquet.github.io/pstore_exporter/>**:

- [Installation](https://fjacquet.github.io/pstore_exporter/getting-started/installation/) ·
  [Configuration](https://fjacquet.github.io/pstore_exporter/getting-started/configuration/) ·
  [Quick Start](https://fjacquet.github.io/pstore_exporter/getting-started/quickstart/)
- [Metrics Reference](https://fjacquet.github.io/pstore_exporter/metrics/)
- [Collection Paths](https://fjacquet.github.io/pstore_exporter/collection-paths/)
- Deployment:
  [Docker](https://fjacquet.github.io/pstore_exporter/deployment/docker/) ·
  [systemd](https://fjacquet.github.io/pstore_exporter/deployment/systemd/) ·
  [Kubernetes](https://fjacquet.github.io/pstore_exporter/deployment/kubernetes/)
- [Dashboards](https://fjacquet.github.io/pstore_exporter/dashboards/) ·
  [OpenTelemetry](https://fjacquet.github.io/pstore_exporter/opentelemetry/) ·
  [CI/CD & SBOM](https://fjacquet.github.io/pstore_exporter/cicd/)

Deployment manifests and examples live in [`deploy/`](deploy/); Grafana dashboards in
[`grafana/`](grafana/).

## Development

```bash
make tools         # install golangci-lint, govulncheck (pinned)
make sure          # fmt + vet + test + build + golangci-lint
make ci            # the gate CI runs (adds go test -race + govulncheck)
```

Validating against a live array — `--once --debug` dumps every collected sample (sorted,
exposition style) and `--trace` logs raw bulk-API response bodies (method/URL/status +
payload; headers — and thus credentials and the `DELL-EMC-TOKEN` — are never logged):

```bash
./bin/pstore_exporter --config config.yaml --once --debug --trace > run.log
grep '^powerstore_' run.log | sort > samples.txt   # diff against docs/metrics.md
grep 'API trace' run.log                           # raw bulk-API payloads
```

Note: `--trace` covers only the raw bulk-CSV HTTP path; gopowerstore offers no
transport hook for its typed calls.

## Notes

- IOPS and bandwidth are already per-second gauges — aggregate with `sum`/`avg` in PromQL,
  never `rate()`.
- Metric names use explicit units: `_bandwidth_bytes_per_second`, `_latency_microseconds`,
  `_bytes`.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
