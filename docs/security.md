# Security Policy

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities through one of these private channels:

1. **GitHub private security advisory** (preferred):
   <https://github.com/fjacquet/pstore_exporter/security/advisories/new>

2. **Email**: Contact the maintainer at the email address listed in the GitHub
   profile (<https://github.com/fjacquet>).

Include in your report:

- A description of the vulnerability and its potential impact.
- Steps to reproduce or a proof-of-concept if available.
- The version(s) affected.

You will receive an acknowledgement within 72 hours and a resolution timeline
once the issue is assessed. Please allow time for a fix to be prepared before
public disclosure.

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | Yes       |

## Security Notes

### Credentials

Array credentials are supplied to the exporter in one of two ways:

- **Environment-variable interpolation** in the config file: `password: "${PSTORE1_PASSWORD}"`.
  Set the variable in the process environment or a secrets manager; never write
  the literal value into `config.yaml`.
- **`passwordFile`**: a path to a file containing the password, readable only
  by the exporter process user.

Never commit credentials to version control. The `.gitignore` excludes common
secret file patterns, but review your config before committing.

### TLS Verification

The `insecureSkipVerify: true` option disables TLS certificate verification for
arrays using self-signed certificates. This is an **operator opt-in** setting:
it is logged as a warning at startup and is not the default. Use it only for
lab or air-gapped environments where a trusted certificate cannot be issued. In
production, provide a valid certificate and leave this option unset (or `false`).

### Exposed Endpoints

The exporter exposes only two HTTP endpoints:

- `/metrics` — Prometheus metrics (read-only, no write path).
- `/health` — liveness/readiness probe (returns `"starting"` or `"ok"`,
  no sensitive data).

There is no authentication on these endpoints by default. If your environment
requires it, place the exporter behind a reverse proxy or use Prometheus's
built-in TLS/auth configuration to scrape via HTTPS with bearer token.

The exporter holds **read-only** API credentials to the PowerStore array and
performs no write operations.
