package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"

	"vasmax/internal/api"
)

// Tuic sing-box Tuic 协议
type Tuic struct{}

func (t *Tuic) Name() string          { return "tuic" }
func (t *Tuic) CoreType() string      { return "singbox" }
func (t *Tuic) DefaultPort() int      { return 10081 }
func (t *Tuic) TransportType() string { return "quic" }
func (t *Tuic) IsCDNCompatible() bool { return false }

func (t *Tuic) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	users := make([]map[string]interface{}, 0, len(params.Users))
	for _, u := range params.Users {
		users = append(users, map[string]interface{}{
			"name":     fmt.Sprintf("user_%d", u.ID),
			"uuid":     u.UUID,
			"password": u.UUID,
		})
	}
	cc := "bbr"
	if params.Tuic != nil && params.Tuic.CongestionControl != "" {
		cc = params.Tuic.CongestionControl
	}
	inbound := map[string]interface{}{
		"type":               "tuic",
		"tag":                params.Tag,
		"listen":             "::",
		"listen_port":        params.Port,
		"users":              users,
		"congestion_control": cc,
		"tls": map[string]interface{}{
			"enabled":          true,
			"certificate_path": params.CertFile,
			"key_path":         params.KeyFile,
			"alpn":             []string{"h3"},
		},
	}
	return json.Marshal(inbound)
}

func (t *Tuic) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"name":     fmt.Sprintf("user_%d", user.ID),
		"uuid":     user.UUID,
		"password": user.UUID,
	}
	return json.Marshal(entry)
}

func (t *Tuic) GenerateURI(user *api.User, info *ServerInfo) string {
	params := url.Values{}
	params.Set("sni", info.Domain)
	params.Set("congestion_control", "bbr")
	params.Set("alpn", "h3")
	params.Set("udp_relay_mode", "native")
	return fmt.Sprintf("tuic://%s:%s@%s:%d?%s#%s", user.UUID, user.UUID, info.Host, info.Port,
		params.Encode(), url.PathEscape(fmt.Sprintf("%s-tuic", info.Domain)))
}

func (t *Tuic) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":                  fmt.Sprintf("%s-tuic", info.Domain),
		"type":                  "tuic",
		"server":                info.Host,
		"port":                  info.Port,
		"uuid":                  user.UUID,
		"password":              user.UUID,
		"sni":                   info.Domain,
		"alpn":                  []string{"h3"},
		"congestion-controller": "bbr",
		"udp-relay-mode":        "native",
		"client-fingerprint":    "chrome",
	}
}

func (t *Tuic) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"type":               "tuic",
		"tag":                fmt.Sprintf("%s-tuic", info.Domain),
		"server":             info.Host,
		"server_port":        info.Port,
		"uuid":               user.UUID,
		"password":           user.UUID,
		"congestion_control": "bbr",
		"udp_relay_mode":     "native",
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
			"alpn":        []string{"h3"},
		},
	}
}
