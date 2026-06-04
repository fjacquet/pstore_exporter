package powerstore

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestOTLPExporterEnsureInstruments(t *testing.T) {
	// Build a SnapshotStore with one ArraySnapshot containing one Sample.
	store := NewSnapshotStore()
	as := &ArraySnapshot{
		Array: "p1",
		Up:    true,
		Samples: []Sample{
			{
				Name:   "powerstore_volume_read_iops",
				Labels: []Label{{"array", "p1"}},
				Value:  42,
			},
		},
	}
	store.Store(BuildSnapshot([]*ArraySnapshot{as}))

	// Use a ManualReader so we can collect without a live gRPC endpoint.
	reader := sdkmetric.NewManualReader()
	exp := newOTLPExporter(reader, store, "test")

	// Register instruments for all metric names in the snapshot.
	if err := exp.EnsureInstruments(); err != nil {
		t.Fatalf("EnsureInstruments: %v", err)
	}

	// Collect via the ManualReader.
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("reader.Collect: %v", err)
	}

	// Find the expected gauge in the collected data.
	var found bool
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "powerstore_volume_read_iops" {
				continue
			}
			gauge, ok := m.Data.(metricdata.Gauge[float64])
			if !ok {
				t.Fatalf("expected Gauge[float64], got %T", m.Data)
			}
			for _, dp := range gauge.DataPoints {
				if dp.Value != 42 {
					t.Fatalf("expected value 42, got %v", dp.Value)
				}
				// Verify array=p1 attribute is present.
				val, ok := dp.Attributes.Value("array")
				if !ok {
					t.Fatal("expected attribute 'array' to be present")
				}
				if val.AsString() != "p1" {
					t.Fatalf("expected array=p1, got %v", val.AsString())
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatal("metric powerstore_volume_read_iops with value 42 and array=p1 not found in collected data")
	}
}
