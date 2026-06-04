# Collection Paths

The exporter supports two collection paths and selects the best one automatically for each
configured array at startup. The active path is published via the `powerstore_array_bulk_api`
metric.

## Bulk CSV (PowerStoreOS 4.1+)

On arrays running PowerStoreOS 4.1 or later, the exporter uses the **bulk stats export**
endpoint. This downloads a single compressed tar archive containing CSV files for all
entity types in one API call.

**Advantages:**
- Minimal API load — one request per collection cycle regardless of entity count.
- Higher cardinality — all volumes, appliances, file systems, and ports in a single fetch.
- Lower latency — no sequential per-type requests.

The exporter decompresses the archive in memory, parses all CSV files, and derives the
full metric set from the rows.

`powerstore_array_bulk_api{array="..."} 1` indicates this path is active.

## Per-Entity REST (fallback)

On arrays running older firmware where the bulk export endpoint is unavailable, the
exporter falls back to issuing one REST query per entity type per collection cycle:

1. `GET /performance_metric/query` for volume metrics
2. `GET /performance_metric/query` for appliance metrics
3. `GET /file_system` for file system capacity
4. `GET /eth_port` / `GET /fc_port` for port link state

`powerstore_array_bulk_api{array="..."} 0` indicates this path is active.

## Path detection

The exporter probes the bulk endpoint during the first collection cycle. If the endpoint
responds with a 200 and a valid archive, bulk mode is activated. If it returns 404 or
another error, per-entity mode is used instead. The result is logged at startup:

```
INFO array=pstore-prod collection_path=bulk
INFO array=pstore-dr   collection_path=per_entity
```

There is nothing to configure — the right path is selected automatically. A single
process can simultaneously use both paths across different arrays.

## Interval recommendation

The default `collection.interval` of `30s` works well for both paths. PowerStore
computes 20-second rolling averages internally, so collecting faster than 20s does not
improve resolution. Collecting slower than 60s may cause gaps in time-series graphs.
