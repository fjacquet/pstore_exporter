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

	counts := make(map[string]int)
	for _, a := range topo.Alerts {
		// Only ACTIVE alerts count; CLEARED ones have been resolved. The metric
		// name (_active) reflects this filter.
		if strings.EqualFold(a.State, "ACTIVE") {
			counts[a.Severity]++
		}
	}

	// Emit the known severities first, in a fixed order and with a zero count
	// when absent, so the series are stable and the output is deterministic. Any
	// unexpected severity then surfaces as its own series rather than being
	// silently dropped.
	out := make([]Sample, 0, len(knownAlertSeverities)+len(counts))
	known := make(map[string]struct{}, len(knownAlertSeverities))
	for _, sev := range knownAlertSeverities {
		known[sev] = struct{}{}
		out = append(out, Sample{"powerstore_alert_active", alertLabels(array, clusterID, sev), float64(counts[sev])})
	}
	for sev, n := range counts {
		if _, ok := known[sev]; !ok {
			out = append(out, Sample{"powerstore_alert_active", alertLabels(array, clusterID, sev), float64(n)})
		}
	}
	return out
}
