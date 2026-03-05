package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"vasmax/internal/api"
)

// --- VMess+WS+TLS ---

// VMessWSTLS VMess+WebSocket+TLS 协议
type VMessWSTLS struct{}

func (v *VMessWSTLS) Name() string          { return "vmess_ws_tls" }
func (v *VMessWSTLS) CoreType() string      { return "xray" }
func (v *VMessWSTLS) DefaultPort() int      { return 443 }
func (v *VMessWSTLS) TransportType() string { return "ws" }
func (v *VMessWSTLS) IsCDNCompatible() bool { return true }

func (v *VMessWSTLS) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vmess",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients": buildVMessClients(params.Users),
		},
		"streamSettings": map[string]interface{}{
			"network":  "ws",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"certificates": []map[string]interface{}{
					{"certificateFile": params.CertFile, "keyFile": params.KeyFile},
				},
				"alpn": []string{"h2", "http/1.1"},
			},
			"wsSettings": map[string]interface{}{
				"path": params.Path,
			},
		},
		"sniffing": map[string]interface{}{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
	}
	return json.Marshal(inbound)
}

func (v *VMessWSTLS) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":       user.UUID,
		"email":    fmt.Sprintf("user_%d", user.ID),
		"security": "auto",
	}
	return json.Marshal(entry)
}

func (v *VMessWSTLS) GenerateURI(user *api.User, info *ServerInfo) string {
	host := effectiveHost(info)
	vmessJSON := map[string]interface{}{
		"v":    "2",
		"ps":   fmt.Sprintf("%s-vmess-ws", info.Domain),
		"add":  host,
		"port": info.Port,
		"id":   user.UUID,
		"aid":  0,
		"scy":  "auto",
		"net":  "ws",
		"type": "none",
		"host": info.Domain,
		"path": info.Path,
		"tls":  "tls",
		"sni":  info.Domain,
	}
	data, _ := json.Marshal(vmessJSON)
	return "vmess://" + base64.StdEncoding.EncodeToString(data)
}

func (v *VMessWSTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-vmess-ws", info.Domain),
		"type":               "vmess",
		"server":             host,
		"port":               info.Port,
		"uuid":               user.UUID,
		"alterId":            0,
		"cipher":             "auto",
		"tls":                true,
		"servername":         info.Domain,
		"network":            "ws",
		"client-fingerprint": "chrome",
		"ws-opts": map[string]interface{}{
			"path": info.Path,
			"headers": map[string]interface{}{
				"Host": info.Domain,
			},
		},
	}
}

func (v *VMessWSTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"type":        "vmess",
		"tag":         fmt.Sprintf("%s-vmess-ws", info.Domain),
		"server":      host,
		"server_port": info.Port,
		"uuid":        user.UUID,
		"security":    "auto",
		"alter_id":    0,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
		},
		"transport": map[string]interface{}{
			"type": "ws",
			"path": info.Path,
			"headers": map[string]interface{}{
				"Host": info.Domain,
			},
		},
	}
}

// --- VMess+HTTPUpgrade+TLS ---

// VMessHTTPUpgradeTLS VMess+HTTPUpgrade+TLS 协议
type VMessHTTPUpgradeTLS struct{}

func (v *VMessHTTPUpgradeTLS) Name() string          { return "vmess_httpupgrade_tls" }
func (v *VMessHTTPUpgradeTLS) CoreType() string      { return "xray" }
func (v *VMessHTTPUpgradeTLS) DefaultPort() int      { return 443 }
func (v *VMessHTTPUpgradeTLS) TransportType() string { return "httpupgrade" }
func (v *VMessHTTPUpgradeTLS) IsCDNCompatible() bool { return true }

func (v *VMessHTTPUpgradeTLS) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vmess",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients": buildVMessClients(params.Users),
		},
		"streamSettings": map[string]interface{}{
			"network":  "httpupgrade",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"certificates": []map[string]interface{}{
					{"certificateFile": params.CertFile, "keyFile": params.KeyFile},
				},
				"alpn": []string{"h2", "http/1.1"},
			},
			"httpupgradeSettings": map[string]interface{}{
				"path": params.Path,
				"host": params.Domain,
			},
		},
		"sniffing": map[string]interface{}{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
	}
	return json.Marshal(inbound)
}

func (v *VMessHTTPUpgradeTLS) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":       user.UUID,
		"email":    fmt.Sprintf("user_%d", user.ID),
		"security": "auto",
	}
	return json.Marshal(entry)
}

func (v *VMessHTTPUpgradeTLS) GenerateURI(user *api.User, info *ServerInfo) string {
	host := effectiveHost(info)
	vmessJSON := map[string]interface{}{
		"v":    "2",
		"ps":   fmt.Sprintf("%s-vmess-httpupgrade", info.Domain),
		"add":  host,
		"port": info.Port,
		"id":   user.UUID,
		"aid":  0,
		"scy":  "auto",
		"net":  "httpupgrade",
		"type": "none",
		"host": info.Domain,
		"path": info.Path,
		"tls":  "tls",
		"sni":  info.Domain,
	}
	data, _ := json.Marshal(vmessJSON)
	return "vmess://" + base64.StdEncoding.EncodeToString(data)
}

func (v *VMessHTTPUpgradeTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-vmess-httpupgrade", info.Domain),
		"type":               "vmess",
		"server":             host,
		"port":               info.Port,
		"uuid":               user.UUID,
		"alterId":            0,
		"cipher":             "auto",
		"tls":                true,
		"servername":         info.Domain,
		"network":            "httpupgrade",
		"client-fingerprint": "chrome",
		"httpupgrade-opts": map[string]interface{}{
			"path": info.Path,
			"host": info.Domain,
		},
	}
}

func (v *VMessHTTPUpgradeTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"type":        "vmess",
		"tag":         fmt.Sprintf("%s-vmess-httpupgrade", info.Domain),
		"server":      host,
		"server_port": info.Port,
		"uuid":        user.UUID,
		"security":    "auto",
		"alter_id":    0,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
		},
		"transport": map[string]interface{}{
			"type": "httpupgrade",
			"path": info.Path,
			"host": info.Domain,
		},
	}
}

// --- 辅助函数 ---

// buildVMessClients 构建 VMess 用户列表
func buildVMessClients(users []*api.User) []map[string]interface{} {
	clients := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		clients = append(clients, map[string]interface{}{
			"id":       u.UUID,
			"email":    fmt.Sprintf("user_%d", u.ID),
			"security": "auto",
		})
	}
	return clients
}
