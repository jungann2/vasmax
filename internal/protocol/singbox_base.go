package protocol

import (
	"encoding/json"
	"path/filepath"

	"vasmax/internal/security"
)

// GenerateSingBoxBaseOutbound 生成 sing-box 基础出站配置（00_outbounds.json）
// 包含 direct 直连出站，这是 sing-box 正常转发流量的必要条件
func GenerateSingBoxBaseOutbound(confDir string) error {
	config := map[string]interface{}{
		"outbounds": []map[string]interface{}{
			{
				"type": "direct",
				"tag":  "direct",
			},
			{
				"type": "block",
				"tag":  "block",
			},
			{
				"type": "dns",
				"tag":  "dns-out",
			},
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(filepath.Join(confDir, "00_outbounds.json"), data, 0644)
}

// GenerateSingBoxBaseDNS 生成 sing-box 基础 DNS 配置（01_dns.json）
func GenerateSingBoxBaseDNS(confDir string) error {
	config := map[string]interface{}{
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"tag":     "google",
					"address": "8.8.8.8",
				},
				{
					"tag":     "cloudflare",
					"address": "1.1.1.1",
				},
			},
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(filepath.Join(confDir, "01_dns.json"), data, 0644)
}

// GenerateSingBoxBaseRoute 生成 sing-box 基础路由配置（02_route.json）
func GenerateSingBoxBaseRoute(confDir string) error {
	config := map[string]interface{}{
		"route": map[string]interface{}{
			"auto_detect_interface": true,
			"final":                 "direct",
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(filepath.Join(confDir, "02_route.json"), data, 0644)
}

// EnsureSingBoxBaseConfigs 确保 sing-box 基础配置文件存在（outbound + dns + route）
func EnsureSingBoxBaseConfigs(confDir string) error {
	if err := GenerateSingBoxBaseOutbound(confDir); err != nil {
		return err
	}
	if err := GenerateSingBoxBaseDNS(confDir); err != nil {
		return err
	}
	return GenerateSingBoxBaseRoute(confDir)
}
