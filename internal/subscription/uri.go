package subscription

import (
	"encoding/base64"
	"strings"

	"vasmax/internal/api"
	"vasmax/internal/protocol"
)

// GenerateURIs 为指定用户生成所有协议的 URI 链接列表
func GenerateURIs(protocols []protocol.Protocol, user *api.User, info *protocol.ServerInfo) []string {
	var uris []string
	for _, p := range protocols {
		uri := p.GenerateURI(user, info)
		if uri != "" {
			uris = append(uris, uri)
		}
	}
	return uris
}

// EncodeBase64Subscription 将 URI 列表编码为 Base64 订阅内容
func EncodeBase64Subscription(uris []string) string {
	content := strings.Join(uris, "\n")
	return base64.StdEncoding.EncodeToString([]byte(content))
}
