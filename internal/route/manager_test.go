package route

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSingboxRuleMapsXrayBlockedOutboundToSingboxBlock(t *testing.T) {
	rule := &RouteRule{
		Type:     "domain_blacklist",
		Domains:  []string{"example.com"},
		Outbound: "blocked",
	}

	got := buildSingboxRule(rule)
	if got["outbound"] != "block" {
		t.Fatalf("sing-box blocked outbound = %v, want block", got["outbound"])
	}
}

func TestBuildSingboxRuleKeepsNonBlockedOutbound(t *testing.T) {
	rule := &RouteRule{
		Type:     "warp_ipv4",
		IPs:      []string{"1.1.1.1/32"},
		Outbound: "warp",
	}

	got := buildSingboxRule(rule)
	if got["outbound"] != "warp" {
		t.Fatalf("sing-box outbound = %v, want warp", got["outbound"])
	}
}

func TestAddRuleRollsBackXrayWhenSingboxWriteFails(t *testing.T) {
	tmp := t.TempDir()
	xrayDir := filepath.Join(tmp, "xray")
	if err := os.MkdirAll(xrayDir, 0755); err != nil {
		t.Fatal(err)
	}
	xrayPath := filepath.Join(xrayDir, "04_routing.json")
	customPath := filepath.Join(tmp, "custom_routes.json")
	initialXray := []byte(`{"routing":{"domainStrategy":"AsIs","rules":[]}}`)
	initialCustom := []byte(`[]`)
	if err := os.WriteFile(xrayPath, initialXray, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customPath, initialCustom, 0644); err != nil {
		t.Fatal(err)
	}
	notDir := filepath.Join(tmp, "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(xrayDir, notDir, nil)
	err := mgr.AddRule(&RouteRule{
		Type:     "domain_blacklist",
		Domains:  []string{"example.com"},
		Outbound: "blocked",
	})
	if err == nil {
		t.Fatal("expected sing-box write error")
	}
	assertFileBytes(t, xrayPath, initialXray)
	assertFileBytes(t, customPath, initialCustom)
}

func TestBlacklistAddRollsBackPersistentListWhenRouteApplyFails(t *testing.T) {
	tmp := t.TempDir()
	xrayDir := filepath.Join(tmp, "xray")
	if err := os.MkdirAll(xrayDir, 0755); err != nil {
		t.Fatal(err)
	}
	notDir := filepath.Join(tmp, "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(xrayDir, notDir, nil)
	bl := NewBlacklistManager(mgr)
	if err := bl.Add("example.com"); err == nil {
		t.Fatal("expected sing-box write error")
	}
	if _, err := os.Stat(bl.blacklistFile); !os.IsNotExist(err) {
		t.Fatalf("blacklist file should be rolled back, stat err=%v", err)
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s = %s, want %s", path, got, want)
	}
}
