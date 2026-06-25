package protocol

import (
	"os"
	"path/filepath"
	"testing"
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
