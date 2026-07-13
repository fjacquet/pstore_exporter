package models

import (
	"errors"
	"testing"

	"gopkg.in/yaml.v2"
)

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
	if c.Server.Port != "9446" {
		t.Fatalf("want default port 9446, got %q", c.Server.Port)
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

func TestEnvBoolUnmarshalNativeBool(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("insecureSkipVerify: true\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !a.InsecureSkipVerify.Bool() {
		t.Fatal("want native bool true to resolve to true without calling Resolve")
	}
}

func TestEnvBoolUnmarshalNativeBoolFalse(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("insecureSkipVerify: false\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.InsecureSkipVerify.Bool() {
		t.Fatal("want native bool false to resolve to false")
	}
}

func TestEnvBoolUnmarshalOmittedDefaultsFalse(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("name: p1\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.InsecureSkipVerify.Bool() {
		t.Fatal("want omitted insecureSkipVerify to default to false")
	}
}

func TestEnvBoolUnmarshalEnvRefStaysUnresolvedUntilResolve(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.InsecureSkipVerify.Bool() {
		t.Fatal("want ${VAR} reference to resolve to false (zero value) before Resolve is called")
	}
}

func TestEnvBoolResolveExpandsVarToTrue(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	expand := func(s string) (string, error) { return "true", nil }
	if err := a.InsecureSkipVerify.Resolve(expand); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !a.InsecureSkipVerify.Bool() {
		t.Fatal("want resolved value true")
	}
}

func TestEnvBoolResolveNoOpWhenNativeBool(t *testing.T) {
	b := NewEnvBool(true)
	expand := func(s string) (string, error) {
		t.Fatal("expand should not be called for a native bool value")
		return "", nil
	}
	if err := b.Resolve(expand); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !b.Bool() {
		t.Fatal("want native bool to remain true")
	}
}

func TestEnvBoolResolveEmptyExpansionDefaultsFalse(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	expand := func(s string) (string, error) { return "", nil }
	if err := a.InsecureSkipVerify.Resolve(expand); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.InsecureSkipVerify.Bool() {
		t.Fatal("want empty expansion to resolve to false")
	}
}

func TestEnvBoolResolveNonBooleanErrors(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	expand := func(s string) (string, error) { return "not-a-bool", nil }
	if err := a.InsecureSkipVerify.Resolve(expand); err == nil {
		t.Fatal("expected error for non-boolean expansion")
	}
}

func TestEnvBoolResolvePropagatesExpandError(t *testing.T) {
	var a ArrayConfig
	if err := yaml.Unmarshal([]byte("insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}\n"), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantErr := errors.New("unset variable")
	expand := func(s string) (string, error) { return "", wantErr }
	if err := a.InsecureSkipVerify.Resolve(expand); !errors.Is(err, wantErr) {
		t.Fatalf("Resolve error = %v, want %v", err, wantErr)
	}
}

func TestEnvBoolUnmarshalRejectsNonScalar(t *testing.T) {
	var a ArrayConfig
	err := yaml.Unmarshal([]byte("insecureSkipVerify: [true, false]\n"), &a)
	if err == nil {
		t.Fatal("expected error for non-scalar insecureSkipVerify")
	}
}
