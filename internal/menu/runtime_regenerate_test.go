package menu

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vasmax/internal/api"
	"vasmax/internal/config"
	"vasmax/internal/protocol"
	"vasmax/internal/user"
)

func TestWriteInstalledProtocolRuntimeFilesIncludesCurrentUsers(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{
		Protocols:     []string{"socks5"},
		ProtocolPorts: map[string]int{"socks5": 12082},
		Paths: config.PathsConfig{
			SingBoxConf: tmp,
		},
	}
	users := user.NewManager()
	users.UpdateUsers([]api.User{{
		ID:   42,
		UUID: "11111111-1111-4111-8111-111111111111",
	}})

	if err := writeInstalledProtocolRuntimeFiles(cfg, protocol.DefaultRegistry(), users); err != nil {
		t.Fatalf("writeInstalledProtocolRuntimeFiles failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "10_socks5_inbounds.json"))
	if err != nil {
		t.Fatalf("read generated inbound: %v", err)
	}
	var wrapper struct {
		Inbounds []struct {
			Type       string `json:"type"`
			ListenPort int    `json:"listen_port"`
			Users      []struct {
				Username string `json:"username"`
				Password string `json:"password"`
			} `json:"users"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("unmarshal generated inbound: %v", err)
	}
	if len(wrapper.Inbounds) != 1 {
		t.Fatalf("inbound count = %d, want 1", len(wrapper.Inbounds))
	}
	inbound := wrapper.Inbounds[0]
	if inbound.Type != "socks" || inbound.ListenPort != 12082 {
		t.Fatalf("generated inbound = type %q port %d, want socks/12082", inbound.Type, inbound.ListenPort)
	}
	if len(inbound.Users) != 1 {
		t.Fatalf("user count = %d, want 1", len(inbound.Users))
	}
	if inbound.Users[0].Username != "user_42" || inbound.Users[0].Password != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("generated user = %#v", inbound.Users[0])
	}
}

func TestWriteInstalledProtocolRuntimeFilesRequiresDependencies(t *testing.T) {
	cfg := &config.Config{Protocols: []string{"socks5"}}
	if err := writeInstalledProtocolRuntimeFiles(cfg, nil, user.NewManager()); err == nil {
		t.Fatal("expected missing registry error")
	}
	if err := writeInstalledProtocolRuntimeFiles(cfg, protocol.DefaultRegistry(), nil); err == nil {
		t.Fatal("expected missing user manager error")
	}
}
