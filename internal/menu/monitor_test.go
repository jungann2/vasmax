package menu

import (
	"testing"

	"vasmax/internal/config"
)

func TestMonitorHasXrayProtocol(t *testing.T) {
	if !(&MonitorMenu{config: &config.Config{Protocols: []string{"vless_reality_vision"}}}).hasXrayProtocol() {
		t.Fatal("expected Reality/Xray protocol to enable Xray stats")
	}
	if (&MonitorMenu{config: &config.Config{Protocols: []string{"anytls"}}}).hasXrayProtocol() {
		t.Fatal("did not expect sing-box-only protocol to enable Xray stats")
	}
}

func TestProxyProcessFromSSLine(t *testing.T) {
	xray := `ESTAB 0 0 127.0.0.1:31305 203.0.113.1:50000 users:(("xray",pid=123,fd=8))`
	if got := proxyProcessFromSSLine(xray); got != "xray" {
		t.Fatalf("expected xray, got %q", got)
	}
	singbox := `ESTAB 0 0 127.0.0.1:31303 203.0.113.1:50001 users:(("sing-box",pid=456,fd=9))`
	if got := proxyProcessFromSSLine(singbox); got != "sing-box" {
		t.Fatalf("expected sing-box, got %q", got)
	}
	other := `ESTAB 0 0 127.0.0.1:22 203.0.113.1:50002 users:(("sshd",pid=789,fd=3))`
	if got := proxyProcessFromSSLine(other); got != "" {
		t.Fatalf("expected no proxy process, got %q", got)
	}
}
