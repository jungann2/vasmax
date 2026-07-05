package protocol

import (
	"testing"

	"vasmax/internal/api"
	"vasmax/internal/config"
)

func TestSingBoxTLSProfilesMarkSelfSignedWhenDomainIsIP(t *testing.T) {
	user := &api.User{ID: 1, UUID: "11111111-1111-1111-1111-111111111111"}
	info := &ServerInfo{Host: "203.0.113.10", Domain: "203.0.113.10", Port: 443}

	protocols := []Protocol{
		&AnyTLS{},
		&Hysteria2{},
		&Naive{},
		&Tuic{},
		&VlessTCPTLSVision{},
		&VlessWSTLS{},
		&VlessGRPCTLS{},
		&VMessWSTLS{},
		&VMessHTTPUpgradeTLS{},
		&TrojanTCPTLS{},
		&TrojanGRPCTLS{},
	}

	for _, p := range protocols {
		proxy := p.GenerateClashProxy(user, info)
		if proxy["skip-cert-verify"] != true {
			t.Fatalf("%s clash proxy missing skip-cert-verify for IP/self-signed mode: %#v", p.Name(), proxy)
		}

		outbound := p.GenerateSingBoxOutbound(user, info)
		tlsMap, ok := outbound["tls"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s sing-box outbound missing tls map: %#v", p.Name(), outbound)
		}
		if tlsMap["insecure"] != true {
			t.Fatalf("%s sing-box outbound missing tls.insecure for IP/self-signed mode: %#v", p.Name(), outbound)
		}
	}
}

func TestSingBoxTLSProfilesKeepVerificationForDomain(t *testing.T) {
	user := &api.User{ID: 1, UUID: "11111111-1111-1111-1111-111111111111"}
	info := &ServerInfo{Host: "node.example.com", Domain: "node.example.com", Port: 443}

	protocols := []Protocol{
		&AnyTLS{},
		&Hysteria2{},
		&Naive{},
		&Tuic{},
		&VlessTCPTLSVision{},
		&VlessWSTLS{},
		&VlessGRPCTLS{},
		&VMessWSTLS{},
		&VMessHTTPUpgradeTLS{},
		&TrojanTCPTLS{},
		&TrojanGRPCTLS{},
	}

	for _, p := range protocols {
		proxy := p.GenerateClashProxy(user, info)
		if _, ok := proxy["skip-cert-verify"]; ok {
			t.Fatalf("%s clash proxy should not skip cert verification for domain mode: %#v", p.Name(), proxy)
		}

		outbound := p.GenerateSingBoxOutbound(user, info)
		tlsMap, ok := outbound["tls"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s sing-box outbound missing tls map: %#v", p.Name(), outbound)
		}
		if _, ok := tlsMap["insecure"]; ok {
			t.Fatalf("%s sing-box outbound should not set tls.insecure for domain mode: %#v", p.Name(), outbound)
		}
	}
}

func TestClashProxiesEnableUDPForTUNClients(t *testing.T) {
	user := &api.User{ID: 1, UUID: "11111111-1111-1111-1111-111111111111"}
	info := &ServerInfo{
		Host:        "node.example.com",
		Domain:      "node.example.com",
		Port:        443,
		Path:        "/vlessws",
		ServiceName: "vlessgrpc",
		Reality: &config.RealityConfig{
			ServerName: "www.microsoft.com",
			PublicKey:  "public-key",
			ShortID:    "abcdef12",
		},
	}

	protocols := []Protocol{
		&AnyTLS{},
		&Hysteria2{},
		&Tuic{},
		&Socks5{},
		&VlessTCPTLSVision{},
		&VlessWSTLS{},
		&VlessGRPCTLS{},
		&VlessRealityVision{},
		&VlessRealityGRPC{},
		&VlessRealityXHTTP{},
		&VMessWSTLS{},
		&VMessHTTPUpgradeTLS{},
		&TrojanTCPTLS{},
		&TrojanGRPCTLS{},
	}

	for _, p := range protocols {
		proxy := p.GenerateClashProxy(user, info)
		if proxy["udp"] != true {
			t.Fatalf("%s clash proxy should enable udp for TUN clients: %#v", p.Name(), proxy)
		}
	}
}

func TestRealityClashProxiesSetPostQuantumCompatFlag(t *testing.T) {
	user := &api.User{ID: 1, UUID: "11111111-1111-1111-1111-111111111111"}
	info := &ServerInfo{
		Host:   "203.0.113.10",
		Domain: "203.0.113.10",
		Port:   31305,
		Reality: &config.RealityConfig{
			ServerName: "www.microsoft.com",
			PublicKey:  "public-key",
			ShortID:    "abcdef12",
		},
	}

	protocols := []Protocol{
		&VlessRealityVision{},
		&VlessRealityGRPC{},
		&VlessRealityXHTTP{},
	}

	for _, p := range protocols {
		proxy := p.GenerateClashProxy(user, info)
		opts, ok := proxy["reality-opts"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s reality proxy should include reality-opts: %#v", p.Name(), proxy)
		}
		if opts["support-x25519mlkem768"] != false {
			t.Fatalf("%s reality proxy should disable x25519mlkem768 by default: %#v", p.Name(), proxy)
		}
	}
}
