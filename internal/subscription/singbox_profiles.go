package subscription

import (
	"encoding/json"
	"fmt"

	"vasmax/internal/api"
	"vasmax/internal/protocol"
)

// GenerateSingBoxOutbounds 生成 sing-box 出站列表
func GenerateSingBoxOutbounds(protocols []protocol.Protocol, users []*api.User, info *protocol.ServerInfo) []map[string]interface{} {
	var outbounds []map[string]interface{}
	for _, p := range protocols {
		for _, u := range users {
			ob := p.GenerateSingBoxOutbound(u, info)
			if ob != nil {
				if tag, ok := ob["tag"].(string); ok {
					ob["tag"] = fmt.Sprintf("%s-%d", tag, u.ID)
				}
				outbounds = append(outbounds, ob)
			}
		}
	}
	return outbounds
}

// GenerateSingBoxBasic 生成基础 sing-box 客户端配置
func GenerateSingBoxBasic(outbounds []map[string]interface{}) ([]byte, error) {
	tags := make([]string, 0, len(outbounds))
	for _, ob := range outbounds {
		if tag, ok := ob["tag"].(string); ok {
			tags = append(tags, tag)
		}
	}

	// 构建完整出站列表：selector + urltest + 用户出站 + direct + block + dns
	allOutbounds := []map[string]interface{}{
		{
			"type":      "selector",
			"tag":       "手动切换",
			"outbounds": append([]string{"自动选择"}, tags...),
			"default":   "自动选择",
		},
		{
			"type":      "urltest",
			"tag":       "自动选择",
			"outbounds": tags,
			"url":       "https://www.gstatic.com/generate_204",
			"interval":  "3m",
		},
	}
	allOutbounds = append(allOutbounds, outbounds...)
	allOutbounds = append(allOutbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "block", "tag": "block"},
		map[string]interface{}{"type": "dns", "tag": "dns-out"},
	)

	config := map[string]interface{}{
		"outbounds": allOutbounds,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal singbox config: %w", err)
	}
	return data, nil
}

// GenerateSingBoxFullProfile 生成完整 sing-box 客户端配置
func GenerateSingBoxFullProfile(outbounds []map[string]interface{}) ([]byte, error) {
	tags := make([]string, 0, len(outbounds))
	for _, ob := range outbounds {
		if tag, ok := ob["tag"].(string); ok {
			tags = append(tags, tag)
		}
	}

	allOutbounds := []map[string]interface{}{
		{
			"type":      "selector",
			"tag":       "手动切换",
			"outbounds": append([]string{"自动选择", "direct"}, tags...),
			"default":   "自动选择",
		},
		{
			"type":      "urltest",
			"tag":       "自动选择",
			"outbounds": tags,
			"url":       "https://www.gstatic.com/generate_204",
			"interval":  "3m",
		},
	}
	allOutbounds = append(allOutbounds, outbounds...)
	allOutbounds = append(allOutbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "block", "tag": "block"},
		map[string]interface{}{"type": "dns", "tag": "dns-out"},
	)

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
		},
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{"tag": "google", "address": "https://dns.google/dns-query", "detour": "手动切换"},
				{"tag": "local", "address": "https://223.5.5.5/dns-query", "detour": "direct"},
			},
			"rules": []map[string]interface{}{
				{"geosite": []string{"cn"}, "server": "local"},
			},
		},
		"inbounds": []map[string]interface{}{
			{
				"type":          "tun",
				"tag":           "tun-in",
				"inet4_address": "172.19.0.1/30",
				"auto_route":    true,
				"strict_route":  true,
				"sniff":         true,
			},
		},
		"outbounds": allOutbounds,
		"route": map[string]interface{}{
			"auto_detect_interface": true,
			"rules": []map[string]interface{}{
				{"protocol": "dns", "outbound": "dns-out"},
				{"geosite": []string{"cn"}, "geoip": []string{"cn", "private"}, "outbound": "direct"},
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal full singbox profile: %w", err)
	}
	return data, nil
}
