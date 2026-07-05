package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"

	"vasmax/internal/config"
	"vasmax/internal/security"
)

// GenerateBaseOutboundConfig 生成 Xray 基础出站配置（02_outbounds.json）
// 包含 freedom 直连出站和 blackhole 屏蔽出站，这是 Xray 正常转发流量的必要条件
func GenerateBaseOutboundConfig(confDir string, serverDNS ...config.ServerDNSConfig) error {
	settings := map[string]interface{}{}
	if dnsCfg, ok := optionalServerDNS(serverDNS); ok && dnsCfg.EffectiveMode() != config.ServerDNSModeSystem {
		settings["domainStrategy"] = xrayDomainStrategy(dnsCfg.EffectiveStrategy())
	}
	config := map[string]interface{}{
		"outbounds": []map[string]interface{}{
			{
				"tag":      "direct",
				"protocol": "freedom",
				"settings": settings,
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
func GenerateBaseDNSConfig(confDir string, serverDNS ...config.ServerDNSConfig) error {
	dnsPath := filepath.Join(confDir, "03_dns.json")
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

	dns := map[string]interface{}{
		"dns": map[string]interface{}{
			"servers":       servers,
			"queryStrategy": xrayDomainStrategy(dnsCfg.EffectiveStrategy()),
		},
	}
	data, err := json.MarshalIndent(dns, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(dnsPath, data, 0644)
}

func optionalServerDNS(values []config.ServerDNSConfig) (config.ServerDNSConfig, bool) {
	if len(values) == 0 {
		return config.ServerDNSConfig{}, false
	}
	cfg := values[0]
	cfg.Mode = config.NormalizeServerDNSMode(cfg.Mode)
	cfg.Strategy = config.NormalizeServerDNSStrategy(cfg.Strategy)
	return cfg, true
}

func xrayDomainStrategy(strategy string) string {
	switch config.NormalizeServerDNSStrategy(strategy) {
	case "prefer_ipv6", "ipv6_only":
		return "UseIPv6"
	case "prefer_ipv4", "ipv4_only":
		return "UseIPv4"
	default:
		return "UseIPv4"
	}
}

// EnsureBaseConfigs 确保 Xray 基础配置文件存在（outbound + dns）
func EnsureBaseConfigs(confDir string, cfg ...*config.Config) error {
	var dnsCfg []config.ServerDNSConfig
	if len(cfg) > 0 && cfg[0] != nil {
		dnsCfg = append(dnsCfg, cfg[0].ServerDNS)
	}
	if err := GenerateBaseOutboundConfig(confDir, dnsCfg...); err != nil {
		return err
	}
	return GenerateBaseDNSConfig(confDir, dnsCfg...)
}
