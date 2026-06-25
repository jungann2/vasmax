package subscription

import (
	"strconv"
	"strings"

	"vasmax/internal/config"
)

type DNSOptions struct {
	Mode     string
	Domestic []string
	Global   []string
	Privacy  []string
	Custom   []string
}

type ProfileOptions struct {
	DNS     DNSOptions
	TestURL string
}

const defaultTestURL = "https://www.gstatic.com/generate_204"

var (
	defaultDomesticDNS = []string{
		"https://dns.alidns.com/dns-query",
		"https://doh.pub/dns-query",
	}
	defaultGlobalDNS = []string{
		"https://cloudflare-dns.com/dns-query",
		"https://dns.google/dns-query",
		"https://dns.quad9.net/dns-query",
	}
	defaultPrivacyDNS = []string{
		"https://dns.quad9.net/dns-query",
		"https://cloudflare-dns.com/dns-query",
		"https://dns.adguard-dns.com/dns-query",
	}
)

func DNSOptionsFromConfig(cfg config.SubscriptionConfig) DNSOptions {
	opts := DNSOptions{
		Mode:     normalizeDNSMode(cfg.DNSMode),
		Domestic: normalizeDNSServers(cfg.DNSDomestic),
		Global:   normalizeDNSServers(cfg.DNSGlobal),
		Privacy:  normalizeDNSServers(cfg.DNSPrivacy),
		Custom:   normalizeDNSServers(cfg.DNSCustom),
	}
	if len(opts.Domestic) == 0 {
		opts.Domestic = append([]string(nil), defaultDomesticDNS...)
	}
	if len(opts.Global) == 0 {
		opts.Global = append([]string(nil), defaultGlobalDNS...)
	}
	if len(opts.Privacy) == 0 {
		opts.Privacy = append([]string(nil), defaultPrivacyDNS...)
	}
	return opts
}

func ProfileOptionsFromConfig(cfg config.SubscriptionConfig) ProfileOptions {
	return ProfileOptions{
		DNS:     DNSOptionsFromConfig(cfg),
		TestURL: normalizeTestURL(cfg.TestURL),
	}
}

func DefaultDNSOptions() DNSOptions {
	return DNSOptionsFromConfig(config.SubscriptionConfig{DNSMode: "auto"})
}

func DefaultProfileOptions() ProfileOptions {
	return ProfileOptionsFromConfig(config.SubscriptionConfig{DNSMode: "auto", TestURL: defaultTestURL})
}

func normalizeDNSMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "cn", "global", "privacy", "custom":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "auto"
	}
}

func normalizeDNSServers(servers []string) []string {
	result := make([]string, 0, len(servers))
	seen := make(map[string]struct{}, len(servers))
	for _, server := range servers {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}
		if _, ok := seen[server]; ok {
			continue
		}
		seen[server] = struct{}{}
		result = append(result, server)
	}
	return result
}

func normalizeTestURL(testURL string) string {
	testURL = strings.TrimSpace(testURL)
	if testURL == "" {
		return defaultTestURL
	}
	return testURL
}

func (o DNSOptions) clashServers() (nameserver []string, fallback []string) {
	o = fillDNSDefaults(o)
	switch o.Mode {
	case "cn":
		return o.Domestic, nil
	case "global":
		return o.Global, nil
	case "privacy":
		return o.Privacy, nil
	case "custom":
		if len(o.Custom) > 0 {
			return o.Custom, nil
		}
	}
	return o.Domestic, o.Global
}

func (o DNSOptions) singBoxDNS() map[string]interface{} {
	o = fillDNSDefaults(o)

	switch o.Mode {
	case "cn":
		return buildSingBoxDNS([]dnsGroup{{prefix: "local", servers: o.Domestic, detour: "direct"}}, "local-1", nil)
	case "global":
		return buildSingBoxDNS([]dnsGroup{{prefix: "global", servers: o.Global, detour: "手动切换"}}, "global-1", nil)
	case "privacy":
		return buildSingBoxDNS([]dnsGroup{{prefix: "privacy", servers: o.Privacy, detour: "手动切换"}}, "privacy-1", nil)
	case "custom":
		if len(o.Custom) > 0 {
			return buildSingBoxDNS([]dnsGroup{{prefix: "custom", servers: o.Custom, detour: "direct"}}, "custom-1", nil)
		}
	}

	rules := []map[string]interface{}{
		{"geosite": []string{"cn"}, "server": "local-1"},
	}
	return buildSingBoxDNS([]dnsGroup{
		{prefix: "local", servers: o.Domestic, detour: "direct"},
		{prefix: "global", servers: o.Global, detour: "手动切换"},
	}, "global-1", rules)
}

type dnsGroup struct {
	prefix  string
	servers []string
	detour  string
}

func buildSingBoxDNS(groups []dnsGroup, final string, rules []map[string]interface{}) map[string]interface{} {
	servers := make([]map[string]interface{}, 0)
	for _, group := range groups {
		for i, address := range group.servers {
			entry := map[string]interface{}{
				"tag":     group.prefix + "-" + strconv.Itoa(i+1),
				"address": address,
			}
			if group.detour != "" {
				entry["detour"] = group.detour
			}
			servers = append(servers, entry)
		}
	}

	dns := map[string]interface{}{
		"servers": servers,
		"final":   final,
	}
	if len(rules) > 0 {
		dns["rules"] = rules
	}
	return dns
}

func fillDNSDefaults(o DNSOptions) DNSOptions {
	o.Mode = normalizeDNSMode(o.Mode)
	o.Domestic = normalizeDNSServers(o.Domestic)
	o.Global = normalizeDNSServers(o.Global)
	o.Privacy = normalizeDNSServers(o.Privacy)
	o.Custom = normalizeDNSServers(o.Custom)
	if len(o.Domestic) == 0 {
		o.Domestic = append([]string(nil), defaultDomesticDNS...)
	}
	if len(o.Global) == 0 {
		o.Global = append([]string(nil), defaultGlobalDNS...)
	}
	if len(o.Privacy) == 0 {
		o.Privacy = append([]string(nil), defaultPrivacyDNS...)
	}
	return o
}
