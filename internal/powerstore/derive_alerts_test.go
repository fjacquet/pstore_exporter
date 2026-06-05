package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

// sampleBySeverity returns the value of the powerstore_alert_active sample whose
// severity label equals sev, and whether such a sample exists.
func sampleBySeverity(s []Sample, sev string) (float64, bool) {
	for _, x := range s {
		if x.Name != "powerstore_alert_active" {
			continue
		}
		for _, l := range x.Labels {
			if l.Name == "severity" && l.Value == sev {
				return x.Value, true
			}
		}
	}
	return 0, false
}

func TestDeriveAlertsActiveBySeverity(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "1"}, nil, nil, nil, nil, nil, nil, nil)
	topo.Alerts = []gopowerstore.Alert{
		{ID: "a1", Severity: "Critical", State: "ACTIVE"},
		{ID: "a2", Severity: "Critical", State: "ACTIVE"},
		{ID: "a3", Severity: "Major", State: "ACTIVE"},
		{ID: "a4", Severity: "Minor", State: "CLEARED"}, // cleared → not counted
		{ID: "a5", Severity: "Major", State: "Cleared"}, // case-insensitive state, not counted
	}

	got := deriveAlerts("p1", topo)

	if v, ok := sampleBySeverity(got, "Critical"); !ok || v != 2 {
		t.Fatalf("Critical active count: want 2, got %v (present=%v)", v, ok)
	}
	if v, ok := sampleBySeverity(got, "Major"); !ok || v != 1 {
		t.Fatalf("Major active count: want 1, got %v (present=%v)", v, ok)
	}
	// Minor's only alert was CLEARED, but the series must still be emitted as 0
	// so alerting rules see a stable series rather than a vanishing one.
	if v, ok := sampleBySeverity(got, "Minor"); !ok || v != 0 {
		t.Fatalf("Minor active count: want stable 0 series, got %v (present=%v)", v, ok)
	}
	// Info/None are seeded even with no alerts at all.
	if v, ok := sampleBySeverity(got, "Info"); !ok || v != 0 {
		t.Fatalf("Info active count: want stable 0 series, got %v (present=%v)", v, ok)
	}
}

func TestDeriveAlertsEmptyStillEmitsStableSeries(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "1"}, nil, nil, nil, nil, nil, nil, nil)
	got := deriveAlerts("p1", topo)
	for _, sev := range []string{"Critical", "Major", "Minor", "Info", "None"} {
		if v, ok := sampleBySeverity(got, sev); !ok || v != 0 {
			t.Fatalf("severity %q: want 0 series present, got %v (present=%v)", sev, v, ok)
		}
	}
}

func TestDeriveAlertsUnknownSeveritySurfaces(t *testing.T) {
	topo := NewTopology(gopowerstore.Cluster{ID: "1"}, nil, nil, nil, nil, nil, nil, nil)
	topo.Alerts = []gopowerstore.Alert{{ID: "x", Severity: "Catastrophic", State: "ACTIVE"}}
	got := deriveAlerts("p1", topo)
	if v, ok := sampleBySeverity(got, "Catastrophic"); !ok || v != 1 {
		t.Fatalf("unexpected severity must still surface: want 1, got %v (present=%v)", v, ok)
	}
}
