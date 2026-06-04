package powerstore

import "testing"

func TestBaseLabels(t *testing.T) {
	got := baseLabels("p1", "CLU-1")
	if len(got) != 2 || got[0].Name != "array" || got[0].Value != "p1" || got[1].Name != "cluster_id" {
		t.Fatalf("unexpected base labels: %+v", got)
	}
}

func TestVolumeLabelsCanonicalOrder(t *testing.T) {
	got := volumeLabels("p1", "CLU-1", "vol1", "v-1", "appl-1", "ApplianceA", "vgA", "vg-1")
	names := make([]string, len(got))
	for i, l := range got {
		names[i] = l.Name
	}
	want := []string{"array", "cluster_id", "volume_name", "volume_id", "appliance_id", "appliance_name", "volume_group_name", "volume_group_id"}
	if len(names) != len(want) {
		t.Fatalf("len mismatch: %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("label[%d]=%q want %q", i, names[i], want[i])
		}
	}
}
