BIN     = pstore_exporter
DIST    = dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)

# Skip flags for GoReleaser snapshot runs (no publish, no signing, no image push,
# no Homebrew upload) — used for local SBOM generation and dry-run releases.
GORELEASER_SNAPSHOT_SKIP = publish,sign,docker,homebrew

# Pinned tool versions (installed by `make tools`).
GOLANGCI_LINT_VERSION ?= v2.12.2
GOVULNCHECK_VERSION   ?= latest

all: cli test docker

# Install pinned dev/CI tooling into $(GOBIN)/$GOPATH/bin.
# Release tooling (goreleaser, syft, cosign) is provided by CI actions; install
# locally with Homebrew: `brew install goreleaser syft cosign`.
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

# --- quality gates (used by CI) ---

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed in:"; gofmt -l .; exit 1)

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test ./...

test-race:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

test-coverage: test-race
	go tool cover -html=coverage.out -o coverage.html

vuln:
	govulncheck ./...

# Aggregate gate run by CI.
ci: fmt-check vet lint test-race vuln

# Local convenience: format, vet, test, build, lint.
sure: fmt vet test
	go build ./...
	golangci-lint run

# --- artifacts ---

cli:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BIN) .

# CycloneDX SBOMs via GoReleaser (syft). Runs the release pipeline in snapshot
# mode without publishing/signing/image, leaving dist/*.cdx.json — the same SBOMs
# attached to real releases. Requires goreleaser + syft on PATH.
sbom: release-snapshot

# Full release: cross-platform binaries, CycloneDX SBOMs, multi-arch GHCR image
# (with SBOM + provenance attestations), cosign signatures, and a Homebrew cask.
# Normally run by CI on a v* tag — see .github/workflows/release.yml.
release:
	goreleaser release --clean

# Local dry-run of the full pipeline: builds, archives, and SBOMs in $(DIST)/
# without publishing, signing, pushing the image, or uploading the Homebrew cask.
release-snapshot:
	goreleaser release --snapshot --clean --skip=$(GORELEASER_SNAPSHOT_SKIP)

docker:
	docker build -t $(BIN):$(VERSION) -t $(BIN):latest .

run-cli: cli
	./bin/$(BIN) --config config.yaml

clean-dist:
	rm -rf $(DIST)

clean: clean-dist
	rm -f bin/$(BIN) coverage.out coverage.html

.PHONY: all tools fmt-check fmt vet lint test test-race test-coverage vuln ci sure \
        cli sbom release release-snapshot docker run-cli clean-dist clean
