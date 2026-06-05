# CLAUDE.md

Dell PowerStore Prometheus + OTLP exporter (Go). Binary `pstore_exporter`, metrics
port 9101, metric prefix `powerstore_`.

## Commands
```bash
make cli          # build bin/pstore_exporter
make test         # go test ./...
make test-race    # go test -race + coverage
make ci           # full gate: fmt-check, vet, lint, test -race, vuln (run before pushing)
make sure         # fmt + vet + test + build + lint (local dev loop)
PSTORE1_PASSWORD=x ./bin/pstore_exporter --config config.yaml --once   # one cycle, no server
docker compose up # exporter:9101 + Prometheus:9090 + Grafana:3000 + otel-collector
make release-snapshot # local goreleaser dry-run: archives + SBOM, no publish/sign/push
```

## Architecture
Background collection loop → immutable `Snapshot` in `SnapshotStore` → dual export
(Prometheus `/metrics` + optional OTLP push). One process, many arrays (each metric
carries an `array` label); per-array failures degrade gracefully (`powerstore_up=0`).
- `main.go` — CLI (cobra), HTTP server, wiring, SIGHUP/file-watch reload.
- `internal/powerstore/` — `client.go` (gopowerstore wrapper), `collector.go` (loop),
  `topology.go` (inventory + label-resolution indices), `perentity.go`/`derive_perentity.go`
  and `bulk.go`/`derive_bulk.go` (the two metric paths), `snapshot.go`, `prometheus.go`, `otlp.go`.
- `internal/models` config, `internal/{utils,logging,telemetry,config}` support.
Decisions are recorded in `docs/adr/`; metric catalog in `docs/metrics.md`.

## Non-obvious constraints (READ BEFORE EDITING)
- **Metric parity invariant:** the bulk and per-entity paths MUST emit identical metric
  names and label KEYS (a test enforces volume parity). Edit both `derive_*.go` together;
  use the shared label builders in `metrics.go`.
- **Values are gauges, not counters:** IOPS are per-second, bandwidth bytes/sec, latency µs.
  Aggregate with `sum`/`avg` in PromQL — NEVER `rate()`.
- **gopowerstore v1.22 limits:** no list-appliances method (enumerate distinct IDs from
  volumes+ports → `GetAppliance`); no version field on appliances (use
  `GetSoftwareMajorMinorVersion` for bulk capability ≥4.1); no drive-list method (enumerate
  via generic `APIClient().Query`). NOTE: `PerformanceMetricsByFileSystem` **does** exist in
  v1.22 — ADR-0003's "no FS perf" note is stale (superseded by ADR-0009). Live FS performance
  is now collected (`derive_filesystem_perf.go`) alongside inventory-derived FS capacity. See
  `docs/adr/0003`, `docs/adr/0009`, and `docs/reconciliation-2026-06-05.md`.
- **Serve before collect:** the HTTP server must start before the first collection cycle —
  gopowerstore's login isn't bounded by the collection timeout, so blocking startup on it
  stalls `/metrics`. See `docs/adr/0007`.
- **Semgrep policy:** no inline `//nolint` or semgrep suppressions; fix the finding.

## Testing
TDD. Offline coverage is at the `Client`-interface mock (`collector_test.go`) + per-derive
unit tests + the pipeline integration test; the live bulk HTTP download is not offline-testable.

## CI / Release
- `ci.yml` (PRs + `main`): `make ci` (lint/vet/test-race/govulncheck) + CycloneDX SBOM
  (GoReleaser snapshot, syft) + Semgrep. Default branch `main`.
- `release.yml` (`v*` tags): one **GoReleaser** job (`.goreleaser.yaml`) → archives,
  per-archive CycloneDX SBOM, multi-arch GHCR image (`dockers_v2`, SBOM+provenance),
  keyless cosign signature, Homebrew cask. Change release behavior in `.goreleaser.yaml`,
  NOT a make loop. `Dockerfile.goreleaser` copies the prebuilt binary; `./Dockerfile` is
  local/dev only. Needs secret `HOMEBREW_TAP_GITHUB_TOKEN`. See `docs/adr/0008`.
- `docs.yml`: GitHub Pages on push to `main` (`docs/**`).
- **Pin Actions to commit SHAs** (`uses: owner/repo@<sha> # vX`), never tags — supply-chain
  policy; bump deliberately. Exception: `semgrep/semgrep` container stays on a rolling tag.

<!-- rtk-instructions v2 -->
# RTK (Rust Token Killer) - Token-Optimized Commands

## Golden Rule

**Always prefix commands with `rtk`**. If RTK has a dedicated filter, it uses it. If not, it passes through unchanged. This means RTK is always safe to use.

**Important**: Even in command chains with `&&`, use `rtk`:
```bash
# ❌ Wrong
git add . && git commit -m "msg" && git push

# ✅ Correct
rtk git add . && rtk git commit -m "msg" && rtk git push
```

## RTK Commands by Workflow

### Build & Compile (80-90% savings)
```bash
rtk cargo build         # Cargo build output
rtk cargo check         # Cargo check output
rtk cargo clippy        # Clippy warnings grouped by file (80%)
rtk tsc                 # TypeScript errors grouped by file/code (83%)
rtk lint                # ESLint/Biome violations grouped (84%)
rtk prettier --check    # Files needing format only (70%)
rtk next build          # Next.js build with route metrics (87%)
```

### Test (60-99% savings)
```bash
rtk cargo test          # Cargo test failures only (90%)
rtk go test             # Go test failures only (90%)
rtk jest                # Jest failures only (99.5%)
rtk vitest              # Vitest failures only (99.5%)
rtk playwright test     # Playwright failures only (94%)
rtk pytest              # Python test failures only (90%)
rtk rake test           # Ruby test failures only (90%)
rtk rspec               # RSpec test failures only (60%)
rtk test <cmd>          # Generic test wrapper - failures only
```

### Git (59-80% savings)
```bash
rtk git status          # Compact status
rtk git log             # Compact log (works with all git flags)
rtk git diff            # Compact diff (80%)
rtk git show            # Compact show (80%)
rtk git add             # Ultra-compact confirmations (59%)
rtk git commit          # Ultra-compact confirmations (59%)
rtk git push            # Ultra-compact confirmations
rtk git pull            # Ultra-compact confirmations
rtk git branch          # Compact branch list
rtk git fetch           # Compact fetch
rtk git stash           # Compact stash
rtk git worktree        # Compact worktree
```

Note: Git passthrough works for ALL subcommands, even those not explicitly listed.

### GitHub (26-87% savings)
```bash
rtk gh pr view <num>    # Compact PR view (87%)
rtk gh pr checks        # Compact PR checks (79%)
rtk gh run list         # Compact workflow runs (82%)
rtk gh issue list       # Compact issue list (80%)
rtk gh api              # Compact API responses (26%)
```

### JavaScript/TypeScript Tooling (70-90% savings)
```bash
rtk pnpm list           # Compact dependency tree (70%)
rtk pnpm outdated       # Compact outdated packages (80%)
rtk pnpm install        # Compact install output (90%)
rtk npm run <script>    # Compact npm script output
rtk npx <cmd>           # Compact npx command output
rtk prisma              # Prisma without ASCII art (88%)
```

### Files & Search (60-75% savings)
```bash
rtk ls <path>           # Tree format, compact (65%)
rtk read <file>         # Code reading with filtering (60%)
rtk grep <pattern>      # Search grouped by file (75%). Format flags (-c, -l, -L, -o, -Z) run raw.
rtk find <pattern>      # Find grouped by directory (70%)
```

### Analysis & Debug (70-90% savings)
```bash
rtk err <cmd>           # Filter errors only from any command
rtk log <file>          # Deduplicated logs with counts
rtk json <file>         # JSON structure without values
rtk deps                # Dependency overview
rtk env                 # Environment variables compact
rtk summary <cmd>       # Smart summary of command output
rtk diff                # Ultra-compact diffs
```

### Infrastructure (85% savings)
```bash
rtk docker ps           # Compact container list
rtk docker images       # Compact image list
rtk docker logs <c>     # Deduplicated logs
rtk kubectl get         # Compact resource list
rtk kubectl logs        # Deduplicated pod logs
```

### Network (65-70% savings)
```bash
rtk curl <url>          # Compact HTTP responses (70%)
rtk wget <url>          # Compact download output (65%)
```

### Meta Commands
```bash
rtk gain                # View token savings statistics
rtk gain --history      # View command history with savings
rtk discover            # Analyze Claude Code sessions for missed RTK usage
rtk proxy <cmd>         # Run command without filtering (for debugging)
rtk init                # Add RTK instructions to CLAUDE.md
rtk init --global       # Add RTK to ~/.claude/CLAUDE.md
```

## Token Savings Overview

| Category | Commands | Typical Savings |
|----------|----------|-----------------|
| Tests | vitest, playwright, cargo test | 90-99% |
| Build | next, tsc, lint, prettier | 70-87% |
| Git | status, log, diff, add, commit | 59-80% |
| GitHub | gh pr, gh run, gh issue | 26-87% |
| Package Managers | pnpm, npm, npx | 70-90% |
| Files | ls, read, grep, find | 60-75% |
| Infrastructure | docker, kubectl | 85% |
| Network | curl, wget | 65-70% |

Overall average: **60-90% token reduction** on common development operations.
<!-- /rtk-instructions -->