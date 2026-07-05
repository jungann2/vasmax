package menu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultTLSCertPairPrefersFullchain(t *testing.T) {
	tmp := t.TempDir()
	domain := "node.example.com"
	if err := os.WriteFile(filepath.Join(tmp, domain+".fullchain.crt"), []byte("cert"), 0644); err != nil {
		t.Fatal(err)
	}
	cert, key := defaultTLSCertPairInDir(tmp, domain)
	if cert != filepath.Join(tmp, domain+".fullchain.crt") {
		t.Fatalf("cert = %q, want fullchain", cert)
	}
	if key != filepath.Join(tmp, domain+".key") {
		t.Fatalf("key = %q", key)
	}
}

func TestDefaultTLSCertPairFallsBackToLeafCert(t *testing.T) {
	tmp := t.TempDir()
	domain := "node.example.com"
	cert, key := defaultTLSCertPairInDir(tmp, domain)
	if cert != filepath.Join(tmp, domain+".crt") {
		t.Fatalf("cert = %q, want leaf cert fallback", cert)
	}
	if key != filepath.Join(tmp, domain+".key") {
		t.Fatalf("key = %q", key)
	}
}
