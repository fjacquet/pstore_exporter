# Docker

## Image

Multi-arch images are published to GHCR with SBOM and provenance attestations:

```bash
docker pull ghcr.io/fjacquet/pstore_exporter:0.1.0   # or :latest
```

## Run

Mount a config and pass array secrets via the environment (referenced as
`${PSTORE1_PASSWORD}` in the config):

```bash
docker run -d --name pstore_exporter \
  -p 9446:9446 \
  -e PSTORE1_PASSWORD='your-monitor-password' \
  -v "$PWD/config.yaml:/etc/pstore_exporter/config.yaml:ro" \
  ghcr.io/fjacquet/pstore_exporter:0.1.0
```

The image runs as a non-root user (`uid 10001`).

### Logging

Logs always go to **stdout** (captured by `docker logs` / `docker compose logs`). When
`server.logName` in `config.yaml` is also set to a file path, that file is written *in
addition* to stdout. Set `logName: ""` to disable the file entirely.

## Compose stack

`docker-compose.yml` brings up the exporter together with Prometheus, Grafana (with the
bundled dashboards auto-provisioned), and an OpenTelemetry Collector. It **builds** the
exporter image locally:

```bash
PSTORE1_PASSWORD='your-monitor-password' docker compose up --build
```

If you'd rather run the **published** image instead of building, use
`docker-compose.ghcr.yml` — same stack, but the exporter is pulled from GHCR:

```bash
# :latest
PSTORE1_PASSWORD='your-monitor-password' docker compose -f docker-compose.ghcr.yml up -d
# pin a version
PSTORE_TAG=0.1.0 PSTORE1_PASSWORD='...' docker compose -f docker-compose.ghcr.yml up -d
# refresh images later
docker compose -f docker-compose.ghcr.yml pull
```

| Service | Port | Purpose |
|---|---|---|
| `pstore_exporter` | 9446 | `/metrics` + `/health` |
| `prometheus` | 9090 | scrapes the exporter (`prometheus.yml`) |
| `grafana` | 3000 | dashboards (login `admin` / `admin`), Prometheus datasource + block/file folders auto-provisioned |
| `otel-collector` | 4317 / 8889 | receives the OTLP push (when enabled) and re-exposes it |

Open Grafana at <http://localhost:3000> — the PowerStore dashboards appear under the
**block** and **file** folders, already wired to the Prometheus datasource. To exercise
the OTLP path, set `opentelemetry.metrics.enabled: true` and
`endpoint: "otel-collector:4317"` in `config.yaml`.

### Grafana login

Credentials are set on the `grafana` service in `docker-compose.yml`:

```yaml
environment:
  - GF_SECURITY_ADMIN_USER=admin
  - GF_SECURITY_ADMIN_PASSWORD=admin
```

| | |
|---|---|
| URL | <http://localhost:3000> (or `http://<host>:3000`) |
| Username | `admin` |
| Password | `admin` |

!!! warning
    These are local test-stack credentials. Change them (and avoid exposing port 3000)
    before using this stack anywhere shared.
