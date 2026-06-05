# Contributing to pstore_exporter

Thank you for contributing! This guide covers prerequisites, local development
workflow, invariants you must not break, and the PR checklist.

## Prerequisites

| Tool | Version | How to get |
|------|---------|------------|
| Go | see `go.mod` (`go 1.26.4`) | <https://go.dev/dl/> |
| golangci-lint | pinned in Makefile | `make tools` |
| govulncheck | pinned in Makefile | `make tools` |
| semgrep | latest | `pip install semgrep` or `brew install semgrep` |
| mkdocs + mkdocs-material | latest | `pip install mkdocs-material pymdown-extensions` |
| Docker + Compose | any recent | for the stack target |
| goreleaser + syft + cosign | latest | release/SBOM tooling — `brew install goreleaser syft cosign` (optional) |

Run `make tools` to install the pinned Go tooling (golangci-lint, govulncheck)
into your `$GOPATH/bin`. Release tooling (`goreleaser`, `syft`, `cosign`) is only
needed for `make sbom`/`make release` and is provided by CI actions otherwise.

## Local Development Workflow

```bash
# Build the binary
make cli

# Run unit tests
make test

# Full CI gate (fmt-check, vet, lint, test-race, govulncheck)
make ci

# Local convenience gate (fmt, vet, test, build, lint)
make sure
```

Individual targets:

| Target | What it does |
|--------|-------------|
| `make fmt` | `go fmt ./...` |
| `make fmt-check` | Fail if gofmt would change anything (used by CI) |
| `make vet` | `go vet ./...` |
| `make lint` | `golangci-lint run ./...` |
| `make test` | `go test ./...` |
| `make test-race` | Race-detector test + coverage profile |
| `make vuln` | `govulncheck ./...` |
| `make ci` | fmt-check + vet + lint + test-race + vuln |
| `make sure` | fmt + vet + test + build + lint |

## Test-Driven Development

New features and bug fixes must include tests. The project follows a TDD
approach:

1. Write a failing test that describes the expected behaviour.
2. Write the minimum code to make it pass.
3. Refactor.

Run the race detector locally before opening a PR: `make test-race`.

## Metric Name / Label Parity Invariant

The exporter supports two collection paths (bulk CSV and per-entity REST — see
[ADR 0004](adr/0004-dual-metrics-paths-auto-detection.md)). A parity test
in the test suite asserts that both paths emit **identical metric names and label
keys** for every entity type.

**Do not break this invariant.** If you add a new metric:

1. Add it to both the bulk-parse path and the per-entity path.
2. Use the shared label-builder function for that entity type.
3. Confirm the parity test passes: `make test`.

## Semgrep Gate

The CI pipeline runs semgrep for security checks. This project policy prohibits
inline suppressions (`//nolint` comments for linter, semgrep `nosec`/`nosemgrep`
annotations). If a finding is a false positive, address it in the project-level
semgrep config rather than suppressing inline.

## Commit Style

Use [Conventional Commits](https://www.conventionalcommits.org/) for commit
messages:

```
<type>(<scope>): <short summary>

[optional body]
[optional footer]
```

Common types: `feat`, `fix`, `docs`, `test`, `refactor`, `ci`, `chore`.

Examples:
```
feat(collector): add port link-status metric
fix(bulk): handle empty CSV response without panic
docs(adr): add ADR 0008 for new design decision
```

## Running the Docker Compose Stack

```bash
# Set at least one array password
export PSTORE1_PASSWORD='your-monitor-password'

# Bring up the full stack (exporter + Prometheus + Grafana + OTEL Collector)
docker compose up --build

# Or use the GHCR-published image without building
docker compose -f docker-compose.ghcr.yml up
```

The exporter metrics are at <http://localhost:9101/metrics>; Grafana at
<http://localhost:3000> (admin / admin by default).

## Pull Request Checklist

Before opening a PR, confirm all of the following:

- [ ] `make ci` passes locally (fmt-check, vet, lint, test-race, govulncheck)
- [ ] New or changed metrics appear in both bulk and per-entity paths (parity invariant)
- [ ] New or changed metrics are documented in `docs/metrics.md`
- [ ] No inline `//nolint` suppressions added
- [ ] Commit messages follow Conventional Commits style
- [ ] If the change is architecturally significant, a new ADR has been added under
      `docs/adr/` and referenced in `docs/adr/README.md`
- [ ] `mkdocs build --strict` passes if docs were changed
