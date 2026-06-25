package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"

	"vasmax/internal/security"
)

// GenerateBaseOutboundConfig 生成 Xray 基础出站配置（02_outbounds.json）
// 包含 freedom 直连出站和 blackhole 屏蔽出站，这是 Xray 正常转发流量的必要条件
func GenerateBaseOutboundConfig(confDir string) error {
	config := map[string]interface{}{
		"outbounds": []map[string]interface{}{
			{
				"tag":      "direct",
				"protocol": "freedom",
				"settings": map[string]interface{}{},
			},
			{
				"tag":      "blocked",
				"protocol": "blackhole",
				"settings": map[string]interface{}{},
			},
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(filepath.Join(confDir, "02_outbounds.json"), data, 0644)
}

// GenerateBaseDNSConfig removes the legacy fixed public DNS config.
// Without an explicit DNS block, Xray uses the system resolver, which better
// matches the server's region, network, and provider DNS setup.
func GenerateBaseDNSConfig(confDir string) error {
	dnsPath := filepath.Join(confDir, "03_dns.json")
	if err := os.Remove(dnsPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// EnsureBaseConfigs 确保 Xray 基础配置文件存在（outbound + dns）
func EnsureBaseConfigs(confDir string) error {
	if err := GenerateBaseOutboundConfig(confDir); err != nil {
		return err
	}
	return GenerateBaseDNSConfig(confDir)
}
