package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"

	"vasmax/internal/api"
)

// --- Trojan+TCP+TLS ---

// TrojanTCPTLS Trojan+TCP+TLS 协议
type TrojanTCPTLS struct{}

func (t *TrojanTCPTLS) Name() string          { return "trojan_tcp_tls" }
func (t *TrojanTCPTLS) CoreType() string      { return "xray" }
func (t *TrojanTCPTLS) DefaultPort() int      { return 443 }
func (t *TrojanTCPTLS) TransportType() string { return "tcp" }
func (t *TrojanTCPTLS) IsCDNCompatible() bool { return false }

func (t *TrojanTCPTLS) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "trojan",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients": buildTrojanClients(params.Users),
		},
		"streamSettings": map[string]interface{}{
			"network":  "tcp",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"certificates": []map[string]interface{}{
					{"certificateFile": params.CertFile, "keyFile": params.KeyFile},
				},
				"alpn": []string{"h2", "http/1.1"},
			},
		},
		"sniffing": map[string]interface{}{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
	}
	return json.Marshal(inbound)
}

func (t *TrojanTCPTLS) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"password": user.UUID,
		"email":    fmt.Sprintf("user_%d", user.ID),
	}
	return json.Marshal(entry)
}

func (t *TrojanTCPTLS) GenerateURI(user *api.User, info *ServerInfo) string {
	params := url.Values{}
	params.Set("type", "tcp")
	params.Set("security", "tls")
	params.Set("sni", info.Domain)
	params.Set("alpn", "h2,http/1.1")
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", user.UUID, info.Host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-trojan", info.Domain)))
}

func (t *TrojanTCPTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-trojan", info.Domain),
		"type":               "trojan",
		"server":             info.Host,
		"port":               info.Port,
		"password":           user.UUID,
		"sni":                info.Domain,
		"alpn":               []string{"h2", "http/1.1"},
		"client-fingerprint": "chrome",
	}
}

func (t *TrojanTCPTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"type":        "trojan",
		"tag":         fmt.Sprintf("%s-trojan", info.Domain),
		"server":      info.Host,
		"server_port": info.Port,
		"password":    user.UUID,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
			"alpn":        []string{"h2", "http/1.1"},
		},
	}
}

// --- Trojan+gRPC+TLS ---

// TrojanGRPCTLS Trojan+gRPC+TLS 协议
type TrojanGRPCTLS struct{}

func (t *TrojanGRPCTLS) Name() string          { return "trojan_grpc_tls" }
func (t *TrojanGRPCTLS) CoreType() string      { return "xray" }
func (t *TrojanGRPCTLS) DefaultPort() int      { return 443 }
func (t *TrojanGRPCTLS) TransportType() string { return "grpc" }
func (t *TrojanGRPCTLS) IsCDNCompatible() bool { return true }

func (t *TrojanGRPCTLS) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "trojan",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients": buildTrojanClients(params.Users),
		},
		"streamSettings": map[string]interface{}{
			"network":  "grpc",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"certificates": []map[string]interface{}{
					{"certificateFile": params.CertFile, "keyFile": params.KeyFile},
				},
				"alpn": []string{"h2"},
			},
			"grpcSettings": map[string]interface{}{
				"serviceName": params.ServiceName,
			},
		},
		"sniffing": map[string]interface{}{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
	}
	return json.Marshal(inbound)
}

func (t *TrojanGRPCTLS) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"password": user.UUID,
		"email":    fmt.Sprintf("user_%d", user.ID),
	}
	return json.Marshal(entry)
}

func (t *TrojanGRPCTLS) GenerateURI(user *api.User, info *ServerInfo) string {
	host := effectiveHost(info)
	params := url.Values{}
	params.Set("type", "grpc")
	params.Set("security", "tls")
	params.Set("sni", info.Domain)
	params.Set("serviceName", info.ServiceName)
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", user.UUID, host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-trojan-grpc", info.Domain)))
}

func (t *TrojanGRPCTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-trojan-grpc", info.Domain),
		"type":               "trojan",
		"server":             host,
		"port":               info.Port,
		"password":           user.UUID,
		"sni":                info.Domain,
		"network":            "grpc",
		"client-fingerprint": "chrome",
		"grpc-opts": map[string]interface{}{
			"grpc-service-name": info.ServiceName,
		},
	}
}

func (t *TrojanGRPCTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"type":        "trojan",
		"tag":         fmt.Sprintf("%s-trojan-grpc", info.Domain),
		"server":      host,
		"server_port": info.Port,
		"password":    user.UUID,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
		},
		"transport": map[string]interface{}{
			"type":         "grpc",
			"service_name": info.ServiceName,
		},
	}
}

// --- 辅助函数 ---

// buildTrojanClients 构建 Trojan 用户列表
func buildTrojanClients(users []*api.User) []map[string]interface{} {
	clients := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		clients = append(clients, map[string]interface{}{
			"password": u.UUID,
			"email":    fmt.Sprintf("user_%d", u.ID),
		})
	}
	return clients
}
