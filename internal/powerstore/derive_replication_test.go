package powerstore

import (
	"reflect"
	"testing"

	"github.com/dell/gopowerstore"
)

func sampleByLabel(s []Sample, name, labelName, labelValue string) (float64, bool) {
	for _, x := range s {
		if x.Name != name {
			continue
		}
		for _, l := range x.Labels {
			if l.Name == labelName && l.Value == labelValue {
				return x.Value, true
			}
		}
	}
	return 0, false
}

func TestDeriveReplicationSessionInfo(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	sessions := []gopowerstore.ReplicationSession{
		{ID: "rs-1", State: gopowerstore.RsStateOk, Role: "Source", Type: "Asynchronous",
			ResourceType: "volume", RemoteSystemID: "remote-9", LocalResourceID: "v-1"},
		{ID: "rs-2", State: gopowerstore.RsStateError, Role: "Source", Type: "Synchronous",
			ResourceType: "volume_group", RemoteSystemID: "remote-9", LocalResourceID: "vg-1"},
	}

	got := deriveReplicationSessions("p1", topo, sessions)

	// Info series: value is always 1; the state lives in a label.
	if v, ok := sampleByLabel(got, "powerstore_replication_session_state", "session_id", "rs-1"); !ok || v != 1 {
		t.Fatalf("rs-1 info series: want value 1, got %v (present=%v)", v, ok)
	}
	if v, ok := sampleByLabel(got, "powerstore_replication_session_state", "state", "OK"); !ok || v != 1 {
		t.Fatalf("expected a series carrying state=OK, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "state", "Error"); !ok {
		t.Fatal("expected a series carrying state=Error for the failed session")
	}
}

func TestDeriveReplicationRPOSeconds(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	rules := []gopowerstore.ReplicationRule{
		{ID: "rule-5m", Rpo: gopowerstore.RpoFiveMinutes, RemoteSystemID: "remote-9"},
		{ID: "rule-1d", Rpo: gopowerstore.RpoOneDay, RemoteSystemID: "remote-9"},
		{ID: "rule-sync", Rpo: gopowerstore.RpoZero, RemoteSystemID: "remote-9"},
		{ID: "rule-bad", Rpo: gopowerstore.RPOEnum("Nonsense"), RemoteSystemID: "remote-9"},
	}

	got := deriveReplicationRules("p1", topo, rules)

	if v, ok := sampleByLabel(got, "powerstore_replication_rpo_seconds", "rule_id", "rule-5m"); !ok || v != 300 {
		t.Fatalf("5-minute RPO: want 300, got %v (present=%v)", v, ok)
	}
	if v, ok := sampleByLabel(got, "powerstore_replication_rpo_seconds", "rule_id", "rule-1d"); !ok || v != 86400 {
		t.Fatalf("1-day RPO: want 86400, got %v (present=%v)", v, ok)
	}
	if v, ok := sampleByLabel(got, "powerstore_replication_rpo_seconds", "rule_id", "rule-sync"); !ok || v != 0 {
		t.Fatalf("sync (Zero) RPO: want 0, got %v (present=%v)", v, ok)
	}
	// Unknown RPO values are skipped rather than emitted as a misleading 0.
	if _, ok := sampleByLabel(got, "powerstore_replication_rpo_seconds", "rule_id", "rule-bad"); ok {
		t.Fatal("unknown RPO enum must not emit a series")
	}
}

func TestDeriveReplicationTransferLatestSample(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	// Time-ordered ascending; the latest (last) sample wins.
	samples := []gopowerstore.VolumeMirrorTransferRateResponse{
		{ID: "v-1", MirrorBandwidth: 100, DataRemaining: 5000},
		{ID: "v-1", MirrorBandwidth: 250, DataRemaining: 4000},
	}

	got := deriveReplicationTransfer("p1", topo, "v-1", "volume", samples)

	if v, ok := sampleByLabel(got, "powerstore_replication_transfer_rate_bytes_per_second", "resource_id", "v-1"); !ok || v != 250 {
		t.Fatalf("transfer rate: want latest 250, got %v (present=%v)", v, ok)
	}
	if v, ok := sampleByLabel(got, "powerstore_replication_data_remaining_bytes", "resource_id", "v-1"); !ok || v != 4000 {
		t.Fatalf("data remaining: want latest 4000, got %v (present=%v)", v, ok)
	}
}

func TestDeriveReplicationTransferEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	got := deriveReplicationTransfer("p1", topo, "v-1", "volume", nil)
	if len(got) != 0 {
		t.Fatalf("no samples should emit nothing, got %+v", got)
	}
}

func TestReplicatedVolumeResources(t *testing.T) {
	sessions := []gopowerstore.ReplicationSession{
		{ID: "rs-1", ResourceType: "volume", LocalResourceID: "v-1"},
		{ID: "rs-2", ResourceType: "file_system", LocalResourceID: "fs-1"}, // not a volume → skip
		{ID: "rs-3", ResourceType: "volume", LocalResourceID: ""},          // no id → skip
		{ID: "rs-4", ResourceType: "volume", LocalResourceID: "v-2"},
		// A volume protected by both Metro and an async DR session (3-site
		// protection) appears in two volume-type sessions with the same
		// LocalResourceID — must be deduped to a single entry.
		{ID: "rs-5", ResourceType: "volume", LocalResourceID: "v-1"},
	}
	got := replicatedVolumeResources(sessions)
	want := []string{"v-1", "v-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestDeriveReplicationSessionMetro(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	sessions := []gopowerstore.ReplicationSession{
		{ID: "rs-metro", State: gopowerstore.RsStatePaused, Role: "Metro_Preferred",
			Type: "Metro_Active_Active", ResourceType: "volume",
			RemoteSystemID: "remote-9", LocalResourceID: "v-9"},
	}
	got := deriveReplicationSessions("p1", topo, sessions)

	if v, ok := sampleByLabel(got, "powerstore_replication_session_state", "state", "Paused"); !ok || v != 1 {
		t.Fatalf("metro session state=Paused: want 1, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "role", "Metro_Preferred"); !ok {
		t.Fatal("expected a role=Metro_Preferred label")
	}
}

func TestDeriveReplicationSessionResourceNames(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "c1"},
		nil,
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1"}},
		[]gopowerstore.VolumeGroup{{ID: "vg-1", Name: "vgA"}},
		[]gopowerstore.NAS{{ID: "nas-1", Name: "NAS01"}},
		[]gopowerstore.FileSystem{{ID: "fs-1", Name: "FS01"}},
		nil, nil,
	)
	sessions := []gopowerstore.ReplicationSession{
		{ID: "rs-vol", ResourceType: "volume", LocalResourceID: "v-1", State: gopowerstore.RsStateOk},
		{ID: "rs-vg", ResourceType: "volume_group", LocalResourceID: "vg-1", State: gopowerstore.RsStateOk},
		{ID: "rs-fs", ResourceType: "file_system", LocalResourceID: "fs-1", State: gopowerstore.RsStateOk},
		{ID: "rs-nas", ResourceType: "nas_server", LocalResourceID: "nas-1", State: gopowerstore.RsStateOk},
		{ID: "rs-ghost", ResourceType: "volume", LocalResourceID: "v-missing", State: gopowerstore.RsStateOk},
	}

	got := deriveReplicationSessions("p1", topo, sessions)

	for _, want := range []string{"vol1", "vgA", "FS01", "NAS01"} {
		if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "local_resource_name", want); !ok {
			t.Fatalf("expected a session series with local_resource_name=%q", want)
		}
	}
	// An id absent from inventory degrades to the id, never to "".
	if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "local_resource_name", "v-missing"); !ok {
		t.Fatal("unresolvable resource must fall back to local_resource_name=<id>")
	}
	if _, ok := sampleByLabel(got, "powerstore_replication_session_state", "local_resource_name", ""); ok {
		t.Fatal("local_resource_name must never be empty")
	}
}

func TestDeriveReplicationTransferResourceName(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: "c1"}, nil,
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1"}},
		nil, nil, nil, nil, nil,
	)
	samples := []gopowerstore.VolumeMirrorTransferRateResponse{
		{ID: "v-1", MirrorBandwidth: 250, DataRemaining: 4000},
	}

	got := deriveReplicationTransfer("p1", topo, "v-1", "volume", samples)

	if v, ok := sampleByLabel(got, "powerstore_replication_transfer_rate_bytes_per_second", "resource_name", "vol1"); !ok || v != 250 {
		t.Fatalf("transfer rate by resource_name: want 250, got %v (present=%v)", v, ok)
	}
	if v, ok := sampleByLabel(got, "powerstore_replication_data_remaining_bytes", "resource_name", "vol1"); !ok || v != 4000 {
		t.Fatalf("data remaining by resource_name: want 4000, got %v (present=%v)", v, ok)
	}
}

func TestDeriveReplicationTransferUnknownResourceFallsBackToID(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	samples := []gopowerstore.VolumeMirrorTransferRateResponse{
		{ID: "v-9", MirrorBandwidth: 1, DataRemaining: 2},
	}

	got := deriveReplicationTransfer("p1", topo, "v-9", "volume", samples)

	if _, ok := sampleByLabel(got, "powerstore_replication_transfer_rate_bytes_per_second", "resource_name", "v-9"); !ok {
		t.Fatal("unresolvable resource must fall back to resource_name=<id>")
	}
}
