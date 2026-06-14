# 13. `--trace` HTTP Transport and Its Scope Limitation

## Status

Accepted

## Context

Validating that the exporter parses live-array payloads correctly requires
seeing the raw HTTP responses the array returns — payload shapes drift between
PowerStore OS versions, and the bulk metrics endpoints return CSV-in-an-archive
that is awkward to reason about blind. A `--trace` flag was added to dump those
raw bodies during debugging against a live array.

Two constraints shape the design. First, the traced data is sensitive:
PowerStore authenticates with HTTP Basic credentials and a DELL-EMC-TOKEN CSRF
header, and `/login_session` responses carry session material. Second,
gopowerstore v1.22.0 constructs its `*http.Client` inside `api.New` and exposes
no `ClientOptions` seam to inject a custom transport or `RoundTripper`
(documented in ADR-0003 and the project memory of SDK constraints). There is no
way to wrap the typed SDK calls.

## Decision

`--trace` installs a `tracingRoundTripper` that logs the method, URL, status,
and full response **body** of every request that flows through it. Its scope and
redaction are deliberately bounded:

- **Headers are never logged** — Basic-auth credentials and the DELL-EMC-TOKEN
  travel in headers, so both request and response headers are unconditionally
  omitted.
- **`/login_session` responses are skipped entirely** — they contain session
  material that must not reach logs.
- **Scope is the repo-owned raw HTTP path only.** Because gopowerstore offers no
  transport seam, the wrapper can only cover the raw HTTP path the repo owns —
  the bulk `latest_five_min_metrics` endpoints. Typed SDK calls (topology,
  per-entity performance, capability detection) are *not* observable through
  `--trace`. This is a known, accepted limitation, not a bug.
- The round-tripper reads the body in full and replaces it, so the caller still
  receives an intact stream. `--trace` is verbose and intended for live
  debugging only, paired with `--once --debug` for sample dumping.

## Consequences

- Live bulk-API payloads can be inspected against a real array to validate
  parsing across PowerStore OS versions, without leaking credentials or session
  tokens.
- The trace is incomplete by construction: it shows the bulk HTTP path but is
  blind to all typed gopowerstore calls. Anyone debugging a per-entity or
  topology issue must fall back to other means (mock-level tests, SDK-side
  logging) — `--trace` will show them nothing.
- If a future gopowerstore release adds a transport/`RoundTripper` injection
  seam, this ADR should be revisited: the same wrapper could then cover the
  typed calls too, closing the observability gap.
- The header-redaction and `/login_session` skip are security-critical
  invariants of the tracer; any change to `trace_transport.go` must preserve
  them (consistent with the no-suppression Semgrep policy in CLAUDE.md).
