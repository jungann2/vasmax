package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vasmax/internal/config"
)

func TestGenerateSingBoxBaseDNSRemovesSystemDNSFile(t *testing.T) {
	dir := t.TempDir()
	dnsPath := filepath.Join(dir, "01_dns.json")
	if err := os.WriteFile(dnsPath, []byte(`{"dns":{"servers":[]}}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateSingBoxBaseDNS(dir, config.ServerDNSConfig{Mode: config.ServerDNSModeSystem}); err != nil {
		t.Fatalf("GenerateSingBoxBaseDNS failed: %v", err)
	}
	if _, err := os.Stat(dnsPath); !os.IsNotExist(err) {
		t.Fatalf("expected system mode dns file removed, stat err=%v", err)
	}
}

func TestGenerateSingBoxBaseDNSWritesExplicitServerDNS(t *testing.T) {
	dir := t.TempDir()

	dnsCfg := config.ServerDNSConfig{Mode: config.ServerDNSModeQuad9, Strategy: "ipv4_only"}
	if err := GenerateSingBoxBaseDNS(dir, dnsCfg); err != nil {
		t.Fatalf("GenerateSingBoxBaseDNS failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "01_dns.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	dns := got["dns"]
	if dns["strategy"] != "ipv4_only" {
		t.Fatalf("expected ipv4_only strategy, got %#v", dns["strategy"])
	}
	if dns["final"] != "server-dns-1" {
		t.Fatalf("expected server-dns-1 final, got %#v", dns["final"])
	}
	servers, ok := dns["servers"].([]interface{})
	if !ok || len(servers) != 2 {
		t.Fatalf("unexpected sing-box dns servers: %#v", dns["servers"])
	}
	first := servers[0].(map[string]interface{})
	if first["type"] != "udp" || first["server"] != "9.9.9.9" {
		t.Fatalf("unexpected first dns server: %#v", first)
	}
}
