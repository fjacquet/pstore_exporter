# PowerStore Exporter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `pstore_exporter`, a Dell PowerStore Prometheus + OTLP exporter that mirrors the architecture of `pflex_exporter` (collection loop → immutable snapshot → dual export), with Grafana dashboards, docker-compose, Kubernetes manifests, MkDocs docs, and CI.

**Architecture:** A single background collection loop polls each configured PowerStore array in parallel (errgroup, graceful degradation), builds an immutable `Snapshot` of `[]Sample`, and publishes it to a `SnapshotStore`. A Prometheus unchecked collector and an OTLP periodic exporter both read from the store. PowerStore is reached via Dell's typed `gopowerstore` client for auth + topology + per-entity metrics; an auto-detected bulk-CSV path (PowerStoreOS ≥ 4.1) uses the client's raw `APIClient()`. Both metric paths emit identical metric names and label sets.

**Tech Stack:** Go 1.26, `github.com/dell/gopowerstore` v1.22.0, `prometheus/client_golang`, OpenTelemetry SDK, `spf13/cobra`, `sirupsen/logrus`, `fsnotify`, `gopkg.in/yaml.v2`, `golang.org/x/sync/errgroup`.

**Template:** `/Users/fjacquet/Projects/pflex_exporter` (TEMPLATE). Many files copy-adapt with these global renames unless a task says otherwise:
- module path `github.com/fjacquet/pflex_exporter` → `github.com/fjacquet/pstore_exporter`
- package `powerflex` → `powerstore`; dir `internal/powerflex/` → `internal/powerstore/`
- identifiers/labels `cluster` → `array`, `Cluster`→`Array` (for the per-array config/snapshot; keep `cluster_id` label as the PowerStore cluster's global id)
- metric prefix `pflex_` → `powerstore_`
- default port `2112` → `9101`

**Out of scope for v1 (YAGNI):** Kubernetes workload enricher (the `Enricher`/k8s PV-label feature in the template), Gen2-style decimation. Keep the collector simple. Kubernetes *deploy manifests* ARE in scope.

---

## File Structure

```
pstore_exporter/
├── main.go                              # CLI, HTTP server, collection loop wiring, hot reload
├── go.mod / go.sum
├── config.yaml                          # example config
├── Makefile / Dockerfile / docker-compose.yml / docker-compose.ghcr.yml
├── prometheus.yml / otel-collector-config.yaml / mkdocs.yml / .gitignore / README.md / LICENSE
├── internal/
│   ├── models/
│   │   ├── config.go                    # Config, ArrayConfig, validation, getters
│   │   └── safe_config.go               # SafeConfig (RWMutex) + ReloadConfig
│   ├── utils/  env.go  file.go          # ${ENV} interpolation, passwordFile, YAML read
│   ├── logging/ logging.go              # log file init
│   ├── telemetry/ manager.go            # OTel tracer provider init
│   ├── config/ watcher.go               # SIGHUP + fsnotify hot reload
│   └── powerstore/
│       ├── metrics.go                   # Sample, Label, baseLabels, label builders, toSnake
│       ├── topology.go                  # Topology struct + lookup indices
│       ├── snapshot.go                  # ArraySnapshot, Snapshot, SnapshotStore
│       ├── interface.go                 # Client interface
│       ├── client.go                    # ArrayClient over gopowerstore
│       ├── capability.go                # bulk-API capability detection
│       ├── perentity.go                 # per-entity metrics via gopowerstore
│       ├── derive_perentity.go          # gopowerstore responses → []Sample
│       ├── bulk.go                      # bulk-CSV path via raw APIClient()
│       ├── derive_bulk.go               # CSV rows → []Sample
│       ├── prometheus.go                # unchecked PromCollector
│       ├── otlp.go                      # OTLP observable gauges + periodic push
│       ├── tracing.go                   # OTel span helper
│       ├── collector.go                 # background loop, per-array fetch
│       └── *_test.go                    # mock gateway + unit tests
├── grafana/ block/ file/ provisioning/
├── deploy/ kubernetes/ prometheus/ pstore_exporter.service pstore_exporter.env.example
├── docs/  (MkDocs Material)
└── .github/workflows/ ci.yml release.yml docs.yml
```

---

## Phase 1 — Scaffold, models, utils, logging, telemetry

### Task 1: Initialize the Go module and base files

**Files:**
- Create: `go.mod`, `.gitignore`, `LICENSE`

- [ ] **Step 1: Init module**

Run from `/Users/fjacquet/Projects/pstore_exporter`:
```bash
go mod init github.com/fjacquet/pstore_exporter
go get github.com/dell/gopowerstore@v1.22.0
go get github.com/prometheus/client_golang@v1.23.2
go get github.com/sirupsen/logrus@v1.9.4
go get github.com/spf13/cobra@v1.10.2
go get github.com/fsnotify/fsnotify@v1.10.1
go get gopkg.in/yaml.v2@v2.4.0
go get golang.org/x/sync@v0.20.0
go get go.opentelemetry.io/otel@v1.44.0
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc@v1.44.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.44.0
go get go.opentelemetry.io/otel/sdk@v1.44.0
go get go.opentelemetry.io/otel/sdk/metric@v1.44.0
```

- [ ] **Step 2: Copy infra files**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/.gitignore .gitignore
cp /Users/fjacquet/Projects/pflex_exporter/LICENSE LICENSE
```

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum .gitignore LICENSE
git commit -m "chore: initialize pstore_exporter module"
```

### Task 2: Port `internal/utils` (env interpolation + file read)

**Files:**
- Create: `internal/utils/env.go`, `internal/utils/file.go`, `internal/utils/env_test.go`

- [ ] **Step 1: Copy verbatim**

```bash
mkdir -p internal/utils
cp /Users/fjacquet/Projects/pflex_exporter/internal/utils/env.go internal/utils/env.go
cp /Users/fjacquet/Projects/pflex_exporter/internal/utils/file.go internal/utils/file.go
cp /Users/fjacquet/Projects/pflex_exporter/internal/utils/env_test.go internal/utils/env_test.go 2>/dev/null || true
```

- [ ] **Step 2: Fix import paths** — in every copied file replace `github.com/fjacquet/pflex_exporter` with `github.com/fjacquet/pstore_exporter`.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/utils/...`
Expected: PASS (or "no test files" if env_test.go absent — then write a minimal test asserting `${FOO}` interpolation and missing-var error).

- [ ] **Step 4: Commit**

```bash
git add internal/utils
git commit -m "feat: add env interpolation and file utils"
```

### Task 3: Write `internal/models/config.go` (Array-based config)

**Files:**
- Create: `internal/models/config.go`
- Test: `internal/models/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/models/...`
Expected: FAIL (undefined `ArrayConfig`, `Config`).

- [ ] **Step 3: Write the implementation**

```go
// Package models defines configuration and shared data structures for the PowerStore exporter.
package models

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ArrayConfig holds the connection details for one PowerStore array. One exporter
// process monitors many arrays; Name becomes the `array` label on every metric.
type ArrayConfig struct {
	Name               string `yaml:"name"`
	Endpoint           string `yaml:"endpoint"` // https://<ip>/api/rest
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	PasswordFile       string `yaml:"passwordFile"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	// Interval is the per-entity metrics interval for the fallback path
	// (Twenty_Sec | Five_Mins | One_Hour | One_Day). Defaults to Five_Mins.
	Interval string `yaml:"interval"`
}

// MetricsInterval returns the configured per-entity interval or the Five_Mins default.
func (a ArrayConfig) MetricsInterval() string {
	if a.Interval == "" {
		return "Five_Mins"
	}
	return a.Interval
}

// MaskPassword returns a masked password suitable for logging.
func (a ArrayConfig) MaskPassword() string {
	if len(a.Password) <= 8 {
		return "****"
	}
	return a.Password[:2] + "****" + a.Password[len(a.Password)-2:]
}

// OTelExportConfig holds settings shared by the metrics-push and tracing exporters.
type OTelExportConfig struct {
	Enabled      bool    `yaml:"enabled"`
	Endpoint     string  `yaml:"endpoint"`
	Insecure     bool    `yaml:"insecure"`
	Interval     string  `yaml:"interval"`
	SamplingRate float64 `yaml:"samplingRate"`
}

// Config is the complete application configuration.
type Config struct {
	Server struct {
		Host    string `yaml:"host"`
		Port    string `yaml:"port"`
		URI     string `yaml:"uri"`
		LogName string `yaml:"logName"`
	} `yaml:"server"`

	Collection struct {
		Interval string `yaml:"interval"`
		Timeout  string `yaml:"timeout"`
	} `yaml:"collection"`

	OpenTelemetry struct {
		Metrics OTelExportConfig `yaml:"metrics"`
		Tracing OTelExportConfig `yaml:"tracing"`
	} `yaml:"opentelemetry"`

	Arrays []ArrayConfig `yaml:"arrays"`
}

// SetDefaults fills optional fields with sensible defaults.
func (c *Config) SetDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == "" {
		c.Server.Port = "9101"
	}
	if c.Server.URI == "" {
		c.Server.URI = "/metrics"
	}
	if c.Collection.Interval == "" {
		c.Collection.Interval = "30s"
	}
	if c.Collection.Timeout == "" {
		c.Collection.Timeout = "20s"
	}
	if c.OpenTelemetry.Metrics.Interval == "" {
		c.OpenTelemetry.Metrics.Interval = c.Collection.Interval
	}
}

// Validate applies defaults then checks the configuration.
func (c *Config) Validate() error {
	c.SetDefaults()
	if err := c.validateServer(); err != nil {
		return err
	}
	if err := c.validateCollection(); err != nil {
		return err
	}
	if err := c.validateArrays(); err != nil {
		return err
	}
	if err := c.validateOTel("metrics", c.OpenTelemetry.Metrics); err != nil {
		return err
	}
	return c.validateOTel("tracing", c.OpenTelemetry.Tracing)
}

func (c *Config) validateServer() error {
	if c.Server.Host == "" {
		return errors.New("server host is required")
	}
	if err := validatePort(c.Server.Port); err != nil {
		return fmt.Errorf("invalid server port: %s", c.Server.Port)
	}
	if c.Server.URI == "" {
		return errors.New("server URI is required")
	}
	return nil
}

func (c *Config) validateCollection() error {
	if _, err := time.ParseDuration(c.Collection.Interval); err != nil {
		return fmt.Errorf("invalid collection interval '%s': %w (expected 30s, 1m)", c.Collection.Interval, err)
	}
	if _, err := time.ParseDuration(c.Collection.Timeout); err != nil {
		return fmt.Errorf("invalid collection timeout '%s': %w (expected 20s)", c.Collection.Timeout, err)
	}
	return nil
}

func (c *Config) validateArrays() error {
	if len(c.Arrays) == 0 {
		return errors.New("at least one array must be configured")
	}
	seen := make(map[string]struct{}, len(c.Arrays))
	for i, a := range c.Arrays {
		if a.Name == "" {
			return fmt.Errorf("array[%d]: name is required", i)
		}
		if _, dup := seen[a.Name]; dup {
			return fmt.Errorf("duplicate array name: %s", a.Name)
		}
		seen[a.Name] = struct{}{}
		if a.Endpoint == "" {
			return fmt.Errorf("array %q: endpoint is required (e.g. https://10.0.0.1/api/rest)", a.Name)
		}
		if a.Username == "" {
			return fmt.Errorf("array %q: username is required", a.Name)
		}
		if a.Password == "" {
			return fmt.Errorf("array %q: password is required (set password or passwordFile)", a.Name)
		}
	}
	return nil
}

func (c *Config) validateOTel(name string, o OTelExportConfig) error {
	if !o.Enabled {
		return nil
	}
	if o.Endpoint == "" {
		return fmt.Errorf("opentelemetry.%s endpoint is required when enabled", name)
	}
	host, port, err := splitHostPort(o.Endpoint)
	if err != nil || host == "" {
		return fmt.Errorf("invalid opentelemetry.%s endpoint: %s (expected host:port)", name, o.Endpoint)
	}
	if err := validatePort(port); err != nil {
		return fmt.Errorf("invalid opentelemetry.%s endpoint port: %s", name, port)
	}
	if name == "metrics" {
		if _, err := time.ParseDuration(o.Interval); err != nil {
			return fmt.Errorf("invalid opentelemetry.metrics interval '%s': %w", o.Interval, err)
		}
	}
	if name == "tracing" && (o.SamplingRate < 0.0 || o.SamplingRate > 1.0) {
		return fmt.Errorf("opentelemetry.tracing samplingRate must be between 0.0 and 1.0, got %f", o.SamplingRate)
	}
	return nil
}

// GetServerAddress returns host:port for the HTTP server.
func (c *Config) GetServerAddress() string { return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port) }

// GetCollectionInterval returns the background loop period.
func (c *Config) GetCollectionInterval() time.Duration { return mustDuration(c.Collection.Interval, 30*time.Second) }

// GetCollectionTimeout returns the per-array timeout.
func (c *Config) GetCollectionTimeout() time.Duration { return mustDuration(c.Collection.Timeout, 20*time.Second) }

// GetMetricsPushInterval returns the OTLP metric push period.
func (c *Config) GetMetricsPushInterval() time.Duration {
	return mustDuration(c.OpenTelemetry.Metrics.Interval, c.GetCollectionInterval())
}

// IsOTelMetricsEnabled reports whether OTLP metric push is enabled.
func (c *Config) IsOTelMetricsEnabled() bool { return c.OpenTelemetry.Metrics.Enabled }

// IsOTelTracingEnabled reports whether OTLP tracing is enabled.
func (c *Config) IsOTelTracingEnabled() bool { return c.OpenTelemetry.Tracing.Enabled }

func mustDuration(s string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

func validatePort(portStr string) error {
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func splitHostPort(endpoint string) (host, port string, err error) {
	lastColon := -1
	for i := len(endpoint) - 1; i >= 0; i-- {
		if endpoint[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon == -1 {
		return "", "", errors.New("missing port in endpoint")
	}
	host = endpoint[:lastColon]
	port = endpoint[lastColon+1:]
	if host == "" || port == "" {
		return "", "", errors.New("invalid host:port format")
	}
	return host, port, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/models/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/models/config.go internal/models/config_test.go
git commit -m "feat: add array-based exporter configuration"
```

### Task 4: Port `internal/models/safe_config.go`

**Files:**
- Create: `internal/models/safe_config.go`

- [ ] **Step 1: Copy and adapt**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/internal/models/safe_config.go internal/models/safe_config.go
```
Then: replace import path `pflex_exporter`→`pstore_exporter`. The template `ReloadConfig` returns `clustersChanged bool` based on the cluster set; rename that to `arraysChanged` and base it on the `Arrays` slice (compare array names). Drop any reference to PowerFlex-specific fields. Use `internal/utils` for env interpolation and file reading exactly as the template does.

- [ ] **Step 2: Write/adapt the test**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/internal/models/safe_config_test.go internal/models/safe_config_test.go 2>/dev/null || true
```
If present, adapt cluster→array and ensure it compiles. If absent, write a test that loads a temp YAML file with one array and asserts `Get()` returns it, then reloads with two arrays and asserts `arraysChanged == true`.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/models/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/models/safe_config.go internal/models/safe_config_test.go
git commit -m "feat: add thread-safe config with reload"
```

### Task 5: Port `logging`, `telemetry`, `config/watcher`

**Files:**
- Create: `internal/logging/logging.go`, `internal/telemetry/manager.go`, `internal/config/watcher.go`

- [ ] **Step 1: Copy verbatim and fix imports**

```bash
mkdir -p internal/logging internal/telemetry internal/config
cp /Users/fjacquet/Projects/pflex_exporter/internal/logging/logging.go internal/logging/logging.go
cp /Users/fjacquet/Projects/pflex_exporter/internal/telemetry/manager.go internal/telemetry/manager.go
cp /Users/fjacquet/Projects/pflex_exporter/internal/config/watcher.go internal/config/watcher.go
```
In all three: replace `pflex_exporter`→`pstore_exporter`. In `watcher.go` rename any `clustersChanged`→`arraysChanged` to match Task 4. In `telemetry/manager.go` change the service-name string `pflex-exporter`→`pstore-exporter`.

- [ ] **Step 2: Build**

Run: `go build ./internal/...`
Expected: success (powerstore package not yet present; that's fine — these three don't import it).

- [ ] **Step 3: Commit**

```bash
git add internal/logging internal/telemetry internal/config
git commit -m "feat: add logging, telemetry, and config watcher"
```

---

## Phase 2 — Sample model, snapshot, prometheus collector (no PowerStore I/O yet)

### Task 6: Create `internal/powerstore/metrics.go` (Sample/Label + label builders)

**Files:**
- Create: `internal/powerstore/metrics.go`
- Test: `internal/powerstore/metrics_test.go`

- [ ] **Step 1: Write the failing test**

```go
package powerstore

import "testing"

func TestBaseLabels(t *testing.T) {
	got := baseLabels("p1", "CLU-1")
	if len(got) != 2 || got[0].Name != "array" || got[0].Value != "p1" || got[1].Name != "cluster_id" {
		t.Fatalf("unexpected base labels: %+v", got)
	}
}

func TestVolumeLabelsCanonicalOrder(t *testing.T) {
	got := volumeLabels("p1", "CLU-1", "vol1", "v-1", "appl-1", "ApplianceA", "vgA", "vg-1")
	names := make([]string, len(got))
	for i, l := range got {
		names[i] = l.Name
	}
	want := []string{"array", "cluster_id", "volume_name", "volume_id", "appliance_id", "appliance_name", "volume_group_name", "volume_group_id"}
	if len(names) != len(want) {
		t.Fatalf("len mismatch: %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("label[%d]=%q want %q", i, names[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/...`
Expected: FAIL (undefined `baseLabels`, `volumeLabels`).

- [ ] **Step 3: Write the implementation**

```go
// Package powerstore provides the Dell PowerStore client, metric collection, and the
// dual (Prometheus + OTLP) export paths.
package powerstore

import "strings"

// Label is a single metric label name-value pair.
type Label struct {
	Name  string
	Value string
}

// Sample is one exported metric data point. The first label is always "array".
type Sample struct {
	Name   string
	Labels []Label
	Value  float64
}

// baseLabels returns the array identity labels every metric carries.
func baseLabels(arrayName, clusterID string) []Label {
	return []Label{
		{Name: "array", Value: arrayName},
		{Name: "cluster_id", Value: clusterID},
	}
}

// volumeLabels builds the canonical Volume label set so the bulk and per-entity paths
// emit identical label keys. Inapplicable values are passed empty.
func volumeLabels(arrayName, clusterID, volName, volID, applID, applName, vgName, vgID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"volume_name", volName},
		Label{"volume_id", volID},
		Label{"appliance_id", applID},
		Label{"appliance_name", applName},
		Label{"volume_group_name", vgName},
		Label{"volume_group_id", vgID},
	)
}

// applianceLabels builds the canonical Appliance label set.
func applianceLabels(arrayName, clusterID, applName, applID, model string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"appliance_name", applName},
		Label{"appliance_id", applID},
		Label{"model", model},
	)
}

// volumeGroupLabels builds the canonical VolumeGroup label set.
func volumeGroupLabels(arrayName, clusterID, vgName, vgID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"volume_group_name", vgName},
		Label{"volume_group_id", vgID},
	)
}

// fileSystemLabels builds the canonical FileSystem label set.
func fileSystemLabels(arrayName, clusterID, fsName, fsID, nasName, nasID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"file_system_name", fsName},
		Label{"file_system_id", fsID},
		Label{"nas_server_name", nasName},
		Label{"nas_server_id", nasID},
	)
}

// nasLabels builds the canonical NAS server label set.
func nasLabels(arrayName, clusterID, nasName, nasID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"nas_server_name", nasName},
		Label{"nas_server_id", nasID},
	)
}

// portLabels builds the canonical port label set (kind is "eth" or "fc").
func portLabels(arrayName, clusterID, portName, portID, kind, applID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"port_name", portName},
		Label{"port_id", portID},
		Label{"port_type", kind},
		Label{"appliance_id", applID},
	)
}

// driveLabels builds the canonical drive label set.
func driveLabels(arrayName, clusterID, driveID, applID string) []Label {
	return append(baseLabels(arrayName, clusterID),
		Label{"drive_id", driveID},
		Label{"appliance_id", applID},
	)
}

// toSnake converts camelCase to snake_case for metric name fragments.
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/powerstore/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/metrics.go internal/powerstore/metrics_test.go
git commit -m "feat: add powerstore sample model and label builders"
```

### Task 7: Create `internal/powerstore/snapshot.go`

**Files:**
- Create: `internal/powerstore/snapshot.go`
- Test: `internal/powerstore/snapshot_test.go`

- [ ] **Step 1: Write the failing test**

```go
package powerstore

import "testing"

func TestBuildSnapshotIndexesByName(t *testing.T) {
	cs := &ArraySnapshot{Array: "p1", Up: true, Samples: []Sample{
		{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 10},
		{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 20},
		{Name: "powerstore_volume_write_iops", Labels: []Label{{"array", "p1"}}, Value: 5},
	}}
	snap := BuildSnapshot([]*ArraySnapshot{cs, nil})
	if got := len(snap.SamplesByName("powerstore_volume_read_iops")); got != 2 {
		t.Fatalf("want 2 read_iops samples, got %d", got)
	}
	if len(snap.MetricNames()) != 2 {
		t.Fatalf("want 2 metric names, got %d", len(snap.MetricNames()))
	}
}

func TestSnapshotStoreLoadStore(t *testing.T) {
	st := NewSnapshotStore()
	if st.Load() == nil {
		t.Fatal("expected non-nil seed snapshot")
	}
	st.Store(BuildSnapshot([]*ArraySnapshot{{Array: "p1", Up: true}}))
	if _, ok := st.Load().PerArray["p1"]; !ok {
		t.Fatal("expected p1 in stored snapshot")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/... -run Snapshot`
Expected: FAIL (undefined `ArraySnapshot`, `BuildSnapshot`).

- [ ] **Step 3: Write the implementation**

Copy `/Users/fjacquet/Projects/pflex_exporter/internal/powerflex/snapshot.go` and apply: `package powerstore`; rename `ClusterSnapshot`→`ArraySnapshot`, field `Cluster`→`Array`, map `PerCluster`→`PerArray`; replace the `Generation string` field with `BulkCapable bool`. Resulting file:

```go
package powerstore

import (
	"sync"
	"time"
)

// ArraySnapshot is the collected state for a single array at one collection cycle.
type ArraySnapshot struct {
	Array       string
	Up          bool
	BulkCapable bool
	ScrapeError string
	LastScrape  time.Time
	Samples     []Sample
}

// Snapshot is an immutable view of all arrays' samples, indexed by metric name.
type Snapshot struct {
	PerArray map[string]*ArraySnapshot
	byName   map[string][]Sample
	names    []string
}

// BuildSnapshot assembles an immutable Snapshot from per-array results.
func BuildSnapshot(arrays []*ArraySnapshot) *Snapshot {
	snap := &Snapshot{
		PerArray: make(map[string]*ArraySnapshot, len(arrays)),
		byName:   make(map[string][]Sample),
	}
	for _, as := range arrays {
		if as == nil {
			continue
		}
		snap.PerArray[as.Array] = as
		for _, s := range as.Samples {
			snap.byName[s.Name] = append(snap.byName[s.Name], s)
		}
	}
	snap.names = make([]string, 0, len(snap.byName))
	for name := range snap.byName {
		snap.names = append(snap.names, name)
	}
	return snap
}

// SamplesByName returns all samples (across arrays) for a metric name.
func (s *Snapshot) SamplesByName(name string) []Sample { return s.byName[name] }

// MetricNames returns the distinct metric names present in the snapshot.
func (s *Snapshot) MetricNames() []string { return s.names }

// SnapshotStore holds the latest published Snapshot under an RWMutex.
type SnapshotStore struct {
	mu      sync.RWMutex
	current *Snapshot
}

// NewSnapshotStore returns a store seeded with an empty snapshot.
func NewSnapshotStore() *SnapshotStore { return &SnapshotStore{current: BuildSnapshot(nil)} }

// Load returns the current snapshot (safe for concurrent readers).
func (st *SnapshotStore) Load() *Snapshot {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.current
}

// Store publishes a new snapshot.
func (st *SnapshotStore) Store(s *Snapshot) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.current = s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/powerstore/... -run Snapshot`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/snapshot.go internal/powerstore/snapshot_test.go
git commit -m "feat: add array snapshot model and store"
```

### Task 8: Create `internal/powerstore/prometheus.go`

**Files:**
- Create: `internal/powerstore/prometheus.go`
- Test: `internal/powerstore/prometheus_test.go`

- [ ] **Step 1: Write the failing test**

```go
package powerstore

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPromCollectorEmitsUpAndSamples(t *testing.T) {
	store := NewSnapshotStore()
	store.Store(BuildSnapshot([]*ArraySnapshot{{
		Array: "p1", Up: true, BulkCapable: true,
		Samples: []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 42}},
	}}))
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewPromCollector(store))

	out := testutil.CollectAndCount(NewPromCollector(store), "powerstore_volume_read_iops")
	if out != 1 {
		t.Fatalf("want 1 read_iops series, got %d", out)
	}
	mf, _ := reg.Gather()
	var sawUp bool
	for _, m := range mf {
		if strings.HasSuffix(m.GetName(), "powerstore_up") {
			sawUp = true
		}
	}
	if !sawUp {
		t.Fatal("expected powerstore_up metric")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/... -run PromCollector`
Expected: FAIL (undefined `NewPromCollector`).

- [ ] **Step 3: Write the implementation**

Copy `/Users/fjacquet/Projects/pflex_exporter/internal/powerflex/prometheus.go` and apply: `package powerstore`; metric names `pflex_up`→`powerstore_up`, `pflex_last_scrape_timestamp_seconds`→`powerstore_last_scrape_timestamp_seconds`; replace the `generation` Desc with a `bulkAPI` Desc named `powerstore_array_bulk_api` (labels `["array"]`, value 1 if `BulkCapable`); label `cluster`→`array`; iterate `snap.PerArray`; help text "PowerFlex metric"→"PowerStore metric"; keep `sampleLabelNames`/`sampleLabelValues` and the duplicate-signature dedupe exactly as in the template (see TEMPLATE prometheus.go:48-107).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/powerstore/... -run PromCollector`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/prometheus.go internal/powerstore/prometheus_test.go
git commit -m "feat: add prometheus collector reading from snapshot"
```

---

## Phase 3 — Topology, client, capability detection

### Task 9: Create `internal/powerstore/topology.go`

**Files:**
- Create: `internal/powerstore/topology.go`
- Test: `internal/powerstore/topology_test.go`

- [ ] **Step 1: Write the failing test**

```go
package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestTopologyIndices(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: 1, Name: "CLU"},
		[]gopowerstore.ApplianceInstance{{ID: "appl-1", Name: "ApplianceA", Model: "PS1000"}},
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"}},
		[]gopowerstore.VolumeGroup{{ID: "vg-1", Name: "vgA"}},
		nil, nil, nil, nil,
	)
	if topo.ApplianceName("appl-1") != "ApplianceA" {
		t.Fatalf("appliance lookup failed: %q", topo.ApplianceName("appl-1"))
	}
	if topo.ClusterID() != "1" {
		t.Fatalf("cluster id: %q", topo.ClusterID())
	}
}
```

> Note: confirm `gopowerstore.Volume` exposes `ApplianceID` and `VolumeGroup` membership. If volume→VG mapping is only available via `GetVolumeGroupsByVolumeID`, populate `volumeVGID` in `client.GetTopology` (Task 11) by iterating volume groups' member volume IDs instead; adjust `NewTopology` signature accordingly. The test above is the contract; tighten field names to the actual struct when you run Step 2.

- [ ] **Step 2: Run test to verify it fails (and verify struct fields)**

Run: `go doc github.com/dell/gopowerstore.Volume` and `go doc github.com/dell/gopowerstore.ApplianceInstance` to confirm field names, then `go test ./internal/powerstore/... -run Topology`.
Expected: FAIL (undefined `NewTopology`).

- [ ] **Step 3: Write the implementation**

```go
package powerstore

import (
	"strconv"

	"github.com/dell/gopowerstore"
)

// Topology is one array's inventory plus lookup indices used to resolve metric labels.
type Topology struct {
	Cluster      gopowerstore.Cluster
	Appliances   []gopowerstore.ApplianceInstance
	Volumes      []gopowerstore.Volume
	VolumeGroups []gopowerstore.VolumeGroup
	NASServers   []gopowerstore.NAS
	FileSystems  []gopowerstore.FileSystem
	FCPorts      []gopowerstore.FcPort
	EthPorts     []gopowerstore.EthPort

	applianceName map[string]string // appliance id -> name
	vgName        map[string]string // vg id -> name
	volumeVGID    map[string]string // volume id -> vg id
	nasName       map[string]string // nas id -> name
}

// NewTopology builds indices from the inventory slices.
func NewTopology(
	cluster gopowerstore.Cluster,
	appliances []gopowerstore.ApplianceInstance,
	volumes []gopowerstore.Volume,
	vgs []gopowerstore.VolumeGroup,
	nas []gopowerstore.NAS,
	fs []gopowerstore.FileSystem,
	fc []gopowerstore.FcPort,
	eth []gopowerstore.EthPort,
) *Topology {
	t := &Topology{
		Cluster: cluster, Appliances: appliances, Volumes: volumes, VolumeGroups: vgs,
		NASServers: nas, FileSystems: fs, FCPorts: fc, EthPorts: eth,
		applianceName: make(map[string]string),
		vgName:        make(map[string]string),
		volumeVGID:    make(map[string]string),
		nasName:       make(map[string]string),
	}
	for _, a := range appliances {
		t.applianceName[a.ID] = a.Name
	}
	for _, g := range vgs {
		t.vgName[g.ID] = g.Name
		for _, v := range g.Volumes { // VolumeGroup.Volumes is []Volume of members
			t.volumeVGID[v.ID] = g.ID
		}
	}
	for _, n := range nas {
		t.nasName[n.ID] = n.Name
	}
	return t
}

// ClusterID returns the PowerStore cluster's global id as a string.
func (t *Topology) ClusterID() string { return strconv.Itoa(t.Cluster.ID) }

// ApplianceName resolves an appliance id to its name (empty if unknown).
func (t *Topology) ApplianceName(id string) string { return t.applianceName[id] }

// VolumeGroupOf returns (vgID, vgName) for a volume id (empty if none).
func (t *Topology) VolumeGroupOf(volID string) (string, string) {
	vgID := t.volumeVGID[volID]
	return vgID, t.vgName[vgID]
}

// NASName resolves a NAS server id to its name.
func (t *Topology) NASName(id string) string { return t.nasName[id] }
```

> When you run Step 2, adjust `gopowerstore.Cluster.ID` type handling (it may be `int`), `VolumeGroup.Volumes` field name, and `appliance.Model` to the real struct. Keep the public method signatures stable — later tasks depend on `ClusterID()`, `ApplianceName()`, `VolumeGroupOf()`, `NASName()`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/powerstore/... -run Topology`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/topology.go internal/powerstore/topology_test.go
git commit -m "feat: add topology model with label-resolution indices"
```

### Task 10: Create `internal/powerstore/interface.go` and `tracing.go`

**Files:**
- Create: `internal/powerstore/interface.go`, `internal/powerstore/tracing.go`

- [ ] **Step 1: Copy tracing helper**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/internal/powerflex/tracing.go internal/powerstore/tracing.go
```
Apply: `package powerstore`; fix import path; service-name string `pflex`→`pstore`.

- [ ] **Step 2: Write the Client interface**

```go
package powerstore

import "context"

// Client is the per-array PowerStore API abstraction. Satisfied by ArrayClient and
// mocked in tests so the collector can run without a live array.
type Client interface {
	// Name returns the configured array name (the `array` label value).
	Name() string
	// GetTopology fetches the array's inventory and builds lookup indices.
	GetTopology(ctx context.Context) (*Topology, error)
	// BulkCapable reports whether the array supports the bulk CSV metrics API
	// (PowerStoreOS >= 4.1), derived from topology/version data.
	BulkCapable(ctx context.Context, topo *Topology) bool
	// PerEntityMetrics collects metrics one entity at a time via the typed client.
	PerEntityMetrics(ctx context.Context, topo *Topology) ([]Sample, error)
	// BulkMetrics collects metrics via the bulk CSV API.
	BulkMetrics(ctx context.Context, topo *Topology) ([]Sample, error)
	// Close releases client resources.
	Close() error
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/powerstore/...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/powerstore/interface.go internal/powerstore/tracing.go
git commit -m "feat: add powerstore client interface and tracing helper"
```

### Task 11: Create `internal/powerstore/client.go` (gopowerstore wrapper) + `capability.go`

**Files:**
- Create: `internal/powerstore/client.go`, `internal/powerstore/capability.go`
- Test: `internal/powerstore/capability_test.go`

- [ ] **Step 1: Write the failing capability test**

```go
package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestBulkCapableByVersion(t *testing.T) {
	cases := map[string]bool{"4.1.0.0": true, "4.0.0.0": false, "3.6.0.0": false, "5.0.0.0": true, "": false}
	for ver, want := range cases {
		topo := &Topology{Appliances: []gopowerstore.ApplianceInstance{}}
		if got := bulkCapableFromVersion(ver); got != want {
			t.Fatalf("version %q: got %v want %v", ver, got, want)
		}
		_ = topo
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/... -run BulkCapable`
Expected: FAIL (undefined `bulkCapableFromVersion`).

- [ ] **Step 3: Write `capability.go`**

```go
package powerstore

import (
	"strconv"
	"strings"
)

// bulkCapableFromVersion returns true when a PowerStoreOS version string is >= 4.1.
// The bulk CSV metrics API (/latest_five_min_metrics) was introduced in 4.1.0.
func bulkCapableFromVersion(version string) bool {
	major, minor, ok := parseMajorMinor(version)
	if !ok {
		return false
	}
	return major > 4 || (major == 4 && minor >= 1)
}

func parseMajorMinor(version string) (major, minor int, ok bool) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return maj, min, true
}
```

- [ ] **Step 4: Write `client.go`**

```go
package powerstore

import (
	"context"
	"time"

	"github.com/dell/gopowerstore"
	"github.com/fjacquet/pstore_exporter/internal/models"
	log "github.com/sirupsen/logrus"
)

const defaultAPITimeout = 60 * time.Second

// ArrayClient is the PowerStore REST client for a single array, wrapping the typed
// gopowerstore client. It owns no token state directly — gopowerstore manages the
// login_session lifecycle internally.
type ArrayClient struct {
	name     string
	interval gopowerstore.MetricsIntervalEnum
	gp       gopowerstore.Client
}

// NewArrayClient builds a client for one array from its config.
func NewArrayClient(cfg models.ArrayConfig) (*ArrayClient, error) {
	if cfg.InsecureSkipVerify {
		log.Warnf("array %q: TLS certificate verification disabled (insecureSkipVerify=true)", cfg.Name)
	}
	opts := gopowerstore.NewClientOptions()
	opts.SetInsecure(cfg.InsecureSkipVerify)
	opts.SetDefaultTimeout(uint64(defaultAPITimeout.Seconds()))

	gp, err := gopowerstore.NewClientWithArgs(cfg.Endpoint, cfg.Username, cfg.Password, opts)
	if err != nil {
		return nil, err
	}
	return &ArrayClient{
		name:     cfg.Name,
		interval: gopowerstore.MetricsIntervalEnum(cfg.MetricsInterval()),
		gp:       gp,
	}, nil
}

// Name returns the array name.
func (c *ArrayClient) Name() string { return c.name }

// GetTopology fetches inventory and builds indices. Missing optional resources (e.g.
// file when the array is block-only) are tolerated: log and continue with empties.
func (c *ArrayClient) GetTopology(ctx context.Context) (*Topology, error) {
	cluster, err := c.gp.GetCluster(ctx)
	if err != nil {
		return nil, err
	}
	appliances, err := c.gp.GetAppliances(ctx)
	if err != nil {
		return nil, err
	}
	volumes, _ := c.gp.GetVolumes(ctx)
	vgs, _ := c.gp.GetVolumeGroups(ctx)
	nas, _ := c.gp.GetNASServers(ctx)
	fs, _ := c.gp.ListFS(ctx)
	fc, _ := c.gp.GetFCPorts(ctx)
	eth, _ := c.gp.GetEthPorts(ctx)

	return NewTopology(cluster, appliances, volumes, vgs, nas, fs, fc, eth), nil
}

// BulkCapable reports bulk-API support from the appliances' software version.
func (c *ArrayClient) BulkCapable(_ context.Context, topo *Topology) bool {
	for _, a := range topo.Appliances {
		if bulkCapableFromVersion(applianceVersion(a)) {
			return true
		}
	}
	return false
}

// Close is a no-op; gopowerstore has no explicit close.
func (c *ArrayClient) Close() error { return nil }
```

> When you run Step 5: verify the exact gopowerstore method names against `go doc github.com/dell/gopowerstore.Client` (the explore notes list `GetCluster`, `GetAppliances`/`GetAppliance`, `GetVolumes`, `GetVolumeGroups`, `GetNASServers`, `ListFS`, `GetFCPorts`, `GetEthPorts`). If `GetAppliances` (plural) does not exist, fetch via the documented list method and adjust. Implement `applianceVersion(a gopowerstore.ApplianceInstance) string` to read the software/OS version field on the appliance (check `go doc` — likely a nested release/version). If the version is not on the appliance, fetch it from a software-installed endpoint via `c.gp.APIClient().Query(...)` and cache it. `PerEntityMetrics` and `BulkMetrics` are added in Tasks 12 and 14 — until then, add temporary stubs returning `nil, nil` so the package builds.

- [ ] **Step 5: Add temporary stubs, then build + test**

Add to `client.go`:
```go
// PerEntityMetrics is implemented in perentity.go (Task 12).
// BulkMetrics is implemented in bulk.go (Task 14).
```
Add stubs in `client.go` (replaced in later tasks):
```go
func (c *ArrayClient) PerEntityMetrics(ctx context.Context, topo *Topology) ([]Sample, error) { return nil, nil }
func (c *ArrayClient) BulkMetrics(ctx context.Context, topo *Topology) ([]Sample, error)      { return nil, nil }
```
Run: `go build ./... && go test ./internal/powerstore/... -run BulkCapable`
Expected: build success, test PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/powerstore/client.go internal/powerstore/capability.go internal/powerstore/capability_test.go
git commit -m "feat: add gopowerstore array client and bulk-capability detection"
```

---

## Phase 4 — Per-entity metrics path

### Task 12: Create `derive_perentity.go` (response → samples)

**Files:**
- Create: `internal/powerstore/derive_perentity.go`
- Test: `internal/powerstore/derive_perentity_test.go`

- [ ] **Step 1: Write the failing test**

```go
package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestDeriveVolumePerfSamples(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: 1, Name: "CLU"},
		[]gopowerstore.ApplianceInstance{{ID: "appl-1", Name: "ApplianceA"}},
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"}},
		nil, nil, nil, nil, nil,
	)
	resp := gopowerstore.PerformanceMetricsByVolumeResponse{}
	resp.VolumeID = "v-1"
	resp.ReadIops = 100
	resp.WriteIops = 50
	resp.AvgReadLatency = 200
	samples := deriveVolumePerf("p1", topo, []gopowerstore.PerformanceMetricsByVolumeResponse{resp})

	if !hasSample(samples, "powerstore_volume_read_iops", 100) {
		t.Fatalf("missing read_iops sample: %+v", samples)
	}
	if !hasSample(samples, "powerstore_volume_read_latency_microseconds", 200) {
		t.Fatalf("missing read latency sample")
	}
}

func hasSample(s []Sample, name string, val float64) bool {
	for _, x := range s {
		if x.Name == name && x.Value == val {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/... -run DeriveVolumePerf`
Expected: FAIL (undefined `deriveVolumePerf`).

- [ ] **Step 3: Write the implementation**

```go
package powerstore

import "github.com/dell/gopowerstore"

// deriveVolumePerf maps the latest performance sample per volume to []Sample.
// PowerStore returns a time-series slice; we take the last (newest) element.
func deriveVolumePerf(array string, topo *Topology, resp []gopowerstore.PerformanceMetricsByVolumeResponse) []Sample {
	clusterID := topo.ClusterID()
	// Keep only the newest sample per volume id.
	latest := make(map[string]gopowerstore.PerformanceMetricsByVolumeResponse)
	for _, r := range resp {
		latest[r.VolumeID] = r // input is time-ordered ascending; last wins
	}
	var out []Sample
	for volID, r := range latest {
		volName, applID := volID, ""
		for _, v := range topo.Volumes {
			if v.ID == volID {
				volName, applID = v.Name, v.ApplianceID
				break
			}
		}
		vgID, vgName := topo.VolumeGroupOf(volID)
		labels := volumeLabels(array, clusterID, volName, volID, applID, topo.ApplianceName(applID), vgName, vgID)
		out = append(out,
			Sample{"powerstore_volume_read_iops", labels, float64(r.ReadIops)},
			Sample{"powerstore_volume_write_iops", labels, float64(r.WriteIops)},
			Sample{"powerstore_volume_total_iops", labels, float64(r.TotalIops)},
			Sample{"powerstore_volume_read_bandwidth_bytes_per_second", labels, float64(r.ReadBandwidth)},
			Sample{"powerstore_volume_write_bandwidth_bytes_per_second", labels, float64(r.WriteBandwidth)},
			Sample{"powerstore_volume_read_latency_microseconds", labels, float64(r.AvgReadLatency)},
			Sample{"powerstore_volume_write_latency_microseconds", labels, float64(r.AvgWriteLatency)},
			Sample{"powerstore_volume_avg_io_size_bytes", labels, float64(r.AvgIoSize)},
		)
	}
	return out
}

// deriveAppliancePerf maps the latest performance sample per appliance to []Sample.
func deriveAppliancePerf(array string, topo *Topology, resp []gopowerstore.PerformanceMetricsByApplianceResponse) []Sample {
	clusterID := topo.ClusterID()
	latest := make(map[string]gopowerstore.PerformanceMetricsByApplianceResponse)
	for _, r := range resp {
		latest[r.ApplianceID] = r
	}
	var out []Sample
	for applID, r := range latest {
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, applianceModel(topo, applID))
		out = append(out,
			Sample{"powerstore_appliance_read_iops", labels, float64(r.ReadIops)},
			Sample{"powerstore_appliance_write_iops", labels, float64(r.WriteIops)},
			Sample{"powerstore_appliance_total_iops", labels, float64(r.TotalIops)},
			Sample{"powerstore_appliance_read_bandwidth_bytes_per_second", labels, float64(r.ReadBandwidth)},
			Sample{"powerstore_appliance_write_bandwidth_bytes_per_second", labels, float64(r.WriteBandwidth)},
			Sample{"powerstore_appliance_read_latency_microseconds", labels, float64(r.AvgReadLatency)},
			Sample{"powerstore_appliance_write_latency_microseconds", labels, float64(r.AvgWriteLatency)},
			Sample{"powerstore_appliance_io_workload_cpu_utilization", labels, float64(r.IoWorkloadCPUUtilization)},
		)
	}
	return out
}

// deriveApplianceSpace maps the latest space sample per appliance to []Sample.
func deriveApplianceSpace(array string, topo *Topology, resp []gopowerstore.SpaceMetricsByApplianceResponse) []Sample {
	clusterID := topo.ClusterID()
	latest := make(map[string]gopowerstore.SpaceMetricsByApplianceResponse)
	for _, r := range resp {
		latest[r.ApplianceID] = r
	}
	var out []Sample
	for applID, r := range latest {
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, applianceModel(topo, applID))
		out = append(out,
			Sample{"powerstore_appliance_physical_total_bytes", labels, deref(r.PhysicalTotal)},
			Sample{"powerstore_appliance_physical_used_bytes", labels, deref(r.PhysicalUsed)},
			Sample{"powerstore_appliance_logical_provisioned_bytes", labels, deref(r.LogicalProvisioned)},
			Sample{"powerstore_appliance_logical_used_bytes", labels, deref(r.LogicalUsed)},
			Sample{"powerstore_appliance_data_reduction_ratio", labels, float64(r.DataReduction)},
			Sample{"powerstore_appliance_efficiency_ratio", labels, float64(r.EfficiencyRatio)},
			Sample{"powerstore_appliance_snapshot_savings_ratio", labels, float64(r.SnapshotSavings)},
			Sample{"powerstore_appliance_thin_savings_ratio", labels, float64(r.ThinSavings)},
		)
	}
	return out
}

func applianceModel(topo *Topology, applID string) string {
	for _, a := range topo.Appliances {
		if a.ID == applID {
			return a.Model
		}
	}
	return ""
}

func deref(p *int64) float64 {
	if p == nil {
		return 0
	}
	return float64(*p)
}
```

> When you run Step 2: verify struct field names with `go doc github.com/dell/gopowerstore.PerformanceMetricsByVolumeResponse` etc. The explore notes confirm `ReadIops`, `WriteIops`, `TotalIops` (float64), `ReadBandwidth`, `WriteBandwidth`, `AvgReadLatency`, `AvgWriteLatency`, `AvgIoSize`, and on appliance `IoWorkloadCPUUtilization`; space fields `PhysicalTotal`/`PhysicalUsed`/`LogicalProvisioned`/`LogicalUsed` are `*int64`, ratios are `float32`. Fix any mismatch and keep the metric names exactly as written (the bulk path in Task 15 must match these).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/powerstore/... -run DeriveVolumePerf`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/derive_perentity.go internal/powerstore/derive_perentity_test.go
git commit -m "feat: derive samples from per-entity metrics responses"
```

### Task 13: Implement `PerEntityMetrics` in `perentity.go`

**Files:**
- Create: `internal/powerstore/perentity.go` (move the method off the stub in `client.go`)
- Test: extend with a mock-based test in Task 16

- [ ] **Step 1: Remove the `PerEntityMetrics` stub** from `client.go` (added in Task 11 Step 5).

- [ ] **Step 2: Write `perentity.go`**

```go
package powerstore

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// PerEntityMetrics collects performance + space metrics one entity type at a time via
// the typed gopowerstore client. A failed sub-query is logged and skipped (graceful
// degradation) so a single bad entity type doesn't drop the whole array.
func (c *ArrayClient) PerEntityMetrics(ctx context.Context, topo *Topology) ([]Sample, error) {
	var samples []Sample

	// Appliances: performance + space (one call each per appliance).
	for _, a := range topo.Appliances {
		if perf, err := c.gp.PerformanceMetricsByAppliance(ctx, a.ID, c.interval); err != nil {
			log.Warnf("array %q: appliance %s perf failed: %v", c.name, a.ID, err)
		} else {
			samples = append(samples, deriveAppliancePerf(c.name, topo, perf)...)
		}
		if space, err := c.gp.SpaceMetricsByAppliance(ctx, a.ID, c.interval); err != nil {
			log.Warnf("array %q: appliance %s space failed: %v", c.name, a.ID, err)
		} else {
			samples = append(samples, deriveApplianceSpace(c.name, topo, space)...)
		}
	}

	// Volumes: performance per volume.
	for _, v := range topo.Volumes {
		if perf, err := c.gp.PerformanceMetricsByVolume(ctx, v.ID, c.interval); err != nil {
			log.Warnf("array %q: volume %s perf failed: %v", c.name, v.ID, err)
		} else {
			samples = append(samples, deriveVolumePerf(c.name, topo, perf)...)
		}
	}

	// File systems: performance per fs (file-enabled arrays only).
	for _, fs := range topo.FileSystems {
		if perf, err := c.gp.PerformanceMetricsByFileSystem(ctx, fs.ID, c.interval); err != nil {
			log.Debugf("array %q: filesystem %s perf failed: %v", c.name, fs.ID, err)
		} else {
			samples = append(samples, deriveFileSystemPerf(c.name, topo, perf)...)
		}
	}

	return samples, nil
}
```

> Add `deriveFileSystemPerf` to `derive_perentity.go` following the same pattern as `deriveVolumePerf`, using `fileSystemLabels` (resolve NAS via `fs.NasServerID` → `topo.NASName`). Confirm `gopowerstore.FileSystem` exposes `NasServerID`. Add ports/drives/VG metrics here too if their per-entity methods are confirmed (`PerformanceMetricsByFeEthPort`, `PerformanceMetricsByFeFcPort`, `PerformanceMetricsByVg`, `WearMetricsByDrive`) — mirror the volume loop. Keep metric names aligned with the bulk path.

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/powerstore/perentity.go internal/powerstore/client.go internal/powerstore/derive_perentity.go
git commit -m "feat: implement per-entity metrics collection path"
```

---

## Phase 5 — Bulk CSV path

### Task 14: Create `bulk.go` (enable + download + untar)

**Files:**
- Create: `internal/powerstore/bulk.go`
- Test: `internal/powerstore/bulk_test.go`

- [ ] **Step 1: Write the failing test (CSV/tar parsing, no network)**

```go
package powerstore

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func makeGzTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestParseBulkArchive(t *testing.T) {
	csv := "appliance_id,read_iops,write_iops\nappl-1,100,50\n"
	archive := makeGzTar(t, map[string]string{"performance_metrics_by_appliance.csv": csv})
	files, err := parseBulkArchive(archive)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rows := files["performance_metrics_by_appliance.csv"]
	if len(rows) != 1 || rows[0]["read_iops"] != "100" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/... -run ParseBulkArchive`
Expected: FAIL (undefined `parseBulkArchive`).

- [ ] **Step 3: Write `bulk.go`**

```go
package powerstore

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
)

// parseBulkArchive decompresses a gzipped tar of CSV files into a map of
// filename -> rows, where each row is a column→value map keyed by CSV header.
func parseBulkArchive(archive []byte) (map[string][]map[string]string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	out := make(map[string][]map[string]string)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		rows, err := parseCSV(tr)
		if err != nil {
			return nil, fmt.Errorf("csv %s: %w", hdr.Name, err)
		}
		out[baseName(hdr.Name)] = rows
	}
	return out, nil
}

func parseCSV(r io.Reader) ([]map[string]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 1 {
		return nil, nil
	}
	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(rec) {
				row[col] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/powerstore/... -run ParseBulkArchive`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/bulk.go internal/powerstore/bulk_test.go
git commit -m "feat: add bulk CSV archive parser"
```

### Task 15: Create `derive_bulk.go` and wire `BulkMetrics`

**Files:**
- Create: `internal/powerstore/derive_bulk.go`
- Modify: `internal/powerstore/client.go` (remove `BulkMetrics` stub), add the real method
- Test: `internal/powerstore/derive_bulk_test.go`

- [ ] **Step 1: Write the failing test (label/metric parity with per-entity)**

```go
package powerstore

import (
	"testing"

	"github.com/dell/gopowerstore"
)

func TestBulkVolumeSamplesMatchPerEntityNames(t *testing.T) {
	topo := NewTopology(
		gopowerstore.Cluster{ID: 1, Name: "CLU"},
		[]gopowerstore.ApplianceInstance{{ID: "appl-1", Name: "ApplianceA"}},
		[]gopowerstore.Volume{{ID: "v-1", Name: "vol1", ApplianceID: "appl-1"}},
		nil, nil, nil, nil, nil,
	)
	rows := []map[string]string{{
		"volume_id": "v-1", "read_iops": "100", "write_iops": "50", "total_iops": "150",
		"read_bandwidth": "1000", "write_bandwidth": "500",
		"avg_read_latency": "200", "avg_write_latency": "300", "avg_io_size": "8192",
	}}
	got := deriveBulkVolumePerf("p1", topo, rows)
	if !hasSample(got, "powerstore_volume_read_iops", 100) {
		t.Fatalf("missing read_iops: %+v", got)
	}
	// label keys must equal the per-entity volume label keys
	want := volumeLabels("p1", "1", "vol1", "v-1", "appl-1", "ApplianceA", "", "")
	for _, s := range got {
		if s.Name == "powerstore_volume_read_iops" {
			assertSameLabelKeys(t, s.Labels, want)
		}
	}
}

func assertSameLabelKeys(t *testing.T, a, b []Label) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("label count %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Fatalf("label[%d] %q != %q", i, a[i].Name, b[i].Name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/... -run BulkVolumeSamples`
Expected: FAIL (undefined `deriveBulkVolumePerf`).

- [ ] **Step 3: Write `derive_bulk.go`**

```go
package powerstore

import "strconv"

func csvFloat(row map[string]string, key string) float64 {
	v, err := strconv.ParseFloat(row[key], 64)
	if err != nil {
		return 0
	}
	return v
}

// deriveBulkVolumePerf maps performance_metrics_by_volume.csv rows to []Sample,
// emitting the SAME metric names and label keys as deriveVolumePerf.
func deriveBulkVolumePerf(array string, topo *Topology, rows []map[string]string) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, r := range rows {
		volID := r["volume_id"]
		volName, applID := volID, ""
		for _, v := range topo.Volumes {
			if v.ID == volID {
				volName, applID = v.Name, v.ApplianceID
				break
			}
		}
		vgID, vgName := topo.VolumeGroupOf(volID)
		labels := volumeLabels(array, clusterID, volName, volID, applID, topo.ApplianceName(applID), vgName, vgID)
		out = append(out,
			Sample{"powerstore_volume_read_iops", labels, csvFloat(r, "read_iops")},
			Sample{"powerstore_volume_write_iops", labels, csvFloat(r, "write_iops")},
			Sample{"powerstore_volume_total_iops", labels, csvFloat(r, "total_iops")},
			Sample{"powerstore_volume_read_bandwidth_bytes_per_second", labels, csvFloat(r, "read_bandwidth")},
			Sample{"powerstore_volume_write_bandwidth_bytes_per_second", labels, csvFloat(r, "write_bandwidth")},
			Sample{"powerstore_volume_read_latency_microseconds", labels, csvFloat(r, "avg_read_latency")},
			Sample{"powerstore_volume_write_latency_microseconds", labels, csvFloat(r, "avg_write_latency")},
			Sample{"powerstore_volume_avg_io_size_bytes", labels, csvFloat(r, "avg_io_size")},
		)
	}
	return out
}

// deriveBulkAppliancePerf maps performance_metrics_by_appliance.csv rows to []Sample.
func deriveBulkAppliancePerf(array string, topo *Topology, rows []map[string]string) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, r := range rows {
		applID := r["appliance_id"]
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, applianceModel(topo, applID))
		out = append(out,
			Sample{"powerstore_appliance_read_iops", labels, csvFloat(r, "read_iops")},
			Sample{"powerstore_appliance_write_iops", labels, csvFloat(r, "write_iops")},
			Sample{"powerstore_appliance_total_iops", labels, csvFloat(r, "total_iops")},
			Sample{"powerstore_appliance_read_bandwidth_bytes_per_second", labels, csvFloat(r, "read_bandwidth")},
			Sample{"powerstore_appliance_write_bandwidth_bytes_per_second", labels, csvFloat(r, "write_bandwidth")},
			Sample{"powerstore_appliance_read_latency_microseconds", labels, csvFloat(r, "avg_read_latency")},
			Sample{"powerstore_appliance_write_latency_microseconds", labels, csvFloat(r, "avg_write_latency")},
			Sample{"powerstore_appliance_io_workload_cpu_utilization", labels, csvFloat(r, "avg_io_workload_cpu_utilization")},
		)
	}
	return out
}

// deriveBulkApplianceSpace maps space_metrics_by_appliance.csv rows to []Sample.
func deriveBulkApplianceSpace(array string, topo *Topology, rows []map[string]string) []Sample {
	clusterID := topo.ClusterID()
	var out []Sample
	for _, r := range rows {
		applID := r["appliance_id"]
		labels := applianceLabels(array, clusterID, topo.ApplianceName(applID), applID, applianceModel(topo, applID))
		out = append(out,
			Sample{"powerstore_appliance_physical_total_bytes", labels, csvFloat(r, "last_physical_total")},
			Sample{"powerstore_appliance_physical_used_bytes", labels, csvFloat(r, "last_physical_used")},
			Sample{"powerstore_appliance_logical_provisioned_bytes", labels, csvFloat(r, "last_logical_provisioned")},
			Sample{"powerstore_appliance_logical_used_bytes", labels, csvFloat(r, "last_logical_used")},
			Sample{"powerstore_appliance_data_reduction_ratio", labels, csvFloat(r, "last_data_reduction")},
			Sample{"powerstore_appliance_efficiency_ratio", labels, csvFloat(r, "last_efficiency_ratio")},
			Sample{"powerstore_appliance_snapshot_savings_ratio", labels, csvFloat(r, "last_snapshot_savings")},
			Sample{"powerstore_appliance_thin_savings_ratio", labels, csvFloat(r, "last_thin_savings")},
		)
	}
	return out
}
```

> CSV column names above are best-effort from Dell's bulk schema (see `powerstore-metrics-exporter` bulkClient CSV mappings). When a real array or fixture is available, confirm exact headers and adjust the `csvFloat(r, "...")` keys — but DO NOT change the `Sample` names or label builders; those are the contract shared with the per-entity path.

- [ ] **Step 4: Write the real `BulkMetrics` in `client.go`** (remove the stub):

```go
// BulkMetrics enables the bulk five-minute metrics export, downloads the gzipped tar
// of CSVs via the raw authenticated API client, and derives samples. Falls back to an
// error (caller then has already chosen this path via BulkCapable) — the collector logs
// and records ScrapeError if it fails.
func (c *ArrayClient) BulkMetrics(ctx context.Context, topo *Topology) ([]Sample, error) {
	archive, err := c.downloadBulkArchive(ctx)
	if err != nil {
		return nil, err
	}
	files, err := parseBulkArchive(archive)
	if err != nil {
		return nil, err
	}
	var samples []Sample
	samples = append(samples, deriveBulkAppliancePerf(c.name, topo, files["performance_metrics_by_appliance.csv"])...)
	samples = append(samples, deriveBulkApplianceSpace(c.name, topo, files["space_metrics_by_appliance.csv"])...)
	samples = append(samples, deriveBulkVolumePerf(c.name, topo, files["performance_metrics_by_volume.csv"])...)
	// file/port/drive CSVs added once headers are confirmed
	return samples, nil
}
```

> Implement `downloadBulkArchive(ctx) ([]byte, error)` in `bulk.go` using `c.gp.APIClient()`. The bulk endpoint returns a gzipped tar, not JSON, so `Query()` (which JSON-decodes) may not fit. Preferred: call `c.gp.APIClient().Query(...)` with a `[]byte`/`io.Reader` target if supported; otherwise add a minimal authenticated `net/http` GET to `<endpoint>/latest_five_min_metrics` reusing `cfg.Username/Password` (HTTP basic) + `insecureSkipVerify`, reading the raw body. Confirm the exact bulk endpoint path and any "enable" precondition against Dell's `powerstore-metrics-exporter/bulkClient` before finalizing. Add a `// nosemgrep`-free implementation; run `semgrep_scan` on the file.

- [ ] **Step 5: Run test + build**

Run: `go test ./internal/powerstore/... -run BulkVolumeSamples && go build ./...`
Expected: PASS + build success.

- [ ] **Step 6: Commit**

```bash
git add internal/powerstore/derive_bulk.go internal/powerstore/bulk.go internal/powerstore/client.go internal/powerstore/derive_bulk_test.go
git commit -m "feat: implement bulk CSV metrics path with per-entity parity"
```

---

## Phase 6 — Collector + mock gateway tests

### Task 16: Create `collector.go` and the mock client

**Files:**
- Create: `internal/powerstore/collector.go`
- Test: `internal/powerstore/collector_test.go`

- [ ] **Step 1: Write the failing test (mock Client)**

```go
package powerstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockClient struct {
	name     string
	topoErr  error
	bulk     bool
	perEnt   []Sample
	bulkSamp []Sample
}

func (m *mockClient) Name() string { return m.name }
func (m *mockClient) GetTopology(context.Context) (*Topology, error) {
	if m.topoErr != nil {
		return nil, m.topoErr
	}
	return &Topology{}, nil
}
func (m *mockClient) BulkCapable(context.Context, *Topology) bool                 { return m.bulk }
func (m *mockClient) PerEntityMetrics(context.Context, *Topology) ([]Sample, error) { return m.perEnt, nil }
func (m *mockClient) BulkMetrics(context.Context, *Topology) ([]Sample, error)      { return m.bulkSamp, nil }
func (m *mockClient) Close() error                                                  { return nil }

func TestCollectOnceGracefulDegradation(t *testing.T) {
	good := &mockClient{name: "ok", bulk: false, perEnt: []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "ok"}}, Value: 1}}}
	bad := &mockClient{name: "down", topoErr: errors.New("unreachable")}
	store := NewSnapshotStore()
	c := NewCollector([]Client{good, bad}, store, time.Second, time.Second, nil)
	snap := c.CollectOnce(context.Background())

	if !snap.PerArray["ok"].Up {
		t.Fatal("expected ok array up")
	}
	if snap.PerArray["down"].Up {
		t.Fatal("expected down array not up")
	}
	if snap.PerArray["down"].ScrapeError == "" {
		t.Fatal("expected scrape error recorded for down array")
	}
}

func TestCollectChoosesBulkWhenCapable(t *testing.T) {
	m := &mockClient{name: "p1", bulk: true,
		bulkSamp: []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 7}},
		perEnt:   []Sample{{Name: "powerstore_volume_read_iops", Labels: []Label{{"array", "p1"}}, Value: 99}}}
	store := NewSnapshotStore()
	c := NewCollector([]Client{m}, store, time.Second, time.Second, nil)
	snap := c.CollectOnce(context.Background())
	got := snap.SamplesByName("powerstore_volume_read_iops")
	if len(got) != 1 || got[0].Value != 7 {
		t.Fatalf("expected bulk value 7, got %+v", got)
	}
	if !snap.PerArray["p1"].BulkCapable {
		t.Fatal("expected BulkCapable true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/powerstore/... -run Collect`
Expected: FAIL (undefined `NewCollector`).

- [ ] **Step 3: Write `collector.go`**

```go
package powerstore

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// Collector runs the background collection loop: every interval it polls all arrays in
// parallel and publishes a fresh Snapshot. One array's failure does not affect others.
type Collector struct {
	clients  []Client
	store    *SnapshotStore
	interval time.Duration
	timeout  time.Duration
	tracing  *TracerWrapper
}

// NewCollector creates a collection loop over the given per-array clients.
func NewCollector(clients []Client, store *SnapshotStore, interval, timeout time.Duration, tp trace.TracerProvider) *Collector {
	return &Collector{
		clients:  clients,
		store:    store,
		interval: interval,
		timeout:  timeout,
		tracing:  NewTracerWrapper(tp, "pstore-exporter/collector"),
	}
}

// CollectOnce runs a single cycle and publishes the result.
func (c *Collector) CollectOnce(ctx context.Context) *Snapshot {
	snap := c.collectAll(ctx)
	c.store.Store(snap)
	return snap
}

// Run drives the loop until ctx is cancelled (assumes an initial CollectOnce).
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.store.Store(c.collectAll(ctx))
		}
	}
}

func (c *Collector) collectAll(ctx context.Context) *Snapshot {
	ctx, span := c.tracing.StartSpan(ctx, "collect.cycle", trace.SpanKindInternal)
	defer span.End()

	results := make([]*ArraySnapshot, len(c.clients))
	g, gctx := errgroup.WithContext(ctx)
	for i, client := range c.clients {
		i, client := i, client
		g.Go(func() error {
			results[i] = c.collectArray(gctx, client)
			return nil // graceful degradation
		})
	}
	_ = g.Wait()
	return BuildSnapshot(results)
}

func (c *Collector) collectArray(ctx context.Context, client Client) *ArraySnapshot {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	as := &ArraySnapshot{Array: client.Name(), LastScrape: time.Now()}

	topo, err := client.GetTopology(ctx)
	if err != nil {
		log.Warnf("array %q: topology fetch failed: %v", client.Name(), err)
		as.ScrapeError = err.Error()
		return as
	}

	as.BulkCapable = client.BulkCapable(ctx, topo)
	var samples []Sample
	if as.BulkCapable {
		samples, err = client.BulkMetrics(ctx, topo)
		if err != nil {
			log.Warnf("array %q: bulk metrics failed, falling back to per-entity: %v", client.Name(), err)
			samples, err = client.PerEntityMetrics(ctx, topo)
		}
	} else {
		samples, err = client.PerEntityMetrics(ctx, topo)
	}
	if err != nil {
		as.ScrapeError = err.Error()
		return as
	}
	as.Samples = samples
	as.Up = true
	return as
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/powerstore/... -run Collect`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/powerstore/collector.go internal/powerstore/collector_test.go
git commit -m "feat: add collection loop with bulk/per-entity selection"
```

---

## Phase 7 — OTLP export

### Task 17: Create `internal/powerstore/otlp.go`

**Files:**
- Create: `internal/powerstore/otlp.go`
- Test: `internal/powerstore/otlp_test.go`

- [ ] **Step 1: Copy and adapt**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/internal/powerflex/otlp.go internal/powerstore/otlp.go
```
Apply: `package powerstore`; fix import path; replace any `pflex`/`cluster` service strings with `pstore`/`array`; ensure it reads `store.Load().MetricNames()` and `SamplesByName` (unchanged API). The OTLP exporter registers a Float64ObservableGauge per metric name whose callback observes each sample's value with its labels as attributes — keep that logic verbatim.

- [ ] **Step 2: Copy/adapt the OTLP test**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/internal/powerflex/otlp_test.go internal/powerstore/otlp_test.go 2>/dev/null || true
```
If present, adapt cluster→array, pflex_→powerstore_, and the ManualReader assertions. If absent, write a test that builds a snapshot with one sample, constructs the exporter with a `sdkmetric.NewManualReader()`, calls `EnsureInstruments()`, collects, and asserts the gauge value/attributes.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/powerstore/... -run OTLP`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/powerstore/otlp.go internal/powerstore/otlp_test.go
git commit -m "feat: add OTLP observable-gauge exporter"
```

---

## Phase 8 — Wiring: main.go + config.yaml

### Task 18: Create `main.go`

**Files:**
- Create: `main.go`
- Create: `config.yaml`

- [ ] **Step 1: Copy the template main.go as a starting point**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/main.go main.go
```

- [ ] **Step 2: Adapt `main.go`**

Apply these changes (the template structure stays; see TEMPLATE main.go for the Server struct, HTTP mux `/metrics`+`/health`, SIGHUP/file-watch reload, and `waitForShutdown`):
- import path `pflex_exporter`→`pstore_exporter`; package alias `powerflex`→`powerstore`.
- Build clients with `powerstore.NewArrayClient(cfg)` (returns error — handle it; skip a failing array with a logged warning, fail startup only if zero clients build).
- Iterate `cfg.Arrays` instead of `cfg.Clusters`.
- Remove the Kubernetes enricher wiring and the `WithEnricher`/`WithDecimation`/`WithMaxConcurrentClusters` collector options (not present in our simplified `NewCollector`). Call `powerstore.NewCollector(clients, store, cfg.GetCollectionInterval(), cfg.GetCollectionTimeout(), tracerProvider)`.
- CLI name `pflex_exporter`→`pstore_exporter`; flag help text updated.
- Keep: initial synchronous `CollectOnce`, background `Run` (unless `--once`), `/metrics` via `promhttp.HandlerFor(registry,…)` with the registered `PromCollector`, OTLP exporter start when `cfg.IsOTelMetricsEnabled()`, SIGHUP + fsnotify reload that rebuilds clients when `arraysChanged`.

- [ ] **Step 3: Write `config.yaml`**

```yaml
server:
  host: "0.0.0.0"
  port: "9101"
  uri: "/metrics"
  logName: ""

collection:
  interval: "30s"
  timeout: "20s"

opentelemetry:
  metrics:
    enabled: false
    endpoint: "localhost:4317"
    insecure: true
    interval: "30s"
  tracing:
    enabled: false
    endpoint: "localhost:4317"
    insecure: true
    samplingRate: 0.1

arrays:
  - name: pstore-1
    endpoint: "https://10.0.0.1/api/rest"
    username: admin
    password: "${PSTORE1_PASSWORD}"
    insecureSkipVerify: true
    # interval: Five_Mins
```

- [ ] **Step 4: Build and smoke-test the CLI**

Run: `go build -o bin/pstore_exporter . && ./bin/pstore_exporter --help`
Expected: build success; help shows `--config`, `--debug`, `--once`.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add main.go config.yaml
git commit -m "feat: wire CLI, HTTP server, collection loop, and hot reload"
```

### Task 19: Integration test — `--once` against an httptest gateway

**Files:**
- Test: `internal/powerstore/client_integration_test.go`

- [ ] **Step 1: Write an httptest-based test** that stands up a TLS `httptest.Server` serving minimal `login_session`, `cluster`, `appliance`, and `metrics/generate` responses, constructs `NewArrayClient` pointed at it (with `InsecureSkipVerify`), and asserts `GetTopology` + `PerEntityMetrics` return expected samples. Model the mock on TEMPLATE `internal/powerflex/client_test.go` (httptest TLS, fixture JSON, `writeBytes` helper to avoid Semgrep findings).

> If gopowerstore's auth flow is awkward to satisfy with httptest (custom headers/cookies), prefer keeping coverage at the `Client`-interface mock level (Task 16) and assert the gopowerstore-specific glue with a thin fake of the few methods used. Document whichever approach in the test file header.

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/powerstore/... -run Integration
git add internal/powerstore/client_integration_test.go
git commit -m "test: integration coverage for topology and per-entity collection"
```

---

## Phase 9 — Grafana dashboards

### Task 20: Build block + file dashboards and provisioning

**Files:**
- Create: `grafana/provisioning/datasources/datasource.yml`, `grafana/provisioning/dashboards/dashboards.yml`
- Create: `grafana/block/01-cluster-overview.json`, `02-appliances.json`, `03-volumes.json`, `04-volume-groups.json`, `05-ports.json`, `06-capacity.json`, `07-drives.json`
- Create: `grafana/file/01-nas-overview.json`, `02-file-systems.json`

- [ ] **Step 1: Copy provisioning from template and adapt**

```bash
mkdir -p grafana/provisioning/datasources grafana/provisioning/dashboards grafana/block grafana/file
cp /Users/fjacquet/Projects/pflex_exporter/grafana/provisioning/datasources/*.y*ml grafana/provisioning/datasources/
cp /Users/fjacquet/Projects/pflex_exporter/grafana/provisioning/dashboards/*.y*ml grafana/provisioning/dashboards/
```
In `dashboards.yml` set provider paths to `/var/lib/grafana/dashboards/block` and `/var/lib/grafana/dashboards/file`.

- [ ] **Step 2: Author `grafana/block/01-cluster-overview.json`**

Use a template variable `array` from `label_values(powerstore_up, array)`. Panels:
- "Arrays Up" — stat, `powerstore_up{array=~"$array"}`, mappings 0→DOWN(red)/1→UP(green).
- "Bulk API" — stat, `powerstore_array_bulk_api{array=~"$array"}`.
- "Total IOPS" — timeseries, `sum by (array) (powerstore_appliance_total_iops{array=~"$array"})` (NO `rate()` — gauges already per-second).
- "Read/Write Bandwidth" — timeseries, `sum by (array) (powerstore_appliance_read_bandwidth_bytes_per_second{array=~"$array"})` and write.
- "Avg Latency" — timeseries, `avg by (array) (powerstore_appliance_read_latency_microseconds{array=~"$array"})`.
- "Capacity Used %" — gauge, `100 * sum(powerstore_appliance_physical_used_bytes{array=~"$array"}) / sum(powerstore_appliance_physical_total_bytes{array=~"$array"})`.

Start from a copied template dashboard (`/Users/fjacquet/Projects/pflex_exporter/grafana/gen2/01-cluster-overview.json`) and swap metric names/labels. Keep `"datasource": {"type":"prometheus","uid":"${DS_PROMETHEUS}"}` consistent with the datasource provisioning.

- [ ] **Step 3: Author the remaining dashboards** following the same pattern, each with the `array` variable:
  - `02-appliances.json` — per-appliance IOPS/bandwidth/latency/CPU (`powerstore_appliance_*`, `by (appliance_name)`).
  - `03-volumes.json` — top-N volumes by IOPS/latency (`powerstore_volume_*`, `topk(10, …) by (volume_name)`).
  - `04-volume-groups.json` — `by (volume_group_name)` aggregations of volume metrics.
  - `05-ports.json` — `powerstore_eth_port_*` / `powerstore_fc_port_*` by `port_name`.
  - `06-capacity.json` — appliance space metrics, data-reduction & efficiency ratios.
  - `07-drives.json` — `powerstore_drive_wear_*`.
  - `file/01-nas-overview.json`, `file/02-file-systems.json` — `powerstore_file_system_*` by `nas_server_name`/`file_system_name`.

> Only include panels whose metrics are actually emitted by the implemented derive functions. If ports/drives/file metrics were deferred in Tasks 13/15, mark those dashboards as TODO in the README rather than shipping empty panels.

- [ ] **Step 4: Validate JSON**

Run: `for f in grafana/block/*.json grafana/file/*.json; do python3 -m json.tool "$f" >/dev/null && echo "ok $f"; done`
Expected: every file prints `ok`.

- [ ] **Step 5: Commit**

```bash
git add grafana
git commit -m "feat: add block and file Grafana dashboards with provisioning"
```

---

## Phase 10 — Docker + compose

### Task 21: Dockerfile, compose, prometheus.yml, otel config

**Files:**
- Create: `Dockerfile`, `docker-compose.yml`, `docker-compose.ghcr.yml`, `prometheus.yml`, `otel-collector-config.yaml`, `Makefile`

- [ ] **Step 1: Copy and adapt the Dockerfile**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/Dockerfile Dockerfile
```
Adapt: binary name `pflex_exporter`→`pstore_exporter`; config path `/etc/pflex_exporter/`→`/etc/pstore_exporter/`; user name `pflex`→`pstore`; `EXPOSE 2112`→`EXPOSE 9101`; log dir `/var/log/pstore_exporter`. (The build is `CGO_ENABLED=0 go build -ldflags="-s -w" -o pstore_exporter .`.)

- [ ] **Step 2: Copy and adapt compose files**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/docker-compose.yml docker-compose.yml
cp /Users/fjacquet/Projects/pflex_exporter/docker-compose.ghcr.yml docker-compose.ghcr.yml
cp /Users/fjacquet/Projects/pflex_exporter/prometheus.yml prometheus.yml
cp /Users/fjacquet/Projects/pflex_exporter/otel-collector-config.yaml otel-collector-config.yaml
```
Adapt in `docker-compose.yml`: service name `pflex_exporter`→`pstore_exporter`; port `2112:2112`→`9101:9101`; env var `FLEX1_PASSWORD`→`PSTORE1_PASSWORD`; grafana dashboard mounts `./grafana/gen1`,`./grafana/gen2` → `./grafana/block`,`./grafana/file` mapped to `/var/lib/grafana/dashboards/block`,`/var/lib/grafana/dashboards/file`; config mount `./config.yaml:/etc/pstore_exporter/config.yaml`. In `prometheus.yml` set the scrape target to `pstore_exporter:9101` and `scrape_interval: 30s`.

- [ ] **Step 3: Copy and adapt the Makefile**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/Makefile Makefile
```
Adapt binary name and module references `pflex`→`pstore`. Keep targets `tools`, `sure`, `ci`, `cli`, `test`, `test-race`, `sbom`, `release`, `docker`.

- [ ] **Step 4: Verify build + compose config**

Run: `docker build -t pstore_exporter:dev . && docker compose config >/dev/null && echo compose-ok`
Expected: image builds; `compose-ok` prints.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile docker-compose.yml docker-compose.ghcr.yml prometheus.yml otel-collector-config.yaml Makefile
git commit -m "feat: add Dockerfile, docker-compose stack, and Makefile"
```

### Task 22: End-to-end compose smoke test

- [ ] **Step 1: Bring up the stack (no real array needed for wiring check)**

Run:
```bash
export PSTORE1_PASSWORD='dummy'
docker compose up -d
sleep 10
curl -fsS http://localhost:9101/health && echo
curl -fsS http://localhost:9101/metrics | grep -c '^powerstore_' || true
```
Expected: `/health` returns OK; `/metrics` responds (sample count may be 0 with an unreachable dummy array, but `powerstore_up{array="pstore-1"} 0` must be present).

- [ ] **Step 2: Verify Prometheus + Grafana came up**

Run: `curl -fsS http://localhost:9090/-/ready && curl -fsS http://localhost:3000/api/health`
Expected: both healthy. Tear down: `docker compose down`.

- [ ] **Step 3: Commit any compose fixes**

```bash
git add -A && git commit -m "test: validate docker-compose stack wiring" --allow-empty
```

---

## Phase 11 — Deploy manifests, docs, CI

### Task 23: Kubernetes manifests, alert rules, systemd unit

**Files:**
- Create: `deploy/kubernetes/{deployment,service,servicemonitor,configmap,secret.example,kustomization}.yaml`
- Create: `deploy/prometheus/pstore.rules.yml`
- Create: `deploy/pstore_exporter.service`, `deploy/pstore_exporter.env.example`

- [ ] **Step 1: Copy and adapt from template**

```bash
mkdir -p deploy/kubernetes deploy/prometheus
cp /Users/fjacquet/Projects/pflex_exporter/deploy/kubernetes/*.yaml deploy/kubernetes/
cp /Users/fjacquet/Projects/pflex_exporter/deploy/prometheus/*.yml deploy/prometheus/ 2>/dev/null || true
cp /Users/fjacquet/Projects/pflex_exporter/deploy/pflex_exporter.service deploy/pstore_exporter.service
cp /Users/fjacquet/Projects/pflex_exporter/deploy/pflex_exporter.env.example deploy/pstore_exporter.env.example
```
Adapt: names `pflex`→`pstore`; container port `2112`→`9101`; image ref to the pstore GHCR path; ServiceMonitor port name/label selectors; rename alert rules file to `pstore.rules.yml` and rewrite expressions to `powerstore_*` (e.g. `powerstore_up == 0`, capacity > 85%, scrape staleness via `time() - powerstore_last_scrape_timestamp_seconds > 300`).

- [ ] **Step 2: Validate YAML**

Run: `for f in deploy/kubernetes/*.yaml deploy/prometheus/*.yml; do python3 -c "import yaml,sys; list(yaml.safe_load_all(open('$f')))" && echo "ok $f"; done`
Expected: every file prints `ok`.

- [ ] **Step 3: Commit**

```bash
git add deploy
git commit -m "feat: add kubernetes manifests, alert rules, and systemd unit"
```

### Task 24: MkDocs documentation

**Files:**
- Create: `mkdocs.yml`, `docs/index.md`, `docs/metrics.md`, `docs/dashboards.md`, `docs/opentelemetry.md`, `docs/alerting.md`, `docs/cicd.md`, `docs/getting-started/*.md`, `README.md`

- [ ] **Step 1: Copy and adapt**

```bash
cp /Users/fjacquet/Projects/pflex_exporter/mkdocs.yml mkdocs.yml
cp -r /Users/fjacquet/Projects/pflex_exporter/docs/* docs/   # keeps docs/superpowers/ untouched (different subtree)
cp /Users/fjacquet/Projects/pflex_exporter/README.md README.md
```
Rewrite content: PowerFlex→PowerStore; Gen1/Gen2→bulk/per-entity paths; gateway→endpoint; `pflex_`→`powerstore_`; port 2112→9101; document the metric catalog from the implemented derive functions; note "metrics are per-second/absolute gauges — use `sum`/`avg`, not `rate()`". Update `mkdocs.yml` `site_name`, `repo_url`, and nav. Do NOT overwrite `docs/superpowers/`.

- [ ] **Step 2: Build the docs site**

Run: `uvx mkdocs build --strict 2>/dev/null || mkdocs build --strict`
Expected: build succeeds with no broken-link/strict errors (fix any).

- [ ] **Step 3: Commit**

```bash
git add mkdocs.yml docs README.md
git commit -m "docs: add MkDocs site and README for PowerStore exporter"
```

### Task 25: GitHub Actions CI/release/docs

**Files:**
- Create: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/docs.yml`

- [ ] **Step 1: Copy and adapt**

```bash
mkdir -p .github/workflows
cp /Users/fjacquet/Projects/pflex_exporter/.github/workflows/ci.yml .github/workflows/ci.yml
cp /Users/fjacquet/Projects/pflex_exporter/.github/workflows/release.yml .github/workflows/release.yml
cp /Users/fjacquet/Projects/pflex_exporter/.github/workflows/docs.yml .github/workflows/docs.yml
```
Adapt: binary/image names `pflex`→`pstore`; GHCR image path; any module-path references. Keep the Semgrep gate, SBOM (cyclonedx-gomod), govulncheck, multi-arch matrix.

- [ ] **Step 2: Lint workflows locally (best-effort)**

Run: `for f in .github/workflows/*.yml; do python3 -c "import yaml; yaml.safe_load(open('$f'))" && echo "ok $f"; done`
Expected: each prints `ok`.

- [ ] **Step 3: Run the full local CI gate**

Run: `make ci`
Expected: fmt-check, vet, lint, `go test -race`, govulncheck all pass. Fix anything that fails.

- [ ] **Step 4: Semgrep scan the new Go code**

Use the `semgrep_scan` MCP tool on `internal/powerstore/` and `main.go` (per global security rules). Resolve findings without inline suppressions.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows
git commit -m "ci: add CI, release, and docs workflows"
```

---

## Final verification

- [ ] `go build ./... && go test ./... -race` — all green.
- [ ] `make ci` — full gate passes; Semgrep clean.
- [ ] `go run . --config config.yaml --once` against the integration mock (Task 19) prints `powerstore_*` metrics; the bulk and per-entity paths produce identical sample names/label keys (asserted by Task 15 test).
- [ ] `docker compose up` — exporter `:9101/metrics` scraped by Prometheus; Grafana auto-provisions block + file dashboards.
- [ ] If a real PowerStore array is reachable: point `config.yaml` at it, confirm `powerstore_up=1`, `powerstore_array_bulk_api` reflects OS version, and dashboards populate.
- [ ] OTLP: enable in config, run the otel-collector service, confirm `powerstore_*` metrics arrive on the collector's `:8889`.

---

## Self-Review Notes (for the implementer)

- **gopowerstore field/method names are the #1 risk.** Tasks 9, 11, 12, 13, 15 each include a `go doc` verification step. Treat the metric NAMES and LABEL BUILDERS as the fixed contract; adjust the gopowerstore-side field reads to reality.
- **Bulk endpoint specifics** (exact path, "enable" precondition, gzip-tar handling through `APIClient()`) are confirmed in Task 15 Step 4 against Dell's `powerstore-metrics-exporter/bulkClient`. If `APIClient().Query()` can't yield raw bytes, the documented fallback is a thin authenticated `net/http` GET.
- **Label parity** between bulk and per-entity paths is enforced by the Task 15 test (`assertSameLabelKeys`) — keep it green when adding ports/drives/file metrics.
- **Scope discipline:** ports/drives/file metrics are wired where Dell's API is confirmed; anything deferred must be reflected in dashboards (Task 20) and README (Task 24) rather than shipping empty panels.
