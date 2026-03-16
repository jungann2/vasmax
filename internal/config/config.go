package config

import (
	"fmt"
	"os"
	"strings"
	"vasmax/internal/security"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath is the default configuration file path.
const DefaultConfigPath = "/etc/vasmax/config.yaml"

// Config is the main VasmaX configuration structure.
type Config struct {
	Standalone        bool               `yaml:"standalone"`
	Listen            string             `yaml:"listen"`
	APIHost           string             `yaml:"api_host"`
	APIToken          string             `yaml:"api_token"`
	NodeID            int                `yaml:"node_id"`
	NodeType          string             `yaml:"node_type"`
	TLS               TLSConfig          `yaml:"tls"`
	Log               LogConfig          `yaml:"log"`
	Audit             AuditConfig        `yaml:"audit"`
	Lang              string             `yaml:"lang"`
	Protocols         []string           `yaml:"protocols"`
	ProtocolModes     map[string]string  `yaml:"protocol_modes"`   // 协议安装模式: "domain" 或 "nodomain"
	ProtocolPorts     map[string]int     `yaml:"protocol_ports"`   // 协议自定义端口
	ProtocolDomains   map[string]string  `yaml:"protocol_domains"` // 协议独立域名（多域名模式）
	CoreType          string             `yaml:"core_type"`
	CDN               CDNConfig          `yaml:"cdn"`
	Subscription      SubscriptionConfig `yaml:"subscription"`
	Hysteria2         Hysteria2Config    `yaml:"hysteria2"`
	Tuic              TuicConfig         `yaml:"tuic"`
	Reality           RealityConfig      `yaml:"reality"`
	Paths             PathsConfig        `yaml:"paths"`
	MonitoringEnabled bool               `yaml:"monitoring_enabled"`
	ExtraPorts        []ExtraPort        `yaml:"extra_ports"`
	ALPN              ALPNConfig         `yaml:"alpn"`
}

// TLSConfig holds TLS certificate settings.
type TLSConfig struct {
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	Domain     string `yaml:"domain"`
	Provider   string `yaml:"provider"`
	MinVersion string `yaml:"min_version"` // "1.0", "1.1", "1.2", "1.3"（默认 "1.2"）
	MaxVersion string `yaml:"max_version"` // "1.2", "1.3"（默认 "1.3"）
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level    string `yaml:"level"`
	FilePath string `yaml:"file_path"`
}

// AuditConfig holds audit logging settings.
type AuditConfig struct {
	Enabled  bool   `yaml:"enabled"`
	FilePath string `yaml:"file_path"`
	MaxSize  int    `yaml:"max_size"`
	MaxFiles int    `yaml:"max_files"`
}

// CDNConfig holds CDN relay settings.
type CDNConfig struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
}

// SubscriptionConfig holds subscription generation settings.
type SubscriptionConfig struct {
	Salt   string `yaml:"salt"`
	Domain string `yaml:"domain"`
}

// Hysteria2Config holds Hysteria2 protocol settings.
type Hysteria2Config struct {
	Port     int `yaml:"port"`
	DownMbps int `yaml:"down_mbps"`
	UpMbps   int `yaml:"up_mbps"`
	HopStart int `yaml:"hop_start"`
	HopEnd   int `yaml:"hop_end"`
}

// TuicConfig holds Tuic protocol settings.
type TuicConfig struct {
	Port              int    `yaml:"port"`
	CongestionControl string `yaml:"congestion_control"`
}

// RealityConfig holds Reality protocol settings.
type RealityConfig struct {
	PrivateKey string `yaml:"private_key"`
	PublicKey  string `yaml:"public_key"`
	ShortID    string `yaml:"short_id"`
	Dest       string `yaml:"dest"`
	ServerName string `yaml:"server_name"`
	Port       int    `yaml:"port"` // Reality 监听端口，默认 443，阿里云等用 8443
}

// PathsConfig holds file system path settings for various components.
type PathsConfig struct {
	XrayConf    string `yaml:"xray_conf"`
	SingBoxConf string `yaml:"singbox_conf"`
	Subscribe   string `yaml:"subscribe"`
	Cache       string `yaml:"cache"`
	NginxConf   string `yaml:"nginx_conf"`
}

// ExtraPort 额外开放的端口规则
type ExtraPort struct {
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol"` // "tcp" / "udp" / "both"
	Note     string `yaml:"note"`
}

// ALPNConfig 全局 ALPN 设置
type ALPNConfig struct {
	Mode string `yaml:"mode"` // "h2_http11"（默认）/ "h2_only" / "http11_only" / "h3_only"
}

// EffectivePort 返回 Reality 有效端口（默认 443）
func (r *RealityConfig) EffectivePort() int {
	if r.Port > 0 {
		return r.Port
	}
	return 443
}

// ALPNList 根据 ALPN 配置返回当前 ALPN 列表（供协议生成时使用）
func (a *ALPNConfig) ALPNList() []string {
	switch a.Mode {
	case "h2_only":
		return []string{"h2"}
	case "http11_only":
		return []string{"http/1.1"}
	case "h3_only":
		return []string{"h3"}
	case "all":
		return []string{"h2", "http/1.1", "h3"}
	default:
		return []string{"h2", "http/1.1"}
	}
}

// setDefaults fills in default values for fields that are empty.
func (c *Config) setDefaults() {
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Paths.XrayConf == "" {
		c.Paths.XrayConf = "/etc/vasmax/xray/conf/"
	}
	if c.Paths.SingBoxConf == "" {
		c.Paths.SingBoxConf = "/etc/vasmax/sing-box/conf/config/"
	}
	if c.Paths.Subscribe == "" {
		c.Paths.Subscribe = "/etc/vasmax/subscribe/"
	}
	if c.Paths.Cache == "" {
		c.Paths.Cache = "/etc/vasmax/cache/"
	}
	if c.Paths.NginxConf == "" {
		c.Paths.NginxConf = "/etc/nginx/conf.d/"
	}
	if c.ProtocolModes == nil {
		c.ProtocolModes = make(map[string]string)
	}
	if c.ProtocolPorts == nil {
		c.ProtocolPorts = make(map[string]int)
	}
	if c.ProtocolDomains == nil {
		c.ProtocolDomains = make(map[string]string)
	}
}

// GetProtocolDomain 返回协议的域名（优先使用独立域名，否则使用全局域名）
func (c *Config) GetProtocolDomain(protoName string) string {
	if d, ok := c.ProtocolDomains[protoName]; ok && d != "" {
		return d
	}
	return c.TLS.Domain
}

// LoadConfig reads a YAML configuration file from path, unmarshals it into
// a Config struct, decrypts any "ENC:" prefixed credentials, and applies
// default values for empty fields.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	cfg.MonitoringEnabled = true
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Decrypt ENC: prefixed credentials.
	if strings.HasPrefix(cfg.APIToken, "ENC:") {
		decrypted, err := security.DecryptCredential(cfg.APIToken)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt api_token: %w", err)
		}
		cfg.APIToken = decrypted
	}

	cfg.setDefaults()

	return &cfg, nil
}

// SaveConfig atomically writes the configuration to path with permission 0600.
func SaveConfig(path string, cfg *Config) error {
	return security.AtomicWriteYAML(path, cfg, 0600)
}
