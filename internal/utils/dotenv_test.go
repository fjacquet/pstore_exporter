package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fjacquet/pstore_exporter/internal/models"
)

func TestLoadDotEnvSetsUnsetVars(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DOTENV_TEST_HOST=h1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTENV_TEST_HOST", "") // register cleanup, then unset for real
	_ = os.Unsetenv("DOTENV_TEST_HOST")

	LoadDotEnv(cfg)
	if got := os.Getenv("DOTENV_TEST_HOST"); got != "h1" {
		t.Errorf("DOTENV_TEST_HOST = %q, want h1", got)
	}
}

func TestLoadDotEnvNeverOverridesRealEnv(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DOTENV_TEST_PW=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTENV_TEST_PW", "from-env")

	LoadDotEnv(cfg)
	if got := os.Getenv("DOTENV_TEST_PW"); got != "from-env" {
		t.Errorf("DOTENV_TEST_PW = %q, want from-env (real env must win)", got)
	}
}

func TestLoadDotEnvMissingFileIsNoop(t *testing.T) {
	LoadDotEnv(filepath.Join(t.TempDir(), "config.yaml")) // must not panic or log fatal
}

func TestLoadDotEnvFeedsPstoreInterpolation(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, ".env"),
		[]byte("PSTORE_DOTENV_HOST=pstore.example.com\nPSTORE_DOTENV_USER=mon\nPSTORE_DOTENV_PW=s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
arrays:
  - name: a1
    endpoint: "https://${PSTORE_DOTENV_HOST}/api/rest"
    username: "${PSTORE_DOTENV_USER}"
    password: "${PSTORE_DOTENV_PW}"
    insecureSkipVerify: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"PSTORE_DOTENV_HOST", "PSTORE_DOTENV_USER", "PSTORE_DOTENV_PW"} {
		t.Setenv(v, "")
		_ = os.Unsetenv(v)
	}

	LoadDotEnv(cfgPath)

	cfg := &models.Config{Arrays: []models.ArrayConfig{
		{
			Name:     "a1",
			Endpoint: "https://${PSTORE_DOTENV_HOST}/api/rest",
			Username: "${PSTORE_DOTENV_USER}",
			Password: "${PSTORE_DOTENV_PW}",
		},
	}}
	if err := ResolveSecrets(cfg); err != nil {
		t.Fatalf("ResolveSecrets: %v", err)
	}
	a := cfg.Arrays[0]
	if a.Endpoint != "https://pstore.example.com/api/rest" || a.Username != "mon" || a.Password != "s3cret" {
		t.Errorf("interpolated array = %+v", a)
	}
}
