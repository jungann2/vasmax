package config

import (
	"os"
	"path/filepath"
	"runtime"
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
