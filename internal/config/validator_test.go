package config

import (
	"strings"
	"testing"
)

func TestValidate_StandaloneMode_ValidConfig(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for valid standalone config, got: %v", err)
	}
}

func TestValidate_StandaloneMode_AllLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := &Config{
			Standalone: true,
			Log:        LogConfig{Level: level},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("log level %q should be valid, got: %v", level, err)
		}
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "verbose"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if !strings.Contains(err.Error(), "log.level") {
		t.Errorf("error should mention log.level, got: %v", err)
	}
}

func TestValidate_ManagedMode_ValidConfig(t *testing.T) {
	cfg := &Config{
		Standalone: false,
		APIHost:    "https://panel.example.com",
		APIToken:   "secret-token",
		NodeID:     1,
		Log:        LogConfig{Level: "info"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for valid managed config, got: %v", err)
	}
}

func TestValidate_ManagedMode_MissingAPIHost(t *testing.T) {
	cfg := &Config{
		Standalone: false,
		APIHost:    "",
		APIToken:   "token",
		NodeID:     1,
		Log:        LogConfig{Level: "info"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing api_host")
	}
	if !strings.Contains(err.Error(), "api_host") {
		t.Errorf("error should mention api_host, got: %v", err)
	}
}

func TestValidate_ManagedMode_InvalidAPIHostURL(t *testing.T) {
	cfg := &Config{
		Standalone: false,
		APIHost:    "http://insecure.example.com",
		APIToken:   "token",
		NodeID:     1,
		Log:        LogConfig{Level: "info"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for non-HTTPS api_host")
	}
	if !strings.Contains(err.Error(), "api_host") {
		t.Errorf("error should mention api_host, got: %v", err)
	}
}

func TestValidate_ManagedMode_MissingAPIToken(t *testing.T) {
	cfg := &Config{
		Standalone: false,
		APIHost:    "https://panel.example.com",
		APIToken:   "",
		NodeID:     1,
		Log:        LogConfig{Level: "info"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing api_token")
	}
	if !strings.Contains(err.Error(), "api_token") {
		t.Errorf("error should mention api_token, got: %v", err)
	}
}

func TestValidate_ManagedMode_InvalidNodeID(t *testing.T) {
	cfg := &Config{
		Standalone: false,
		APIHost:    "https://panel.example.com",
		APIToken:   "token",
		NodeID:     0,
		Log:        LogConfig{Level: "info"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for node_id=0")
	}
	if !strings.Contains(err.Error(), "node_id") {
		t.Errorf("error should mention node_id, got: %v", err)
	}
}

func TestValidate_StandaloneMode_SkipsManagedFields(t *testing.T) {
	// In standalone mode, empty api_host/api_token and zero node_id are fine.
	cfg := &Config{
		Standalone: true,
		APIHost:    "",
		APIToken:   "",
		NodeID:     0,
		Log:        LogConfig{Level: "info"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("standalone mode should not require managed fields, got: %v", err)
	}
}

func TestValidate_TLSDomain_Valid(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		TLS:        TLSConfig{Domain: "example.com"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for valid TLS domain, got: %v", err)
	}
}

func TestValidate_TLSDomain_Invalid(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		TLS:        TLSConfig{Domain: "not a domain!"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid TLS domain")
	}
	if !strings.Contains(err.Error(), "tls.domain") {
		t.Errorf("error should mention tls.domain, got: %v", err)
	}
}

func TestValidate_Hysteria2Port_Valid(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Hysteria2:  Hysteria2Config{Port: 443},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for valid hysteria2 port, got: %v", err)
	}
}

func TestValidate_Hysteria2Port_Invalid(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Hysteria2:  Hysteria2Config{Port: 70000},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid hysteria2 port")
	}
	if !strings.Contains(err.Error(), "hysteria2.port") {
		t.Errorf("error should mention hysteria2.port, got: %v", err)
	}
}

func TestValidate_TuicPort_Valid(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Tuic:       TuicConfig{Port: 8443},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for valid tuic port, got: %v", err)
	}
}

func TestValidate_TuicPort_Invalid(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		Tuic:       TuicConfig{Port: -1},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid tuic port")
	}
	if !strings.Contains(err.Error(), "tuic.port") {
		t.Errorf("error should mention tuic.port, got: %v", err)
	}
}

func TestValidate_CDN_EnabledWithoutAddress(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		CDN:        CDNConfig{Enabled: true, Address: ""},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for CDN enabled without address")
	}
	if !strings.Contains(err.Error(), "cdn.address") {
		t.Errorf("error should mention cdn.address, got: %v", err)
	}
}

func TestValidate_CDN_EnabledWithAddress(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		CDN:        CDNConfig{Enabled: true, Address: "cdn.example.com"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for CDN with address, got: %v", err)
	}
}

func TestValidate_CDN_DisabledWithoutAddress(t *testing.T) {
	cfg := &Config{
		Standalone: true,
		Log:        LogConfig{Level: "info"},
		CDN:        CDNConfig{Enabled: false, Address: ""},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("CDN disabled should not require address, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &Config{
		Standalone: false,
		APIHost:    "",
		APIToken:   "",
		NodeID:     0,
		Log:        LogConfig{Level: "invalid"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected multiple validation errors")
	}
	errStr := err.Error()
	// Should collect all errors, not stop at the first one.
	if !strings.Contains(errStr, "log.level") {
		t.Error("expected log.level error")
	}
	if !strings.Contains(errStr, "api_host") {
		t.Error("expected api_host error")
	}
	if !strings.Contains(errStr, "api_token") {
		t.Error("expected api_token error")
	}
	if !strings.Contains(errStr, "node_id") {
		t.Error("expected node_id error")
	}
}
