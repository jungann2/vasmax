// Package config handles VasmaX configuration loading, validation and persistence.
// Supports YAML format with hot-reload capability.
package config

// DefaultConfigPath is the default path for the VasmaX configuration file.
const DefaultConfigPath = "/etc/vasmax/config.yaml"

// Config represents the main VasmaX configuration structure.
type Config struct {
	Standalone        bool             `yaml:"standalone"`
	Log               LogConfig        `yaml:"log"`
	Audit             AuditConfig      `yaml:"audit"`
	Lang              string           `yaml:"lang"`
	Protocols         []ProtocolConfig `yaml:"protocols"`
	CoreType          string           `yaml:"core_type"`
	MonitoringEnabled bool             `yaml:"monitoring_enabled"`
	Xboard            *XboardConfig    `yaml:"xboard,omitempty"`
	TLS               *TLSConfig       `yaml:"tls,omitempty"`
}

// LogConfig defines logging settings.
type LogConfig struct {
	Level    string `yaml:"level"`
	FilePath string `yaml:"file_path"`
}

// AuditConfig defines audit logging settings.
type AuditConfig struct {
	Enabled  bool   `yaml:"enabled"`
	FilePath string `yaml:"file_path"`
	MaxSize  int    `yaml:"max_size"`
	MaxFiles int    `yaml:"max_files"`
}

// ProtocolConfig defines a single protocol installation.
type ProtocolConfig struct {
	Name        string `yaml:"name"`
	Core        string `yaml:"core"`
	Port        int    `yaml:"port"`
	Domain      string `yaml:"domain,omitempty"`
	Mode        string `yaml:"mode"`
	TLSProvider string `yaml:"tls_provider,omitempty"`
}

// XboardConfig defines Xboard panel integration settings.
type XboardConfig struct {
	APIHost  string `yaml:"api_host"`
	APIKey   string `yaml:"api_key"`
	NodeID   int    `yaml:"node_id"`
	Interval int    `yaml:"interval"`
}

// TLSConfig defines TLS certificate settings.
type TLSConfig struct {
	CertDir    string `yaml:"cert_dir"`
	MinVersion string `yaml:"min_version"`
	MaxVersion string `yaml:"max_version"`
}
