# CI/CD & SBOM

Every check CI runs is a Makefile target, so it reproduces locally. Releases are
driven by [GoReleaser](https://goreleaser.com) — see
[ADR 0008](adr/0008-goreleaser-sha-pinning-and-signing.md) for the rationale.

## Supply-chain hardening

All GitHub Actions (first- and third-party) are pinned to **full commit SHAs**
with a `# vX` comment, so a moved or compromised tag cannot silently change what
runs. Bump them like dependencies (e.g. Dependabot/Renovate keyed on the comment).
The `semgrep/semgrep` scanner container is intentionally left on a rolling tag so
it runs the latest engine and rules.

## CI (`.github/workflows/ci.yml`)

Runs on push to `main` and on pull requests:

- **quality** — `make ci`: `gofmt` check, `go vet`, `golangci-lint`, `go test -race`
  with coverage, and `govulncheck`. Coverage is uploaded as an artifact.
- **sbom** — `goreleaser release --snapshot` (no publish/sign/image): produces the
  same CycloneDX SBOMs as a real release and uploads `dist/*.cdx.json`.
- **semgrep** — a static security scan (`semgrep scan --config auto --error`).

```bash
make tools   # install pinned golangci-lint, govulncheck
make ci      # the same gate CI runs
make sbom    # CycloneDX SBOMs in dist/ (needs goreleaser + syft)
```

## Release (`.github/workflows/release.yml`)

Runs on `v*` tags as a single GoReleaser job that produces:

- **Binaries** — `linux/{amd64,arm64}` and `darwin/{amd64,arm64}` `.tar.gz`
  archives plus `checksums.txt`, attached to a GitHub Release.
- **CycloneDX SBOM** — one `*.cdx.json` per archive (syft).
- **Container image** — multi-arch `ghcr.io/fjacquet/pstore_exporter` built with
  buildx, carrying **SBOM and provenance attestations**.
- **Signatures** — keyless **cosign** signature of `checksums.txt`
  (`checksums.txt.sigstore.json`), via GitHub OIDC.
- **Homebrew cask** — published to `fjacquet/homebrew-tap`.

```bash
git tag v0.1.0 && git push origin v0.1.0   # triggers the release
make release-snapshot                       # local dry-run, no publish/sign/push
```

### Required secrets

- `GITHUB_TOKEN` — automatic; creates the Release and pushes the GHCR image.
- `HOMEBREW_TAP_GITHUB_TOKEN` — a PAT with `contents:write` on
  `fjacquet/homebrew-tap` (the default token cannot push to another repo).

## SBOM

GoReleaser is the single SBOM source (syft):

- **Release SBOM** — a CycloneDX `*.cdx.json` per archive, attached to the release.
- **Image SBOM** — `dockers_v2` (`sbom: true`) attaches an SBOM and provenance
  attestation to the pushed container image.

Verify a downloaded release:

```bash
cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt   # signature
sha256sum -c checksums.txt                                              # integrity
```

## Versioning

The build version is injected via `-ldflags "-X main.version={{ .Version }}"`,
derived by GoReleaser from the git tag. Check it with:

```bash
pstore_exporter --version
```

## Documentation

This site is built with MkDocs Material and deployed to GitHub Pages by
`.github/workflows/docs.yml` on every push to `main`.

```bash
uvx --with mkdocs-material --with pymdown-extensions mkdocs serve   # preview locally
```
