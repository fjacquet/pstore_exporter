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
  maxConcurrency: 16   # fleet-wide cap on concurrent API requests per array (default 16)

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
    insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}
    # interval: Five_Mins   # optional: override the stats interval on the PowerStore side
    # maxConcurrency: 2     # optional: override the fleet concurrency cap for this array
    #                       # (e.g. a PowerStore 500; use 1 for fully sequential)
```

## Sections

| Section | Key | Notes |
|---|---|---|
| `server` | `host`, `port`, `uri` | HTTP bind address and Prometheus metrics path. Default port is `9446`. |
| `server` | `logName` | Log file path (use an **absolute** path so it resolves the same in containers); empty string logs to stdout (recommended under systemd/k8s). If the path is not writable, logging falls back to stdout with a warning instead of failing to start. |
| `collection` | `interval` | Background poll period for every array. Matches Prometheus scrape cadence well at `30s`. |
| `collection` | `timeout` | Per-array timeout; a slow/unreachable array fails fast without blocking others. |
| `collection` | `maxConcurrency` | Cap on concurrent PowerStore API requests per array's per-entity fan-outs (replication, FS/VG perf, appliance enumeration). Default `16`. Valid range: `1` (fully sequential — gentlest load) up to any positive integer; `0`/unset uses the default. Lower it to reduce load on a busy/degraded array or an entry-level model — cycles run slower, so a larger `timeout` may be needed. Override per array with `arrays[].maxConcurrency` to throttle one array without slowing the rest of the fleet. See [Tuning concurrency for small or busy arrays](#tuning-concurrency-for-small-or-busy-arrays). |
| `opentelemetry.metrics` | `enabled`, `endpoint`, `interval` | OTLP gRPC metric push. |
| `opentelemetry.tracing` | `enabled`, `endpoint`, `samplingRate` | OTLP gRPC tracing for diagnosing slow cycles. |
| `arrays[]` | `name` | Unique; becomes the `array` label/attribute on every metric. |
| `arrays[]` | `endpoint` | Full URL to the PowerStore REST API, e.g. `https://10.0.0.1/api/rest`. |
| `arrays[]` | `username`, `password` | Credentials. |
| `arrays[]` | `insecureSkipVerify` | Skip TLS certificate verification (accepts self-signed management certificates). Accepts a native boolean or a `${VAR}` environment reference (e.g. `${PSTORE1_SKIP_CERTIFICATE}`), resolved the same way as `endpoint`/`username`/`password`. Defaults to `false`. |

## Tuning concurrency for small or busy arrays

Each collection cycle fans out per-entity PowerStore API calls — replication sessions,
file-system and volume-group performance, appliance enumeration — and `maxConcurrency`
caps how many of those run at once against a **single** array. The default of `16` suits
mid-range and larger models; on an entry-level array such as a **PowerStore 500**, or any
array that is busy or degraded, that fan-out can add noticeable management-plane load.

Lower the cap for the affected array. Because the per-array setting overrides the fleet
default, you can throttle one small array without slowing collection for the rest:

```yaml
collection:
  maxConcurrency: 16          # fleet default — unchanged for the bigger arrays

arrays:
  - name: pstore-500
    endpoint: "https://10.0.0.9/api/rest"
    username: admin
    password: "${PSTORE500_PASSWORD}"
    insecureSkipVerify: true
    maxConcurrency: 2         # gentle on an entry-level array; drop to 1 if still stressed
```

Guidance:

- **Start at `2`**, and set `1` (fully sequential — one API request in flight at a time)
  if the array is still stressed. `1` is the floor; there is no separate "off" value.
- **Lower concurrency means longer cycles.** If a cycle starts hitting
  `collection.timeout` (default `20s`), raise the timeout, and/or lengthen
  `collection.interval` so cycles don't overlap.
- **Diminishing returns:** going from `2` to `1` roughly halves peak concurrent load but
  can noticeably lengthen cycles on arrays with many entities. Measure with `--once
  --debug` before and after.

## Environment variables / .env

`${ENV_VAR}` references in `endpoint`, `username`, `password`, and
`insecureSkipVerify` are expanded at config load; an unset variable fails startup
loudly instead of silently authenticating with an empty value (or silently
disabling TLS verification). `passwordFile` is still supported.

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
    insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}
  - name: pstore-dr
    endpoint: "https://10.0.0.2/api/rest"
    username: admin
    password: "${PSTORE2_PASSWORD}"
    insecureSkipVerify: ${PSTORE2_SKIP_CERTIFICATE}
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

### Passwords with special characters

Any character is safe end to end — the password is sent via HTTP Basic authentication
(base64-encoded in the `Authorization` header), so nothing needs URL-encoding. The only
place quoting matters is **parsing at load time**, and it differs by where you put the
password:

| Source | Rule |
|---|---|
| `.env`, single-quoted `'…'` | Fully literal — no `$` expansion, no `\` escapes, no `#` comment. Best default. Cannot contain a literal `'`. |
| `.env`, double-quoted `"…"` | Expands `$VAR`/`${VAR}` and processes `\` escapes. `$`, `\`, `"` are special — write `\$`, `\\`, `\"`. |
| `.env`, unquoted | `$VAR` expands; a ` #` (space-hash) starts a comment; a value **starting** with `'`/`"` is treated as quoted. |
| `config.yaml` inline | Only the exact `${NAME}` token is interpolated (`os.LookupEnv`), so a literal password containing `${NAME}` is treated as an env ref. Prefer referencing an env var. |
| `passwordFile` | Read **verbatim** (only surrounding whitespace trimmed) — no interpolation, no escaping. The bulletproof option. |

For quotes inside the password specifically: use double quotes to include a `'`, single
quotes to include a `"`. If the password has **both** `'` and `"` (or a `\`, or starts
with a quote), use `passwordFile` — it needs no escaping at all. When referencing an env
var from `config.yaml` (`password: "${PSTORE1_PASSWORD}"`) the value is inserted verbatim
and never re-scanned, so the env var itself may contain `$`, `${…}`, or any character.

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
