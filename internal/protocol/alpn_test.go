package protocol

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"vasmax/internal/api"
)

func TestALPNConfigUsedByDirectTLSProtocols(t *testing.T) {
	params := &InboundParams{
		Port:     443,
		Domain:   "example.com",
		CertFile: "/tmp/cert.pem",
		KeyFile:  "/tmp/key.pem",
		Tag:      "vless_tcp_tls_vision",
		Users: []*api.User{{
			ID:   1,
			UUID: "11111111-1111-1111-1111-111111111111",
		}},
		ALPN: []string{"h2"},
	}

	raw, err := (&VlessTCPTLSVision{}).GenerateInbound(params)
	if err != nil {
		t.Fatal(err)
	}
	var inbound map[string]interface{}
	if err := json.Unmarshal(raw, &inbound); err != nil {
		t.Fatal(err)
	}
	stream := inbound["streamSettings"].(map[string]interface{})
	tlsSettings := stream["tlsSettings"].(map[string]interface{})
	alpn := tlsSettings["alpn"].([]interface{})
	if len(alpn) != 1 || alpn[0] != "h2" {
		t.Fatalf("expected configured ALPN in inbound, got %#v", alpn)
	}

	info := &ServerInfo{
		Host:   "example.com",
		Domain: "example.com",
		Port:   443,
		ALPN:   []string{"h2"},
	}
	uri := (&VlessTCPTLSVision{}).GenerateURI(params.Users[0], info)
	if !strings.Contains(uri, "alpn=h2") {
		t.Fatalf("expected configured ALPN in URI, got %s", uri)
	}
}

func TestALPNConfigUsedByAnyTLS(t *testing.T) {
	params := &InboundParams{
		Port:     4433,
		Domain:   "example.com",
		CertFile: "/tmp/cert.pem",
		KeyFile:  "/tmp/key.pem",
		Tag:      "anytls",
		Users: []*api.User{{
			ID:   1,
			UUID: "11111111-1111-1111-1111-111111111111",
		}},
		ALPN: []string{"http/1.1"},
	}
	raw, err := (&AnyTLS{}).GenerateInbound(params)
	if err != nil {
		t.Fatal(err)
	}
	var inbound map[string]interface{}
	if err := json.Unmarshal(raw, &inbound); err != nil {
		t.Fatal(err)
	}
	tlsSettings := inbound["tls"].(map[string]interface{})
	alpn := tlsSettings["alpn"].([]interface{})
	if len(alpn) != 1 || alpn[0] != "http/1.1" {
		t.Fatalf("expected configured ALPN in AnyTLS inbound, got %#v", alpn)
	}

	info := &ServerInfo{Host: "example.com", Domain: "example.com", Port: 4433, ALPN: []string{"http/1.1"}}
	outbound := (&AnyTLS{}).GenerateSingBoxOutbound(params.Users[0], info)
	tls := outbound["tls"].(map[string]interface{})
	outALPN := tls["alpn"].([]string)
	if len(outALPN) != 1 || outALPN[0] != "http/1.1" {
		t.Fatalf("expected configured ALPN in AnyTLS outbound, got %#v", outALPN)
	}
}

func TestTLSALPNFiltersH3ForDirectTLSProtocols(t *testing.T) {
	user := &api.User{
		ID:   1,
		UUID: "11111111-1111-1111-1111-111111111111",
	}
	params := &InboundParams{
		Port:     443,
		Domain:   "example.com",
		CertFile: "/tmp/cert.pem",
		KeyFile:  "/tmp/key.pem",
		Tag:      "vless_tcp_tls_vision",
		Users:    []*api.User{user},
		ALPN:     []string{"h3"},
	}

	raw, err := (&VlessTCPTLSVision{}).GenerateInbound(params)
	if err != nil {
		t.Fatal(err)
	}
	var inbound map[string]interface{}
	if err := json.Unmarshal(raw, &inbound); err != nil {
		t.Fatal(err)
	}
	stream := inbound["streamSettings"].(map[string]interface{})
	tlsSettings := stream["tlsSettings"].(map[string]interface{})
	inboundALPN := tlsSettings["alpn"].([]interface{})
	if got := alpnInterfaces(inboundALPN); strings.Join(got, ",") != "h2,http/1.1" {
		t.Fatalf("expected h3-only direct TLS inbound to fall back to h2,http/1.1, got %#v", got)
	}

	info := &ServerInfo{
		Host:   "example.com",
		Domain: "example.com",
		Port:   443,
		ALPN:   []string{"h2", "h3", "http/1.1"},
	}
	uri := (&VlessTCPTLSVision{}).GenerateURI(user, info)
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Query().Get("alpn"); got != "h2,http/1.1" {
		t.Fatalf("expected URI ALPN to exclude h3, got %q from %s", got, uri)
	}

	outbound := (&AnyTLS{}).GenerateSingBoxOutbound(user, &ServerInfo{
		Host:   "example.com",
		Domain: "example.com",
		Port:   4433,
		ALPN:   []string{"h3"},
	})
	tls := outbound["tls"].(map[string]interface{})
	outALPN := tls["alpn"].([]string)
	if strings.Join(outALPN, ",") != "h2,http/1.1" {
		t.Fatalf("expected h3-only AnyTLS outbound to fall back to h2,http/1.1, got %#v", outALPN)
	}
}

func alpnInterfaces(values []interface{}) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if s, ok := value.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
