package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vasmax/internal/config"
)

func TestGenerateBaseDNSConfigRemovesLegacyDNSFile(t *testing.T) {
	dir := t.TempDir()
	dnsPath := filepath.Join(dir, "03_dns.json")
	if err := os.WriteFile(dnsPath, []byte(`{"dns":{"servers":["8.8.8.8"]}}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateBaseDNSConfig(dir); err != nil {
		t.Fatalf("GenerateBaseDNSConfig failed: %v", err)
	}
	if _, err := os.Stat(dnsPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy dns file removed, stat err=%v", err)
	}
}

func TestGenerateBaseDNSConfigWritesExplicitServerDNS(t *testing.T) {
	dir := t.TempDir()

	dnsCfg := config.ServerDNSConfig{Mode: config.ServerDNSModeCloudflare, Strategy: "ipv4_only"}
	if err := GenerateBaseDNSConfig(dir, dnsCfg); err != nil {
		t.Fatalf("GenerateBaseDNSConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "03_dns.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	dns := got["dns"]
	if dns["queryStrategy"] != "UseIPv4" {
		t.Fatalf("expected UseIPv4 queryStrategy, got %#v", dns["queryStrategy"])
	}
	servers, ok := dns["servers"].([]interface{})
	if !ok || len(servers) != 2 || servers[0] != "1.1.1.1" {
		t.Fatalf("unexpected xray dns servers: %#v", dns["servers"])
	}
}

func TestGenerateBaseOutboundConfigUsesServerDNSStrategy(t *testing.T) {
	dir := t.TempDir()

	dnsCfg := config.ServerDNSConfig{Mode: config.ServerDNSModeQuad9, Strategy: "ipv4_only"}
	if err := GenerateBaseOutboundConfig(dir, dnsCfg); err != nil {
		t.Fatalf("GenerateBaseOutboundConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "02_outbounds.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string][]map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	settings := got["outbounds"][0]["settings"].(map[string]interface{})
	if settings["domainStrategy"] != "UseIPv4" {
		t.Fatalf("expected UseIPv4 domainStrategy, got %#v", settings["domainStrategy"])
	}
}
