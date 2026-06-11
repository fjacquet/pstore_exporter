# Quick Start

```bash
make cli
export PSTORE1_PASSWORD='your-monitor-password'
./bin/pstore_exporter --config config.yaml
```

Then:

```bash
curl -s localhost:9101/metrics | grep '^powerstore_up'
curl -s localhost:9101/health
```

You should see `powerstore_up{array="pstore-1"} 1` once the first collection cycle
completes, and `/health` returns `200 OK`.

## One-shot mode

Run a single collection cycle and exit — handy for verifying connectivity and that the
expected metrics are produced, without starting the server loop:

```bash
./bin/pstore_exporter --config config.yaml --once --debug
```

Useful flags:

- `--once` — run a single collection cycle, log the result, and exit (connectivity check).
- `--debug` — verbose logging. Combined with `--once`, it also prints **every collected
  sample** (sorted, exposition style) so you can diff a live array against the
  [metrics reference](../metrics.md).
- `--trace` — log raw bulk-API response bodies (method, URL, status, payload). Headers are
  **never** logged, so the Basic-auth credentials and the `DELL-EMC-TOKEN` CSRF token
  cannot leak; `login_session` responses are skipped entirely. Scope: only the raw
  bulk-CSV HTTP path (`latest_five_min_metrics`) is traced — gopowerstore builds its HTTP
  client internally with no transport hook, so typed SDK calls cannot be traced.

Validating against a real array:

```bash
./bin/pstore_exporter --config config.yaml --once --debug --trace > run.log
grep '^powerstore_' run.log | sort > samples.txt   # every collected sample (compare with docs/metrics.md)
grep 'API trace' run.log                           # raw bulk-API payloads for anything missing or suspicious
```

Sample lines start with `powerstore_`, while log records are JSON objects, so the two are
easy to separate even though both go to stdout. The exporter never guesses values: an
unexpected payload shape shows up as a *missing* sample, and the trace shows what the
array actually returned.

## Local stack (Docker Compose)

Bring up the exporter alongside Prometheus, Grafana (dashboards auto-provisioned), and an
OpenTelemetry Collector:

```bash
PSTORE1_PASSWORD='your-monitor-password' docker compose up --build
```

- Exporter metrics: <http://localhost:9101/metrics>
- Prometheus: <http://localhost:9090>
- Grafana: <http://localhost:3000> (login `admin` / `admin`; PowerStore dashboards under the **block** and **file** folders)
- OTLP collector receives the push when `opentelemetry.metrics.enabled: true`.

To run the **published** image instead of building locally, use the pull-based stack:

```bash
PSTORE1_PASSWORD='your-monitor-password' docker compose -f docker-compose.ghcr.yml up -d
```

See [Docker deployment](../deployment/docker.md) for both stacks, image tags, and Grafana details.

## What to look at

- Per-array health: `powerstore_up`, `powerstore_last_scrape_timestamp_seconds`,
  `powerstore_array_bulk_api`.
- Capacity: `powerstore_appliance_physical_used_bytes` vs `powerstore_appliance_physical_total_bytes`.
- Performance: `powerstore_appliance_total_iops`, `powerstore_appliance_read_bandwidth_bytes_per_second`.
- File: `powerstore_file_system_size_used_bytes` / `powerstore_file_system_size_total_bytes`.

See the [Metrics Reference](../metrics.md) for the full list and the
[Dashboards](../dashboards.md) page for ready-made Grafana panels.
