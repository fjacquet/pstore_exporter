// Package powerstore integration tests.
//
// Part 1 (this file): end-to-end pipeline test — mockClient → Collector →
// SnapshotStore → PromCollector → prometheus.Registry.  Runs fully offline.
//
// Part 2 (gopowerstore HTTP-level): skipped. The gopowerstore client uses an
// opaque login_session cookie/token flow with a per-call context lock and
// internal retry logic that makes it impractical to reliably fake with
// httptest.  Offline coverage is provided at the Client-interface level via
// mockClient (see collector_test.go), plus unit tests in
// derive_perentity_test.go and derive_bulk_test.go that exercise every
// metric-derivation function independently.  Live/manual integration testing
// against a real array or the PowerStore simulator is the intended mechanism
// for validating the HTTP glue in client.go.
package powerstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

// gatherValue scans a slice of MetricFamily values, finds the first metric
// whose label set contains labelName=labelValue, and returns its gauge value.
// ok is false if no matching metric was found.
func gatherValue(mfs []*dto.MetricFamily, metricName, labelName, labelValue string) (float64, bool) {
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == labelName && lp.GetValue() == labelValue {
					return m.GetGauge().GetValue(), true
				}
			}
		}
	}
	return 0, false
}

// hasFamilyWithLabel returns true when metricName is present and has at least
// one metric with labelName=labelValue.
func hasFamilyWithLabel(mfs []*dto.MetricFamily, metricName, labelName, labelValue string) bool {
	_, ok := gatherValue(mfs, metricName, labelName, labelValue)
	return ok
}

// TestIntegrationPipeline is the end-to-end pipeline integration test:
// mockClient → NewCollector.CollectOnce → SnapshotStore → NewPromCollector →
// prometheus.Registry.Gather.  It requires no network access.
func TestIntegrationPipeline(t *testing.T) {
	t.Parallel()

	const (
		healthyArray = "ps-healthy"
		downArray    = "ps-down"
	)

	// Two realistic per-entity samples for the healthy array: volume IOPS and
	// appliance space, covering two different entity types.
	healthySamples := []Sample{
		{
			Name: "powerstore_volume_read_iops",
			Labels: []Label{
				{"array", healthyArray},
				{"volume_name", "vol-1"},
				{"volume_id", "v-001"},
			},
			Value: 123.5,
		},
		{
			Name: "powerstore_volume_write_iops",
			Labels: []Label{
				{"array", healthyArray},
				{"volume_name", "vol-1"},
				{"volume_id", "v-001"},
			},
			Value: 45.0,
		},
		{
			Name: "powerstore_appliance_space_total_bytes",
			Labels: []Label{
				{"array", healthyArray},
				{"appliance_name", "appl-A"},
				{"appliance_id", "a-001"},
			},
			Value: 1_099_511_627_776, // 1 TiB
		},
	}

	// Healthy client: per-entity path (bulk == false).
	healthy := &mockClient{
		name:   healthyArray,
		bulk:   false,
		perEnt: healthySamples,
	}

	// Down client: topology fetch returns an error (simulates unreachable array).
	down := &mockClient{
		name:    downArray,
		topoErr: fmt.Errorf("connection refused"),
	}

	// Wire up the collection pipeline.
	store := NewSnapshotStore()
	col := NewCollector([]Client{healthy, down}, store, time.Second, 5*time.Second, nil)
	snap := col.CollectOnce(context.Background())

	// ── Snapshot-level assertions ────────────────────────────────────────────

	t.Run("snapshot/healthy_array_is_up", func(t *testing.T) {
		as, ok := snap.PerArray[healthyArray]
		if !ok {
			t.Fatalf("healthy array %q missing from snapshot", healthyArray)
		}
		if !as.Up {
			t.Fatalf("healthy array: Up = false, ScrapeError = %q", as.ScrapeError)
		}
	})

	t.Run("snapshot/down_array_is_not_up", func(t *testing.T) {
		as, ok := snap.PerArray[downArray]
		if !ok {
			t.Fatalf("down array %q missing from snapshot", downArray)
		}
		if as.Up {
			t.Fatal("down array: Up = true, want false")
		}
		if as.ScrapeError == "" {
			t.Fatal("down array: ScrapeError is empty, want non-empty error message")
		}
	})

	t.Run("snapshot/healthy_read_iops_sample_present", func(t *testing.T) {
		got := snap.SamplesByName("powerstore_volume_read_iops")
		if len(got) == 0 {
			t.Fatal("no powerstore_volume_read_iops samples in snapshot")
		}
		var found bool
		for _, s := range got {
			for _, l := range s.Labels {
				if l.Name == "array" && l.Value == healthyArray {
					found = true
					if s.Value != 123.5 {
						t.Fatalf("read_iops value: want 123.5, got %v", s.Value)
					}
				}
			}
		}
		if !found {
			t.Fatalf("no read_iops sample with array=%q", healthyArray)
		}
	})

	// ── Prometheus-layer assertions ──────────────────────────────────────────

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewPromCollector(store))

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("reg.Gather: %v", err)
	}

	t.Run("prom/powerstore_up_healthy_equals_1", func(t *testing.T) {
		v, ok := gatherValue(mfs, "powerstore_up", "array", healthyArray)
		if !ok {
			t.Fatalf("powerstore_up{array=%q} not found", healthyArray)
		}
		if v != 1.0 {
			t.Fatalf("powerstore_up{array=%q} = %v, want 1", healthyArray, v)
		}
	})

	t.Run("prom/powerstore_up_down_equals_0", func(t *testing.T) {
		v, ok := gatherValue(mfs, "powerstore_up", "array", downArray)
		if !ok {
			t.Fatalf("powerstore_up{array=%q} not found", downArray)
		}
		if v != 0.0 {
			t.Fatalf("powerstore_up{array=%q} = %v, want 0", downArray, v)
		}
	})

	t.Run("prom/bulk_api_emitted_for_healthy_array", func(t *testing.T) {
		if !hasFamilyWithLabel(mfs, "powerstore_array_bulk_api", "array", healthyArray) {
			t.Fatalf("powerstore_array_bulk_api{array=%q} not found", healthyArray)
		}
	})

	t.Run("prom/bulk_api_emitted_for_down_array", func(t *testing.T) {
		if !hasFamilyWithLabel(mfs, "powerstore_array_bulk_api", "array", downArray) {
			t.Fatalf("powerstore_array_bulk_api{array=%q} not found", downArray)
		}
	})

	t.Run("prom/volume_read_iops_correct_value", func(t *testing.T) {
		v, ok := gatherValue(mfs, "powerstore_volume_read_iops", "array", healthyArray)
		if !ok {
			t.Fatalf("powerstore_volume_read_iops{array=%q} not found", healthyArray)
		}
		if v != 123.5 {
			t.Fatalf("powerstore_volume_read_iops{array=%q} = %v, want 123.5", healthyArray, v)
		}
	})

	t.Run("prom/appliance_space_sample_present", func(t *testing.T) {
		if !hasFamilyWithLabel(mfs, "powerstore_appliance_space_total_bytes", "array", healthyArray) {
			t.Fatalf("powerstore_appliance_space_total_bytes{array=%q} not found", healthyArray)
		}
	})
}
