package powerstore

import (
	"encoding/json"
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

func TestDriveInfoDecodesLifecycleState(t *testing.T) {
	// Shape of a hardware?type=eq.Drive row. The PowerStore property is
	// "lifecycle_state" (not "life_cycle_state"); a mismatched struct tag
	// silently decodes it to "" — which is the bug this guards against.
	const payload = `[{"id":"d-1","name":"Drive-0","appliance_id":"a-1",` +
		`"lifecycle_state":"Healthy","extra_details":{"drive_wear_level":30}}]`

	var got []driveInfo
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 drive, got %d", len(got))
	}
	if got[0].LifeCycleState != "Healthy" {
		t.Fatalf("lifecycle_state must decode into LifeCycleState; got %q", got[0].LifeCycleState)
	}
	if got[0].ExtraDetails.DriveWearLevel == nil || *got[0].ExtraDetails.DriveWearLevel != 30 {
		t.Fatalf("drive_wear_level must decode; got %v", got[0].ExtraDetails.DriveWearLevel)
	}
}
