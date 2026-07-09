// Package powerstore provides the Dell PowerStore client, metric collection, and the
// dual (Prometheus + OTLP) export paths.
package powerstore

// Label is a single metric label name-value pair.
type Label struct {
	Name  string
	Value string
}

// Sample is one exported metric data point. The first label is always "array".
type Sample struct {
	Name   string
	Labels []Label
	Value  float64
}

// baseLabels returns the array identity labels every metric carries.
func baseLabels(arrayName, clusterID string) []Label {
	return []Label{
		{Name: "array", Value: arrayName},
		{Name: "cluster_id", Value: clusterID},
	}
}

// volumeLabels builds the canonical Volume label set so the bulk and per-entity paths
// emit identical label keys. Inapplicable values are passed empty.
func volumeLabels(arrayName, clusterID, volName, volID, applID, applName, vgName, vgID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"volume_name", volName},
		Label{"volume_id", volID},
		Label{"appliance_id", applID},
		Label{"appliance_name", applName},
		Label{"volume_group_name", vgName},
		Label{"volume_group_id", vgID},
	)
}

// applianceLabels builds the canonical Appliance label set.
func applianceLabels(arrayName, clusterID, applName, applID, serviceTag string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"appliance_name", applName},
		Label{"appliance_id", applID},
		Label{"service_tag", serviceTag},
	)
}

// fileSystemLabels builds the canonical FileSystem label set.
func fileSystemLabels(arrayName, clusterID, fsName, fsID, nasName, nasID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"file_system_name", fsName},
		Label{"file_system_id", fsID},
		Label{"nas_server_name", nasName},
		Label{"nas_server_id", nasID},
	)
}

// portLabels builds the canonical port label set (kind is "eth" or "fc").
func portLabels(arrayName, clusterID, portName, portID, kind, applID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"port_name", portName},
		Label{"port_id", portID},
		Label{"port_type", kind},
		Label{"appliance_id", applID},
	)
}

// alertLabels builds the canonical alert label set. Alerts are aggregated by
// severity (not per-alert) to keep the series count bounded and stable.
func alertLabels(arrayName, clusterID, severity string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"severity", severity},
	)
}

// driveLabels builds the canonical drive label set.
func driveLabels(arrayName, clusterID, driveID, driveName, applianceID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"drive_id", driveID},
		Label{"drive_name", driveName},
		Label{"appliance_id", applianceID},
	)
}

// driveStateLabels builds the drive label set plus the lifecycle-state label for
// the drive info series, keeping the full series definition in one place rather
// than appending the state label at the emission site.
func driveStateLabels(arrayName, clusterID, driveID, driveName, applianceID, state string) []Label {
	return append(driveLabels(arrayName, clusterID, driveID, driveName, applianceID),
		Label{"state", state},
	)
}

// witnessStateLabels builds the label set for the Metro witness service info
// series. `state` is the WitnessStateEnum value as a string.
func witnessStateLabels(arrayName, clusterID, witnessID, witnessName, state string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"witness_id", witnessID},
		Label{"witness_name", witnessName},
		Label{"state", state},
	)
}

// witnessConnectionLabels builds the per-node connection label set for the Metro
// witness. `state` is the WitnessConnectionStateEnum value as a string.
func witnessConnectionLabels(arrayName, clusterID, witnessID, applianceID, nodeID, state string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"witness_id", witnessID},
		Label{"appliance_id", applianceID},
		Label{"node_id", nodeID},
		Label{"state", state},
	)
}

// volumeGroupLabels builds the canonical volume-group label set.
func volumeGroupLabels(arrayName, clusterID, vgName, vgID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"volume_group_name", vgName},
		Label{"volume_group_id", vgID},
	)
}

// replicationSessionLabels builds the label set for a replication session's
// info series. `state` is the current RSStateEnum value as a string.
// localResourceName is the resolved display name of the replicated resource; it
// falls back to localResourceID when the resource is absent from the inventory.
func replicationSessionLabels(arrayName, clusterID, sessionID, localResourceID, localResourceName, resourceType, role, sessionType, remoteSystemID, state string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"session_id", sessionID},
		Label{"local_resource_id", localResourceID},
		Label{"local_resource_name", localResourceName},
		Label{"resource_type", resourceType},
		Label{"role", role},
		Label{"type", sessionType},
		Label{"remote_system_id", remoteSystemID},
		Label{"state", state},
	)
}

// replicationResourceLabels builds the label set for per-resource replication
// transfer metrics (resourceType is e.g. "volume"). resourceName falls back to
// resourceID when the resource is absent from the inventory.
func replicationResourceLabels(arrayName, clusterID, resourceID, resourceName, resourceType string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"resource_id", resourceID},
		Label{"resource_name", resourceName},
		Label{"resource_type", resourceType},
	)
}

// replicationRuleLabels builds the label set for replication-rule metrics (RPO).
func replicationRuleLabels(arrayName, clusterID, ruleID, remoteSystemID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"rule_id", ruleID},
		Label{"remote_system_id", remoteSystemID},
	)
}

// b2f converts a bool to a float64 metric value (1 for true, 0 for false).
func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
