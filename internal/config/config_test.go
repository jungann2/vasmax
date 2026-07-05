package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"vasmax/internal/security"

	"gopkg.in/yaml.v3"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
standalone: true
listen: "0.0.0.0:8080"
api_host: "https://example.com"
api_token: "my-secret-token"
node_id: 42
node_type: "vless"
tls:
  cert_file: "/etc/tls/cert.pem"
  key_file: "/etc/tls/key.pem"
  domain: "example.com"
  provider: "acme"
log:
  level: "debug"
  file_path: "/var/log/test.log"
protocols:
  - "vless_ws_tls"
  - "hysteria2"
core_type: "dual"
paths:
  xray_conf: "/custom/xray/"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.Standalone {
		t.Error("expected standalone=true")
	}
	if cfg.Listen != "0.0.0.0:8080" {
		t.Errorf("expected listen=0.0.0.0:8080, got %s", cfg.Listen)
	}
	if cfg.APIToken != "my-secret-token" {
		t.Errorf("expected api_token=my-secret-token, got %s", cfg.APIToken)
	}
	if cfg.NodeID != 42 {
		t.Errorf("expected node_id=42, got %d", cfg.NodeID)
	}
	if cfg.TLS.Domain != "example.com" {
		t.Errorf("expected tls.domain=example.com, got %s", cfg.TLS.Domain)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log.level=debug, got %s", cfg.Log.Level)
	}
	if len(cfg.Protocols) != 2 {
		t.Errorf("expected 2 protocols, got %d", len(cfg.Protocols))
	}
	// Custom path should be preserved.
	if cfg.Paths.XrayConf != "/custom/xray/" {
		t.Errorf("expected custom xray_conf, got %s", cfg.Paths.XrayConf)
	}
	// Defaults should be applied for empty paths.
	if cfg.Paths.SingBoxConf != "/etc/vasmax/sing-box/conf/config/" {
		t.Errorf("expected default singbox_conf, got %s", cfg.Paths.SingBoxConf)
	}
	if cfg.Paths.Subscribe != "/etc/vasmax/subscribe/" {
		t.Errorf("expected default subscribe path, got %s", cfg.Paths.Subscribe)
	}
	if cfg.Paths.Cache != "/etc/vasmax/cache/" {
		t.Errorf("expected default cache path, got %s", cfg.Paths.Cache)
	}
	if cfg.Paths.NginxConf != "/etc/nginx/conf.d/" {
		t.Errorf("expected default nginx_conf, got %s", cfg.Paths.NginxConf)
	}
	if cfg.Subscription.DNSMode != "auto" {
		t.Errorf("expected default subscription dns_mode auto, got %s", cfg.Subscription.DNSMode)
	}
	if cfg.Subscription.TestURL != "https://www.gstatic.com/generate_204" {
		t.Errorf("expected default subscription test_url, got %s", cfg.Subscription.TestURL)
	}
	if cfg.ServerDNS.Mode != "system" {
		t.Errorf("expected default server_dns mode system, got %s", cfg.ServerDNS.Mode)
	}
	if cfg.ServerDNS.Strategy != "ipv4_only" {
		t.Errorf("expected default server_dns strategy ipv4_only, got %s", cfg.ServerDNS.Strategy)
	}
	if cfg.Nginx.LongConnectionTimeout != "86400s" {
		t.Errorf("expected default nginx long_connection_timeout, got %s", cfg.Nginx.LongConnectionTimeout)
	}
	if cfg.Sync.EmptyUsersApplyThreshold != 3 {
		t.Errorf("expected default sync empty_users_apply_threshold 3, got %d", cfg.Sync.EmptyUsersApplyThreshold)
	}
	if cfg.Sync.MinPullIntervalSeconds != 30 {
		t.Errorf("expected default sync min_pull_interval_seconds 30, got %d", cfg.Sync.MinPullIntervalSeconds)
	}
	if cfg.Sync.MinPushIntervalSeconds != 30 {
		t.Errorf("expected default sync min_push_interval_seconds 30, got %d", cfg.Sync.MinPushIntervalSeconds)
	}
	if cfg.Connection.KeepAliveMode != "auto" {
		t.Errorf("expected default connection keepalive_mode auto, got %s", cfg.Connection.KeepAliveMode)
	}
	if cfg.Connection.KeepAliveIdleSeconds != 8 {
		t.Errorf("expected default connection keepalive_idle_seconds 8, got %d", cfg.Connection.KeepAliveIdleSeconds)
	}
	if cfg.Connection.KeepAliveIntervalSeconds != 8 {
		t.Errorf("expected default connection keepalive_interval_seconds 8, got %d", cfg.Connection.KeepAliveIntervalSeconds)
	}
	if cfg.Connection.KeepAliveProbes != 3 {
		t.Errorf("expected default connection keepalive_probes 3, got %d", cfg.Connection.KeepAliveProbes)
	}
	if cfg.Connection.WebSocketHeartbeatSeconds != 8 {
		t.Errorf("expected default connection websocket_heartbeat_seconds 8, got %d", cfg.Connection.WebSocketHeartbeatSeconds)
	}
}

func TestLoadConfig_DefaultLogLevel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `standalone: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Log.Level != "info" {
		t.Errorf("expected default log level 'info', got %s", cfg.Log.Level)
	}
}

func TestLoadConfig_DecryptENCToken(t *testing.T) {
	// Set a fixed test key for encryption/decryption.
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i)
	}
	security.SetKeyForTesting(testKey)
	defer security.SetKeyForTesting(nil)

	plainToken := "super-secret-api-token"
	encrypted, err := security.EncryptCredential(plainToken)
	if err != nil {
		t.Fatalf("EncryptCredential failed: %v", err)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := "standalone: false\napi_token: \"" + encrypted + "\"\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.APIToken != plainToken {
		t.Errorf("expected decrypted token %q, got %q", plainToken, cfg.APIToken)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Use a YAML string that will fail to unmarshal into Config due to type mismatch.
	// "standalone" expects a bool but we give it a nested map.
	invalidContent := "standalone:\n  nested:\n    - key: value\n"
	if err := os.WriteFile(cfgPath, []byte(invalidContent), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSaveConfig_WritesAndReads(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	original := &Config{
		Standalone: true,
		Listen:     "127.0.0.1:9090",
		APIHost:    "https://panel.example.com",
		APIToken:   "token123",
		NodeID:     7,
		NodeType:   "vmess",
		Log: LogConfig{
			Level:    "warn",
			FilePath: "/var/log/test.log",
		},
		Protocols: []string{"vless_ws_tls", "trojan_tcp_tls"},
		CoreType:  "xray",
		Paths: PathsConfig{
			XrayConf:    "/custom/xray/",
			SingBoxConf: "/custom/singbox/",
			Subscribe:   "/custom/subscribe/",
			Cache:       "/custom/cache/",
			NginxConf:   "/custom/nginx/",
		},
	}

	if err := SaveConfig(cfgPath, original); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file permissions (only meaningful on Unix-like systems).
	if runtime.GOOS != "windows" {
		info, err := os.Stat(cfgPath)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("expected file permission 0600, got %o", perm)
		}
	}

	// Read back and verify round-trip.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.Standalone != original.Standalone {
		t.Errorf("standalone mismatch: got %v", loaded.Standalone)
	}
	if loaded.Listen != original.Listen {
		t.Errorf("listen mismatch: got %s", loaded.Listen)
	}
	if loaded.APIToken != original.APIToken {
		t.Errorf("api_token mismatch: got %s", loaded.APIToken)
	}
	if loaded.NodeID != original.NodeID {
		t.Errorf("node_id mismatch: got %d", loaded.NodeID)
	}
	if loaded.Log.Level != original.Log.Level {
		t.Errorf("log.level mismatch: got %s", loaded.Log.Level)
	}
	if len(loaded.Protocols) != len(original.Protocols) {
		t.Errorf("protocols length mismatch: got %d", len(loaded.Protocols))
	}
	if loaded.Paths.XrayConf != original.Paths.XrayConf {
		t.Errorf("paths.xray_conf mismatch: got %s", loaded.Paths.XrayConf)
	}
}

func TestSaveConfig_AtomicNoPartialWrite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		Standalone: true,
		APIToken:   "test-token",
	}

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify the file exists and is valid YAML.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var check Config
	if err := yaml.Unmarshal(data, &check); err != nil {
		t.Fatalf("saved file is not valid YAML: %v", err)
	}

	if check.APIToken != "test-token" {
		t.Errorf("expected api_token=test-token, got %s", check.APIToken)
	}
}

func TestSetDefaults_AllPaths(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()

	if cfg.Log.Level != "info" {
		t.Errorf("expected default log level 'info', got %s", cfg.Log.Level)
	}
	if cfg.Paths.XrayConf != "/etc/vasmax/xray/conf/" {
		t.Errorf("unexpected xray_conf default: %s", cfg.Paths.XrayConf)
	}
	if cfg.Paths.SingBoxConf != "/etc/vasmax/sing-box/conf/config/" {
		t.Errorf("unexpected singbox_conf default: %s", cfg.Paths.SingBoxConf)
	}
	if cfg.Paths.Subscribe != "/etc/vasmax/subscribe/" {
		t.Errorf("unexpected subscribe default: %s", cfg.Paths.Subscribe)
	}
	if cfg.Paths.Cache != "/etc/vasmax/cache/" {
		t.Errorf("unexpected cache default: %s", cfg.Paths.Cache)
	}
	if cfg.Paths.NginxConf != "/etc/nginx/conf.d/" {
		t.Errorf("unexpected nginx_conf default: %s", cfg.Paths.NginxConf)
	}
	if cfg.Subscription.DNSMode != "auto" {
		t.Errorf("unexpected subscription dns_mode default: %s", cfg.Subscription.DNSMode)
	}
	if cfg.Subscription.TestURL != "https://www.gstatic.com/generate_204" {
		t.Errorf("unexpected subscription test_url default: %s", cfg.Subscription.TestURL)
	}
	if cfg.ServerDNS.Mode != "system" {
		t.Errorf("unexpected server_dns mode default: %s", cfg.ServerDNS.Mode)
	}
	if cfg.ServerDNS.Strategy != "ipv4_only" {
		t.Errorf("unexpected server_dns strategy default: %s", cfg.ServerDNS.Strategy)
	}
	if cfg.Nginx.LongConnectionTimeout != "86400s" {
		t.Errorf("unexpected nginx long_connection_timeout default: %s", cfg.Nginx.LongConnectionTimeout)
	}
	if cfg.Sync.EmptyUsersApplyThreshold != 3 {
		t.Errorf("unexpected sync empty_users_apply_threshold default: %d", cfg.Sync.EmptyUsersApplyThreshold)
	}
	if cfg.Sync.MinPullIntervalSeconds != 30 {
		t.Errorf("unexpected sync min_pull_interval_seconds default: %d", cfg.Sync.MinPullIntervalSeconds)
	}
	if cfg.Sync.MinPushIntervalSeconds != 30 {
		t.Errorf("unexpected sync min_push_interval_seconds default: %d", cfg.Sync.MinPushIntervalSeconds)
	}
	if cfg.Connection.KeepAliveMode != "auto" {
		t.Errorf("unexpected connection keepalive_mode default: %s", cfg.Connection.KeepAliveMode)
	}
	if cfg.Connection.KeepAliveIdleSeconds != 8 {
		t.Errorf("unexpected connection keepalive_idle_seconds default: %d", cfg.Connection.KeepAliveIdleSeconds)
	}
	if cfg.Connection.KeepAliveIntervalSeconds != 8 {
		t.Errorf("unexpected connection keepalive_interval_seconds default: %d", cfg.Connection.KeepAliveIntervalSeconds)
	}
	if cfg.Connection.KeepAliveProbes != 3 {
		t.Errorf("unexpected connection keepalive_probes default: %d", cfg.Connection.KeepAliveProbes)
	}
	if cfg.Connection.WebSocketHeartbeatSeconds != 8 {
		t.Errorf("unexpected connection websocket_heartbeat_seconds default: %d", cfg.Connection.WebSocketHeartbeatSeconds)
	}
}

func TestValidate_SubscriptionDNSMode(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Subscription: SubscriptionConfig{
			DNSMode: "bad-mode",
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid dns_mode error")
	}

	cfg.Subscription.DNSMode = "custom"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected custom dns mode without dns_custom to fail")
	}

	cfg.Subscription.DNSCustom = []string{"   "}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected custom dns mode with blank dns_custom to fail")
	}

	cfg.Subscription.DNSCustom = []string{"https://dns.example/dns-query"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected custom dns mode with dns_custom to pass: %v", err)
	}

	cfg.Subscription.TestURL = "http://example.com/generate_204"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-https test_url to fail")
	}

	cfg.Subscription.TestURL = "https://www.gstatic.com/generate_204"
	cfg.Subscription.ServerIP = "not-an-ip"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid subscription.server_ip to fail")
	}

	cfg.Subscription.ServerIP = "203.0.113.10"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid subscription.server_ip to pass: %v", err)
	}
}

func TestValidate_ServerDNS(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		ServerDNS:  ServerDNSConfig{Mode: "bad-mode", Strategy: "ipv4_only"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid server_dns.mode to fail")
	}

	cfg.ServerDNS.Mode = ServerDNSModeCustom
	cfg.ServerDNS.Servers = []string{"https://dns.example/dns-query"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected custom server_dns without plain IP to fail")
	}

	cfg.ServerDNS.Servers = []string{"1.1.1.1", "1.1.1.1", "9.9.9.9"}
	cfg.ServerDNS.Strategy = "bad-strategy"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid server_dns.strategy to fail")
	}

	cfg.ServerDNS.Strategy = "ipv4_only"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid server_dns custom config, got %v", err)
	}
	servers := cfg.ServerDNS.EffectiveServers()
	if len(servers) != 2 || servers[0] != "1.1.1.1" || servers[1] != "9.9.9.9" {
		t.Fatalf("expected deduplicated custom server dns, got %#v", servers)
	}
}

func TestValidate_NginxLongConnectionTimeout(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Nginx:      NginxConfig{LongConnectionTimeout: "172800s"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid nginx long_connection_timeout to pass: %v", err)
	}

	cfg.Nginx.LongConnectionTimeout = "172800"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected nginx long_connection_timeout without unit to fail")
	}
}

func TestValidate_SyncEmptyUsersApplyThreshold(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Sync:       SyncConfig{EmptyUsersApplyThreshold: -1},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected -1 empty_users_apply_threshold to disable protection: %v", err)
	}

	cfg.Sync.EmptyUsersApplyThreshold = -2
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty_users_apply_threshold below -1 to fail")
	}
}

func TestValidate_SyncMinIntervals(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Sync: SyncConfig{
			MinPullIntervalSeconds: 30,
			MinPushIntervalSeconds: 30,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid sync min intervals to pass: %v", err)
	}

	cfg.Sync.MinPullIntervalSeconds = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative min_pull_interval_seconds to fail")
	}

	cfg.Sync.MinPullIntervalSeconds = 30
	cfg.Sync.MinPushIntervalSeconds = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative min_push_interval_seconds to fail")
	}
}

func TestValidate_ConnectionKeepAlive(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Connection: ConnectionConfig{
			KeepAliveMode:             "auto",
			KeepAliveIdleSeconds:      8,
			KeepAliveIntervalSeconds:  8,
			KeepAliveProbes:           3,
			WebSocketHeartbeatSeconds: 8,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid connection keepalive config to pass: %v", err)
	}

	cfg.Connection.KeepAliveMode = "bad"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid keepalive_mode to fail")
	}

	cfg.Connection.KeepAliveMode = "auto"
	cfg.Connection.KeepAliveIdleSeconds = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative keepalive_idle_seconds to fail")
	}

	cfg.Connection.KeepAliveIdleSeconds = 8
	cfg.Connection.KeepAliveIntervalSeconds = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative keepalive_interval_seconds to fail")
	}

	cfg.Connection.KeepAliveIntervalSeconds = 8
	cfg.Connection.KeepAliveProbes = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative keepalive_probes to fail")
	}

	cfg.Connection.KeepAliveProbes = 3
	cfg.Connection.WebSocketHeartbeatSeconds = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative websocket_heartbeat_seconds to fail")
	}
}

func TestSaveConfig_WritesReferenceHeader(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &Config{Standalone: true, Log: LogConfig{Level: "info"}}

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "config.reference.yaml") {
		t.Fatalf("expected saved config to mention config.reference.yaml:\n%s", string(data))
	}
}

func TestWriteReferenceConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	if err := WriteReferenceConfig(cfgPath); err != nil {
		t.Fatalf("WriteReferenceConfig failed: %v", err)
	}
	refPath := ReferenceConfigPath(cfgPath)
	data, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "long_connection_timeout") {
		t.Fatalf("expected reference config to document long_connection_timeout:\n%s", string(data))
	}

	var parsed Config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("reference config should be valid YAML: %v", err)
	}
}

func TestSetDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &Config{
		Log: LogConfig{Level: "debug"},
		Paths: PathsConfig{
			XrayConf: "/my/custom/path/",
		},
	}
	cfg.setDefaults()

	if cfg.Log.Level != "debug" {
		t.Errorf("setDefaults should not override existing log level, got %s", cfg.Log.Level)
	}
	if cfg.Paths.XrayConf != "/my/custom/path/" {
		t.Errorf("setDefaults should not override existing xray_conf, got %s", cfg.Paths.XrayConf)
	}
	// Other paths should still get defaults.
	if cfg.Paths.SingBoxConf != "/etc/vasmax/sing-box/conf/config/" {
		t.Errorf("expected default singbox_conf, got %s", cfg.Paths.SingBoxConf)
	}
}
