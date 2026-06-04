package powerstore

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// Collector runs the background collection loop: every interval it polls all arrays in
// parallel and publishes a fresh Snapshot. One array's failure does not affect others.
type Collector struct {
	clients  []Client
	store    *SnapshotStore
	interval time.Duration
	timeout  time.Duration
	tracing  *TracerWrapper
}

// NewCollector creates a collection loop over the given per-array clients.
func NewCollector(clients []Client, store *SnapshotStore, interval, timeout time.Duration, tp trace.TracerProvider) *Collector {
	return &Collector{
		clients:  clients,
		store:    store,
		interval: interval,
		timeout:  timeout,
		tracing:  NewTracerWrapper(tp, "pstore-exporter/collector"),
	}
}

// CollectOnce runs a single cycle and publishes the result.
func (c *Collector) CollectOnce(ctx context.Context) *Snapshot {
	snap := c.collectAll(ctx)
	c.store.Store(snap)
	return snap
}

// Run drives the loop until ctx is cancelled (assumes an initial CollectOnce).
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.store.Store(c.collectAll(ctx))
		}
	}
}

func (c *Collector) collectAll(ctx context.Context) *Snapshot {
	ctx, span := c.tracing.StartSpan(ctx, "collect.cycle", trace.SpanKindInternal)
	defer span.End()

	results := make([]*ArraySnapshot, len(c.clients))
	g, gctx := errgroup.WithContext(ctx)
	for i, client := range c.clients {
		i, client := i, client
		g.Go(func() error {
			results[i] = c.collectArray(gctx, client)
			return nil // graceful degradation
		})
	}
	_ = g.Wait()
	return BuildSnapshot(results)
}

func (c *Collector) collectArray(ctx context.Context, client Client) *ArraySnapshot {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	as := &ArraySnapshot{Array: client.Name(), LastScrape: time.Now()}

	topo, err := client.GetTopology(ctx)
	if err != nil {
		log.Warnf("array %q: topology fetch failed: %v", client.Name(), err)
		as.ScrapeError = err.Error()
		return as
	}

	as.BulkCapable = client.BulkCapable(ctx, topo)
	var samples []Sample
	if as.BulkCapable {
		samples, err = client.BulkMetrics(ctx, topo)
		if err != nil {
			log.Warnf("array %q: bulk metrics failed, falling back to per-entity: %v", client.Name(), err)
			samples, err = client.PerEntityMetrics(ctx, topo)
		}
	} else {
		samples, err = client.PerEntityMetrics(ctx, topo)
	}
	if err != nil {
		as.ScrapeError = err.Error()
		return as
	}
	as.Samples = samples
	as.Up = true
	return as
}
