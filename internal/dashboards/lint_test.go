package dashboards

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// dashboardRoot is relative to this package directory.
const dashboardRoot = "../../grafana/dashboards"

type target struct {
	Expr string `json:"expr"`
}

type colorSpec struct {
	Mode string `json:"mode"`
}

type fieldDefaults struct {
	Color *colorSpec `json:"color"`
}

type property struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

type matcher struct {
	ID      string `json:"id"`
	Options any    `json:"options"`
}

type override struct {
	Matcher    matcher    `json:"matcher"`
	Properties []property `json:"properties"`
}

type fieldConfig struct {
	Defaults  fieldDefaults `json:"defaults"`
	Overrides []override    `json:"overrides"`
}

type panel struct {
	Title       string      `json:"title"`
	Type        string      `json:"type"`
	Targets     []target    `json:"targets"`
	FieldConfig fieldConfig `json:"fieldConfig"`
}

type dashboard struct {
	Panels []panel `json:"panels"`
}

// panelRef is a panel plus the file it came from, for legible failure messages.
type panelRef struct {
	File string
	panel
}

// loadPanels decodes every bundled dashboard and returns its non-row panels.
func loadPanels(t *testing.T) []panelRef {
	t.Helper()

	var out []panelRef
	err := filepath.WalkDir(dashboardRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var dash dashboard
		if err := json.Unmarshal(raw, &dash); err != nil {
			t.Fatalf("%s: invalid dashboard JSON: %v", path, err)
		}
		rel, err := filepath.Rel(dashboardRoot, path)
		if err != nil {
			return err
		}
		// node-exporter-full.json is the verbatim Grafana community dashboard
		// 1860 (uid rYdddlPWk), vendored for the host running the exporter. Its
		// panels query host metrics (node_cpu_*, no `array` label) and are not
		// ours to restyle, so the PowerStore conventions do not apply to it.
		if filepath.Base(path) == "node-exporter-full.json" {
			return nil
		}
		for _, p := range dash.Panels {
			if p.Type == "row" {
				continue
			}
			out = append(out, panelRef{File: rel, panel: p})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dashboardRoot, err)
	}
	if len(out) == 0 {
		t.Fatalf("no panels found under %s", dashboardRoot)
	}
	return out
}

// bareAggregation matches an aggregation operator applied directly to a
// parenthesised expression, i.e. one with no `by (...)` clause.
var bareAggregation = regexp.MustCompile(`\b(sum|avg|count|min|max)\s*\(`)

// TestNoBareAggregation enforces R1: a panel expression must never collapse every
// dimension to a single value. With more than one array selected such a panel
// silently blends arrays, and the reader cannot tell which array it describes.
//
// `sum by (appliance_name) (...)` is deliberately allowed: it groups by *something*.
// That it omits `array` is a separate, tracked concern (see ADR-0016).
func TestNoBareAggregation(t *testing.T) {
	for _, p := range loadPanels(t) {
		for _, tgt := range p.Targets {
			if tgt.Expr == "" {
				continue
			}
			if !bareAggregation.MatchString(tgt.Expr) {
				continue
			}
			if strings.Contains(tgt.Expr, "by (") {
				continue
			}
			t.Errorf("%s: panel %q aggregates with no `by (...)` clause, so it "+
				"collapses all arrays into one value:\n  %s", p.File, p.Title, tgt.Expr)
		}
	}
}
