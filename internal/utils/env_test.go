package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fjacquet/pstore_exporter/internal/models"
	"gopkg.in/yaml.v2"
)

func TestExpandEnvSuccess(t *testing.T) {
	t.Setenv("PFLEX_TEST_SECRET", "hunter2")
	got, err := ExpandEnv("pre-${PFLEX_TEST_SECRET}-post")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "pre-hunter2-post" {
		t.Errorf("ExpandEnv = %q", got)
	}
}

func TestExpandEnvMissing(t *testing.T) {
	if _, err := ExpandEnv("${PFLEX_DEFINITELY_UNSET_VAR}"); err == nil {
		t.Error("expected error for unset variable")
	}
}

func TestResolveSecretsInterpolatesAndLoadsFile(t *testing.T) {
	t.Setenv("PFLEX_PW1", "envpass")

	pwFile := filepath.Join(t.TempDir(), "pw.txt")
	if err := os.WriteFile(pwFile, []byte("  filepass\n"), 0o600); err != nil {
		t.Fatalf("write pw file: %v", err)
	}

	cfg := &models.Config{Arrays: []models.ArrayConfig{
		{Name: "a", Endpoint: "https://10.0.0.1/api/rest", Username: "u", Password: "${PFLEX_PW1}"},
		{Name: "b", Endpoint: "https://10.0.0.2/api/rest", Username: "u", PasswordFile: pwFile},
	}}

	if err := ResolveSecrets(cfg); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if cfg.Arrays[0].Password != "envpass" {
		t.Errorf("env password = %q", cfg.Arrays[0].Password)
	}
	if cfg.Arrays[1].Password != "filepass" {
		t.Errorf("file password = %q (want trimmed 'filepass')", cfg.Arrays[1].Password)
	}
}

func TestResolveSecretsExpandsUsername(t *testing.T) {
	t.Setenv("PSTORE1_USERNAME", "monitor")
	t.Setenv("PSTORE1_PASSWORD", "secret")

	cfg := &models.Config{Arrays: []models.ArrayConfig{
		{Name: "pstore-1", Endpoint: "https://10.0.0.1/api/rest", Username: "${PSTORE1_USERNAME}", Password: "${PSTORE1_PASSWORD}"},
	}}

	if err := ResolveSecrets(cfg); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if cfg.Arrays[0].Username != "monitor" {
		t.Errorf("username = %q, want %q", cfg.Arrays[0].Username, "monitor")
	}
}

func TestResolveSecretsUnsetUsernameFails(t *testing.T) {
	cfg := &models.Config{Arrays: []models.ArrayConfig{
		{Name: "pstore-1", Endpoint: "https://10.0.0.1/api/rest", Username: "${PSTORE1_USERNAME_DEFINITELY_UNSET}", Password: "pw"},
	}}

	if err := ResolveSecrets(cfg); err == nil {
		t.Error("expected error for unset username variable, got nil")
	}
}

// decodedArrayConfig decodes a single-array YAML fragment through the real
// yaml.v2 path so InsecureSkipVerify goes through its actual UnmarshalYAML,
// matching how config.yaml is loaded in production.
func decodedArrayConfig(t *testing.T, insecureSkipVerifyYAML string) models.ArrayConfig {
	t.Helper()
	doc := "name: pstore-1\nendpoint: https://10.0.0.1/api/rest\nusername: u\npassword: p\n" + insecureSkipVerifyYAML
	var a models.ArrayConfig
	if err := yaml.Unmarshal([]byte(doc), &a); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	return a
}

func TestResolveSecretsInsecureSkipVerifyNativeBoolPassesThrough(t *testing.T) {
	a := decodedArrayConfig(t, "insecureSkipVerify: true\n")
	cfg := &models.Config{Arrays: []models.ArrayConfig{a}}

	if err := ResolveSecrets(cfg); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if !cfg.Arrays[0].InsecureSkipVerify.Bool() {
		t.Error("want native bool true to remain true after ResolveSecrets")
	}
}

func TestResolveSecretsInsecureSkipVerifyEnvVarTrue(t *testing.T) {
	t.Setenv("PSTORE1_SKIP_CERTIFICATE", "true")
	a := decodedArrayConfig(t, "insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}\n")
	cfg := &models.Config{Arrays: []models.ArrayConfig{a}}

	if err := ResolveSecrets(cfg); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if !cfg.Arrays[0].InsecureSkipVerify.Bool() {
		t.Error("want ${PSTORE1_SKIP_CERTIFICATE}=true to resolve to true")
	}
}

func TestResolveSecretsInsecureSkipVerifyOmittedDefaultsFalse(t *testing.T) {
	a := decodedArrayConfig(t, "")
	cfg := &models.Config{Arrays: []models.ArrayConfig{a}}

	if err := ResolveSecrets(cfg); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	if cfg.Arrays[0].InsecureSkipVerify.Bool() {
		t.Error("want omitted insecureSkipVerify to default to false")
	}
}

func TestResolveSecretsInsecureSkipVerifyUnsetVarFails(t *testing.T) {
	a := decodedArrayConfig(t, "insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE_DEFINITELY_UNSET}\n")
	cfg := &models.Config{Arrays: []models.ArrayConfig{a}}

	if err := ResolveSecrets(cfg); err == nil {
		t.Error("expected error for unset insecureSkipVerify variable, got nil")
	}
}

func TestResolveSecretsInsecureSkipVerifyNonBooleanErrors(t *testing.T) {
	t.Setenv("PSTORE1_SKIP_CERTIFICATE", "maybe")
	a := decodedArrayConfig(t, "insecureSkipVerify: ${PSTORE1_SKIP_CERTIFICATE}\n")
	cfg := &models.Config{Arrays: []models.ArrayConfig{a}}

	if err := ResolveSecrets(cfg); err == nil {
		t.Error("expected error for non-boolean insecureSkipVerify value, got nil")
	}
}
