package menu

import (
	"strings"
	"testing"

	"vasmax/internal/config"
)

func TestSubscriptionServiceWarningRealityOnlyTLSFallback(t *testing.T) {
	cfg := &config.Config{
		Protocols: []string{"vless_reality_vision"},
		TLS:       config.TLSConfig{Domain: "node.example.com"},
	}

	domain, explicit := subscriptionDisplayDomain(cfg)
	if domain != "node.example.com" {
		t.Fatalf("expected TLS fallback domain, got %q", domain)
	}
	if explicit {
		t.Fatal("TLS fallback domain should not be marked as explicit subscription domain")
	}
	warning := subscriptionServiceWarning(cfg, domain, explicit)
	if !strings.Contains(warning, "Reality-only") {
		t.Fatalf("expected Reality-only warning, got %q", warning)
	}
}

func TestSubscriptionServiceWarningExplicitDomainWithoutNginx(t *testing.T) {
	cfg := &config.Config{
		Protocols:    []string{"vless_reality_vision"},
		Subscription: config.SubscriptionConfig{Domain: "sub.example.com"},
	}

	domain, explicit := subscriptionDisplayDomain(cfg)
	if domain != "sub.example.com" {
		t.Fatalf("expected explicit subscription domain, got %q", domain)
	}
	if !explicit {
		t.Fatal("subscription.domain should be marked explicit")
	}
	warning := subscriptionServiceWarning(cfg, domain, explicit)
	if !strings.Contains(warning, "不会自动创建 /s/") {
		t.Fatalf("expected manual service warning, got %q", warning)
	}
}

func TestSubscriptionServiceWarningWithNginxProtocol(t *testing.T) {
	cfg := &config.Config{
		Protocols:    []string{"vless_ws_tls"},
		Subscription: config.SubscriptionConfig{Domain: "sub.example.com"},
	}

	domain, explicit := subscriptionDisplayDomain(cfg)
	if warning := subscriptionServiceWarning(cfg, domain, explicit); warning != "" {
		t.Fatalf("expected no warning when Nginx proxy protocol exists, got %q", warning)
	}
}
