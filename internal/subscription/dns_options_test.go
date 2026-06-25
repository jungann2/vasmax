package subscription

import (
	"fmt"
	"strings"
	"testing"

	"vasmax/internal/config"
)

func TestDNSOptionsAutoUsesDomesticAndGlobal(t *testing.T) {
	opts := DNSOptionsFromConfig(config.SubscriptionConfig{DNSMode: "auto"})

	nameserver, fallback := opts.clashServers()
	if !containsString(nameserver, "https://dns.alidns.com/dns-query") {
		t.Fatalf("expected domestic DNS in nameserver, got %#v", nameserver)
	}
	if !containsString(fallback, "https://cloudflare-dns.com/dns-query") {
		t.Fatalf("expected global DNS in fallback, got %#v", fallback)
	}

	sb := opts.singBoxDNS()
	if sb["final"] != "global-1" {
		t.Fatalf("expected global final DNS, got %#v", sb["final"])
	}
	if !strings.Contains(toJSONLikeString(sb), "local-1") {
		t.Fatalf("expected local DNS rule in sing-box config: %#v", sb)
	}
}

func TestDNSOptionsCustomDeduplicatesServers(t *testing.T) {
	opts := DNSOptionsFromConfig(config.SubscriptionConfig{
		DNSMode:   "custom",
		DNSCustom: []string{" https://dns.example/dns-query ", "https://dns.example/dns-query", "https://backup.example/dns-query"},
	})

	nameserver, fallback := opts.clashServers()
	if len(fallback) != 0 {
		t.Fatalf("expected no fallback for custom mode, got %#v", fallback)
	}
	if len(nameserver) != 2 {
		t.Fatalf("expected deduplicated custom DNS list, got %#v", nameserver)
	}
	if nameserver[0] != "https://dns.example/dns-query" {
		t.Fatalf("expected trimmed custom DNS, got %#v", nameserver)
	}

	sb := opts.singBoxDNS()
	if sb["final"] != "custom-1" {
		t.Fatalf("expected custom final DNS, got %#v", sb["final"])
	}
}

func TestProfileOptionsUsesConfiguredTestURL(t *testing.T) {
	opts := ProfileOptionsFromConfig(config.SubscriptionConfig{
		DNSMode: "auto",
		TestURL: " https://cp.cloudflare.com/generate_204 ",
	})
	if opts.TestURL != "https://cp.cloudflare.com/generate_204" {
		t.Fatalf("expected trimmed test URL, got %q", opts.TestURL)
	}

	clashData, err := GenerateClashFullProfileWithOptions(nil, "node.example.com", opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(clashData), "https://cp.cloudflare.com/generate_204") {
		t.Fatalf("expected custom test URL in clash profile:\n%s", string(clashData))
	}

	singBoxData, err := GenerateSingBoxFullProfileWithOptions(nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(singBoxData), "https://cp.cloudflare.com/generate_204") {
		t.Fatalf("expected custom test URL in sing-box profile:\n%s", string(singBoxData))
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func toJSONLikeString(v interface{}) string {
	return strings.ReplaceAll(fmt.Sprintf("%#v", v), "\n", " ")
}
