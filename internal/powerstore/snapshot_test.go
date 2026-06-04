package powerstore

import "testing"

func TestBuildSnapshotIndexesByName(t *testing.T) {
	cs := &ArraySnapshot{Array: "p1", Up: true, Samples: []Sample{
		{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 10},
		{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 20},
		{Name: "powerstore_volume_write_iops", Labels: []Label{{"array", "p1"}}, Value: 5},
	}}
	snap := BuildSnapshot([]*ArraySnapshot{cs, nil})
	if got := len(snap.SamplesByName("powerstore_volume_read_iops")); got != 2 {
		t.Fatalf("want 2 read_iops samples, got %d", got)
	}
	if len(snap.MetricNames()) != 2 {
		t.Fatalf("want 2 metric names, got %d", len(snap.MetricNames()))
	}
}

func TestSnapshotStoreLoadStore(t *testing.T) {
	st := NewSnapshotStore()
	if st.Load() == nil {
		t.Fatal("expected non-nil seed snapshot")
	}
	st.Store(BuildSnapshot([]*ArraySnapshot{{Array: "p1", Up: true}}))
	if _, ok := st.Load().PerArray["p1"]; !ok {
		t.Fatal("expected p1 in stored snapshot")
	}
}
