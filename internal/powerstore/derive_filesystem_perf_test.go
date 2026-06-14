package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestDeriveFileSystemPerfLatestSample(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "c1"}, nil, nil, nil,
		[]gopowerstore.NAS{{ID: "nas-1", Name: "nasA"}},
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "fsA", NasServerID: "nas-1"}},
		nil, nil,
	)
	fs := topo.FileSystems[0]

	// Time-ordered ascending; the latest (last) sample wins.
	var older, newer gopowerstore.PerformanceMetricsByFileSystemResponse
	older.FileSystemID = "fs-1"
	older.AvgReadIops = 10
	newer.FileSystemID = "fs-1"
	newer.AvgReadIops = 42
	newer.AvgWriteIops = 7
	newer.AvgReadBandwidth = 1024
	newer.AvgReadLatency = 250
	newer.AvgSize = 8192

	got := deriveFileSystemPerf("p1", topo, fs, []gopowerstore.PerformanceMetricsByFileSystemResponse{older, newer})

	if !hasSample(got, "powerstore_file_system_read_iops", 42) {
		t.Fatalf("read_iops: want latest 42, got %+v", got)
	}
	if !hasSample(got, "powerstore_file_system_write_iops", 7) {
		t.Fatalf("write_iops: want 7")
	}
	if !hasSample(got, "powerstore_file_system_read_bandwidth_bytes_per_second", 1024) {
		t.Fatalf("read bandwidth: want 1024")
	}
	if !hasSample(got, "powerstore_file_system_read_latency_microseconds", 250) {
		t.Fatalf("read latency: want 250")
	}
	if !hasSample(got, "powerstore_file_system_avg_io_size_bytes", 8192) {
		t.Fatalf("avg io size: want 8192")
	}

	// Labels must match the file-system capacity metric's label set so dashboards
	// can join performance and capacity on the same series.
	if _, ok := sampleByLabel(got, "powerstore_file_system_read_iops", "file_system_name", "fsA"); !ok {
		t.Fatal("missing file_system_name label")
	}
	if _, ok := sampleByLabel(got, "powerstore_file_system_read_iops", "nas_server_name", "nasA"); !ok {
		t.Fatal("missing resolved nas_server_name label")
	}
}

// TestDeriveFileSystemPerfPrefersSpecAlignedFields covers the 4.4.0 reconciliation
// finding: base_performance_metrics_by_file_system defines the bare read_iops/
// write_iops/total_iops/read_bandwidth/write_bandwidth fields, while the SDK struct
// also carries avg_*-tagged variants. The derive must read the spec-aligned bare
// fields when the array populates them (avg_* left zero), and fall back to avg_*
// otherwise — so it is correct regardless of which the live array emits.
func TestDeriveFileSystemPerfPrefersSpecAlignedFields(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil,
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "fsA"}}, nil, nil)
	fs := topo.FileSystems[0]

	// Spec-aligned bare fields populated; avg_* counterparts left zero.
	var s gopowerstore.PerformanceMetricsByFileSystemResponse
	s.FileSystemID = "fs-1"
	s.ReadIops = 42
	s.WriteIops = 7
	s.TotalIops = 49
	s.ReadBandwidth = 1024
	s.WriteBandwidth = 512

	got := deriveFileSystemPerf("p1", topo, fs, []gopowerstore.PerformanceMetricsByFileSystemResponse{s})

	for _, tc := range []struct {
		name string
		want float64
	}{
		{"powerstore_file_system_read_iops", 42},
		{"powerstore_file_system_write_iops", 7},
		{"powerstore_file_system_total_iops", 49},
		{"powerstore_file_system_read_bandwidth_bytes_per_second", 1024},
		{"powerstore_file_system_write_bandwidth_bytes_per_second", 512},
	} {
		if !hasSample(got, tc.name, tc.want) {
			t.Fatalf("%s: want %v from spec-aligned bare field, got %+v", tc.name, tc.want, got)
		}
	}
}

func TestDeriveFileSystemPerfEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil,
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "fsA"}}, nil, nil)
	got := deriveFileSystemPerf("p1", topo, topo.FileSystems[0], nil)
	if len(got) != 0 {
		t.Fatalf("no samples should emit nothing, got %+v", got)
	}
}
