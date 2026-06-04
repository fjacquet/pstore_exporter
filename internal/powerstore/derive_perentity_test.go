package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func hasSample(s []Sample, name string, val float64) bool {
	for _, x := range s {
		if x.Name == name && x.Value == val {
			return true
		}
	}
	return false
}

func TestDeriveVolumePerfSamples(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1", Name: "CLU"},
		[]gopowerstore.ApplianceInstance{{ID: "appl-1", Name: "ApplianceA", ServiceTag: "ST1"}},
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"}},
		nil, nil, nil, nil, nil,
	)
	var r gopowerstore.PerformanceMetricsByVolumeResponse
	r.VolumeID = "v-1"
	r.ReadIops = 100
	r.WriteIops = 50
	r.AvgReadLatency = 200
	got := deriveVolumePerf("p1", topo, []gopowerstore.PerformanceMetricsByVolumeResponse{r})
	if !hasSample(got, "powerstore_volume_read_iops", 100) {
		t.Fatalf("missing read_iops: %+v", got)
	}
	if !hasSample(got, "powerstore_volume_read_latency_microseconds", 200) {
		t.Fatalf("missing read latency")
	}
}

func TestDeriveFileSystemCapacity(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1"}, nil, nil, nil,
		[]gopowerstore.NAS{{ID: "nas-1", Name: "nasA"}},
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "fsA", NasServerID: "nas-1", SizeTotal: 1000, SizeUsed: 400}},
		nil, nil,
	)
	got := deriveFileSystemCapacity("p1", topo)
	if !hasSample(got, "powerstore_file_system_size_total_bytes", 1000) || !hasSample(got, "powerstore_file_system_size_used_bytes", 400) {
		t.Fatalf("missing fs capacity: %+v", got)
	}
}
