package nginx

import (
	"strings"
	"testing"
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
