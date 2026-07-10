package dashboards

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

// bareAggregation matches a dimension-collapsing aggregation operator applied
// directly to a parenthesised expression — the bare `sum(...)` form. The grouped
// `sum by (array) (...)` form is NOT matched: after the operator comes ` by`, not
// `(`, so `\s*\(` fails. topk/bottomk/count_values are deliberately excluded: they
// return several series rather than collapsing to one value (e.g. the legitimate
// `topk(10, sum by (volume_name) (...))` panels).
var bareAggregation = regexp.MustCompile(`\b(sum|avg|count|min|max|stddev|stdvar|group|quantile)\s*\(`)

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
			t.Errorf("%s: panel %q aggregates with no `by (...)` clause, so it "+
				"collapses all arrays into one value:\n  %s", p.File, p.Title, tgt.Expr)
		}
	}
}

// directionMatcher matches the byRegexp options used for the read/write series
// convention, e.g. "/ read$/" and "/ write$/".
var directionMatcher = regexp.MustCompile(`(read|write)\$/$`)

// isDirectionOverride reports whether o selects series by read/write direction.
func isDirectionOverride(o override) bool {
	if o.Matcher.ID != "byRegexp" {
		return false
	}
	opts, ok := o.Matcher.Options.(string)
	return ok && directionMatcher.MatchString(opts)
}

// TestNoFixedDirectionColour enforces R2. Colouring by direction with a fixed
// colour erases the entity dimension: on a single-direction panel every appliance
// matches `/ read$/` and every appliance renders blue.
func TestNoFixedDirectionColour(t *testing.T) {
	for _, p := range loadPanels(t) {
		for _, o := range p.FieldConfig.Overrides {
			if !isDirectionOverride(o) {
				continue
			}
			for _, prop := range o.Properties {
				if prop.ID != "color" {
					continue
				}
				var c colorSpec
				if err := json.Unmarshal(prop.Value, &c); err != nil {
					t.Fatalf("%s: panel %q: bad color value: %v", p.File, p.Title, err)
				}
				if c.Mode == "fixed" {
					t.Errorf("%s: panel %q pins a fixed colour on a read/write matcher, "+
						"collapsing every entity into one colour", p.File, p.Title)
				}
			}
		}
	}
}

// TestDirectionPanelsColourByName enforces R3. A timeseries panel that still
// distinguishes direction must derive hue from the series name, so an array or
// appliance keeps one colour across every dashboard.
func TestDirectionPanelsColourByName(t *testing.T) {
	for _, p := range loadPanels(t) {
		if p.Type != "timeseries" {
			continue
		}
		var hasDirection bool
		for _, o := range p.FieldConfig.Overrides {
			if isDirectionOverride(o) {
				hasDirection = true
				break
			}
		}
		if !hasDirection {
			continue
		}
		got := ""
		if c := p.FieldConfig.Defaults.Color; c != nil {
			got = c.Mode
		}
		if got != "palette-classic-by-name" {
			t.Errorf("%s: panel %q distinguishes direction but has color.mode=%q, "+
				"want %q", p.File, p.Title, got, "palette-classic-by-name")
		}
	}
}

// isCatchAll reports whether an override targets every series (e.g. "/.*/").
func isCatchAll(o override) bool {
	if o.Matcher.ID != "byRegexp" {
		return false
	}
	opts, ok := o.Matcher.Options.(string)
	return ok && (opts == "/.*/" || opts == "/.+/")
}

// TestNoCatchAllFixedColour enforces that a timeseries panel never pins every
// series to one fixed colour. A catch-all fixed-colour override collapses all
// entities (volumes, appliances, arrays) into one indistinguishable colour —
// the exact defect reported on the Appliances and Volumes dashboards.
func TestNoCatchAllFixedColour(t *testing.T) {
	for _, p := range loadPanels(t) {
		if p.Type != "timeseries" {
			continue
		}
		for _, o := range p.FieldConfig.Overrides {
			if !isCatchAll(o) {
				continue
			}
			for _, prop := range o.Properties {
				if prop.ID != "color" {
					continue
				}
				var c colorSpec
				if err := json.Unmarshal(prop.Value, &c); err != nil {
					t.Fatalf("%s: panel %q: bad color value: %v", p.File, p.Title, err)
				}
				if c.Mode == "fixed" {
					t.Errorf("%s: panel %q pins every series to one fixed colour via a "+
						"catch-all matcher, collapsing all entities into one colour", p.File, p.Title)
				}
			}
		}
	}
}
