package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"

	"vasmax/internal/api"
)

// Naive sing-box Naive 协议
type Naive struct{}

func (n *Naive) Name() string          { return "naive" }
func (n *Naive) CoreType() string      { return "singbox" }
func (n *Naive) DefaultPort() int      { return 443 }
func (n *Naive) TransportType() string { return "tcp" }
func (n *Naive) IsCDNCompatible() bool { return false }

func (n *Naive) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	users := make([]map[string]interface{}, 0, len(params.Users))
	for _, u := range params.Users {
		users = append(users, map[string]interface{}{
			"username": fmt.Sprintf("user_%d", u.ID),
			"password": u.UUID,
		})
	}
	inbound := map[string]interface{}{
		"type":        "naive",
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
	return json.Marshal(inbound)
}

func (n *Naive) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"username": fmt.Sprintf("user_%d", user.ID),
		"password": user.UUID,
	}
	return json.Marshal(entry)
}

func (n *Naive) GenerateURI(user *api.User, info *ServerInfo) string {
	username := fmt.Sprintf("user_%d", user.ID)
	return fmt.Sprintf("naive+https://%s:%s@%s:%d#%s",
		url.PathEscape(username), url.PathEscape(user.UUID),
		info.Host, info.Port,
		url.PathEscape(fmt.Sprintf("%s-naive", info.Domain)))
}

func (n *Naive) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	// ClashMeta 不原生支持 naive，返回基础信息
	return map[string]interface{}{
		"name":     fmt.Sprintf("%s-naive", info.Domain),
		"type":     "naive",
		"server":   info.Host,
		"port":     info.Port,
		"username": fmt.Sprintf("user_%d", user.ID),
		"password": user.UUID,
		"sni":      info.Domain,
	}
}

func (n *Naive) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"type":        "naive",
		"tag":         fmt.Sprintf("%s-naive", info.Domain),
		"server":      info.Host,
		"server_port": info.Port,
		"username":    fmt.Sprintf("user_%d", user.ID),
		"password":    user.UUID,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": info.Domain,
		},
	}
}
