package config

import (
	"net"
	"strings"
)

const (
	ServerDNSModeSystem     = "system"
	ServerDNSModeCloudflare = "cloudflare"
	ServerDNSModeQuad9      = "quad9"
	ServerDNSModeGoogle     = "google"
	ServerDNSModeCustom     = "custom"
)

// NormalizeServerDNSMode returns a supported server DNS mode.
func NormalizeServerDNSMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ServerDNSModeCloudflare, ServerDNSModeQuad9, ServerDNSModeGoogle, ServerDNSModeCustom:
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ServerDNSModeSystem
	}
}

// NormalizeServerDNSStrategy returns a supported IP selection strategy.
func NormalizeServerDNSStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "prefer_ipv4", "prefer_ipv6", "ipv6_only":
		return strings.ToLower(strings.TrimSpace(strategy))
	default:
		return "ipv4_only"
	}
}

// EffectiveMode returns the normalized server DNS mode.
func (d ServerDNSConfig) EffectiveMode() string {
	return NormalizeServerDNSMode(d.Mode)
}

// EffectiveStrategy returns the normalized server DNS IP strategy.
func (d ServerDNSConfig) EffectiveStrategy() string {
	return NormalizeServerDNSStrategy(d.Strategy)
}

// EffectiveServers returns the DNS servers used by the selected mode.
func (d ServerDNSConfig) EffectiveServers() []string {
	switch d.EffectiveMode() {
	case ServerDNSModeCloudflare:
		return []string{"1.1.1.1", "1.0.0.1"}
	case ServerDNSModeQuad9:
		return []string{"9.9.9.9", "149.112.112.112"}
	case ServerDNSModeGoogle:
		return []string{"8.8.8.8", "8.8.4.4"}
	case ServerDNSModeCustom:
		return NormalizeServerDNSServers(d.Servers)
	default:
		return nil
	}
}

// NormalizeServerDNSServers trims, deduplicates and keeps only plain IP servers.
func NormalizeServerDNSServers(servers []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(servers))
	for _, server := range servers {
		server = strings.TrimSpace(server)
		if server == "" || strings.Contains(server, "://") {
			continue
		}
		host := server
		if strings.Contains(server, ":") {
			if h, _, err := net.SplitHostPort(server); err == nil {
				host = strings.Trim(h, "[]")
			}
		}
		if ip := net.ParseIP(host); ip == nil {
			continue
		}
		if !seen[host] {
			seen[host] = true
			out = append(out, host)
		}
	}
	return out
}
