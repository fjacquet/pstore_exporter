package powerstore

// witnessConnection is the subset of a PowerStore witness_connection_instance we
// export: one node's connection to the witness service.
type witnessConnection struct {
	State       string `json:"state"`
	ApplianceID string `json:"appliance_id"`
	NodeID      string `json:"node_id"`
}

// witnessInfo is the subset of a PowerStore witness_instance we map to metrics.
// PowerStore (and gopowerstore) expose no typed witness method, so these are
// decoded from a generic GET on the /witness resource (see ADR-0009, ADR-0015).
type witnessInfo struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	State       string              `json:"state"`
	Connections []witnessConnection `json:"connections"`
}

// deriveWitness emits the Metro witness service state and one connection-state
// series per node. Both are info-style: the value is always 1 and the current
// state is carried as a label (the kube-state-metrics enum idiom). Operators
// alert on undesirable states, e.g. {state=~"Disconnected|Partially_Connected"}.
func deriveWitness(array string, topo *Topology, witnesses []witnessInfo) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, w := range witnesses {
		if w.ID == "" {
			continue
		}
		if w.State != "" {
			out = append(out, Sample{"powerstore_metro_witness_state",
				witnessStateLabels(array, clusterID, w.ID, w.Name, w.State), 1})
		}
		for _, conn := range w.Connections {
			if conn.State == "" {
				continue
			}
			out = append(out, Sample{"powerstore_metro_witness_connection_state",
				witnessConnectionLabels(array, clusterID, w.ID, conn.ApplianceID, conn.NodeID, conn.State), 1})
		}
	}
	return out
}
