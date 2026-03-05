package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"vasmax/internal/api"
)

// Socks5 sing-box Socks5 协议
type Socks5 struct{}

func (s *Socks5) Name() string          { return "socks5" }
func (s *Socks5) CoreType() string      { return "singbox" }
func (s *Socks5) DefaultPort() int      { return 10082 }
func (s *Socks5) TransportType() string { return "tcp" }
func (s *Socks5) IsCDNCompatible() bool { return false }

func (s *Socks5) GenerateInbound(params *InboundParams) (json.RawMessage, error) {
	users := make([]map[string]interface{}, 0, len(params.Users))
	for _, u := range params.Users {
		users = append(users, map[string]interface{}{
			"username": fmt.Sprintf("user_%d", u.ID),
			"password": u.UUID,
		})
	}
	inbound := map[string]interface{}{
		"type":        "socks",
		"tag":         params.Tag,
		"listen":      "::",
		"listen_port": params.Port,
		"users":       users,
	}
	return json.Marshal(inbound)
}

func (s *Socks5) GenerateUserEntry(user *api.User) (json.RawMessage, error) {
	entry := map[string]interface{}{
		"username": fmt.Sprintf("user_%d", user.ID),
		"password": user.UUID,
	}
	return json.Marshal(entry)
}

func (s *Socks5) GenerateURI(user *api.User, info *ServerInfo) string {
	return fmt.Sprintf("socks5://%s:%s@%s:%d#%s",
		fmt.Sprintf("user_%d", user.ID), user.UUID,
		info.Host, info.Port,
		fmt.Sprintf("%s-socks5", info.Domain))
}

func (s *Socks5) GenerateClashProxy(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":     fmt.Sprintf("%s-socks5", info.Domain),
		"type":     "socks5",
		"server":   info.Host,
		"port":     info.Port,
		"username": fmt.Sprintf("user_%d", user.ID),
		"password": user.UUID,
	}
}

func (s *Socks5) GenerateSingBoxOutbound(user *api.User, info *ServerInfo) map[string]interface{} {
	return map[string]interface{}{
		"type":        "socks",
		"tag":         fmt.Sprintf("%s-socks5", info.Domain),
		"server":      info.Host,
		"server_port": info.Port,
		"username":    fmt.Sprintf("user_%d", user.ID),
		"password":    user.UUID,
	}
}

// randomHex 生成指定字节数的随机十六进制字符串
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
