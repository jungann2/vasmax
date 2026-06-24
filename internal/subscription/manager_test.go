package subscription

import (
	"testing"

	"vasmax/internal/config"
	"vasmax/internal/protocol"
)

func TestBuildServerInfoUsesProtocolDefaults(t *testing.T) {
	cfg := &config.Config{
		TLS: config.TLSConfig{Domain: "node.example.com"},
	}
	m := &Manager{config: cfg}

	tests := []struct {
		name        string
		protocol    protocol.Protocol
		path        string
		serviceName string
		port        int
	}{
		{
			name:     "vmess_httpupgrade_path",
			protocol: &protocol.VMessHTTPUpgradeTLS{},
			path:     "/vmesshup",
			port:     443,
		},
		{
			name:        "vless_grpc_service_name",
			protocol:    &protocol.VlessGRPCTLS{},
			serviceName: "vlessgrpc",
			port:        443,
		},
		{
			name:     "reality_xhttp_path",
			protocol: &protocol.VlessRealityXHTTP{},
			path:     "/realityxhttp",
			port:     31307,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := m.buildServerInfoForProtocol(tt.protocol, func() string { return "203.0.113.10" })
			if info.Path != tt.path {
				t.Fatalf("expected path %q, got %q", tt.path, info.Path)
			}
			if info.ServiceName != tt.serviceName {
				t.Fatalf("expected serviceName %q, got %q", tt.serviceName, info.ServiceName)
			}
			if info.Port != tt.port {
				t.Fatalf("expected port %d, got %d", tt.port, info.Port)
			}
		})
	}
}

func TestBuildServerInfoUsesConfiguredDirectPort(t *testing.T) {
	cfg := &config.Config{
		TLS:           config.TLSConfig{Domain: "node.example.com"},
		ProtocolPorts: map[string]int{"anytls": 4433},
	}
	m := &Manager{config: cfg}

	info := m.buildServerInfoForProtocol(&protocol.AnyTLS{}, func() string { return "203.0.113.10" })
	if info.Port != 4433 {
		t.Fatalf("expected configured anytls port 4433, got %d", info.Port)
	}
}
