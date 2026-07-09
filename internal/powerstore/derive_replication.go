package powerstore

import "github.com/dell/gopowerstore"

// rpoSeconds maps the PowerStore RPO enum to a duration in seconds. RpoZero is a
// synchronous rule (RPO of 0). Unknown values are absent so the caller can skip
// them rather than emit a misleading 0.
var rpoSeconds = map[gopowerstore.RPOEnum]float64{
	gopowerstore.RpoZero:           0,
	gopowerstore.RpoFiveMinutes:    5 * 60,
	gopowerstore.RpoFifteenMinutes: 15 * 60,
	gopowerstore.RpoThirtyMinutes:  30 * 60,
	gopowerstore.RpoOneHour:        60 * 60,
	gopowerstore.RpoSixHours:       6 * 60 * 60,
	gopowerstore.RpoTwelveHours:    12 * 60 * 60,
	gopowerstore.RpoOneDay:         24 * 60 * 60,
}

// deriveReplicationSessions emits one info-style series per replication session:
// powerstore_replication_session_state is always 1 and carries the session's
// current state as a label (the kube-state-metrics enum idiom). Operators alert
// on undesirable states, e.g. {state=~"Error|Fractured|System_Paused"}.
func deriveReplicationSessions(array string, topo *Topology, sessions []gopowerstore.ReplicationSession) []Sample {
	clusterID := topo.ClusterID()
	out := make([]Sample, 0, len(sessions))
	for _, s := range sessions {
		if s.ID == "" {
			continue
		}
		name := topo.ResourceName(s.ResourceType, s.LocalResourceID)
		labels := replicationSessionLabels(array, clusterID, s.ID, s.LocalResourceID, name,
			s.ResourceType, s.Role, s.Type, s.RemoteSystemID, string(s.State))
		out = append(out, Sample{"powerstore_replication_session_state", labels, 1})
	}
	return out
}

// deriveReplicationRules emits powerstore_replication_rpo_seconds per replication
// rule. Rules whose RPO enum is unrecognized are skipped.
func deriveReplicationRules(array string, topo *Topology, rules []gopowerstore.ReplicationRule) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, r := range rules {
		secs, ok := rpoSeconds[r.Rpo]
		if !ok {
			continue
		}
		out = append(out, Sample{
			"powerstore_replication_rpo_seconds",
			replicationRuleLabels(array, clusterID, r.ID, r.RemoteSystemID),
			secs,
		})
	}
	return out
}

// deriveReplicationTransfer emits the replication transfer rate and remaining
// backlog for one resource from its mirror-transfer-rate time series. The samples
// are time-ordered ascending, so the last entry is the most recent.
func deriveReplicationTransfer(array string, topo *Topology, resourceID, resourceType string, samples []gopowerstore.VolumeMirrorTransferRateResponse) []Sample {
	if len(samples) == 0 {
		return nil
	}
	latest := samples[len(samples)-1]
	name := topo.ResourceName(resourceType, resourceID)
	labels := replicationResourceLabels(array, topo.ClusterID(), resourceID, name, resourceType)
	return []Sample{
		{"powerstore_replication_transfer_rate_bytes_per_second", labels, float64(latest.MirrorBandwidth)},
		{"powerstore_replication_data_remaining_bytes", labels, float64(latest.DataRemaining)},
	}
}

// replicatedVolumeResources returns the deduplicated local resource IDs of
// volume-type replication sessions — the resources whose live mirror transfer
// rate is worth querying. Non-volume sessions (e.g. volume_group, file_system)
// and sessions with no local resource id are skipped. A single volume can
// appear in more than one volume-type session (e.g. Metro plus an async DR
// session in a 3-site protection setup), so ids are deduped, preserving
// first-seen order, to avoid querying — and emitting — the same resource's
// transfer metrics twice (which would produce a duplicate/invalid Prometheus
// series). Driving transfer queries from real sessions (rather than from every
// protection-policy volume) avoids phantom 0/0 series.
func replicatedVolumeResources(sessions []gopowerstore.ReplicationSession) []string {
	var ids []string
	seen := make(map[string]struct{})
	for _, s := range sessions {
		if s.ResourceType != "volume" || s.LocalResourceID == "" {
			continue
		}
		if _, ok := seen[s.LocalResourceID]; ok {
			continue
		}
		seen[s.LocalResourceID] = struct{}{}
		ids = append(ids, s.LocalResourceID)
	}
	return ids
}
