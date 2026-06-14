package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

// TestBulkApplianceCPUUtilSpecAlignedColumn covers the 4.4.0 reconciliation
// finding: the bulk appliance perf read for CPU utilization had no fallback to the
// spec-aligned bare column name. base_performance_metrics_by_appliance defines
// io_workload_cpu_utilization (no avg_ prefix); the read must accept it.
func TestBulkApplianceCPUUtilSpecAlignedColumn(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1", Name: "CLU"},
		[]gopowerstore.ApplianceInstance{{ID: "appl-1", Name: "ApplianceA"}},
		nil, nil, nil, nil, nil, nil,
	)
	rows := []map[string]string{{
		"appliance_id": "appl-1", "io_workload_cpu_utilization": "73.5",
	}}
	got := deriveBulkAppliancePerf("p1", topo, rows)
	if !hasSample(got, "powerstore_appliance_io_workload_cpu_utilization", 73.5) {
		t.Fatalf("cpu util: want 73.5 from spec-aligned column, got %+v", got)
	}
}

func TestBulkVolumeSamplesMatchPerEntity(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1", Name: "CLU"},
		[]gopowerstore.ApplianceInstance{{ID: "appl-1", Name: "ApplianceA"}},
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"}},
		nil, nil, nil, nil, nil,
	)
	rows := []map[string]string{{
		"volume_id": "v-1", "read_iops": "100", "write_iops": "50", "total_iops": "150",
		"read_bandwidth": "1000", "write_bandwidth": "500",
		"avg_read_latency": "200", "avg_write_latency": "300", "avg_io_size": "8192",
	}}
	got := deriveBulkVolumePerf("p1", topo, rows)
	if !hasSample(got, "powerstore_volume_read_iops", 100) {
		t.Fatalf("missing read_iops: %+v", got)
	}
	want := volumeLabels("p1", "1", "vol1", "v-1", "appl-1", "ApplianceA", "", "")
	for _, s := range got {
		if s.Name == "powerstore_volume_read_iops" {
			if len(s.Labels) != len(want) {
				t.Fatalf("label count %d != %d", len(s.Labels), len(want))
			}
			for i := range want {
				if s.Labels[i].Name != want[i].Name {
					t.Fatalf("label[%d] %q != %q", i, s.Labels[i].Name, want[i].Name)
				}
			}
		}
	}
}
