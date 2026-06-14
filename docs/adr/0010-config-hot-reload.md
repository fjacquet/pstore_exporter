# 10. Configuration Hot-Reload via SIGHUP and File-Watch

## Status

Accepted

## Context

The exporter is a long-running process that monitors many arrays. Operators
need to add, remove, or re-credential arrays without restarting the process and
dropping the `/metrics` endpoint — a restart blanks all series and, against a
slow array, incurs the startup login delay described in ADR-0007.

Two reload triggers are common in this space: an explicit `SIGHUP` (the
traditional daemon convention, easy to wire into config-management tools) and an
automatic file-watch (convenient for Kubernetes ConfigMap updates and local
edits). Reloading configuration concurrently with a running collection loop
raises a correctness concern: the client pool, the collector, and the OTLP
instrument set must all be swapped without tearing a snapshot or leaking
clients.

## Decision

Configuration is reloadable at runtime through two triggers, both routing to the
same `Server.ReloadConfig` entry point:

- **SIGHUP** — `config.SetupSIGHUPHandler` installs a handler that reloads on
  signal.
- **File-watch** — `config.WatchConfigFile` watches the *directory* containing
  the config file (not the file inode) so atomic-rename writes and editor
  swap-file behavior are detected. File-watch failure is non-fatal; SIGHUP
  remains available and is logged as the fallback.

Reload is change-gated and minimally disruptive:

- `SafeConfig.ReloadConfig` re-parses the file and reports whether the **array
  set** changed. If it did not, reload is a no-op — scrapes are never disturbed
  by unrelated edits.
- When the array set changes, a new client pool is built, swapped under a mutex
  (`Server.mu`), and handed to the running collector via `collector.SetClients`.
  The *old* clients are closed only after the swap. The tracing transport
  survives the rebuild so `--trace` keeps working across reloads.
- New metric names introduced by the new array set get OTLP instruments
  registered via `EnsureInstruments` (see ADR-0011).

The collection loop and the immutable snapshot store (ADR-0002) are never
restarted by a reload; only the client pool is replaced.

## Consequences

- Arrays can be added, removed, or re-credentialed with `kill -HUP` or a
  ConfigMap update — no process restart, no gap in `/metrics`.
- Watching the directory rather than the file makes the watcher robust to the
  atomic-rename update pattern used by Kubernetes and most editors.
- Reload is bounded to array-set changes; edits that do not change the array set
  are intentionally ignored to avoid needless client churn. Changing *only* a
  password therefore requires the secret source to change, not just the file —
  operators relying on credential rotation should rotate the referenced env/file
  (see ADR-0012), or change the array set to force a rebuild.
- Client lifecycle is mutex-guarded and close-after-swap, preventing both torn
  reads and leaked connections during reload.
