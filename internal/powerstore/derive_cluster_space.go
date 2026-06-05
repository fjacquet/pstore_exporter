package powerstore

import "github.com/dell/gopowerstore"

// deriveClusterSpace maps the newest cluster space sample to []Sample, emitting
// cluster-wide capacity and efficiency parallel to the appliance space metrics.
// These feed full-by-date / capacity-forecasting dashboards. The samples are
// time-ordered ascending; the last entry is newest.
func deriveClusterSpace(array string, topo *Topology, resp []gopowerstore.SpaceMetricsByClusterResponse) []Sample {
	if len(resp) == 0 {
		return nil
	}
	latest := resp[len(resp)-1]
	labels := baseLabels(array, topo.ClusterID())

	return []Sample{
		{"powerstore_cluster_physical_total_bytes", labels, deref(latest.PhysicalTotal)},
		{"powerstore_cluster_physical_used_bytes", labels, deref(latest.PhysicalUsed)},
		{"powerstore_cluster_logical_provisioned_bytes", labels, deref(latest.LogicalProvisioned)},
		{"powerstore_cluster_logical_used_bytes", labels, deref(latest.LogicalUsed)},
		{"powerstore_cluster_data_reduction_ratio", labels, float64(latest.DataReduction)},
		{"powerstore_cluster_efficiency_ratio", labels, float64(latest.EfficiencyRatio)},
	}
}
