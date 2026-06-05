package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func i64(v int64) *int64 { return &v }

func TestDeriveClusterSpaceLatestSample(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)

	var older, newer gopowerstore.SpaceMetricsByClusterResponse
	older.PhysicalUsed = i64(100)
	newer.PhysicalTotal = i64(10_000)
	newer.PhysicalUsed = i64(4_000)
	newer.LogicalProvisioned = i64(50_000)
	newer.LogicalUsed = i64(20_000)
	newer.DataReduction = 3.5
	newer.EfficiencyRatio = 8

	got := deriveClusterSpace("p1", topo, []gopowerstore.SpaceMetricsByClusterResponse{older, newer})

	if !hasSample(got, "powerstore_cluster_physical_total_bytes", 10_000) {
		t.Fatalf("physical total: want latest 10000, got %+v", got)
	}
	if !hasSample(got, "powerstore_cluster_physical_used_bytes", 4_000) {
		t.Fatalf("physical used: want 4000")
	}
	if !hasSample(got, "powerstore_cluster_logical_provisioned_bytes", 50_000) {
		t.Fatalf("logical provisioned: want 50000")
	}
	if !hasSample(got, "powerstore_cluster_logical_used_bytes", 20_000) {
		t.Fatalf("logical used: want 20000")
	}
	if !hasSample(got, "powerstore_cluster_data_reduction_ratio", 3.5) {
		t.Fatalf("data reduction: want 3.5")
	}
	if !hasSample(got, "powerstore_cluster_efficiency_ratio", 8) {
		t.Fatalf("efficiency ratio: want 8")
	}
	if _, ok := sampleByLabel(got, "powerstore_cluster_physical_total_bytes", "cluster_id", "c1"); !ok {
		t.Fatal("missing cluster_id label")
	}
}

func TestDeriveClusterSpaceEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	got := deriveClusterSpace("p1", topo, nil)
	if len(got) != 0 {
		t.Fatalf("no samples should emit nothing, got %+v", got)
	}
}
