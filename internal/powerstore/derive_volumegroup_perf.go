package powerstore

import "github.com/dell/gopowerstore"

// deriveVolumeGroupPerf maps the newest performance sample for one volume group
// to []Sample, emitting metric names and units parallel to the volume performance
// metrics. The samples are time-ordered ascending; the last entry is newest.
func deriveVolumeGroupPerf(array string, topo *Topology, vg gopowerstore.VolumeGroup, resp []gopowerstore.PerformanceMetricsByVgResponse) []Sample {
	if len(resp) == 0 {
		return nil
	}
	latest := resp[len(resp)-1]

	vgName := vg.Name
	if vgName == "" {
		vgName = vg.ID
	}
	labels := volumeGroupLabels(array, topo.ClusterID(), vgName, vg.ID)

	return []Sample{
		{"powerstore_volume_group_read_iops", labels, float64(latest.ReadIops)},
		{"powerstore_volume_group_write_iops", labels, float64(latest.WriteIops)},
		{"powerstore_volume_group_total_iops", labels, float64(latest.TotalIops)},
		{"powerstore_volume_group_read_bandwidth_bytes_per_second", labels, float64(latest.ReadBandwidth)},
		{"powerstore_volume_group_write_bandwidth_bytes_per_second", labels, float64(latest.WriteBandwidth)},
		{"powerstore_volume_group_read_latency_microseconds", labels, float64(latest.AvgReadLatency)},
		{"powerstore_volume_group_write_latency_microseconds", labels, float64(latest.AvgWriteLatency)},
		{"powerstore_volume_group_avg_io_size_bytes", labels, float64(latest.AvgIoSize)},
	}
}
