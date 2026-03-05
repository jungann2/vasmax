package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"

	"vasmax/internal/api"
)

// Hysteria2 sing-box Hysteria2 协议
type Hysteria2 struct{}

func (h *Hysteria2) Name() string          { return "hysteria2" }
func (h *Hysteria2) CoreType() string      { return "singbox" }
func (h *Hysteria2) DefaultPort() int      { return 10080 }
func (h *Hysteria2) TransportType() string { return "quic" }
func (h *Hysteria2) IsCDNCompatible() bool { return false }

func (h *Hysteria2) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	users := make([]map[string]interface{}, 0, len(params.Users))
	for _, u := range params.Users {
		users = append(users, map[string]interface{}{
			"name":     fmt.Sprintf("user_%d", u.ID),
			"password": u.UUID,
		})
	}
	inbound := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         params.Tag,
		"listen":      "::",
		"listen_port": params.Port,
		"users":       users,
		"tls": map[string]interface{}{
			"enabled":          true,
			"certificate_path": params.CertFile,
			"key_path":         params.KeyFile,
		},
	}
	if params.Hysteria2 != nil && params.Hysteria2.DownMbps > 0 {
		inbound["down_mbps"] = params.Hysteria2.DownMbps
		inbound["up_mbps"] = params.Hysteria2.UpMbps
	}
	return json.Marshal(inbound)
}

func (h *Hysteria2) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"name":     fmt.Sprintf("user_%d", user.ID),
		"password": user.UUID,
	}
	return json.Marshal(entry)
}

func (h *Hysteria2) GenerateURI(user *api.User, info *ServerInfo) string {
	params := url.Values{}
	params.Set("sni", info.Domain)
	params.Set("insecure", "0")
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", user.UUID, info.Host, info.Port, params.Encode(),
		url.PathEscape(fmt.Sprintf("%s-hysteria2", info.Domain)))
}

func (h *Hysteria2) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":               fmt.Sprintf("%s-hysteria2", info.Domain),
		"type":               "hysteria2",
		"server":             info.Host,
		"port":               info.Port,
		"password":           user.UUID,
		"sni":                info.Domain,
		"client-fingerprint": "chrome",
	}
}

func (h *Hysteria2) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"type":        "hysteria2",
		"tag":         fmt.Sprintf("%s-hysteria2", info.Domain),
		"server":      info.Host,
		"server_port": info.Port,
		"password":    user.UUID,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
		},
	}
}
