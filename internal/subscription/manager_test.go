package subscription

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"vasmax/internal/api"
	"vasmax/internal/config"
	"vasmax/internal/protocol"
	"vasmax/internal/user"
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
			info, err := m.buildServerInfoForProtocol(tt.protocol, func() (string, error) { return "203.0.113.10", nil })
			if err != nil {
				t.Fatal(err)
			}
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

	info, err := m.buildServerInfoForProtocol(&protocol.AnyTLS{}, func() (string, error) { return "203.0.113.10", nil })
	if err != nil {
		t.Fatal(err)
	}
	if info.Port != 4433 {
		t.Fatalf("expected configured anytls port 4433, got %d", info.Port)
	}
}

func TestBuildServerInfoNoDomainRequiresServerIP(t *testing.T) {
	cfg := &config.Config{
		ProtocolModes: map[string]string{"anytls": "nodomain"},
	}
	m := &Manager{config: cfg}

	if _, err := m.buildServerInfoForProtocol(&protocol.AnyTLS{}, func() (string, error) {
		return "", assertErr("no public ip")
	}); err == nil {
		t.Fatal("expected nodomain server info to fail when server IP is unavailable")
	}
}

func TestResolveServerIPUsesConfiguredServerIP(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{ServerIP: "203.0.113.10"},
	}
	m := &Manager{config: cfg}

	ip, err := m.resolveServerIP()
	if err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("expected configured server ip, got %s", ip)
	}
}

func TestGenerateAllReturnsUserErrors(t *testing.T) {
	subscribePath := filepath.Join(t.TempDir(), "subscribe-file")
	if err := os.WriteFile(subscribePath, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}

	um := user.NewManager()
	um.UpdateUsers([]api.User{{ID: 1, UUID: "11111111-1111-1111-1111-111111111111"}})
	m := &Manager{
		config: &config.Config{
			Paths:     config.PathsConfig{Subscribe: subscribePath},
			TLS:       config.TLSConfig{Domain: "node.example.com"},
			Protocols: []string{"anytls"},
		},
		registry: protocol.DefaultRegistry(),
		users:    um,
		salt:     "test-salt",
		logger:   testLogger(),
	}

	err := m.GenerateAll()
	if err == nil {
		t.Fatal("expected GenerateAll to return subscription generation error")
	}
	if !strings.Contains(err.Error(), "user 1") {
		t.Fatalf("expected user id in aggregated error, got %v", err)
	}
}

func TestGenerateForUserReturnsBaseSubscriptionWriteError(t *testing.T) {
	subscribePath := t.TempDir()
	u := &user.UserEntry{ID: 1, UUID: "11111111-1111-1111-1111-111111111111", Email: "user_1"}
	m := &Manager{
		config: &config.Config{
			Paths:     config.PathsConfig{Subscribe: subscribePath},
			TLS:       config.TLSConfig{Domain: "node.example.com"},
			Protocols: []string{"anytls"},
		},
		registry: protocol.DefaultRegistry(),
		salt:     "test-salt",
		logger:   testLogger(),
	}
	subDir := filepath.Join(subscribePath, GenerateSubscribePath(u.Email, m.salt))
	if err := os.MkdirAll(filepath.Join(subDir, "default"), 0755); err != nil {
		t.Fatal(err)
	}

	err := m.GenerateForUser(u)
	if err == nil {
		t.Fatal("expected GenerateForUser to return write error")
	}
	if !strings.Contains(err.Error(), "failed to write base64 subscription") {
		t.Fatalf("expected base64 write error, got %v", err)
	}
}

func TestSubscriptionManagerAllowsNilLogger(t *testing.T) {
	cfg := &config.Config{
		Paths: config.PathsConfig{Subscribe: t.TempDir()},
	}
	um := user.NewManager()
	m, err := NewManager(cfg, protocol.DefaultRegistry(), um, nil)
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}
	if err := m.GenerateAll(); err != nil {
		t.Fatalf("GenerateAll with nil logger and no users should not fail: %v", err)
	}
}

func TestGenerateAllPrunesStaleSubscriptionDirs(t *testing.T) {
	subscribePath := t.TempDir()
	staleDir := filepath.Join(subscribePath, "00000000000000000000000000000000")
	keepDir := filepath.Join(subscribePath, "subscribe_local")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(keepDir, 0755); err != nil {
		t.Fatal(err)
	}

	um := user.NewManager()
	um.UpdateUsers([]api.User{{ID: 1, UUID: "11111111-1111-4111-8111-111111111111"}})
	m := &Manager{
		config: &config.Config{
			Paths:     config.PathsConfig{Subscribe: subscribePath},
			TLS:       config.TLSConfig{Domain: "node.example.com"},
			Protocols: []string{"anytls"},
		},
		registry: protocol.DefaultRegistry(),
		users:    um,
		salt:     "test-salt",
		logger:   testLogger(),
	}

	if err := m.GenerateAll(); err != nil {
		t.Fatalf("GenerateAll failed: %v", err)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Fatalf("expected stale subscription dir removed, stat err=%v", err)
	}
	if _, err := os.Stat(keepDir); err != nil {
		t.Fatalf("expected non-generated dir preserved: %v", err)
	}
	currentDir := filepath.Join(subscribePath, GenerateSubscribePath("user_1", m.salt))
	if _, err := os.Stat(filepath.Join(currentDir, "default")); err != nil {
		t.Fatalf("expected current user subscription generated: %v", err)
	}
}

func TestGenerateAllWithNoUsersPrunesOldSubscriptionDirs(t *testing.T) {
	subscribePath := t.TempDir()
	staleDir := filepath.Join(subscribePath, "ffffffffffffffffffffffffffffffff")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	um := user.NewManager()
	m := &Manager{
		config: &config.Config{Paths: config.PathsConfig{Subscribe: subscribePath}},
		users:  um,
		salt:   "test-salt",
		logger: testLogger(),
	}

	if err := m.GenerateAll(); err != nil {
		t.Fatalf("GenerateAll failed: %v", err)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Fatalf("expected stale subscription dir removed with no users, stat err=%v", err)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func testLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}
