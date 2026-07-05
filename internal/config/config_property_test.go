package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// genAPIHost generates a random API host URL.
func genAPIHost() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("http", "https"),
		gen.RegexMatch(`[a-z]{3,10}\.[a-z]{2,5}`),
	).Map(func(vals []interface{}) string {
		return fmt.Sprintf("%s://%s", vals[0].(string), vals[1].(string))
	})
}

// genAPIToken generates a random API token string.
func genAPIToken() gopter.Gen {
	return gen.RegexMatch(`[a-zA-Z0-9]{8,32}`)
}

// genNodeType generates a random valid node type.
func genNodeType() gopter.Gen {
	return gen.OneConstOf("vless", "vmess", "trojan", "hysteria", "tuic", "shadowsocks")
}

// writeYAMLToTempFile writes YAML content to a temp file and returns the path.
func writeYAMLToTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

// buildBaseYAML builds a minimal valid YAML config string from random values.
// If monitoringField is non-empty, it is appended as a line.
func buildBaseYAML(apiHost, apiToken, nodeType string, nodeID int, monitoringField string) string {
	yaml := fmt.Sprintf(`api_host: %s
api_token: %s
node_id: %d
node_type: %s
`, apiHost, apiToken, nodeID, nodeType)
	if monitoringField != "" {
		yaml += monitoringField + "\n"
	}
	return yaml
}

// Feature: node-monitoring-system, Property 7: MonitoringEnabled 默认值为 true
// **Validates: Requirements 3.4, 15.2**
func TestProperty7_MonitoringEnabledDefaultTrue(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Sub-property 1: YAML without monitoring_enabled → MonitoringEnabled defaults to true
	properties.Property("config without monitoring_enabled field defaults to true", prop.ForAll(
		func(apiHost, apiToken, nodeType string, nodeID int) bool {
			yaml := buildBaseYAML(apiHost, apiToken, nodeType, nodeID, "")
			path := writeYAMLToTempFile(t, yaml)

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Logf("LoadConfig error: %v", err)
				return false
			}
			if !cfg.MonitoringEnabled {
				t.Logf("MonitoringEnabled should be true when not specified, got false")
				return false
			}
			return true
		},
		genAPIHost(),
		genAPIToken(),
		genNodeType(),
		gen.IntRange(1, 10000),
	))

	// Sub-property 2: YAML with explicit monitoring_enabled: false → MonitoringEnabled is false
	properties.Property("config with explicit monitoring_enabled: false is false", prop.ForAll(
		func(apiHost, apiToken, nodeType string, nodeID int) bool {
			yaml := buildBaseYAML(apiHost, apiToken, nodeType, nodeID, "monitoring_enabled: false")
			path := writeYAMLToTempFile(t, yaml)

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Logf("LoadConfig error: %v", err)
				return false
			}
			if cfg.MonitoringEnabled {
				t.Logf("MonitoringEnabled should be false when explicitly set to false, got true")
				return false
			}
			return true
		},
		genAPIHost(),
		genAPIToken(),
		genNodeType(),
		gen.IntRange(1, 10000),
	))

	// Sub-property 3: YAML with explicit monitoring_enabled: true → MonitoringEnabled is true
	properties.Property("config with explicit monitoring_enabled: true is true", prop.ForAll(
		func(apiHost, apiToken, nodeType string, nodeID int) bool {
			yaml := buildBaseYAML(apiHost, apiToken, nodeType, nodeID, "monitoring_enabled: true")
			path := writeYAMLToTempFile(t, yaml)

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Logf("LoadConfig error: %v", err)
				return false
			}
			if !cfg.MonitoringEnabled {
				t.Logf("MonitoringEnabled should be true when explicitly set to true, got false")
				return false
			}
			return true
		},
		genAPIHost(),
		genAPIToken(),
		genNodeType(),
		gen.IntRange(1, 10000),
	))

	properties.TestingRun(t)
}

// genLogLevel generates a random valid log level.
func genLogLevel() gopter.Gen {
	return gen.OneConstOf("debug", "info", "warn", "error")
}

// genConfig generates a random Config struct with all fields populated,
// ensuring fields that have defaults are non-empty to avoid setDefaults() interference.
func genConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.Bool(),                // Standalone
		genAPIHost(),              // APIHost
		genAPIToken(),             // APIToken (no ENC: prefix)
		gen.IntRange(1, 10000),    // NodeID
		genNodeType(),             // NodeType
		gen.Bool(),                // MonitoringEnabled
		genLogLevel(),             // Log.Level
		gen.Bool(),                // Audit.Enabled
		gen.IntRange(1, 100),      // Audit.MaxSize
		gen.IntRange(1, 50),       // Audit.MaxFiles
		gen.Bool(),                // CDN.Enabled
		gen.IntRange(1000, 65535), // Hysteria2.Port
		gen.IntRange(10, 1000),    // Hysteria2.DownMbps
		gen.IntRange(10, 1000),    // Hysteria2.UpMbps
		gen.IntRange(1000, 60000), // Hysteria2.HopStart
	).FlatMap(func(v interface{}) gopter.Gen {
		vals := v.([]interface{})
		return gopter.CombineGens(
			gen.IntRange(vals[14].(int)+1, 65535), // Hysteria2.HopEnd > HopStart
			gen.IntRange(1000, 65535),             // Tuic.Port
		).Map(func(vals2 []interface{}) *Config {
			cfg := &Config{
				Standalone:        vals[0].(bool),
				APIHost:           vals[1].(string),
				APIToken:          vals[2].(string),
				NodeID:            vals[3].(int),
				NodeType:          vals[4].(string),
				MonitoringEnabled: vals[5].(bool),
				Log: LogConfig{
					Level:    vals[6].(string),
					FilePath: "/var/log/vasmax/test.log",
				},
				Audit: AuditConfig{
					Enabled:  vals[7].(bool),
					FilePath: "/var/log/vasmax/audit.log",
					MaxSize:  vals[8].(int),
					MaxFiles: vals[9].(int),
				},
				CDN: CDNConfig{
					Enabled: vals[10].(bool),
					Address: "cdn.example.com",
				},
				Hysteria2: Hysteria2Config{
					Port:     vals[11].(int),
					DownMbps: vals[12].(int),
					UpMbps:   vals[13].(int),
					HopStart: vals[14].(int),
					HopEnd:   vals2[0].(int),
				},
				Tuic: TuicConfig{
					Port:              vals2[1].(int),
					CongestionControl: "bbr",
				},
				Nginx: NginxConfig{
					LongConnectionTimeout: "86400s",
				},
				Sync: SyncConfig{
					EmptyUsersApplyThreshold: 3,
					MinPullIntervalSeconds:   30,
					MinPushIntervalSeconds:   30,
				},
				Connection: ConnectionConfig{
					KeepAliveMode:             "auto",
					KeepAliveIdleSeconds:      8,
					KeepAliveIntervalSeconds:  8,
					KeepAliveProbes:           3,
					WebSocketHeartbeatSeconds: 8,
				},
				// Use non-empty paths to avoid setDefaults() overwriting them
				Paths: PathsConfig{
					XrayConf:    "/custom/xray/conf/",
					SingBoxConf: "/custom/singbox/conf/",
					Subscribe:   "/custom/subscribe/",
					Cache:       "/custom/cache/",
					NginxConf:   "/custom/nginx/",
				},
			}
			return cfg
		})
	}, reflect.TypeOf((*Config)(nil)))
}

// Feature: node-monitoring-system, Property 8: 配置保存加载往返一致性
// **Validates: Requirements 3.6**
func TestProperty8_ConfigSaveLoadRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("SaveConfig then LoadConfig produces equivalent config", prop.ForAll(
		func(original *Config) bool {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")

			// Save the config
			if err := SaveConfig(path, original); err != nil {
				t.Logf("SaveConfig error: %v", err)
				return false
			}

			// Load it back
			loaded, err := LoadConfig(path)
			if err != nil {
				t.Logf("LoadConfig error: %v", err)
				return false
			}

			// Compare all fields
			if original.Standalone != loaded.Standalone {
				t.Logf("Standalone mismatch: %v != %v", original.Standalone, loaded.Standalone)
				return false
			}
			if original.APIHost != loaded.APIHost {
				t.Logf("APIHost mismatch: %q != %q", original.APIHost, loaded.APIHost)
				return false
			}
			if original.APIToken != loaded.APIToken {
				t.Logf("APIToken mismatch: %q != %q", original.APIToken, loaded.APIToken)
				return false
			}
			if original.NodeID != loaded.NodeID {
				t.Logf("NodeID mismatch: %d != %d", original.NodeID, loaded.NodeID)
				return false
			}
			if original.NodeType != loaded.NodeType {
				t.Logf("NodeType mismatch: %q != %q", original.NodeType, loaded.NodeType)
				return false
			}
			if original.MonitoringEnabled != loaded.MonitoringEnabled {
				t.Logf("MonitoringEnabled mismatch: %v != %v", original.MonitoringEnabled, loaded.MonitoringEnabled)
				return false
			}
			if original.Log != loaded.Log {
				t.Logf("Log mismatch: %+v != %+v", original.Log, loaded.Log)
				return false
			}
			if original.Audit != loaded.Audit {
				t.Logf("Audit mismatch: %+v != %+v", original.Audit, loaded.Audit)
				return false
			}
			if original.CDN != loaded.CDN {
				t.Logf("CDN mismatch: %+v != %+v", original.CDN, loaded.CDN)
				return false
			}
			if original.Hysteria2 != loaded.Hysteria2 {
				t.Logf("Hysteria2 mismatch: %+v != %+v", original.Hysteria2, loaded.Hysteria2)
				return false
			}
			if original.Tuic != loaded.Tuic {
				t.Logf("Tuic mismatch: %+v != %+v", original.Tuic, loaded.Tuic)
				return false
			}
			if original.Nginx != loaded.Nginx {
				t.Logf("Nginx mismatch: %+v != %+v", original.Nginx, loaded.Nginx)
				return false
			}
			if original.Sync != loaded.Sync {
				t.Logf("Sync mismatch: %+v != %+v", original.Sync, loaded.Sync)
				return false
			}
			if original.Connection != loaded.Connection {
				t.Logf("Connection mismatch: %+v != %+v", original.Connection, loaded.Connection)
				return false
			}
			if original.Paths != loaded.Paths {
				t.Logf("Paths mismatch: %+v != %+v", original.Paths, loaded.Paths)
				return false
			}

			return true
		},
		genConfig(),
	))

	properties.TestingRun(t)
}

// genValidAPIHost generates a random HTTPS API host URL (Validate requires https).
func genValidAPIHost() gopter.Gen {
	return gen.RegexMatch(`[a-z]{3,10}\.[a-z]{2,5}`).Map(func(domain string) string {
		return fmt.Sprintf("https://%s", domain)
	})
}

// genValidConfig generates a random Config struct that is guaranteed to pass Validate().
func genValidConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.Bool(),                // Standalone
		genValidAPIHost(),         // APIHost (https only)
		genAPIToken(),             // APIToken
		gen.IntRange(1, 10000),    // NodeID
		genNodeType(),             // NodeType
		gen.Bool(),                // MonitoringEnabled
		genLogLevel(),             // Log.Level
		gen.Bool(),                // Audit.Enabled
		gen.IntRange(1, 100),      // Audit.MaxSize
		gen.IntRange(1, 50),       // Audit.MaxFiles
		gen.IntRange(1024, 65535), // Hysteria2.Port (valid port range)
		gen.IntRange(10, 1000),    // Hysteria2.DownMbps
		gen.IntRange(10, 1000),    // Hysteria2.UpMbps
		gen.IntRange(1024, 60000), // Hysteria2.HopStart
	).FlatMap(func(v interface{}) gopter.Gen {
		vals := v.([]interface{})
		return gopter.CombineGens(
			gen.IntRange(vals[13].(int)+1, 65535), // Hysteria2.HopEnd > HopStart
			gen.IntRange(1024, 65535),             // Tuic.Port (valid port range)
		).Map(func(vals2 []interface{}) *Config {
			cfg := &Config{
				Standalone:        vals[0].(bool),
				APIHost:           vals[1].(string),
				APIToken:          vals[2].(string),
				NodeID:            vals[3].(int),
				NodeType:          vals[4].(string),
				MonitoringEnabled: vals[5].(bool),
				Log: LogConfig{
					Level:    vals[6].(string),
					FilePath: "/var/log/vasmax/test.log",
				},
				Audit: AuditConfig{
					Enabled:  vals[7].(bool),
					FilePath: "/var/log/vasmax/audit.log",
					MaxSize:  vals[8].(int),
					MaxFiles: vals[9].(int),
				},
				CDN: CDNConfig{
					Enabled: false, // CDN disabled to avoid needing valid address
				},
				Hysteria2: Hysteria2Config{
					Port:     vals[10].(int),
					DownMbps: vals[11].(int),
					UpMbps:   vals[12].(int),
					HopStart: vals[13].(int),
					HopEnd:   vals2[0].(int),
				},
				Tuic: TuicConfig{
					Port:              vals2[1].(int),
					CongestionControl: "bbr",
				},
				Nginx: NginxConfig{
					LongConnectionTimeout: "86400s",
				},
				Sync: SyncConfig{
					EmptyUsersApplyThreshold: 3,
					MinPullIntervalSeconds:   30,
					MinPushIntervalSeconds:   30,
				},
				Connection: ConnectionConfig{
					KeepAliveMode:             "auto",
					KeepAliveIdleSeconds:      8,
					KeepAliveIntervalSeconds:  8,
					KeepAliveProbes:           3,
					WebSocketHeartbeatSeconds: 8,
				},
				Paths: PathsConfig{
					XrayConf:    "/custom/xray/conf/",
					SingBoxConf: "/custom/singbox/conf/",
					Subscribe:   "/custom/subscribe/",
					Cache:       "/custom/cache/",
					NginxConf:   "/custom/nginx/",
				},
			}
			return cfg
		})
	}, reflect.TypeOf((*Config)(nil)))
}

// Feature: node-monitoring-system, Property 22: 配置校验兼容性
// **Validates: Requirements 15.1**
func TestProperty22_ConfigValidationCompatibility(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Sub-property 1: Valid configs with MonitoringEnabled=true still pass Validate()
	properties.Property("valid config with MonitoringEnabled=true passes Validate", prop.ForAll(
		func(cfg *Config) bool {
			cfg.MonitoringEnabled = true
			if err := cfg.Validate(); err != nil {
				t.Logf("Validate() failed with MonitoringEnabled=true: %v", err)
				return false
			}
			return true
		},
		genValidConfig(),
	))

	// Sub-property 2: Valid configs with MonitoringEnabled=false still pass Validate()
	properties.Property("valid config with MonitoringEnabled=false passes Validate", prop.ForAll(
		func(cfg *Config) bool {
			cfg.MonitoringEnabled = false
			if err := cfg.Validate(); err != nil {
				t.Logf("Validate() failed with MonitoringEnabled=false: %v", err)
				return false
			}
			return true
		},
		genValidConfig(),
	))

	// Sub-property 3: MonitoringEnabled field does not affect Validate() outcome
	properties.Property("MonitoringEnabled value does not change Validate result", prop.ForAll(
		func(cfg *Config, monEnabled bool) bool {
			// First validate without changing MonitoringEnabled
			origResult := cfg.Validate()

			// Then set MonitoringEnabled to random value and validate again
			cfg.MonitoringEnabled = monEnabled
			newResult := cfg.Validate()

			// Both should have the same outcome (both nil or both non-nil)
			if (origResult == nil) != (newResult == nil) {
				t.Logf("Validate() outcome changed: before=%v, after=%v (MonitoringEnabled=%v)",
					origResult, newResult, monEnabled)
				return false
			}
			return true
		},
		genValidConfig(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}
