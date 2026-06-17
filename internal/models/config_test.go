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

func TestGetMaxConcurrencyDefaultsTo16(t *testing.T) {
	c := &Config{}
	if got := c.GetMaxConcurrency(); got != 16 {
		t.Fatalf("want fleet default 16 when unset, got %d", got)
	}
}

func TestGetMaxConcurrencyUsesConfiguredFleetValue(t *testing.T) {
	c := &Config{}
	c.Collection.MaxConcurrency = 32
	if got := c.GetMaxConcurrency(); got != 32 {
		t.Fatalf("want configured fleet value 32, got %d", got)
	}
}

func TestArrayMaxConcurrencyOr(t *testing.T) {
	const fleet = 16
	if got := (ArrayConfig{}).MaxConcurrencyOr(fleet); got != fleet {
		t.Fatalf("unset per-array cap should inherit fleet %d, got %d", fleet, got)
	}
	if got := (ArrayConfig{MaxConcurrency: 8}).MaxConcurrencyOr(fleet); got != 8 {
		t.Fatalf("set per-array cap should win over fleet, got %d", got)
	}
}

func TestValidateRejectsNegativeFleetMaxConcurrency(t *testing.T) {
	c := &Config{Arrays: []ArrayConfig{validArray()}}
	c.Collection.MaxConcurrency = -1
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for negative collection.maxConcurrency")
	}
}

func TestValidateRejectsNegativePerArrayMaxConcurrency(t *testing.T) {
	a := validArray()
	a.MaxConcurrency = -1
	c := &Config{Arrays: []ArrayConfig{a}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for negative array maxConcurrency")
	}
}
