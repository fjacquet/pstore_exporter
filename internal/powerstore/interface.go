package powerstore

import "context"

// Client is the per-array PowerStore API abstraction. Satisfied by ArrayClient and
// mocked in tests so the collector can run without a live array.
type Client interface {
	// Name returns the configured array name (the `array` label value).
	Name() string
	// GetTopology fetches the array's inventory and builds lookup indices.
	GetTopology(ctx context.Context) (*Topology, error)
	// BulkCapable reports whether the array supports the bulk CSV metrics API.
	BulkCapable(ctx context.Context, topo *Topology) bool
	// PerEntityMetrics collects metrics one entity at a time via the typed client.
	PerEntityMetrics(ctx context.Context, topo *Topology) ([]Sample, error)
	// BulkMetrics collects metrics via the bulk CSV API.
	BulkMetrics(ctx context.Context, topo *Topology) ([]Sample, error)
	// Close releases client resources.
	Close() error
}
