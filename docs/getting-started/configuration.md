# Configuration

The exporter is configured with a single YAML file (default example: `config.yaml`).

```yaml
server:
  host: "0.0.0.0"
  port: "9446"
  uri: "/metrics"
  logName: ""          # absolute path -> file + stdout; "" -> stdout only (recommended under systemd/k8s)

collection:
  interval: "30s"      # how often the background loop polls every array
  timeout: "20s"       # per-array collection timeout

opentelemetry:
  metrics:             # OTLP metric push
    enabled: false
    endpoint: "localhost:4317"
    insecure: true
    interval: "30s"
  tracing:             # optional diagnostic tracing
    enabled: false
    endpoint: "localhost:4317"
    insecure: true
    samplingRate: 0.1

arrays:
  - name: pstore-1
    endpoint: "https://${PSTORE1_HOSTNAME}/api/rest"  # PowerStore management IP or FQDN
    username: "${PSTORE1_USERNAME}"
    password: "${PSTORE1_PASSWORD}"
    insecureSkipVerify: true
    # interval: Five_Mins   # optional: override the stats interval on the PowerStore side
```

## Sections

| Section | Key | Notes |
|---|---|---|
| `server` | `host`, `port`, `uri` | HTTP bind address and Prometheus metrics path. Default port is `9446`. |
| `server` | `logName` | Log file path (use an **absolute** path so it resolves the same in containers); empty string logs to stdout (recommended under systemd/k8s). If the path is not writable, logging falls back to stdout with a warning instead of failing to start. |
| `collection` | `interval` | Background poll period for every array. Matches Prometheus scrape cadence well at `30s`. |
| `collection` | `timeout` | Per-array timeout; a slow/unreachable array fails fast without blocking others. |
| `opentelemetry.metrics` | `enabled`, `endpoint`, `interval` | OTLP gRPC metric push. |
| `opentelemetry.tracing` | `enabled`, `endpoint`, `samplingRate` | OTLP gRPC tracing for diagnosing slow cycles. |
| `arrays[]` | `name` | Unique; becomes the `array` label/attribute on every metric. |
| `arrays[]` | `endpoint` | Full URL to the PowerStore REST API, e.g. `https://10.0.0.1/api/rest`. |
| `arrays[]` | `username`, `password` | Credentials. `insecureSkipVerify` accepts self-signed management certificates. |

## Environment variables / .env

`${ENV_VAR}` references in `endpoint`, `username`, and `password` are expanded at
config load; an unset variable fails startup loudly instead of silently
authenticating with an empty value. `passwordFile` is still supported.

### .env loading

The `pstore_exporter` binary loads a `.env` file natively at startup — from the
working directory first, then next to the config file — so `cp .env.example .env`
works for bare-metal and systemd runs exactly like it does under docker compose.
Already-set environment variables **always take precedence** over `.env` values,
so secret injection (systemd `Environment=`, Kubernetes secrets, CI) can never be
shadowed by a stray file.

The `PSTORE1_*` variables wired into `docker-compose.yml` (with literal defaults)
are a quickstart convenience for **exactly one array** — that's what the `1`
means. Copy `.env.example` to `.env` (gitignored; Compose reads it natively) and
set `PSTORE1_HOSTNAME`, `PSTORE1_USERNAME`, `PSTORE1_PASSWORD`.

`config.yaml` remains the source of truth and is always consumed. For multiple
arrays, add one `arrays[]` entry per array — with literal values, or with your own
additional env refs (e.g. `${PSTORE2_HOSTNAME}`) that you must also pass through
in your compose `environment:` block.

## Multi-array

Add as many entries to `arrays[]` as needed. Each is polled independently every
`collection.interval`. A failing array logs an error, emits `powerstore_up{array="..."}=0`,
and does not affect other arrays.

```yaml
arrays:
  - name: pstore-prod
    endpoint: "https://10.0.0.1/api/rest"
    username: admin
    password: "${PSTORE1_PASSWORD}"
    insecureSkipVerify: true
  - name: pstore-dr
    endpoint: "https://10.0.0.2/api/rest"
    username: admin
    password: "${PSTORE2_PASSWORD}"
    insecureSkipVerify: true
```

## Secrets

Array passwords should not be written in plaintext. Two options:

- **Environment interpolation** — `password: "${PSTORE1_PASSWORD}"` is replaced with the
  value of the `PSTORE1_PASSWORD` environment variable at load time. A referenced but unset
  variable is a startup error (fail-loud).
- **File reference** — `passwordFile: /etc/pstore_exporter/pstore1.pass` reads the password
  from a file (trimmed).

```yaml
arrays:
  - name: pstore-prod
    endpoint: "https://10.0.0.1/api/rest"
    username: admin
    passwordFile: /etc/pstore_exporter/pstore1.pass
    insecureSkipVerify: true
```

## Hot reload

The configuration is reloaded without a restart on **SIGHUP** or when the config file
changes on disk. A new config is validated before it is applied — an invalid file is
rejected and the running configuration is left untouched. When the set of arrays changes,
the client pool is rebuilt.

```bash
kill -HUP $(pgrep pstore_exporter)     # or: systemctl reload pstore_exporter
```

## Validation

`pstore_exporter --config config.yaml` validates on startup: port ranges, durations,
unique non-empty array names, required array fields, and OTLP endpoints. Use
`--once` to run a single collection cycle and exit (useful for smoke tests), and
`--debug` for verbose logging.
