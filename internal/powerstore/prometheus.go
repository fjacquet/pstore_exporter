package powerstore

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// PromCollector implements prometheus.Collector by reading the latest Snapshot. It is
// an "unchecked" collector (Describe sends nothing) so it can emit the dynamic set of
// PowerStore metric names without pre-registering every descriptor.
type PromCollector struct {
	store      *SnapshotStore
	up         *prometheus.Desc
	lastScrape *prometheus.Desc
	bulkAPI    *prometheus.Desc
}

// NewPromCollector builds a collector backed by the snapshot store.
func NewPromCollector(store *SnapshotStore) *PromCollector {
	return &PromCollector{
		store: store,
		up: prometheus.NewDesc(
			"powerstore_up",
			"1 if the array was scraped successfully, 0 otherwise",
			[]string{"array"}, nil,
		),
		lastScrape: prometheus.NewDesc(
			"powerstore_last_scrape_timestamp_seconds",
			"Unix timestamp of the last successful collection for the array",
			[]string{"array"}, nil,
		),
		bulkAPI: prometheus.NewDesc(
			"powerstore_array_bulk_api",
			"1 if the array supports the bulk five-minute metrics API, 0 otherwise",
			[]string{"array"}, nil,
		),
	}
}

// Describe sends no descriptors, marking this an unchecked collector.
func (c *PromCollector) Describe(_ chan<- *prometheus.Desc) {}

// Collect reads the latest snapshot and emits per-array health metrics plus every
// collected sample. Dynamic metric names get a Desc built on the fly; duplicate
// label-tuples within a metric name are skipped to avoid registry gather errors.
func (c *PromCollector) Collect(ch chan<- prometheus.Metric) {
	snap := c.store.Load()

	for name, as := range snap.PerArray {
		up := 0.0
		if as.Up {
			up = 1.0
		}
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, up, name)

		if as.Up && !as.LastScrape.IsZero() {
			ch <- prometheus.MustNewConstMetric(c.lastScrape, prometheus.GaugeValue, float64(as.LastScrape.Unix()), name)
		}
		bulk := 0.0
		if as.BulkCapable {
			bulk = 1.0
		}
		ch <- prometheus.MustNewConstMetric(c.bulkAPI, prometheus.GaugeValue, bulk, name)
	}

	for _, name := range snap.MetricNames() {
		samples := snap.SamplesByName(name)
		if len(samples) == 0 {
			continue
		}
		labelNames := sampleLabelNames(samples[0])
		desc := prometheus.NewDesc(name, "PowerStore metric", labelNames, nil)

		seen := make(map[string]struct{}, len(samples))
		for _, s := range samples {
			values := sampleLabelValues(s)
			sig := name + "\x00" + strings.Join(values, "\x00")
			if _, dup := seen[sig]; dup {
				continue
			}
			seen[sig] = struct{}{}

			m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, s.Value, values...)
			if err != nil {
				log.Debugf("skipping metric %s: %v", name, err)
				continue
			}
			ch <- m
		}
	}
}

func sampleLabelNames(s Sample) []string {
	names := make([]string, len(s.Labels))
	for i, l := range s.Labels {
		names[i] = l.Name
	}
	return names
}

func sampleLabelValues(s Sample) []string {
	values := make([]string, len(s.Labels))
	for i, l := range s.Labels {
		values[i] = l.Value
	}
	return values
}
