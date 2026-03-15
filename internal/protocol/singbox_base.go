package protocol

import (
	"encoding/json"
	"os"
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
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(filepath.Join(confDir, "00_outbounds.json"), data, 0644)
}

// GenerateSingBoxBaseDNS 生成 sing-box 基础 DNS 配置（01_dns.json）
// sing-box 1.12+ 废弃了旧版 DNS 格式，这里不生成 DNS 配置，使用系统默认 DNS
func GenerateSingBoxBaseDNS(confDir string) error {
	// 不生成 DNS 配置，sing-box 会使用系统默认 DNS
	// 删除旧版可能存在的 DNS 配置文件
	dnsPath := filepath.Join(confDir, "01_dns.json")
	os.Remove(dnsPath)
	return nil
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
