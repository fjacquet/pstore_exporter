# Installation

## Prerequisites

- A reachable PowerStore array — PowerStoreOS 3.x or later.
- A PowerStore user with **monitor** (read-only) privileges.
- One of: Go 1.25+ toolchain (build from source), Docker, or a Kubernetes cluster.

## With Homebrew

```bash
brew install fjacquet/tap/pstore_exporter
pstore_exporter --version
```

## From a release archive

Download the `.tar.gz` for your platform from the
[releases page](https://github.com/fjacquet/pstore_exporter/releases). Each release
is signed with keyless cosign — verify the checksums, then extract:

```bash
cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt   # optional
sha256sum -c checksums.txt --ignore-missing
tar -xzf pstore_exporter_*_linux_amd64.tar.gz
sudo install pstore_exporter /usr/local/bin/pstore_exporter
pstore_exporter --version
```

Each archive also ships a CycloneDX SBOM (`<archive>.cdx.json`).

## From source

```bash
git clone https://github.com/fjacquet/pstore_exporter
cd pstore_exporter
make cli            # -> bin/pstore_exporter
```

## Container image

Multi-arch images (`linux/amd64`, `linux/arm64`) are published to GHCR with SBOM and
provenance attestations:

```bash
docker pull ghcr.io/fjacquet/pstore_exporter:0.1.0   # or :latest
```

## Next steps

- [Configure](configuration.md) your arrays and exporters.
- [Quick Start](quickstart.md) to run it.
- Deploy via [Docker](../deployment/docker.md), [systemd](../deployment/systemd.md) or [Kubernetes](../deployment/kubernetes.md).
