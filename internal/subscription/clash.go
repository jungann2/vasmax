package subscription

import (
	"fmt"

	"vasmax/internal/api"
	"vasmax/internal/protocol"

	"gopkg.in/yaml.v3"
)

// GenerateClashProxies 生成 ClashMeta proxies 列表
func GenerateClashProxies(protocols []protocol.Protocol, users []*api.User, info *protocol.ServerInfo) []map[string]interface{} {
	var proxies []map[string]interface{}
	for _, p := range protocols {
		for _, u := range users {
			proxy := p.GenerateClashProxy(u, info)
			if proxy != nil {
				// 为每个用户生成唯一名称
				if name, ok := proxy["name"].(string); ok {
					proxy["name"] = fmt.Sprintf("%s-%d", name, u.ID)
				}
				proxies = append(proxies, proxy)
			}
		}
	}
	return proxies
}

// GenerateClashBasic 生成基础 ClashMeta 配置（proxies + proxy-groups）
func GenerateClashBasic(proxies []map[string]interface{}) ([]byte, error) {
	proxyNames := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if name, ok := p["name"].(string); ok {
			proxyNames = append(proxyNames, name)
		}
	}

	config := map[string]interface{}{
		"proxies": proxies,
		"proxy-groups": []map[string]interface{}{
			{
				"name":    "手动切换",
				"type":    "select",
				"proxies": append([]string{"自动选择"}, proxyNames...),
			},
			{
				"name":     "自动选择",
				"type":     "url-test",
				"proxies":  proxyNames,
				"url":      "https://www.gstatic.com/generate_204",
				"interval": 300,
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal clash config: %w", err)
	}
	return data, nil
}
