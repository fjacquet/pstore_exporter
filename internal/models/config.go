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
func (c *Config) GetCollectionInterval() time.Duration {
	return mustDuration(c.Collection.Interval, 30*time.Second)
}

// GetCollectionTimeout returns the per-array timeout.
func (c *Config) GetCollectionTimeout() time.Duration {
	return mustDuration(c.Collection.Timeout, 20*time.Second)
}

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
