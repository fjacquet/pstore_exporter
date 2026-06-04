// Package powerstore provides the Dell PowerStore client, metric collection, and the
// dual (Prometheus + OTLP) export paths.
package powerstore

import "strings"

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

// volumeGroupLabels builds the canonical VolumeGroup label set.
func volumeGroupLabels(arrayName, clusterID, vgName, vgID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"volume_group_name", vgName},
		Label{"volume_group_id", vgID},
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

// nasLabels builds the canonical NAS server label set.
func nasLabels(arrayName, clusterID, nasName, nasID string) []Label {
	return append(baseLabels(arrayName, clusterID),
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

// driveLabels builds the canonical drive label set.
func driveLabels(arrayName, clusterID, driveID, applID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"drive_id", driveID},
		Label{"appliance_id", applID},
	)
}

// toSnake converts camelCase to snake_case for metric name fragments.
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
