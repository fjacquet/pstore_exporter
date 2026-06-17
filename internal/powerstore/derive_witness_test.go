package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestDeriveWitnessStateAndConnections(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	witnesses := []witnessInfo{
		{
			ID:    "w-1",
			Name:  "witness-east",
			State: "OK",
			Connections: []witnessConnection{
				{State: "OK", ApplianceID: "a-1", NodeID: "n-1"},
				{State: "Disconnected", ApplianceID: "a-1", NodeID: "n-2"},
			},
		},
	}

	got := deriveWitness("p1", topo, witnesses)

	// Overall service state: info series, value 1, state in a label.
	if v, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", "w-1"); !ok || v != 1 {
		t.Fatalf("witness state series: want 1, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "state", "OK"); !ok {
		t.Fatal("expected a state=OK witness series")
	}
	// One connection series per node, value 1.
	if v, ok := sampleByLabel(got, "powerstore_metro_witness_connection_state", "node_id", "n-1"); !ok || v != 1 {
		t.Fatalf("n-1 connection series: want 1, got %v (present=%v)", v, ok)
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_connection_state", "state", "Disconnected"); !ok {
		t.Fatal("expected a state=Disconnected connection series for n-2")
	}
}

func TestDeriveWitnessSkipsEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	witnesses := []witnessInfo{
		{ID: "", State: "OK"},                                  // no ID → skipped entirely
		{ID: "w-2", State: ""},                                 // no state → no state series
		{ID: "w-3", State: "OK", Connections: []witnessConnection{{State: "", NodeID: "n-9"}}}, // empty conn state → skipped
	}

	got := deriveWitness("p1", topo, witnesses)

	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", ""); ok {
		t.Fatal("witness with empty ID must emit nothing")
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", "w-2"); ok {
		t.Fatal("witness with empty state must not emit a state series")
	}
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_connection_state", "node_id", "n-9"); ok {
		t.Fatal("connection with empty state must not emit a series")
	}
	// w-3 itself (state OK) should still emit its state series.
	if _, ok := sampleByLabel(got, "powerstore_metro_witness_state", "witness_id", "w-3"); !ok {
		t.Fatal("w-3 should still emit its OK state series")
	}
}

func TestDeriveWitnessEmpty(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)
	if got := deriveWitness("p1", topo, nil); len(got) != 0 {
		t.Fatalf("no witnesses should emit nothing, got %+v", got)
	}
}
