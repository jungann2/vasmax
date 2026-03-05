package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"

	"vasmax/internal/api"
)

// --- VLESS+TCP+TLS+Vision ---

// VlessTCPTLSVision VLESS+TCP+TLS+Vision 协议
type VlessTCPTLSVision struct{}

func (v *VlessTCPTLSVision) Name() string          { return "vless_tcp_tls_vision" }
func (v *VlessTCPTLSVision) CoreType() string      { return "xray" }
func (v *VlessTCPTLSVision) DefaultPort() int      { return 443 }
func (v *VlessTCPTLSVision) TransportType() string { return "tcp" }
func (v *VlessTCPTLSVision) IsCDNCompatible() bool { return false }

func (v *VlessTCPTLSVision) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vless",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients":    buildVLESSClients(params.Users, true),
			"decryption": "none",
		},
		"streamSettings": map[string]interface{}{
			"network":  "tcp",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"certificates": []map[string]interface{}{
					{"certificateFile": params.CertFile, "keyFile": params.KeyFile},
				},
				"minVersion": "1.2",
				"alpn":       []string{"h2", "http/1.1"},
			},
		},
		"sniffing": map[string]interface{}{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
	}
	return json.Marshal(inbound)
}

func (v *VlessTCPTLSVision) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":    user.UUID,
		"email": fmt.Sprintf("user_%d", user.ID),
		"flow":  "xtls-rprx-vision",
	}
	return json.Marshal(entry)
}

func (v *VlessTCPTLSVision) GenerateURI(user *api.User, info *ServerInfo) string {
	host := info.Host
	params := url.Values{}
	params.Set("type", "tcp")
	params.Set("security", "tls")
	params.Set("sni", info.Domain)
	params.Set("flow", "xtls-rprx-vision")
	params.Set("alpn", "h2,http/1.1")
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-vless-vision", info.Domain)))
}

func (v *VlessTCPTLSVision) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-vless-vision", info.Domain),
		"type":               "vless",
		"server":             info.Host,
		"port":               info.Port,
		"uuid":               user.UUID,
		"tls":                true,
		"servername":         info.Domain,
		"flow":               "xtls-rprx-vision",
		"client-fingerprint": "chrome",
		"alpn":               []string{"h2", "http/1.1"},
	}
}

func (v *VlessTCPTLSVision) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"type":        "vless",
		"tag":         fmt.Sprintf("%s-vless-vision", info.Domain),
		"server":      info.Host,
		"server_port": info.Port,
		"uuid":        user.UUID,
		"flow":        "xtls-rprx-vision",
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
			"alpn":        []string{"h2", "http/1.1"},
		},
	}
}

// --- VLESS+WS+TLS ---

// VlessWSTLS VLESS+WebSocket+TLS 协议
type VlessWSTLS struct{}

func (v *VlessWSTLS) Name() string          { return "vless_ws_tls" }
func (v *VlessWSTLS) CoreType() string      { return "xray" }
func (v *VlessWSTLS) DefaultPort() int      { return 443 }
func (v *VlessWSTLS) TransportType() string { return "ws" }
func (v *VlessWSTLS) IsCDNCompatible() bool { return true }

func (v *VlessWSTLS) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vless",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients":    buildVLESSClients(params.Users, false),
			"decryption": "none",
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

func (v *VlessWSTLS) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":    user.UUID,
		"email": fmt.Sprintf("user_%d", user.ID),
	}
	return json.Marshal(entry)
}

func (v *VlessWSTLS) GenerateURI(user *api.User, info *ServerInfo) string {
	host := effectiveHost(info)
	params := url.Values{}
	params.Set("type", "ws")
	params.Set("security", "tls")
	params.Set("sni", info.Domain)
	params.Set("host", info.Domain)
	params.Set("path", info.Path)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-vless-ws", info.Domain)))
}

func (v *VlessWSTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-vless-ws", info.Domain),
		"type":               "vless",
		"server":             host,
		"port":               info.Port,
		"uuid":               user.UUID,
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

func (v *VlessWSTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"type":        "vless",
		"tag":         fmt.Sprintf("%s-vless-ws", info.Domain),
		"server":      host,
		"server_port": info.Port,
		"uuid":        user.UUID,
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

// --- VLESS+gRPC+TLS ---

// VlessGRPCTLS VLESS+gRPC+TLS 协议
type VlessGRPCTLS struct{}

func (v *VlessGRPCTLS) Name() string          { return "vless_grpc_tls" }
func (v *VlessGRPCTLS) CoreType() string      { return "xray" }
func (v *VlessGRPCTLS) DefaultPort() int      { return 443 }
func (v *VlessGRPCTLS) TransportType() string { return "grpc" }
func (v *VlessGRPCTLS) IsCDNCompatible() bool { return true }

func (v *VlessGRPCTLS) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vless",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients":    buildVLESSClients(params.Users, false),
			"decryption": "none",
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

func (v *VlessGRPCTLS) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":    user.UUID,
		"email": fmt.Sprintf("user_%d", user.ID),
	}
	return json.Marshal(entry)
}

func (v *VlessGRPCTLS) GenerateURI(user *api.User, info *ServerInfo) string {
	host := effectiveHost(info)
	params := url.Values{}
	params.Set("type", "grpc")
	params.Set("security", "tls")
	params.Set("sni", info.Domain)
	params.Set("serviceName", info.ServiceName)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-vless-grpc", info.Domain)))
}

func (v *VlessGRPCTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-vless-grpc", info.Domain),
		"type":               "vless",
		"server":             host,
		"port":               info.Port,
		"uuid":               user.UUID,
		"tls":                true,
		"servername":         info.Domain,
		"network":            "grpc",
		"client-fingerprint": "chrome",
		"grpc-opts": map[string]interface{}{
			"grpc-service-name": info.ServiceName,
		},
	}
}

func (v *VlessGRPCTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	host := effectiveHost(info)
	return map[string]interface{}{
		"type":        "vless",
		"tag":         fmt.Sprintf("%s-vless-grpc", info.Domain),
		"server":      host,
		"server_port": info.Port,
		"uuid":        user.UUID,
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

// --- VLESS+Reality+Vision ---

// VlessRealityVision VLESS+Reality+Vision 协议
type VlessRealityVision struct{}

func (v *VlessRealityVision) Name() string          { return "vless_reality_vision" }
func (v *VlessRealityVision) CoreType() string      { return "xray" }
func (v *VlessRealityVision) DefaultPort() int      { return 443 }
func (v *VlessRealityVision) TransportType() string { return "tcp" }
func (v *VlessRealityVision) IsCDNCompatible() bool { return false }

func (v *VlessRealityVision) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	reality := params.Reality
	if reality == nil {
		return nil, fmt.Errorf("reality config is required for vless_reality_vision")
	}
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vless",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients":    buildVLESSClients(params.Users, true),
			"decryption": "none",
		},
		"streamSettings": map[string]interface{}{
			"network":  "tcp",
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"show":        false,
				"dest":        reality.Dest,
				"xver":        0,
				"serverNames": []string{reality.ServerName},
				"privateKey":  reality.PrivateKey,
				"shortIds":    []string{reality.ShortID},
			},
		},
		"sniffing": map[string]interface{}{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
	}
	return json.Marshal(inbound)
}

func (v *VlessRealityVision) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":    user.UUID,
		"email": fmt.Sprintf("user_%d", user.ID),
		"flow":  "xtls-rprx-vision",
	}
	return json.Marshal(entry)
}

func (v *VlessRealityVision) GenerateURI(user *api.User, info *ServerInfo) string {
	params := url.Values{}
	params.Set("type", "tcp")
	params.Set("security", "reality")
	params.Set("flow", "xtls-rprx-vision")
	if info.Reality != nil {
		params.Set("sni", info.Reality.ServerName)
		params.Set("pbk", info.Reality.PublicKey)
		params.Set("sid", info.Reality.ShortID)
	}
	params.Set("fp", "chrome")
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, info.Host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-reality-vision", info.Host)))
}

func (v *VlessRealityVision) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	m := map[string]interface{}{
		"name":               fmt.Sprintf("%s-reality-vision", info.Host),
		"type":               "vless",
		"server":             info.Host,
		"port":               info.Port,
		"uuid":               user.UUID,
		"flow":               "xtls-rprx-vision",
		"tls":                true,
		"client-fingerprint": "chrome",
		"network":            "tcp",
	}
	if info.Reality != nil {
		m["servername"] = info.Reality.ServerName
		m["reality-opts"] = map[string]interface{}{
			"public-key": info.Reality.PublicKey,
			"short-id":   info.Reality.ShortID,
		}
	}
	return m
}

func (v *VlessRealityVision) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	tls := map[string]interface{}{
		"enabled": true,
		"reality": map[string]interface{}{
			"enabled": true,
		},
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
	}
	if info.Reality != nil {
		tls["server_name"] = info.Reality.ServerName
		tls["reality"].(map[string]interface{})["public_key"] = info.Reality.PublicKey
		tls["reality"].(map[string]interface{})["short_id"] = info.Reality.ShortID
	}
	return map[string]interface{}{
		"type":        "vless",
		"tag":         fmt.Sprintf("%s-reality-vision", info.Host),
		"server":      info.Host,
		"server_port": info.Port,
		"uuid":        user.UUID,
		"flow":        "xtls-rprx-vision",
		"tls":         tls,
	}
}

// --- VLESS+Reality+gRPC ---

// VlessRealityGRPC VLESS+Reality+gRPC 协议
type VlessRealityGRPC struct{}

func (v *VlessRealityGRPC) Name() string          { return "vless_reality_grpc" }
func (v *VlessRealityGRPC) CoreType() string      { return "xray" }
func (v *VlessRealityGRPC) DefaultPort() int      { return 443 }
func (v *VlessRealityGRPC) TransportType() string { return "grpc" }
func (v *VlessRealityGRPC) IsCDNCompatible() bool { return false }

func (v *VlessRealityGRPC) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	reality := params.Reality
	if reality == nil {
		return nil, fmt.Errorf("reality config is required for vless_reality_grpc")
	}
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vless",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients":    buildVLESSClients(params.Users, false),
			"decryption": "none",
		},
		"streamSettings": map[string]interface{}{
			"network":  "grpc",
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"show":        false,
				"dest":        reality.Dest,
				"xver":        0,
				"serverNames": []string{reality.ServerName},
				"privateKey":  reality.PrivateKey,
				"shortIds":    []string{reality.ShortID},
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

func (v *VlessRealityGRPC) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":    user.UUID,
		"email": fmt.Sprintf("user_%d", user.ID),
	}
	return json.Marshal(entry)
}

func (v *VlessRealityGRPC) GenerateURI(user *api.User, info *ServerInfo) string {
	params := url.Values{}
	params.Set("type", "grpc")
	params.Set("security", "reality")
	params.Set("serviceName", info.ServiceName)
	params.Set("fp", "chrome")
	if info.Reality != nil {
		params.Set("sni", info.Reality.ServerName)
		params.Set("pbk", info.Reality.PublicKey)
		params.Set("sid", info.Reality.ShortID)
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, info.Host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-reality-grpc", info.Host)))
}

func (v *VlessRealityGRPC) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	m := map[string]interface{}{
		"name":               fmt.Sprintf("%s-reality-grpc", info.Host),
		"type":               "vless",
		"server":             info.Host,
		"port":               info.Port,
		"uuid":               user.UUID,
		"tls":                true,
		"client-fingerprint": "chrome",
		"network":            "grpc",
		"grpc-opts": map[string]interface{}{
			"grpc-service-name": info.ServiceName,
		},
	}
	if info.Reality != nil {
		m["servername"] = info.Reality.ServerName
		m["reality-opts"] = map[string]interface{}{
			"public-key": info.Reality.PublicKey,
			"short-id":   info.Reality.ShortID,
		}
	}
	return m
}

func (v *VlessRealityGRPC) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	tls := map[string]interface{}{
		"enabled": true,
		"reality": map[string]interface{}{
			"enabled": true,
		},
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
	}
	if info.Reality != nil {
		tls["server_name"] = info.Reality.ServerName
		tls["reality"].(map[string]interface{})["public_key"] = info.Reality.PublicKey
		tls["reality"].(map[string]interface{})["short_id"] = info.Reality.ShortID
	}
	return map[string]interface{}{
		"type":        "vless",
		"tag":         fmt.Sprintf("%s-reality-grpc", info.Host),
		"server":      info.Host,
		"server_port": info.Port,
		"uuid":        user.UUID,
		"tls":         tls,
		"transport": map[string]interface{}{
			"type":         "grpc",
			"service_name": info.ServiceName,
		},
	}
}

// --- VLESS+Reality+XHTTP ---

// VlessRealityXHTTP VLESS+Reality+XHTTP 协议
type VlessRealityXHTTP struct{}

func (v *VlessRealityXHTTP) Name() string          { return "vless_reality_xhttp" }
func (v *VlessRealityXHTTP) CoreType() string      { return "xray" }
func (v *VlessRealityXHTTP) DefaultPort() int      { return 443 }
func (v *VlessRealityXHTTP) TransportType() string { return "xhttp" }
func (v *VlessRealityXHTTP) IsCDNCompatible() bool { return false }

func (v *VlessRealityXHTTP) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	reality := params.Reality
	if reality == nil {
		return nil, fmt.Errorf("reality config is required for vless_reality_xhttp")
	}
	inbound := map[string]interface{}{
		"port":     params.Port,
		"protocol": "vless",
		"tag":      params.Tag,
		"settings": map[string]interface{}{
			"clients":    buildVLESSClients(params.Users, false),
			"decryption": "none",
		},
		"streamSettings": map[string]interface{}{
			"network":  "xhttp",
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"show":        false,
				"dest":        reality.Dest,
				"xver":        0,
				"serverNames": []string{reality.ServerName},
				"privateKey":  reality.PrivateKey,
				"shortIds":    []string{reality.ShortID},
			},
			"xhttpSettings": map[string]interface{}{
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

func (v *VlessRealityXHTTP) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"id":    user.UUID,
		"email": fmt.Sprintf("user_%d", user.ID),
	}
	return json.Marshal(entry)
}

func (v *VlessRealityXHTTP) GenerateURI(user *api.User, info *ServerInfo) string {
	params := url.Values{}
	params.Set("type", "xhttp")
	params.Set("security", "reality")
	params.Set("path", info.Path)
	params.Set("fp", "chrome")
	if info.Reality != nil {
		params.Set("sni", info.Reality.ServerName)
		params.Set("pbk", info.Reality.PublicKey)
		params.Set("sid", info.Reality.ShortID)
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, info.Host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-reality-xhttp", info.Host)))
}

func (v *VlessRealityXHTTP) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	m := map[string]interface{}{
		"name":               fmt.Sprintf("%s-reality-xhttp", info.Host),
		"type":               "vless",
		"server":             info.Host,
		"port":               info.Port,
		"uuid":               user.UUID,
		"tls":                true,
		"client-fingerprint": "chrome",
		"network":            "xhttp",
		"xhttp-opts": map[string]interface{}{
			"path": info.Path,
		},
	}
	if info.Reality != nil {
		m["servername"] = info.Reality.ServerName
		m["reality-opts"] = map[string]interface{}{
			"public-key": info.Reality.PublicKey,
			"short-id":   info.Reality.ShortID,
		}
	}
	return m
}

func (v *VlessRealityXHTTP) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	tls := map[string]interface{}{
		"enabled": true,
		"reality": map[string]interface{}{
			"enabled": true,
		},
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
	}
	if info.Reality != nil {
		tls["server_name"] = info.Reality.ServerName
		tls["reality"].(map[string]interface{})["public_key"] = info.Reality.PublicKey
		tls["reality"].(map[string]interface{})["short_id"] = info.Reality.ShortID
	}
	return map[string]interface{}{
		"type":        "vless",
		"tag":         fmt.Sprintf("%s-reality-xhttp", info.Host),
		"server":      info.Host,
		"server_port": info.Port,
		"uuid":        user.UUID,
		"tls":         tls,
		"transport": map[string]interface{}{
			"type": "xhttp",
			"path": info.Path,
		},
	}
}

// --- 辅助函数 ---

// buildVLESSClients 构建 VLESS 用户列表
func buildVLESSClients(users []*api.User, withFlow bool) []map[string]interface{} {
	clients := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		entry := map[string]interface{}{
			"id":    u.UUID,
			"email": fmt.Sprintf("user_%d", u.ID),
		}
		if withFlow {
			entry["flow"] = "xtls-rprx-vision"
		}
		clients = append(clients, entry)
	}
	return clients
}

// effectiveHost 返回有效的连接地址（CDN 优先）
func effectiveHost(info *ServerInfo) string {
	if info.CDNHost != "" {
		return info.CDNHost
	}
	return info.Host
}
