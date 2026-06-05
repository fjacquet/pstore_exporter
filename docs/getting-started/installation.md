# Installation

## Prerequisites

- A reachable PowerStore array — PowerStoreOS 3.x or later.
- A PowerStore user with **monitor** (read-only) privileges.
- One of: Go 1.25+ toolchain (build from source), Docker, or a Kubernetes cluster.

## With Homebrew

The exporter is distributed as a Homebrew cask from the `fjacquet/tap` tap (works on
macOS and Linuxbrew). Installing auto-taps, so a single command is enough:

```bash
brew install fjacquet/tap/pstore_exporter
pstore_exporter --version
```

Equivalently, tap first and then install by short name:

```bash
brew tap fjacquet/tap
brew install pstore_exporter
```

The cask installs only the `pstore_exporter` binary onto your `PATH` (it does **not**
ship a config). Provide your own `config.yaml` and run it:

```bash
# grab the sample config to start from
curl -fsSLO https://raw.githubusercontent.com/fjacquet/pstore_exporter/main/config.yaml
$EDITOR config.yaml                                  # set your array hostname(s)/user
export PSTORE1_PASSWORD='your-monitor-password'      # password via env, not the file
pstore_exporter --config config.yaml
# metrics: http://localhost:9101/metrics   health: http://localhost:9101/health
```

See [Configuration](configuration.md) for the full config reference. Upgrade or
remove with the usual Homebrew commands:

```bash
brew upgrade pstore_exporter      # to the latest tagged release
brew uninstall pstore_exporter    # remove
brew untap fjacquet/tap           # (optional) drop the tap entirely
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
