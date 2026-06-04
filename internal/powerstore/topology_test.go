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
