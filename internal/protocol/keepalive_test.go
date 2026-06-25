package protocol

import (
	"encoding/json"
	"testing"

	"vasmax/internal/config"
)

func TestXrayInboundIncludesTCPKeepAliveSockopt(t *testing.T) {
	p := &VlessWSTLS{}
	raw, err := p.GenerateInbound(&InboundParams{
		Port: 31297,
		Path: "/vlessws",
		KeepAlive: config.ConnectionConfig{
			KeepAliveMode:             "auto",
			KeepAliveIdleSeconds:      8,
			KeepAliveIntervalSeconds:  8,
			WebSocketHeartbeatSeconds: 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var inbound map[string]interface{}
	if err := json.Unmarshal(raw, &inbound); err != nil {
		t.Fatal(err)
	}
	stream := inbound["streamSettings"].(map[string]interface{})
	sockopt := stream["sockopt"].(map[string]interface{})
	if sockopt["tcpKeepAliveIdle"].(float64) != 8 {
		t.Fatalf("expected tcpKeepAliveIdle 8, got %#v", sockopt["tcpKeepAliveIdle"])
	}
	if sockopt["tcpKeepAliveInterval"].(float64) != 8 {
		t.Fatalf("expected tcpKeepAliveInterval 8, got %#v", sockopt["tcpKeepAliveInterval"])
	}
	wsSettings := stream["wsSettings"].(map[string]interface{})
	if wsSettings["heartbeatPeriod"].(float64) != 8 {
		t.Fatalf("expected websocket heartbeatPeriod 8, got %#v", wsSettings["heartbeatPeriod"])
	}
}

func TestSingBoxTCPInboundIncludesKeepAlive(t *testing.T) {
	p := &AnyTLS{}
	raw, err := p.GenerateInbound(&InboundParams{
		Port: 31303,
		KeepAlive: config.ConnectionConfig{
			KeepAliveMode:            "auto",
			KeepAliveIdleSeconds:     8,
			KeepAliveIntervalSeconds: 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var inbound map[string]interface{}
	if err := json.Unmarshal(raw, &inbound); err != nil {
		t.Fatal(err)
	}
	if inbound["tcp_keep_alive"] != "8s" {
		t.Fatalf("expected tcp_keep_alive 8s, got %#v", inbound["tcp_keep_alive"])
	}
	if inbound["tcp_keep_alive_interval"] != "8s" {
		t.Fatalf("expected tcp_keep_alive_interval 8s, got %#v", inbound["tcp_keep_alive_interval"])
	}
}

func TestKeepAliveOffOmitsGeneratedFields(t *testing.T) {
	xrayRaw, err := (&VlessWSTLS{}).GenerateInbound(&InboundParams{
		Port:      31297,
		Path:      "/vlessws",
		KeepAlive: config.ConnectionConfig{KeepAliveMode: "off"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var xrayInbound map[string]interface{}
	if err := json.Unmarshal(xrayRaw, &xrayInbound); err != nil {
		t.Fatal(err)
	}
	stream := xrayInbound["streamSettings"].(map[string]interface{})
	if _, ok := stream["sockopt"]; ok {
		t.Fatalf("expected sockopt omitted when keepalive is off: %#v", stream)
	}
	wsSettings := stream["wsSettings"].(map[string]interface{})
	if _, ok := wsSettings["heartbeatPeriod"]; ok {
		t.Fatalf("expected websocket heartbeatPeriod omitted when keepalive is off: %#v", wsSettings)
	}

	sbRaw, err := (&AnyTLS{}).GenerateInbound(&InboundParams{
		Port:      31303,
		KeepAlive: config.ConnectionConfig{KeepAliveMode: "off"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var sbInbound map[string]interface{}
	if err := json.Unmarshal(sbRaw, &sbInbound); err != nil {
		t.Fatal(err)
	}
	if _, ok := sbInbound["tcp_keep_alive"]; ok {
		t.Fatalf("expected tcp_keep_alive omitted when keepalive is off: %#v", sbInbound)
	}
}
