package powerstore

import "github.com/dell/gopowerstore"

func deref(p *int64) float64 {
	if p == nil {
		return 0
	}
	return float64(*p)
}

func applianceServiceTag(topo *Topology, id string) string {
	for _, a := range topo.Appliances {
		if a.ID == id {
			return a.ServiceTag
		}
	}
	return ""
}

// deriveVolumePerf maps the newest performance sample per volume to []Sample.
func deriveVolumePerf(array string, topo *Topology, resp []gopowerstore.PerformanceMetricsByVolumeResponse) []Sample {
	clusterID := topo.ClusterID()
	latest := make(map[string]gopowerstore.PerformanceMetricsByVolumeResponse)
	for _, r := range resp {
		latest[r.VolumeID] = r // time-ordered ascending; last wins
	}
	var out []Sample
	for volID, r := range latest {
		volName, applID := volID, ""
		for _, v := range topo.Volumes {
			if v.ID == volID {
				volName, applID = v.Name, v.ApplianceID
				break
			}
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
	latest := make(map[string]gopowerstore.PerformanceMetricsByApplianceResponse)
	for _, r := range resp {
		latest[r.ApplianceID] = r
	}
	var out []Sample
	for applID, r := range latest {
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, applianceServiceTag(topo, applID))
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
	latest := make(map[string]gopowerstore.SpaceMetricsByApplianceResponse)
	for _, r := range resp {
		latest[r.ApplianceID] = r
	}
	var out []Sample
	for applID, r := range latest {
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, applianceServiceTag(topo, applID))
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

// deriveFileSystemCapacity emits file-system capacity from inventory (no metrics call,
// since gopowerstore v1.22.0 has no PerformanceMetricsByFileSystem method).
func deriveFileSystemCapacity(array string, topo *Topology) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, fs := range topo.FileSystems {
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

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
