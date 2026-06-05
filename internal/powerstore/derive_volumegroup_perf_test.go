package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestDeriveVolumeGroupPerfLatestSample(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "c1"}, nil, nil,
		[]gopowerstore.VolumeGroup{{ID: "vg-1", Name: "vgA"}},
		nil, nil, nil, nil,
	)
	vg := topo.VolumeGroups[0]

	var older, newer gopowerstore.PerformanceMetricsByVgResponse
	older.VgID = "vg-1"
	older.ReadIops = 5
	newer.VgID = "vg-1"
	newer.ReadIops = 80
	newer.WriteIops = 20
	newer.WriteBandwidth = 2048
	newer.AvgReadLatency = 300
	newer.AvgIoSize = 4096

	got := deriveVolumeGroupPerf("p1", topo, vg, []gopowerstore.PerformanceMetricsByVgResponse{older, newer})

	if !hasSample(got, "powerstore_volume_group_read_iops", 80) {
		t.Fatalf("read_iops: want latest 80, got %+v", got)
	}
	if !hasSample(got, "powerstore_volume_group_write_iops", 20) {
		t.Fatalf("write_iops: want 20")
	}
	if !hasSample(got, "powerstore_volume_group_write_bandwidth_bytes_per_second", 2048) {
		t.Fatalf("write bandwidth: want 2048")
	}
	if !hasSample(got, "powerstore_volume_group_read_latency_microseconds", 300) {
		t.Fatalf("read latency: want 300")
	}
	if !hasSample(got, "powerstore_volume_group_avg_io_size_bytes", 4096) {
		t.Fatalf("avg io size: want 4096")
	}
	if _, ok := sampleByLabel(got, "powerstore_volume_group_read_iops", "volume_group_name", "vgA"); !ok {
		t.Fatal("missing volume_group_name label")
	}
}

func TestDeriveVolumeGroupPerfEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil,
		[]gopowerstore.VolumeGroup{{ID: "vg-1", Name: "vgA"}}, nil, nil, nil, nil)
	got := deriveVolumeGroupPerf("p1", topo, topo.VolumeGroups[0], nil)
	if len(got) != 0 {
		t.Fatalf("no samples should emit nothing, got %+v", got)
	}
}
