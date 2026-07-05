package protocol

import (
	"encoding/json"
	"testing"

	"vasmax/internal/api"
	"vasmax/internal/config"
)

func TestGenerateInboundMessagesExpandsRealityVisionTargets(t *testing.T) {
	reality := &config.RealityConfig{
		PrivateKey: "private-key",
		PublicKey:  "public-key",
		ShortID:    "abcd1234",
		Targets: []config.RealityTarget{
			{Name: "apple", ServerName: "www.apple.com", Dest: "www.apple.com:443", Port: 30001},
			{Name: "bing", ServerName: "www.bing.com", Dest: "www.bing.com:443", Port: 30002},
		},
	}
	params := &InboundParams{
		Port:    30001,
		Tag:     "vless_reality_vision",
		Reality: reality,
		Users: []*api.User{{
			ID:   1,
			UUID: "11111111-1111-1111-1111-111111111111",
		}},
	}

	raws, err := GenerateInboundMessages(&VlessRealityVision{}, params)
	if err != nil {
		t.Fatalf("GenerateInboundMessages returned error: %v", err)
	}
	if len(raws) != 2 {
		t.Fatalf("inbound count = %d, want 2", len(raws))
	}

	first := decodeInbound(t, raws[0])
	second := decodeInbound(t, raws[1])
	if first["tag"] != "vless_reality_vision_apple" {
		t.Fatalf("first tag = %v", first["tag"])
	}
	if second["tag"] != "vless_reality_vision_bing" {
		t.Fatalf("second tag = %v", second["tag"])
	}
	if first["port"].(float64) != 30001 || second["port"].(float64) != 30002 {
		t.Fatalf("ports = %v, %v", first["port"], second["port"])
	}
	if realityDest(first) != "www.apple.com:443" || realityDest(second) != "www.bing.com:443" {
		t.Fatalf("unexpected reality dests: %s / %s", realityDest(first), realityDest(second))
	}
}

func decodeInbound(t *testing.T, raw json.RawMessage) map[string]interface{} {
	t.Helper()
	var inbound map[string]interface{}
	if err := json.Unmarshal(raw, &inbound); err != nil {
		t.Fatalf("failed to unmarshal inbound: %v", err)
	}
	return inbound
}

func realityDest(inbound map[string]interface{}) string {
	stream := inbound["streamSettings"].(map[string]interface{})
	reality := stream["realitySettings"].(map[string]interface{})
	return reality["dest"].(string)
}
