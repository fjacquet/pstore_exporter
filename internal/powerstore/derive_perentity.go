package powerstore

import "github.com/dell/gopowerstore"

func deref(p *int64) float64 {
	if p == nil {
		return 0
	}
	return float64(*p)
}

// deriveVolumePerf maps the newest performance sample per volume to []Sample.
func deriveVolumePerf(array string, topo *Topology, resp []gopowerstore.PerformanceMetricsByVolumeResponse) []Sample {
	clusterID := topo.ClusterID()
	latest := latestByID(resp, func(r gopowerstore.PerformanceMetricsByVolumeResponse) string { return r.VolumeID })
	var out []Sample
	for volID, r := range latest {
		volName, applID, _ := topo.VolumeInfo(volID)
		if volName == "" {
			volName = volID
		}
		vgID, vgName := topo.VolumeGroupOf(volID)
		labels := volumeLabels(array, clusterID, volName, volID, applID, topo.ApplianceName(applID), vgName, vgID)
		out = append(out,
			Sample{"powerstore_volume_read_iops", labels, float64(r.ReadIops)},
			Sample{"powerstore_volume_write_iops", labels, float64(r.WriteIops)},
			Sample{"powerstore_volume_total_iops", labels, float64(r.TotalIops)},
			Sample{"powerstore_volume_read_bandwidth_bytes_per_second", labels, float64(r.ReadBandwidth)},
			Sample{"powerstore_volume_write_bandwidth_bytes_per_second", labels, float64(r.WriteBandwidth)},
			Sample{"powerstore_volume_read_latency_microseconds", labels, float64(r.AvgReadLatency)},
			Sample{"powerstore_volume_write_latency_microseconds", labels, float64(r.AvgWriteLatency)},
			Sample{"powerstore_volume_avg_io_size_bytes", labels, float64(r.AvgIoSize)},
		)
	}
	return out
}

// deriveAppliancePerf maps the newest performance sample per appliance to []Sample.
func deriveAppliancePerf(array string, topo *Topology, resp []gopowerstore.PerformanceMetricsByApplianceResponse) []Sample {
	clusterID := topo.ClusterID()
	latest := latestByID(resp, func(r gopowerstore.PerformanceMetricsByApplianceResponse) string { return r.ApplianceID })
	var out []Sample
	for applID, r := range latest {
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, topo.ApplianceServiceTag(applID))
		out = append(out,
			Sample{"powerstore_appliance_read_iops", labels, float64(r.ReadIops)},
			Sample{"powerstore_appliance_write_iops", labels, float64(r.WriteIops)},
			Sample{"powerstore_appliance_total_iops", labels, float64(r.TotalIops)},
			Sample{"powerstore_appliance_read_bandwidth_bytes_per_second", labels, float64(r.ReadBandwidth)},
			Sample{"powerstore_appliance_write_bandwidth_bytes_per_second", labels, float64(r.WriteBandwidth)},
			Sample{"powerstore_appliance_read_latency_microseconds", labels, float64(r.AvgReadLatency)},
			Sample{"powerstore_appliance_write_latency_microseconds", labels, float64(r.AvgWriteLatency)},
			Sample{"powerstore_appliance_io_workload_cpu_utilization", labels, float64(r.IoWorkloadCPUUtilization)},
		)
	}
	return out
}

// deriveApplianceSpace maps the newest space sample per appliance to []Sample.
func deriveApplianceSpace(array string, topo *Topology, resp []gopowerstore.SpaceMetricsByApplianceResponse) []Sample {
	clusterID := topo.ClusterID()
	latest := latestByID(resp, func(r gopowerstore.SpaceMetricsByApplianceResponse) string { return r.ApplianceID })
	var out []Sample
	for applID, r := range latest {
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, topo.ApplianceServiceTag(applID))
		out = append(out,
			Sample{"powerstore_appliance_physical_total_bytes", labels, deref(r.PhysicalTotal)},
			Sample{"powerstore_appliance_physical_used_bytes", labels, deref(r.PhysicalUsed)},
			Sample{"powerstore_appliance_logical_provisioned_bytes", labels, deref(r.LogicalProvisioned)},
			Sample{"powerstore_appliance_logical_used_bytes", labels, deref(r.LogicalUsed)},
			Sample{"powerstore_appliance_data_reduction_ratio", labels, float64(r.DataReduction)},
			Sample{"powerstore_appliance_efficiency_ratio", labels, float64(r.EfficiencyRatio)},
			Sample{"powerstore_appliance_snapshot_savings_ratio", labels, float64(r.SnapshotSavings)},
			Sample{"powerstore_appliance_thin_savings_ratio", labels, float64(r.ThinSavings)},
		)
	}
	return out
}

// chartableFileSystem reports whether a file system is a real, provisioned data
// filesystem worth charting. Inactive metro/replication-destination stubs report
// size_total as null, which the SDK decodes to 0; PowerStore never provisions a
// real filesystem at 0 bytes, so a 0 total is the reliable "skip me" signal (the
// REST API exposes no is_replication_destination/state flag to filter on).
func chartableFileSystem(fs gopowerstore.FileSystem) bool {
	return fs.SizeTotal > 0
}

// deriveFileSystemCapacity emits file-system capacity from inventory (no metrics call,
// since gopowerstore v1.22.0 has no PerformanceMetricsByFileSystem method).
func deriveFileSystemCapacity(array string, topo *Topology) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, fs := range topo.FileSystems {
		if !chartableFileSystem(fs) {
			continue
		}
		labels := fileSystemLabels(array, clusterID, fs.Name, fs.ID, topo.NASName(fs.NasServerID), fs.NasServerID)
		out = append(out,
			Sample{"powerstore_file_system_size_total_bytes", labels, float64(fs.SizeTotal)},
			Sample{"powerstore_file_system_size_used_bytes", labels, float64(fs.SizeUsed)},
		)
	}
	return out
}

// derivePortLinkStatus emits link-up gauges for eth and fc ports from inventory.
func derivePortLinkStatus(array string, topo *Topology) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, p := range topo.EthPorts {
		out = append(out, Sample{"powerstore_port_link_up", portLabels(array, clusterID, p.Name, p.ID, "eth", p.ApplianceID), b2f(p.IsLinkUp)})
	}
	for _, p := range topo.FCPorts {
		out = append(out, Sample{"powerstore_port_link_up", portLabels(array, clusterID, p.Name, p.ID, "fc", p.ApplianceID), b2f(p.IsLinkUp)})
	}
	return out
}

// latestByID deduplicates a slice of items by string key, keeping the last entry
// per key (caller guarantees time-ordered ascending so last-wins is newest).
func latestByID[V any](items []V, key func(V) string) map[string]V {
	m := make(map[string]V, len(items))
	for _, it := range items {
		m[key(it)] = it
	}
	return m
}
