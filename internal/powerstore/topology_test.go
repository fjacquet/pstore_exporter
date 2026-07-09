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

func TestResourceName(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "1"},
		nil,
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1"}},
		[]gopowerstore.VolumeGroup{{ID: "vg-1", Name: "vgA"}},
		[]gopowerstore.NAS{{ID: "nas-1", Name: "NAS01"}},
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "FS01"}},
		nil, nil,
	)

	cases := []struct {
		name         string
		resourceType string
		id           string
		want         string
	}{
		{"volume resolves", "volume", "v-1", "vol1"},
		{"volume group resolves", "volume_group", "vg-1", "vgA"},
		{"file system resolves", "file_system", "fs-1", "FS01"},
		{"nas server resolves", "nas_server", "nas-1", "NAS01"},
		// Fallback: never empty, always degrades to the id.
		{"unknown id falls back to id", "volume", "v-missing", "v-missing"},
		{"unknown resource type falls back to id", "galaxy", "x-1", "x-1"},
		{"empty id falls back to empty", "volume", "", ""},
		{"right id wrong type falls back to id", "volume", "fs-1", "fs-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := topo.ResourceName(tc.resourceType, tc.id); got != tc.want {
				t.Fatalf("ResourceName(%q, %q) = %q, want %q",
					tc.resourceType, tc.id, got, tc.want)
			}
		})
	}
}
