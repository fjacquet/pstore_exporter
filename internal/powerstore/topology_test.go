package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestTopologyIndices(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1", Name: "CLU"},
		[]gopowerstore.ApplianceInstance{{ID: "appl-1", Name: "ApplianceA"}},
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"}},
		[]gopowerstore.VolumeGroup{{
			ID:      "vg-1",
			Name:    "vgA",
			Volumes: []gopowerstore.Volume{{ID: "v-1", Name: "vol1"}},
		}},
		nil, nil, nil, nil,
	)
	if topo.ApplianceName("appl-1") != "ApplianceA" {
		t.Fatalf("appliance lookup failed: %q", topo.ApplianceName("appl-1"))
	}
	if topo.ClusterID() != "1" {
		t.Fatalf("cluster id: %q", topo.ClusterID())
	}
	if gotID, gotName := topo.VolumeGroupOf("v-1"); gotID != "vg-1" || gotName != "vgA" {
		t.Fatalf("volume group lookup failed: id=%q name=%q", gotID, gotName)
	}
}

func TestVolumeInfoKnown(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1"}, nil,
		[]gopowerstore.Volume{
			{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"},
			{ID: "v-empty", Name: "", ApplianceID: "appl-1"},
		},
		nil, nil, nil, nil, nil,
	)

	if name, _, known := topo.VolumeInfo("v-1"); !known || name != "vol1" {
		t.Fatalf("known volume: want (vol1,true), got (%q,%v)", name, known)
	}
	// Present but empty name → known=true (a genuine empty name, not a miss).
	if name, _, known := topo.VolumeInfo("v-empty"); !known || name != "" {
		t.Fatalf("empty-name volume: want (\"\",true), got (%q,%v)", name, known)
	}
	// Absent id → known=false.
	if _, _, known := topo.VolumeInfo("nope"); known {
		t.Fatal("unknown id must report known=false")
	}
}
