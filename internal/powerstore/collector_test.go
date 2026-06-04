package powerstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockClient struct {
	name     string
	topoErr  error
	bulk     bool
	perEnt   []Sample
	bulkSamp []Sample
}

func (m *mockClient) Name() string { return m.name }
func (m *mockClient) GetTopology(context.Context) (*Topology, error) {
	if m.topoErr != nil {
		return nil, m.topoErr
	}
	return &Topology{}, nil
}
func (m *mockClient) BulkCapable(context.Context, *Topology) bool                   { return m.bulk }
func (m *mockClient) PerEntityMetrics(context.Context, *Topology) ([]Sample, error) { return m.perEnt, nil }
func (m *mockClient) BulkMetrics(context.Context, *Topology) ([]Sample, error)      { return m.bulkSamp, nil }
func (m *mockClient) Close() error                                                  { return nil }

func TestCollectOnceGracefulDegradation(t *testing.T) {
	good := &mockClient{name: "ok", bulk: false, perEnt: []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "ok"}}, Value: 1}}}
	bad := &mockClient{name: "down", topoErr: errors.New("unreachable")}
	store := NewSnapshotStore()
	c := NewCollector([]Client{good, bad}, store, time.Second, time.Second, nil)
	snap := c.CollectOnce(context.Background())

	if !snap.PerArray["ok"].Up {
		t.Fatal("expected ok array up")
	}
	if snap.PerArray["down"].Up {
		t.Fatal("expected down array not up")
	}
	if snap.PerArray["down"].ScrapeError == "" {
		t.Fatal("expected scrape error recorded for down array")
	}
}

func TestCollectChoosesBulkWhenCapable(t *testing.T) {
	m := &mockClient{name: "p1", bulk: true,
		bulkSamp: []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 7}},
		perEnt:   []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 99}}}
	store := NewSnapshotStore()
	c := NewCollector([]Client{m}, store, time.Second, time.Second, nil)
	snap := c.CollectOnce(context.Background())
	got := snap.SamplesByName("powerstore_volume_read_iops")
	if len(got) != 1 || got[0].Value != 7 {
		t.Fatalf("expected bulk value 7, got %+v", got)
	}
	if !snap.PerArray["p1"].BulkCapable {
		t.Fatal("expected BulkCapable true")
	}
}
