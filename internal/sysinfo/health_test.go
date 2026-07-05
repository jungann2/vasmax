package sysinfo

import (
	"errors"
	"testing"

	"vasmax/internal/config"
	"vasmax/internal/protocol"
)

func TestNeededHealthProcessesFollowsConfiguredProtocols(t *testing.T) {
	realityOnly := &config.Config{
		Protocols: []string{"vless_reality_vision"},
	}
	needed := neededHealthProcesses(realityOnly)
	if !needed["xray"] {
		t.Fatal("expected Reality protocol to require xray")
	}
	if needed["singbox"] {
		t.Fatal("did not expect Reality-only config to require sing-box")
	}
	if needed["nginx"] {
		t.Fatal("did not expect no-domain Reality-only config to require nginx")
	}

	withDomain := &config.Config{
		Protocols: []string{"vless_reality_vision"},
		TLS:       config.TLSConfig{Domain: "node.example.com"},
	}
	needed = neededHealthProcesses(withDomain)
	if needed["nginx"] {
		t.Fatal("did not expect stale tls.domain alone to require nginx health check")
	}

	withSubscriptionDomain := &config.Config{
		Protocols:    []string{"vless_reality_vision"},
		Subscription: config.SubscriptionConfig{Domain: "sub.example.com"},
	}
	needed = neededHealthProcesses(withSubscriptionDomain)
	if !needed["nginx"] {
		t.Fatal("expected subscription domain to require nginx health check")
	}

	directDomain := &config.Config{
		Protocols:     []string{"anytls"},
		ProtocolModes: map[string]string{"anytls": "domain"},
		TLS:           config.TLSConfig{Domain: "node.example.com"},
	}
	needed = neededHealthProcesses(directDomain)
	if needed["nginx"] {
		t.Fatal("did not expect direct domain AnyTLS to require nginx")
	}

	dualCore := &config.Config{
		Protocols: []string{"vless_ws_tls", "anytls"},
	}
	needed = neededHealthProcesses(dualCore)
	if !needed["xray"] || !needed["singbox"] || !needed["nginx"] {
		t.Fatalf("expected xray, sing-box, and nginx checks, got %#v", needed)
	}
}

func TestHasNoDomainProtocol(t *testing.T) {
	if !hasNoDomainProtocol(&config.Config{Protocols: []string{"vless_reality_vision"}}) {
		t.Fatal("expected Reality protocol to be treated as no-domain")
	}
	if !hasNoDomainProtocol(&config.Config{
		Protocols:     []string{"anytls"},
		ProtocolModes: map[string]string{"anytls": "nodomain"},
	}) {
		t.Fatal("expected explicit nodomain mode to be detected")
	}
	if hasNoDomainProtocol(&config.Config{Protocols: []string{"vless_ws_tls"}}) {
		t.Fatal("did not expect domain TLS protocol to be no-domain")
	}
	if hasNoDomainProtocol(&config.Config{
		Protocols:     []string{"vless_reality_vision"},
		ProtocolModes: map[string]string{"vless_reality_vision": "domain"},
	}) {
		t.Fatal("did not expect domain-mode Reality protocol to require no-domain subscription IP")
	}
}

func TestRealityHealthTargetsUseDestPort(t *testing.T) {
	cfg := &config.Config{
		Reality: config.RealityConfig{
			ServerName: "example.com",
			Dest:       "example.com:8443",
		},
	}
	got := realityHealthTargets(cfg)
	if len(got) != 1 || got[0] != "example.com:8443" {
		t.Fatalf("expected dest with port, got %#v", got)
	}
}

func TestRealityHealthTargetsUsePoolDestPorts(t *testing.T) {
	cfg := &config.Config{
		Reality: config.RealityConfig{
			Targets: []config.RealityTarget{
				{ServerName: "one.example.com", Dest: "one.example.com:8443", Port: 31305},
				{ServerName: "two.example.com", Dest: "two.example.com:9443", Port: 31306},
			},
		},
	}
	got := realityHealthTargets(cfg)
	want := []string{"one.example.com:8443", "two.example.com:9443"}
	if len(got) != len(want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	}
}

func TestExpectedListenPortsRealityVisionTargetPool(t *testing.T) {
	reg := protocol.DefaultRegistry()
	p, ok := reg.Get("vless_reality_vision")
	if !ok {
		t.Fatal("missing vless_reality_vision protocol")
	}
	cfg := &config.Config{
		ProtocolPorts: map[string]int{"vless_reality_vision": 31305},
		Reality: config.RealityConfig{
			Targets: []config.RealityTarget{
				{ServerName: "one.example.com", Dest: "one.example.com:443", Port: 31305},
				{ServerName: "two.example.com", Dest: "two.example.com:443", Port: 31306},
			},
		},
	}
	got := expectedListenPorts(cfg, p)
	if len(got) != 2 || got[0] != 31305 || got[1] != 31306 {
		t.Fatalf("unexpected target pool ports: %#v", got)
	}
}

func TestParseListenPort(t *testing.T) {
	for _, input := range []string{"0.0.0.0:31303", "[::]:31305", "*:443", "127.0.0.1:8080"} {
		if port, ok := parseListenPort(input); !ok || port <= 0 {
			t.Fatalf("expected %q to parse, got port=%d ok=%t", input, port, ok)
		}
	}
	if _, ok := parseListenPort("not-a-port"); ok {
		t.Fatal("expected invalid listen address to fail")
	}
}

func TestIPv6HealthHonorsExplicitIPv6Strategy(t *testing.T) {
	dialErr := errors.New("network unreachable")
	if got := ipv6HealthFromProbe("ipv6_only", "example.com", false, nil, dialErr); got.Status != "unhealthy" {
		t.Fatalf("expected ipv6_only without IPv6 egress to be unhealthy, got %#v", got)
	}
	if got := ipv6HealthFromProbe("prefer_ipv6", "example.com", false, nil, dialErr); got.Status != "warning" {
		t.Fatalf("expected prefer_ipv6 without IPv6 egress to warn, got %#v", got)
	}
	if got := ipv6HealthFromProbe("ipv4_only", "example.com", false, nil, dialErr); got.Status != "healthy" {
		t.Fatalf("expected ipv4_only without AAAA to stay healthy, got %#v", got)
	}
}
