package powerstore

import "strings"

// knownAlertSeverities are the severity levels PowerStore assigns to alerts
// (Alert.Severity is one of these). They are always emitted — with a zero count
// when no active alert matches — so that the absence of alerts is an observable,
// stable series for alerting rules (`powerstore_alert_active{severity="Critical"} == 0`)
// rather than a series that disappears.
var knownAlertSeverities = []string{"Critical", "Major", "Minor", "Info", "None"}

// deriveAlerts counts ACTIVE alerts grouped by severity and emits
// powerstore_alert_active per severity. It is derived from the per-cycle alert
// list carried on the Topology (no extra API call here), so it is appended to
// both the bulk and per-entity export paths identically — exactly like
// deriveFileSystemCapacity and derivePortLinkStatus — preserving metric parity.
func deriveAlerts(array string, topo *Topology) []Sample {
	clusterID := topo.ClusterID()

	counts := make(map[string]int, len(knownAlertSeverities))
	for _, sev := range knownAlertSeverities {
		counts[sev] = 0 // seed stable zero series
	}
	for _, a := range topo.Alerts {
		// Only ACTIVE alerts count; CLEARED ones have been resolved. The metric
		// name (_active) reflects this filter.
		if !strings.EqualFold(a.State, "ACTIVE") {
			continue
		}
		// Unknown/unexpected severities still surface as their own series rather
		// than being silently dropped.
		counts[a.Severity]++
	}

	out := make([]Sample, 0, len(counts))
	for sev, n := range counts {
		out = append(out, Sample{"powerstore_alert_active", alertLabels(array, clusterID, sev), float64(n)})
	}
	return out
}
