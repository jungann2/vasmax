package nginx

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
)

func TestLocationTagIncludesPath(t *testing.T) {
	wsTag := locationTag("ws", "/vlessws")
	grpcTag := locationTag("grpc", "vlessws")

	if wsTag == grpcTag {
		t.Fatalf("expected protocol/path-specific tags, got %q", wsTag)
	}
	if !strings.Contains(wsTag, "VLESSWS") {
		t.Fatalf("expected path in tag, got %q", wsTag)
	}
}

func TestConfigTransactionRestoresOldConfigOnValidateFailure(t *testing.T) {
	oldValidate := validateNginxConfig
	defer func() { validateNginxConfig = oldValidate }()
	validateNginxConfig = func() error { return errors.New("bad nginx config") }

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "node.example.com.conf")
	oldContent := "server { # old }\n"
	if err := os.WriteFile(confPath, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(confDir, nil)
	err := m.GenerateConfigSafe(&NginxParams{
		Domain:   "node.example.com",
		CertFile: "/etc/ssl/cert.pem",
		KeyFile:  "/etc/ssl/key.pem",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	got, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != oldContent {
		t.Fatalf("expected old config restored, got:\n%s", string(got))
	}
}

func TestConfigTransactionRestoresOldConfigModeOnValidateFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not preserve POSIX file modes")
	}
	oldValidate := validateNginxConfig
	defer func() { validateNginxConfig = oldValidate }()
	validateNginxConfig = func() error { return errors.New("bad nginx config") }

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "node.example.com.conf")
	oldContent := "server { # old }\n"
	if err := os.WriteFile(confPath, []byte(oldContent), 0600); err != nil {
		t.Fatal(err)
	}

	m := NewManager(confDir, nil)
	err := m.GenerateConfigSafe(&NginxParams{
		Domain:   "node.example.com",
		CertFile: "/etc/ssl/cert.pem",
		KeyFile:  "/etc/ssl/key.pem",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	info, statErr := os.Stat(confPath)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected restored mode 0600, got %v", info.Mode().Perm())
	}
}

func TestConfigTransactionRemovesNewConfigOnValidateFailure(t *testing.T) {
	oldValidate := validateNginxConfig
	defer func() { validateNginxConfig = oldValidate }()
	validateNginxConfig = func() error { return errors.New("bad nginx config") }

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "node.example.com.conf")
	m := NewManager(confDir, nil)
	err := m.GenerateConfigSafe(&NginxParams{
		Domain:   "node.example.com",
		CertFile: "/etc/ssl/cert.pem",
		KeyFile:  "/etc/ssl/key.pem",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if _, statErr := os.Stat(confPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected new config removed, stat err=%v", statErr)
	}
}

func TestConfigTransactionRestoresOnReloadFailure(t *testing.T) {
	oldValidate := validateNginxConfig
	oldReload := reloadNginxConfig
	defer func() {
		validateNginxConfig = oldValidate
		reloadNginxConfig = oldReload
	}()
	validateNginxConfig = func() error { return nil }
	reloadNginxConfig = func(_ *logrus.Logger) error { return errors.New("reload failed") }

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "node.example.com.conf")
	oldContent := "server { # old }\n"
	if err := os.WriteFile(confPath, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(confDir, nil)
	tx := m.BeginConfigTransaction()
	if err := tx.GenerateConfig(&NginxParams{
		Domain:   "node.example.com",
		CertFile: "/etc/ssl/cert.pem",
		KeyFile:  "/etc/ssl/key.pem",
	}); err != nil {
		t.Fatalf("generate should pass: %v", err)
	}
	if err := tx.Reload(); err == nil {
		t.Fatal("expected reload error")
	}
	got, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != oldContent {
		t.Fatalf("expected old config restored after reload failure, got:\n%s", string(got))
	}
}

func TestConfigTransactionRemoveLocationRestoresOnValidateFailure(t *testing.T) {
	oldValidate := validateNginxConfig
	defer func() { validateNginxConfig = oldValidate }()
	validateNginxConfig = func() error { return errors.New("bad nginx config") }

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "node.example.com.conf")
	oldContent := strings.Join([]string{
		"server {",
		"    # --- BEGIN WS_VLESSWS ---",
		"    location /vlessws {",
		"        proxy_pass http://127.0.0.1:31297;",
		"    }",
		"    # --- END WS_VLESSWS ---",
		"    # --- END LOCATIONS ---",
		"}",
	}, "\n")
	if err := os.WriteFile(confPath, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(confDir, nil)
	tx := m.BeginConfigTransaction()
	if err := tx.RemoveLocationByPath("node.example.com", "ws", "/vlessws"); err == nil {
		t.Fatal("expected validation error")
	}
	got, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != oldContent {
		t.Fatalf("expected old config restored, got:\n%s", string(got))
	}
}

func TestRemoveMarkedBlockMatchesLegacyByPath(t *testing.T) {
	content := strings.Join([]string{
		"server {",
		"    # --- BEGIN WS ---",
		"    location /vlessws {",
		"        proxy_pass http://127.0.0.1:31297;",
		"    }",
		"    # --- END WS ---",
		"    # --- BEGIN WS ---",
		"    location /vmessws {",
		"        proxy_pass http://127.0.0.1:31301;",
		"    }",
		"    # --- END WS ---",
		"}",
	}, "\n")

	got, removed := removeMarkedBlock(content, "WS", func(block string) bool {
		return strings.Contains(block, "location /vmessws ")
	})
	if !removed {
		t.Fatal("expected block to be removed")
	}
	if strings.Contains(got, "/vmessws") {
		t.Fatalf("expected vmessws block removed:\n%s", got)
	}
	if !strings.Contains(got, "/vlessws") {
		t.Fatalf("expected vlessws block preserved:\n%s", got)
	}
}

func TestLocationBlockUsesLongConnectionSettings(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		path     string
		expected []string
	}{
		{
			name: "websocket",
			typ:  "ws",
			path: "/vlessws",
			expected: []string{
				"proxy_read_timeout 86400s;",
				"proxy_send_timeout 86400s;",
				"proxy_socket_keepalive on;",
				"proxy_buffering off;",
				"proxy_request_buffering off;",
			},
		},
		{
			name: "httpupgrade",
			typ:  "httpupgrade",
			path: "/vmesshup",
			expected: []string{
				"proxy_read_timeout 86400s;",
				"proxy_send_timeout 86400s;",
				"proxy_socket_keepalive on;",
				"proxy_buffering off;",
				"proxy_request_buffering off;",
			},
		},
		{
			name: "grpc",
			typ:  "grpc",
			path: "vlessgrpc",
			expected: []string{
				"grpc_read_timeout 86400s;",
				"grpc_send_timeout 86400s;",
				"grpc_socket_keepalive on;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := generateLocationBlock(tt.typ, tt.path, 31297)
			for _, expected := range tt.expected {
				if !strings.Contains(block, expected) {
					t.Fatalf("expected %q in block:\n%s", expected, block)
				}
			}
		})
	}
}

func TestServerBlockUsesConfiguredLongConnectionTimeout(t *testing.T) {
	conf := generateServerBlock(&NginxParams{
		Domain:                "node.example.com",
		CertFile:              "/etc/vasmax/tls/fullchain.crt",
		KeyFile:               "/etc/vasmax/tls/private.key",
		LongConnectionTimeout: "172800s",
		Protocols: []ProtocolLocation{
			{Type: "ws", Path: "/vlessws", BackendPort: 31297},
			{Type: "grpc", Path: "vlessgrpc", BackendPort: 31302},
		},
	})

	for _, expected := range []string{
		"proxy_read_timeout 172800s;",
		"proxy_send_timeout 172800s;",
		"grpc_read_timeout 172800s;",
		"grpc_send_timeout 172800s;",
	} {
		if !strings.Contains(conf, expected) {
			t.Fatalf("expected %q in generated config:\n%s", expected, conf)
		}
	}
}

func TestServerBlockUsesConfiguredSocketKeepAlive(t *testing.T) {
	conf := generateServerBlock(&NginxParams{
		Domain:   "example.com",
		CertFile: "/etc/ssl/cert.pem",
		KeyFile:  "/etc/ssl/key.pem",
		Connection: config.ConnectionConfig{
			KeepAliveMode:            "auto",
			KeepAliveIdleSeconds:     8,
			KeepAliveIntervalSeconds: 8,
			KeepAliveProbes:          3,
		},
	})

	for _, expected := range []string{
		"listen 80 so_keepalive=8s:8s:3;",
		"listen 443 ssl so_keepalive=8s:8s:3;",
	} {
		if !strings.Contains(conf, expected) {
			t.Fatalf("expected %q in generated config:\n%s", expected, conf)
		}
	}
}
