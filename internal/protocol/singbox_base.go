package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"vasmax/internal/config"
	"vasmax/internal/security"
)

// GenerateSingBoxBaseOutbound 生成 sing-box 基础出站配置（00_outbounds.json）
// 包含 direct 直连出站，这是 sing-box 正常转发流量的必要条件
func GenerateSingBoxBaseOutbound(confDir string, serverDNS ...config.ServerDNSConfig) error {
	direct := map[string]interface{}{
		"type": "direct",
		"tag":  "direct",
	}
	if dnsCfg, ok := optionalServerDNS(serverDNS); ok && dnsCfg.EffectiveMode() != config.ServerDNSModeSystem {
		direct["domain_strategy"] = singBoxDNSStrategy(dnsCfg.EffectiveStrategy())
	}
	config := map[string]interface{}{
		"outbounds": []map[string]interface{}{
			direct,
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

// GenerateSingBoxBaseDNS 生成 sing-box 基础 DNS 配置（01_dns.json）。
// system 模式不生成 DNS 配置，sing-box 使用系统默认 DNS；显式模式使用 1.12+ DNS server 格式。
func GenerateSingBoxBaseDNS(confDir string, serverDNS ...config.ServerDNSConfig) error {
	dnsPath := filepath.Join(confDir, "01_dns.json")
	dnsCfg, ok := optionalServerDNS(serverDNS)
	if !ok || dnsCfg.EffectiveMode() == config.ServerDNSModeSystem {
		if err := os.Remove(dnsPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	servers := dnsCfg.EffectiveServers()
	if len(servers) == 0 {
		if err := os.Remove(dnsPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	dnsServers := make([]map[string]interface{}, 0, len(servers))
	for i, server := range servers {
		dnsServers = append(dnsServers, map[string]interface{}{
			"type":        "udp",
			"tag":         dnsServerTag(i),
			"server":      server,
			"server_port": 53,
		})
	}
	config := map[string]interface{}{
		"dns": map[string]interface{}{
			"servers":  dnsServers,
			"final":    dnsServerTag(0),
			"strategy": singBoxDNSStrategy(dnsCfg.EffectiveStrategy()),
			"timeout":  "5s",
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(dnsPath, data, 0644)
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
func EnsureSingBoxBaseConfigs(confDir string, cfg ...*config.Config) error {
	var dnsCfg []config.ServerDNSConfig
	if len(cfg) > 0 && cfg[0] != nil {
		dnsCfg = append(dnsCfg, cfg[0].ServerDNS)
	}
	if err := GenerateSingBoxBaseOutbound(confDir, dnsCfg...); err != nil {
		return err
	}
	if err := GenerateSingBoxBaseDNS(confDir, dnsCfg...); err != nil {
		return err
	}
	return GenerateSingBoxBaseRoute(confDir)
}

func dnsServerTag(index int) string {
	return "server-dns-" + strconv.Itoa(index+1)
}

func singBoxDNSStrategy(strategy string) string {
	return config.NormalizeServerDNSStrategy(strategy)
}
