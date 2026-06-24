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
