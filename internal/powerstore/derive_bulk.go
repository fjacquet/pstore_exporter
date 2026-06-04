package powerstore

import "strconv"

// csvFloat parses a numeric CSV cell, returning the first candidate key that is
// present and parseable. The PowerStore bulk five-minute CSVs use averaged
// column names (e.g. "avg_read_iops", "avg_read_bandwidth"); the short aliases
// (e.g. "read_iops") are accepted as fallbacks so the same code parses either
// naming convention. Confirmed against the Dell reference
// powerstore-metrics-exporter/collector/bulkClient/api_bulk_model.go.
func csvFloat(row map[string]string, keys ...string) float64 {
	for _, k := range keys {
		raw, ok := row[k]
		if !ok || raw == "" {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		return v
	}
	return 0
}

// deriveBulkVolumePerf maps performance_metrics_by_volume.csv rows to []Sample,
// emitting the SAME metric names and label keys as deriveVolumePerf so the bulk
// and per-entity paths are interchangeable for dashboards.
func deriveBulkVolumePerf(array string, topo *Topology, rows []map[string]string) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, r := range rows {
		volID := r["volume_id"]
		volName, applID := topo.VolumeInfo(volID)
		if volName == "" {
			volName = volID
		}
		if applID == "" {
			applID = r["appliance_id"]
		}
		vgID, vgName := topo.VolumeGroupOf(volID)
		labels := volumeLabels(array, clusterID, volName, volID, applID, topo.ApplianceName(applID), vgName, vgID)
		out = append(out,
			Sample{"powerstore_volume_read_iops", labels, csvFloat(r, "avg_read_iops", "read_iops")},
			Sample{"powerstore_volume_write_iops", labels, csvFloat(r, "avg_write_iops", "write_iops")},
			Sample{"powerstore_volume_total_iops", labels, csvFloat(r, "avg_total_iops", "total_iops")},
			Sample{"powerstore_volume_read_bandwidth_bytes_per_second", labels, csvFloat(r, "avg_read_bandwidth", "read_bandwidth")},
			Sample{"powerstore_volume_write_bandwidth_bytes_per_second", labels, csvFloat(r, "avg_write_bandwidth", "write_bandwidth")},
			Sample{"powerstore_volume_read_latency_microseconds", labels, csvFloat(r, "avg_read_latency")},
			Sample{"powerstore_volume_write_latency_microseconds", labels, csvFloat(r, "avg_write_latency")},
			Sample{"powerstore_volume_avg_io_size_bytes", labels, csvFloat(r, "avg_io_size")},
		)
	}
	return out
}

// deriveBulkAppliancePerf maps performance_metrics_by_appliance.csv rows to []Sample,
// emitting the SAME metric names and label keys as deriveAppliancePerf.
func deriveBulkAppliancePerf(array string, topo *Topology, rows []map[string]string) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, r := range rows {
		applID := r["appliance_id"]
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, topo.ApplianceServiceTag(applID))
		out = append(out,
			Sample{"powerstore_appliance_read_iops", labels, csvFloat(r, "avg_read_iops", "read_iops")},
			Sample{"powerstore_appliance_write_iops", labels, csvFloat(r, "avg_write_iops", "write_iops")},
			Sample{"powerstore_appliance_total_iops", labels, csvFloat(r, "avg_total_iops", "total_iops")},
			Sample{"powerstore_appliance_read_bandwidth_bytes_per_second", labels, csvFloat(r, "avg_read_bandwidth", "read_bandwidth")},
			Sample{"powerstore_appliance_write_bandwidth_bytes_per_second", labels, csvFloat(r, "avg_write_bandwidth", "write_bandwidth")},
			Sample{"powerstore_appliance_read_latency_microseconds", labels, csvFloat(r, "avg_read_latency")},
			Sample{"powerstore_appliance_write_latency_microseconds", labels, csvFloat(r, "avg_write_latency")},
			Sample{"powerstore_appliance_io_workload_cpu_utilization", labels, csvFloat(r, "avg_io_workload_cpu_utilization")},
		)
	}
	return out
}

// deriveBulkApplianceSpace maps space_metrics_by_appliance.csv rows to []Sample,
// emitting the SAME metric names and label keys as deriveApplianceSpace.
func deriveBulkApplianceSpace(array string, topo *Topology, rows []map[string]string) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, r := range rows {
		applID := r["appliance_id"]
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, topo.ApplianceServiceTag(applID))
		out = append(out,
			Sample{"powerstore_appliance_physical_total_bytes", labels, csvFloat(r, "physical_total", "last_physical_total")},
			Sample{"powerstore_appliance_physical_used_bytes", labels, csvFloat(r, "physical_used", "last_physical_used")},
			Sample{"powerstore_appliance_logical_provisioned_bytes", labels, csvFloat(r, "logical_provisioned", "last_logical_provisioned")},
			Sample{"powerstore_appliance_logical_used_bytes", labels, csvFloat(r, "logical_used", "last_logical_used")},
			Sample{"powerstore_appliance_data_reduction_ratio", labels, csvFloat(r, "data_reduction", "last_data_reduction")},
			Sample{"powerstore_appliance_efficiency_ratio", labels, csvFloat(r, "efficiency_ratio", "last_efficiency_ratio")},
			Sample{"powerstore_appliance_snapshot_savings_ratio", labels, csvFloat(r, "snapshot_savings", "last_snapshot_savings")},
			Sample{"powerstore_appliance_thin_savings_ratio", labels, csvFloat(r, "thin_savings", "last_thin_savings")},
		)
	}
	return out
}
