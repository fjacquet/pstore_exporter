package powerstore

// driveExtraDetails is the subset of a hardware instance's extra_details we read.
// DriveWearLevel is the percentage of drive wear (0..100), added in PowerStoreOS
// 4.3.0.0 — it is a pointer so an absent field (older arrays) is distinguishable
// from a genuine 0% (a brand-new drive) and can be skipped.
type driveExtraDetails struct {
	DriveWearLevel *float64 `json:"drive_wear_level"`
}

// driveInfo is the subset of a PowerStore hardware "Drive" instance we map to
// metrics. PowerStore (and gopowerstore) expose no typed list-drives method, so
// these are decoded from a generic GET on the hardware resource (see ADR-0009).
type driveInfo struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	ApplianceID    string            `json:"appliance_id"`
	LifeCycleState string            `json:"life_cycle_state"`
	ExtraDetails   driveExtraDetails `json:"extra_details"`
}

// deriveDrives emits per-drive lifecycle state (info series) and, where the array
// reports it, the drive wear level as a 0..1 ratio. Both come from a single
// hardware enumeration, so this is cheap regardless of drive count.
func deriveDrives(array string, topo *Topology, drives []driveInfo) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, d := range drives {
		if d.ID == "" {
			continue
		}
		base := driveLabels(array, clusterID, d.ID, d.Name, d.ApplianceID)

		if d.LifeCycleState != "" {
			stateLabels := append(append([]Label{}, base...), Label{"state", d.LifeCycleState})
			out = append(out, Sample{"powerstore_drive_state", stateLabels, 1})
		}
		if d.ExtraDetails.DriveWearLevel != nil {
			out = append(out, Sample{"powerstore_drive_wear_level_ratio", base, *d.ExtraDetails.DriveWearLevel / 100})
		}
	}
	return out
}
