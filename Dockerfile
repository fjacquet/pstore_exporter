# Stage 1: Build
FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o pstore_exporter .

# Stage 2: Runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates && \
    adduser -D -u 10001 pstore && \
    mkdir -p /var/log/pstore_exporter && \
    chown pstore:pstore /var/log/pstore_exporter

COPY --from=builder /app/pstore_exporter /usr/bin/pstore_exporter
COPY config.yaml /etc/pstore_exporter/config.yaml

EXPOSE 9101

USER pstore

ENTRYPOINT ["/usr/bin/pstore_exporter"]
CMD ["--config", "/etc/pstore_exporter/config.yaml"]
