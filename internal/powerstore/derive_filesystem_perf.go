package powerstore

import "github.com/dell/gopowerstore"

// deriveFileSystemPerf maps the newest performance sample for one file system to
// []Sample, emitting metric names and units parallel to the volume performance
// metrics so file and block workloads share a consistent dashboard vocabulary.
// The samples are time-ordered ascending; the last entry is the most recent.
//
// The 4.4.0 base_performance_metrics_by_file_system schema defines the bare
// read_iops/write_iops/total_iops/read_bandwidth/write_bandwidth fields, but the
// gopowerstore struct also carries avg_*-tagged variants. We prefer the
// spec-aligned bare field and fall back to the avg_* counterpart when it is zero,
// so the metric is correct regardless of which the live array populates. Latency
// and I/O size are avg_*-prefixed in the spec itself, so they read Avg* directly.
func deriveFileSystemPerf(array string, topo *Topology, fs gopowerstore.FileSystem, resp []gopowerstore.PerformanceMetricsByFileSystemResponse) []Sample {
	if len(resp) == 0 {
		return nil
	}
	latest := resp[len(resp)-1]

	fsName := fs.Name
	if fsName == "" {
		fsName = fs.ID
	}
	labels := fileSystemLabels(array, topo.ClusterID(), fsName, fs.ID, topo.NASName(fs.NasServerID), fs.NasServerID)

	return []Sample{
		{"powerstore_file_system_read_iops", labels, preferNonZero(latest.ReadIops, latest.AvgReadIops)},
		{"powerstore_file_system_write_iops", labels, preferNonZero(latest.WriteIops, latest.AvgWriteIops)},
		{"powerstore_file_system_total_iops", labels, preferNonZero(latest.TotalIops, latest.AvgTotalIops)},
		{"powerstore_file_system_read_bandwidth_bytes_per_second", labels, preferNonZero(latest.ReadBandwidth, latest.AvgReadBandwidth)},
		{"powerstore_file_system_write_bandwidth_bytes_per_second", labels, preferNonZero(latest.WriteBandwidth, latest.AvgWriteBandwidth)},
		{"powerstore_file_system_read_latency_microseconds", labels, float64(latest.AvgReadLatency)},
		{"powerstore_file_system_write_latency_microseconds", labels, float64(latest.AvgWriteLatency)},
		{"powerstore_file_system_avg_io_size_bytes", labels, float64(latest.AvgSize)},
	}
}

// preferNonZero returns the spec-aligned primary field unless it is zero, in which
// case it falls back to the avg_* counterpart. Both are gopowerstore float32 gauges.
func preferNonZero(primary, fallback float32) float64 {
	if primary != 0 {
		return float64(primary)
	}
	return float64(fallback)
}
