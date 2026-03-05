package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"

	"vasmax/internal/api"
)

// AnyTLS sing-box AnyTLS 协议
type AnyTLS struct{}

func (a *AnyTLS) Name() string          { return "anytls" }
func (a *AnyTLS) CoreType() string      { return "singbox" }
func (a *AnyTLS) DefaultPort() int      { return 443 }
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
	inbound := map[string]interface{}{
		"type":        "anytls",
		"tag":         params.Tag,
		"listen":      "::",
		"listen_port": params.Port,
		"users":       users,
		"tls": map[string]interface{}{
			"enabled":          true,
			"server_name":      params.Domain,
			"certificate_path": params.CertFile,
			"key_path":         params.KeyFile,
		},
	}
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
	params.Set("insecure", "0")
	params.Set("sni", info.Domain)
	return fmt.Sprintf("anytls://%s@%s:%d?%s#%s", user.UUID, info.Host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-anytls", info.Domain)))
}

func (a *AnyTLS) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-anytls", info.Domain),
		"type":               "anytls",
		"server":             info.Host,
		"port":               info.Port,
		"password":           user.UUID,
		"sni":                info.Domain,
		"client-fingerprint": "chrome",
		"udp":                true,
		"alpn":               []string{"h2", "http/1.1"},
	}
}

func (a *AnyTLS) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"type":        "anytls",
		"tag":         fmt.Sprintf("%s-anytls", info.Domain),
		"server":      info.Host,
		"server_port": info.Port,
		"password":    user.UUID,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
		},
	}
}
