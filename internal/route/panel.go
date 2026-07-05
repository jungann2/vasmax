package route

import (
	"encoding/json"
	"strings"

	"vasmax/internal/api"
)

// BuildXrayPanelRouting converts Xboard UniProxy routes into an Xray routing
// partial. Unsupported or empty rules are ignored instead of emitting invalid
// core configuration.
func BuildXrayPanelRouting(routes []api.RouteRule) ([]byte, error) {
	rules := make([]map[string]interface{}, 0, len(routes))
	for _, rule := range routes {
		rules = append(rules, compileXrayPanelRoute(rule)...)
	}
	wrapper := map[string]interface{}{
		"routing": map[string]interface{}{
			"domainStrategy": "AsIs",
			"rules":          rules,
		},
	}
	return json.MarshalIndent(wrapper, "", "  ")
}

// BuildSingBoxPanelRoute converts Xboard UniProxy routes into a sing-box route
// partial. It is merged with the base route config by the sing-box manager.
func BuildSingBoxPanelRoute(routes []api.RouteRule) ([]byte, error) {
	rules := make([]map[string]interface{}, 0, len(routes))
	for _, rule := range routes {
		rules = append(rules, compileSingBoxPanelRoute(rule)...)
	}
	wrapper := map[string]interface{}{
		"route": map[string]interface{}{
			"rules": rules,
		},
	}
	return json.MarshalIndent(wrapper, "", "  ")
}

func compileXrayPanelRoute(rule api.RouteRule) []map[string]interface{} {
	if len(rule.Match) == 0 {
		return nil
	}

	var domains, ips []string
	for _, item := range rule.Match {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		switch {
		case strings.HasPrefix(item, "geoip:"), strings.Contains(item, "/"):
			ips = append(ips, item)
		case strings.HasPrefix(item, "geosite:"):
			domains = append(domains, item)
		default:
			domains = append(domains, "domain:"+strings.TrimPrefix(item, "*."))
		}
	}

	outbound := xrayPanelOutbound(rule)
	var compiled []map[string]interface{}
	if len(domains) > 0 {
		compiled = append(compiled, map[string]interface{}{
			"type":        "field",
			"domain":      domains,
			"outboundTag": outbound,
		})
	}
	if len(ips) > 0 {
		compiled = append(compiled, map[string]interface{}{
			"type":        "field",
			"ip":          ips,
			"outboundTag": outbound,
		})
	}
	return compiled
}

func compileSingBoxPanelRoute(rule api.RouteRule) []map[string]interface{} {
	if len(rule.Match) == 0 {
		return nil
	}

	var domains, cidrs, geosites, geoips []string
	for _, item := range rule.Match {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		switch {
		case strings.HasPrefix(item, "geosite:"):
			geosites = append(geosites, strings.TrimPrefix(item, "geosite:"))
		case strings.HasPrefix(item, "geoip:"):
			geoips = append(geoips, strings.TrimPrefix(item, "geoip:"))
		case strings.Contains(item, "/"):
			cidrs = append(cidrs, item)
		default:
			domains = append(domains, strings.TrimPrefix(item, "*."))
		}
	}

	outbound := singBoxPanelOutbound(rule)
	var compiled []map[string]interface{}
	if len(domains) > 0 {
		compiled = append(compiled, map[string]interface{}{
			"domain_suffix": domains,
			"outbound":      outbound,
		})
	}
	if len(cidrs) > 0 {
		compiled = append(compiled, map[string]interface{}{
			"ip_cidr":  cidrs,
			"outbound": outbound,
		})
	}
	if len(geosites) > 0 {
		compiled = append(compiled, map[string]interface{}{
			"geosite":  geosites,
			"outbound": outbound,
		})
	}
	if len(geoips) > 0 {
		compiled = append(compiled, map[string]interface{}{
			"geoip":    geoips,
			"outbound": outbound,
		})
	}
	return compiled
}

func xrayPanelOutbound(rule api.RouteRule) string {
	switch strings.ToLower(strings.TrimSpace(rule.Action)) {
	case "direct":
		return "direct"
	case "proxy":
		if rule.ActionValue != "" {
			return rule.ActionValue
		}
		return "direct"
	default:
		return "blocked"
	}
}

func singBoxPanelOutbound(rule api.RouteRule) string {
	switch strings.ToLower(strings.TrimSpace(rule.Action)) {
	case "direct":
		return "direct"
	case "proxy":
		if rule.ActionValue != "" {
			return rule.ActionValue
		}
		return "direct"
	case "dns":
		if rule.ActionValue != "" {
			return rule.ActionValue
		}
		return "direct"
	default:
		return "block"
	}
}
