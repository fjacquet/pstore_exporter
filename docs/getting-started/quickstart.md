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
