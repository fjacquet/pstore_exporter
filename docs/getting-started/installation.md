# Installation

## Prerequisites

- A reachable PowerStore array — PowerStoreOS 3.x or later.
- A PowerStore user with **monitor** (read-only) privileges.
- One of: Go 1.25+ toolchain (build from source), Docker, or a Kubernetes cluster.

## From a release binary

Download the binary for your platform from the
[releases page](https://github.com/fjacquet/pstore_exporter/releases) and verify it
against `checksums.txt`:

```bash
sha256sum -c checksums.txt --ignore-missing
chmod +x pstore_exporter_*_linux_amd64
sudo install pstore_exporter_*_linux_amd64 /usr/local/bin/pstore_exporter
pstore_exporter --version
```

Each release also ships a CycloneDX SBOM (`sbom.cdx.json`).

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
