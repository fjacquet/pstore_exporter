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

func TestDeriveFileSystemPerfEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil,
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "fsA"}}, nil, nil)
	got := deriveFileSystemPerf("p1", topo, topo.FileSystems[0], nil)
	if len(got) != 0 {
		t.Fatalf("no samples should emit nothing, got %+v", got)
	}
}
