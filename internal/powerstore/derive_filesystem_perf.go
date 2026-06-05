package powerstore

import "github.com/dell/gopowerstore"

// deriveFileSystemPerf maps the newest performance sample for one file system to
// []Sample, emitting metric names and units parallel to the volume performance
// metrics so file and block workloads share a consistent dashboard vocabulary.
// The PerformanceMetricsByFileSystem response carries only averaged fields, so
// these are the Avg* counters. The samples are time-ordered ascending; the last
// entry is the most recent.
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
		{"powerstore_file_system_read_iops", labels, float64(latest.AvgReadIops)},
		{"powerstore_file_system_write_iops", labels, float64(latest.AvgWriteIops)},
		{"powerstore_file_system_total_iops", labels, float64(latest.AvgTotalIops)},
		{"powerstore_file_system_read_bandwidth_bytes_per_second", labels, float64(latest.AvgReadBandwidth)},
		{"powerstore_file_system_write_bandwidth_bytes_per_second", labels, float64(latest.AvgWriteBandwidth)},
		{"powerstore_file_system_read_latency_microseconds", labels, float64(latest.AvgReadLatency)},
		{"powerstore_file_system_write_latency_microseconds", labels, float64(latest.AvgWriteLatency)},
		{"powerstore_file_system_avg_io_size_bytes", labels, float64(latest.AvgSize)},
	}
}
