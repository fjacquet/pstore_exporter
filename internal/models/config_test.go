package models

import "testing"

func validArray() ArrayConfig {
	return ArrayConfig{Name: "p1", Endpoint: "https://10.0.0.1/api/rest", Username: "admin", Password: "secret"}
}

func TestValidateRequiresArray(t *testing.T) {
	c := &Config{}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error when no arrays configured")
	}
}

func TestValidateRejectsDuplicateNames(t *testing.T) {
	c := &Config{Arrays: []ArrayConfig{validArray(), validArray()}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected duplicate-name error")
	}
}

func TestSetDefaultsPortAndInterval(t *testing.T) {
	c := &Config{Arrays: []ArrayConfig{validArray()}}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Server.Port != "9101" {
		t.Fatalf("want default port 9101, got %q", c.Server.Port)
	}
	if c.Collection.Interval != "30s" {
		t.Fatalf("want default interval 30s, got %q", c.Collection.Interval)
	}
}

func TestArrayMetricsInterval(t *testing.T) {
	a := ArrayConfig{}
	if got := a.MetricsInterval(); got != "Five_Mins" {
		t.Fatalf("want Five_Mins default, got %q", got)
	}
}
