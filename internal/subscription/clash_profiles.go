package subscription

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// GenerateClashFullProfile 生成完整 ClashMeta 配置
// 包含 DNS、proxy-providers、proxy-groups、rule-providers、rules、sniffer 等
func GenerateClashFullProfile(proxies []map[string]interface{}, domain string) ([]byte, error) {
	proxyNames := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if name, ok := p["name"].(string); ok {
			proxyNames = append(proxyNames, name)
		}
	}

	config := map[string]interface{}{
		"mixed-port":                7890,
		"allow-lan":                 false,
		"mode":                      "rule",
		"log-level":                 "info",
		"global-client-fingerprint": "chrome",
		"dns": map[string]interface{}{
			"enable":        true,
			"enhanced-mode": "fake-ip",
			"fake-ip-range": "198.18.0.1/16",
			"nameserver": []string{
				"https://dns.google/dns-query",
				"https://cloudflare-dns.com/dns-query",
			},
			"fallback": []string{
				"https://1.0.0.1/dns-query",
				"https://8.8.4.4/dns-query",
			},
		},
		"sniffer": map[string]interface{}{
			"enable":         true,
			"sniffing":       []string{"tls", "http"},
			"force-domain":   []string{},
			"skip-domain":    []string{"Mijia cloud"},
			"port-whitelist": []int{443, 80},
		},
		"proxies":        proxies,
		"proxy-groups":   buildClashProxyGroups(proxyNames),
		"rule-providers": buildClashRuleProviders(),
		"rules":          buildClashRules(),
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal full clash profile: %w", err)
	}
	return data, nil
}

// buildClashProxyGroups 构建 ClashMeta proxy-groups
func buildClashProxyGroups(proxyNames []string) []map[string]interface{} {
	groups := []struct {
		name    string
		gtype   string
		proxies []string
		extra   map[string]interface{}
	}{
		{"手动切换", "select", append([]string{"自动选择", "DIRECT"}, proxyNames...), nil},
		{"自动选择", "url-test", proxyNames, map[string]interface{}{"url": "https://www.gstatic.com/generate_204", "interval": 300}},
		{"国外流量", "select", []string{"手动切换", "自动选择", "DIRECT"}, nil},
		{"流媒体", "select", append([]string{"手动切换", "自动选择"}, proxyNames...), nil},
		{"Telegram", "select", append([]string{"手动切换", "自动选择"}, proxyNames...), nil},
		{"Google", "select", append([]string{"手动切换", "自动选择"}, proxyNames...), nil},
		{"OpenAI", "select", append([]string{"手动切换", "自动选择"}, proxyNames...), nil},
		{"游戏", "select", append([]string{"手动切换", "自动选择", "DIRECT"}, proxyNames...), nil},
		{"微软", "select", []string{"手动切换", "自动选择", "DIRECT"}, nil},
		{"苹果", "select", []string{"手动切换", "自动选择", "DIRECT"}, nil},
		{"国内流量", "select", []string{"DIRECT", "手动切换"}, nil},
		{"广告拦截", "select", []string{"REJECT", "DIRECT"}, nil},
		{"兜底规则", "select", []string{"手动切换", "自动选择", "DIRECT"}, nil},
	}

	result := make([]map[string]interface{}, 0, len(groups))
	for _, g := range groups {
		m := map[string]interface{}{
			"name":    g.name,
			"type":    g.gtype,
			"proxies": g.proxies,
		}
		for k, v := range g.extra {
			m[k] = v
		}
		result = append(result, m)
	}
	return result
}

// buildClashRuleProviders 构建 GeoX 规则集提供者
func buildClashRuleProviders() map[string]interface{} {
	providers := map[string]struct {
		behavior string
		url      string
	}{
		"reject":       {"domain", "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/reject.txt"},
		"proxy":        {"domain", "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/proxy.txt"},
		"direct":       {"domain", "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/direct.txt"},
		"cncidr":       {"ipcidr", "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/cncidr.txt"},
		"telegramcidr": {"ipcidr", "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/telegramcidr.txt"},
		"gfw":          {"domain", "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/gfw.txt"},
	}

	result := make(map[string]interface{}, len(providers))
	for name, p := range providers {
		result[name] = map[string]interface{}{
			"type":     "http",
			"behavior": p.behavior,
			"url":      p.url,
			"path":     fmt.Sprintf("./ruleset/%s.yaml", name),
			"interval": 86400,
		}
	}
	return result
}

// buildClashRules 构建路由规则
func buildClashRules() []string {
	return []string{
		"RULE-SET,reject,广告拦截",
		"RULE-SET,telegramcidr,Telegram",
		"DOMAIN-SUFFIX,openai.com,OpenAI",
		"DOMAIN-SUFFIX,ai.com,OpenAI",
		"DOMAIN-KEYWORD,google,Google",
		"DOMAIN-SUFFIX,googleapis.com,Google",
		"DOMAIN-SUFFIX,youtube.com,流媒体",
		"DOMAIN-SUFFIX,netflix.com,流媒体",
		"DOMAIN-SUFFIX,spotify.com,流媒体",
		"DOMAIN-SUFFIX,twitch.tv,流媒体",
		"DOMAIN-KEYWORD,microsoft,微软",
		"DOMAIN-SUFFIX,windows.net,微软",
		"DOMAIN-SUFFIX,apple.com,苹果",
		"DOMAIN-SUFFIX,icloud.com,苹果",
		"RULE-SET,proxy,国外流量",
		"RULE-SET,gfw,国外流量",
		"RULE-SET,direct,国内流量",
		"RULE-SET,cncidr,国内流量",
		"GEOIP,CN,国内流量",
		"MATCH,兜底规则",
	}
}
