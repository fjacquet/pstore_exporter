# 12. Credentials via Environment, `${ENV}` Interpolation, and Password Files

## Status

Accepted

## Context

Each array requires a username and password to authenticate the gopowerstore
client (ADR-0003). Storing those secrets as plaintext in `config.yaml` is poor
practice: the file is often committed, baked into images, or mounted read-only,
and any of those leaks the credential. The exporter needs a way to keep secrets
out of the config file while staying easy to run on bare metal, under systemd,
in Docker Compose, and in Kubernetes — each of which injects secrets
differently (env vars, mounted files, `.env` files).

## Decision

Secrets are resolved from the environment and the filesystem, never required to
be inline in the config file:

- **`${ENV}` interpolation.** `utils.ResolveSecrets` expands `${VAR}` references
  in each array's `endpoint`, `username`, and `password` fields via
  `utils.ExpandEnv`. A reference to an unset variable is an error, not a silent
  empty string — misconfiguration fails loudly at startup rather than producing
  a confusing auth failure later.
- **Password files.** When `password` is empty and `passwordFile` is set, the
  password is read from that file. This supports Kubernetes/Docker secret mounts
  and systemd credentials without putting the value in any env var or config
  field.
- **`.env` convenience loading, env-wins precedence.** `utils.LoadDotEnv` loads a
  `.env` file (working directory first, then the config file's directory) *before*
  interpolation, so the `cp .env.example .env` quickstart works for bare-metal
  and systemd runs, mirroring what Docker Compose does natively. Crucially,
  `godotenv.Load` only sets variables that are **not already present** — real
  environment and secret injection always take precedence and can never be
  shadowed by a stray `.env` file. A missing `.env` is a no-op. See ADR-0010 for
  how this composes with reload.
- **Masking in logs.** `ArrayConfig.MaskPassword` renders a masked form
  (`ab****yz`, or fully masked when short) for any log line that references a
  password. The `--trace` transport additionally never logs request/response
  headers, where Basic-auth credentials and the DELL-EMC-TOKEN live (see
  ADR-0009 / `trace_transport.go`).

The resolution is wrapped in `models.NewSafeConfig(cfg, utils.ResolveSecrets)`
so the resolver re-runs on reload, keeping rotated secrets honored.

## Consequences

- `config.yaml` can be committed and image-baked safely: it carries `${VAR}`
  references or `passwordFile` paths, not secrets.
- The same config works across deployment models — env vars (`PSTORE1_PASSWORD`),
  mounted secret files, and `.env` quickstart — without per-environment forks.
- Unset `${VAR}` references fail fast at startup, surfacing misconfiguration
  before the first collection cycle rather than as an opaque login error.
- `.env` is strictly a convenience fallback; production secret injection via the
  real environment or a mounted file always wins, so `.env` can never
  accidentally override a deployed credential.
- Passwords are masked in normal logs and absent from `--trace` output, reducing
  the chance of credential leakage through observability surfaces.
