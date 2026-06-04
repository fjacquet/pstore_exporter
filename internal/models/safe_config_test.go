package models

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

const oneArray = `
server: {host: "0.0.0.0", port: "9101", uri: "/metrics"}
arrays:
  - {name: p1, endpoint: "https://10.0.0.1/api/rest", username: u, password: p}
`

const twoArrays = `
server: {host: "0.0.0.0", port: "9101", uri: "/metrics"}
arrays:
  - {name: p1, endpoint: "https://10.0.0.1/api/rest", username: u, password: p}
  - {name: p2, endpoint: "https://10.0.0.2/api/rest", username: u, password: p}
`

func TestReloadDetectsArrayChange(t *testing.T) {
	sc := NewSafeConfig(&Config{Arrays: []ArrayConfig{{Name: "p1", Endpoint: "https://10.0.0.1/api/rest", Username: "u", Password: "p"}}}, nil)

	path := writeConfigFile(t, oneArray)
	changed, err := sc.ReloadConfig(path)
	if err != nil {
		t.Fatalf("reload same arrays: %v", err)
	}
	if changed {
		t.Error("expected arraysChanged=false for identical array set")
	}

	path2 := writeConfigFile(t, twoArrays)
	changed, err = sc.ReloadConfig(path2)
	if err != nil {
		t.Fatalf("reload new arrays: %v", err)
	}
	if !changed {
		t.Error("expected arraysChanged=true when an array is added")
	}
	if len(sc.Get().Arrays) != 2 {
		t.Errorf("expected 2 arrays after reload, got %d", len(sc.Get().Arrays))
	}
}

func TestReloadRejectsInvalidConfigWithoutMutating(t *testing.T) {
	sc := NewSafeConfig(&Config{Arrays: []ArrayConfig{{Name: "p1", Endpoint: "https://10.0.0.1/api/rest", Username: "u", Password: "p"}}}, nil)

	badPath := writeConfigFile(t, "server: {port: \"9101\"}\narrays: []\n")
	if _, err := sc.ReloadConfig(badPath); err == nil {
		t.Fatal("expected validation error for config with no arrays")
	}
	if len(sc.Get().Arrays) != 1 {
		t.Errorf("running config should be unchanged after failed reload, got %d arrays", len(sc.Get().Arrays))
	}
}

func TestReloadAppliesResolver(t *testing.T) {
	resolverCalled := false
	resolver := func(c *Config) error {
		resolverCalled = true
		for i := range c.Arrays {
			c.Arrays[i].Password = "resolved"
		}
		return nil
	}
	sc := NewSafeConfig(&Config{Arrays: []ArrayConfig{{Name: "p1", Endpoint: "https://10.0.0.1/api/rest", Username: "u", Password: "p"}}}, resolver)

	path := writeConfigFile(t, oneArray)
	if _, err := sc.ReloadConfig(path); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !resolverCalled {
		t.Error("expected resolver to be invoked during reload")
	}
	if sc.Get().Arrays[0].Password != "resolved" {
		t.Errorf("expected resolver to set password, got %q", sc.Get().Arrays[0].Password)
	}
}
