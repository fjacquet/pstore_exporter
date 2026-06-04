# 7. Serve HTTP Before First Collection

## Status

Accepted

## Context

The `gopowerstore` client constructor performs a login request to the PowerStore
array as part of initialization. Against an unreachable or slow array this login
is not bounded by the per-array collection timeout — in practice this caused the
exporter to block for approximately 80 seconds before the HTTP server started
listening, meaning `/metrics` and `/health` were unavailable during that window.

This is particularly problematic in Kubernetes, where a slow-starting pod fails
liveness/readiness probes and gets restarted in a loop, preventing the exporter
from ever reaching a healthy state against a slow array.

## Decision

The HTTP server starts listening on the metrics port before any array client
construction or collection cycle runs. Client initialization and the first
collection cycle execute in a background goroutine. The snapshot store is
seeded with an empty (but valid) initial state so that `/metrics` can respond
immediately with an empty metric set. The `/health` endpoint returns HTTP 200
with body `"starting"` until the first collection cycle completes successfully,
then transitions to `"ok"`.

## Consequences

- `/metrics` and `/health` are available in under 1 second of process startup,
  regardless of array reachability.
- Kubernetes liveness and readiness probes function correctly even against slow
  or unreachable arrays.
- A scrape during the startup window returns an empty metric set rather than a
  connection error; monitoring systems should be configured with an appropriate
  `for` duration on absence-based alerts.
- This pattern is a general principle: a monitoring exporter must never block
  its own scrape endpoint on a slow or unavailable backend.
