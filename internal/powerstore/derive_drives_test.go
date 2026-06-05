package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func f64(v float64) *float64 { return &v }

func TestDeriveDrivesWearAndState(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	drives := []driveInfo{
		{ID: "d-1", Name: "Drive-0", ApplianceID: "a-1", LifeCycleState: "Healthy",
			ExtraDetails: driveExtraDetails{DriveWearLevel: f64(12.5)}},
		// d-2: no wear level reported (e.g. PowerStoreOS < 4.3) → state only.
		{ID: "d-2", Name: "Drive-1", ApplianceID: "a-1", LifeCycleState: "Failed"},
	}

	got := deriveDrives("p1", topo, drives)

	// Wear level is emitted as a 0..1 ratio (12.5% → 0.125).
	if v, ok := sampleByLabel(got, "powerstore_drive_wear_level_ratio", "drive_id", "d-1"); !ok || v != 0.125 {
		t.Fatalf("d-1 wear ratio: want 0.125, got %v (present=%v)", v, ok)
	}
	// d-2 has no wear field → no wear series (not a misleading 0).
	if _, ok := sampleByLabel(got, "powerstore_drive_wear_level_ratio", "drive_id", "d-2"); ok {
		t.Fatal("d-2 must not emit a wear series when wear level is absent")
	}

	// Both drives emit a state info series (value 1, state in a label).
	if v, ok := sampleByLabel(got, "powerstore_drive_state", "drive_id", "d-1"); !ok || v != 1 {
		t.Fatalf("d-1 state series: want 1, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_drive_state", "state", "Failed"); !ok {
		t.Fatal("expected a state=Failed series for d-2")
	}
}

func TestDeriveDrivesEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	if got := deriveDrives("p1", topo, nil); len(got) != 0 {
		t.Fatalf("no drives should emit nothing, got %+v", got)
	}
}
