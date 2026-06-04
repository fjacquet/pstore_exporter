// Package models defines the core data structures for the PowerStore exporter.
package models

// ClusterConfig holds the connection details for a single PowerStore appliance.
// The Name becomes the `appliance` label on every metric emitted for this target.
type ClusterConfig struct {
	Name         string `yaml:"name"`
	Gateway      string `yaml:"gateway"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"passwordFile"`
}

// Config represents the complete application configuration for the PowerStore exporter.
type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
		URI  string `yaml:"uri"`
	} `yaml:"server"`

	Collection struct {
		Interval string `yaml:"interval"`
		Timeout  string `yaml:"timeout"`
	} `yaml:"collection"`

	Clusters []ClusterConfig `yaml:"clusters"`
}
