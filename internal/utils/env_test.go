package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fjacquet/pstore_exporter/internal/models"
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
