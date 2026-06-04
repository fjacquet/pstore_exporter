package powerstore

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPromCollectorEmitsUpAndSamples(t *testing.T) {
	store := NewSnapshotStore()
	store.Store(BuildSnapshot([]*ArraySnapshot{{
		Array: "p1", Up: true, BulkCapable: true,
		Samples: []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 42}},
	}}))
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewPromCollector(store))

	if c := testutil.CollectAndCount(NewPromCollector(store), "powerstore_volume_read_iops"); c != 1 {
		t.Fatalf("want 1 read_iops series, got %d", c)
	}
	mf, _ := reg.Gather()
	var sawUp bool
	for _, m := range mf {
		if strings.HasSuffix(m.GetName(), "powerstore_up") {
			sawUp = true
		}
	}
	if !sawUp {
		t.Fatal("expected powerstore_up metric")
	}
}
