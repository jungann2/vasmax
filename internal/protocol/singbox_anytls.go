package protocol

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"

	"vasmax/internal/api"
)

// AnyTLS sing-box AnyTLS 协议
type AnyTLS struct{}

func (a *AnyTLS) Name() string          { return "anytls" }
func (a *AnyTLS) CoreType() string      { return "singbox" }
func (a *AnyTLS) DefaultPort() int      { return 31303 }
func (a *AnyTLS) TransportType() string { return "tcp" }
func (a *AnyTLS) IsCDNCompatible() bool { return false }

func (a *AnyTLS) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	users := make([]map[string]interface{}, 0, len(params.Users))
	for _, u := range params.Users {
		email := fmt.Sprintf("user_%d", u.ID)
		users = append(users, map[string]interface{}{
			"name":     fmt.Sprintf("%s-anytls", email),
			"password": u.UUID,
		})
	}
	tls := map[string]interface{}{
		"enabled":          true,
		"server_name":      params.Domain,
		"certificate_path": params.CertFile,
		"key_path":         params.KeyFile,
		"alpn":             inboundALPN(params, defaultTLSALPN),
	}
	inbound := map[string]interface{}{
		"type":        "anytls",
		"tag":         params.Tag,
		"listen":      "::",
		"listen_port": params.Port,
		"users":       users,
		"tls":         tls,
	}
	if len(params.PaddingScheme) > 0 {
		inbound["padding_scheme"] = params.PaddingScheme
	}
	applySingBoxTCPKeepAlive(inbound, params)
	return json.Marshal(inbound)
}

func (a *AnyTLS) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	email := fmt.Sprintf("user_%d", user.ID)
	entry := map[string]interface{}{
		"name":     fmt.Sprintf("%s-anytls", email),
		"password": user.UUID,
	}
	return json.Marshal(entry)
}

func (a *AnyTLS) GenerateURI(user *api.User, info *ServerInfo) string {
	params := url.Values{}
	params.Set("peer", info.Domain)
	// 自签证书（无域名模式，Domain 是 IP）需要 insecure=1
	if net.ParseIP(info.Domain) != nil {
		params.Set("insecure", "1")
	} else {
		params.Set("insecure", "0")
		params.Set("sni", info.Domain)
	}
	return fmt.Sprintf("anytls://%s@%s:%d?%s#%s", user.UUID, effectiveHost(info), info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-anytls", info.Domain)))
}

func (a *AnyTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	proxy := map[string]interface{}{
		"name":               fmt.Sprintf("%s-anytls", info.Domain),
		"type":               "anytls",
		"server":             info.Host,
		"port":               info.Port,
		"password":           user.UUID,
		"sni":                info.Domain,
		"client-fingerprint": "chrome",
		"udp":                true,
		"alpn":               serverInfoALPN(info, defaultTLSALPN),
	}
	markClashSkipCertVerify(proxy, info)
	return proxy
}

func (a *AnyTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	outbound := map[string]interface{}{
		"type":        "anytls",
		"tag":         fmt.Sprintf("%s-anytls", info.Domain),
		"server":      info.Host,
		"server_port": info.Port,
		"password":    user.UUID,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
			"alpn":        serverInfoALPN(info, defaultTLSALPN),
		},
	}
	markSingBoxTLSInsecure(outbound, info)
	return outbound
}
