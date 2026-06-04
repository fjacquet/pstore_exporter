# 3. Use gopowerstore Client

## Status

Accepted

## Context

The PowerStore REST API requires login-session token/cookie authentication and
returns complex typed responses for volumes, appliances, ports, and performance
metrics. Hand-rolling an HTTP client with session management, retry logic, and
response deserialization would be significant work and would need to track Dell's
API changes independently.

Dell maintains the open-source `gopowerstore` Go client library, which handles
authentication and provides typed request/response structs for most entity types.

## Decision

Use Dell's `github.com/dell/gopowerstore` at v1.22.0 as the API client rather
than implementing a custom HTTP/auth layer. The client handles login-session
token/cookie lifecycle and provides typed responses for volumes, appliances,
ports, and per-entity performance metrics.

## Consequences

**Benefits:**

- Significantly less authentication and deserialization code to maintain.
- Dell keeps the client in sync with PowerStore API changes.
- Typed responses reduce the risk of silent JSON unmarshalling errors.

**Documented workarounds** (be honest about gaps):

- `gopowerstore` does not expose a list-appliances method; we enumerate distinct
  appliance IDs from volumes and ports, then call `GetAppliance` for each.
- The appliance object exposed by the client does not include the software/OS
  version; we call `GetSoftwareMajorMinorVersion` separately for capability
  detection (bulk vs. per-entity path selection).
- There is no `PerformanceMetricsByFileSystem` method; we emit file-system
  capacity metrics from inventory data (`GetFileSystem`) rather than live
  performance counters.
- The client does not wrap the bulk compressed-CSV API endpoint; we use a thin
  authenticated HTTP client (sharing the session cookie) for that path.

Overall the trade-off is favorable: less code, better maintainability, at the
cost of these documented workarounds which are stable given the API surface.
