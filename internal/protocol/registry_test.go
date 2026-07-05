package protocol

import "testing"

func TestDomainInstallOrderPrefersRealityVision(t *testing.T) {
	protocols := DefaultRegistry().ListAllOrdered()
	if len(protocols) == 0 {
		t.Fatal("expected registered protocols")
	}
	if got := protocols[0].Name(); got != "vless_reality_vision" {
		t.Fatalf("domain install first protocol = %s, want vless_reality_vision", got)
	}
}

func TestNoDomainInstallOrderPrefersRealityVision(t *testing.T) {
	protocols := DefaultRegistry().ListNoDomainOrdered()
	if len(protocols) == 0 {
		t.Fatal("expected no-domain protocols")
	}
	if got := protocols[0].Name(); got != "vless_reality_vision" {
		t.Fatalf("no-domain install first protocol = %s, want vless_reality_vision", got)
	}
}
