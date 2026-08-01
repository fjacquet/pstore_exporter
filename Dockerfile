# Stage 1: Build
FROM golang:1.26.5 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o pstore_exporter .

# Stage 2: Runtime
FROM alpine:latest

# Create the runtime user and log dir. These are busybox builtins (no network).
RUN adduser -D -u 10001 pstore && \
    mkdir -p /var/log/pstore_exporter && \
    chown pstore:pstore /var/log/pstore_exporter

# Copy the CA bundle from the builder stage instead of `apk add ca-certificates`.
# The latter fetches from the Alpine CDN over TLS, which fails behind a corporate
# MITM proxy: the bare alpine image has no CA bundle yet to validate the proxy
# cert (chicken-and-egg). The Debian-based golang builder already ships the bundle.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /app/pstore_exporter /usr/bin/pstore_exporter
COPY config.yaml /etc/pstore_exporter/config.yaml

EXPOSE 9446

# /livez never depends on target reachability or the collection cycle, so it
# can never flag a healthy process as down over an unreachable PowerStore
# array (see ADR-0017 / architecture.md "Health probes").
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:9446/livez || exit 1

USER pstore

ENTRYPOINT ["/usr/bin/pstore_exporter"]
CMD ["--config", "/etc/pstore_exporter/config.yaml"]
