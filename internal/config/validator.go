package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"vasmax/internal/security"
)

// validLogLevels defines the allowed log level values.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

var nginxDurationRegex = regexp.MustCompile(`^[1-9][0-9]*(ms|s|m|h|d)$`)

// Validate checks the Config for completeness and correctness.
// In standalone mode only local fields are validated.
// In managed mode api_host, api_token, and node_id are additionally required.
func (c *Config) Validate() error {
	var errs []string

	// 1. Always validate log level.
	if !validLogLevels[c.Log.Level] {
		errs = append(errs, fmt.Sprintf("log.level: must be one of debug/info/warn/error, got %q", c.Log.Level))
	}

	// 2. Managed mode: validate xboard API fields.
	if !c.Standalone {
		if c.APIHost == "" {
			errs = append(errs, "api_host: must not be empty in managed mode")
		} else if err := security.ValidateHTTPURL(c.APIHost); err != nil {
			errs = append(errs, fmt.Sprintf("api_host: %v", err))
		}

		if c.APIToken == "" {
			errs = append(errs, "api_token: must not be empty in managed mode")
		}

		if c.APIPrefix != "" {
			if err := security.ValidateAPIPrefix(c.APIPrefix); err != nil {
				errs = append(errs, fmt.Sprintf("api_prefix: %v", err))
			}
		}

		if c.NodeID <= 0 {
			errs = append(errs, fmt.Sprintf("node_id: must be > 0 in managed mode, got %d", c.NodeID))
		}
	}

	// 3. Validate TLS domain if set.
	if c.TLS.Domain != "" {
		if err := security.ValidateDomain(c.TLS.Domain); err != nil {
			errs = append(errs, fmt.Sprintf("tls.domain: %v", err))
		}
	}

	// 4. Validate Hysteria2 port if set.
	if c.Hysteria2.Port != 0 {
		if err := security.ValidatePort(c.Hysteria2.Port); err != nil {
			errs = append(errs, fmt.Sprintf("hysteria2.port: %v", err))
		}
	}

	// 5. Validate Tuic port if set.
	if c.Tuic.Port != 0 {
		if err := security.ValidatePort(c.Tuic.Port); err != nil {
			errs = append(errs, fmt.Sprintf("tuic.port: %v", err))
		}
	}

	// 6. Validate CDN address if CDN is enabled.
	if c.CDN.Enabled && c.CDN.Address == "" {
		errs = append(errs, "cdn.address: must not be empty when cdn is enabled")
	}

	// 7. Validate ExtraPorts entries.
	for i, ep := range c.ExtraPorts {
		if ep.Port < 1 || ep.Port > 65535 {
			errs = append(errs, fmt.Sprintf("extra_ports[%d].port: must be 1-65535, got %d", i, ep.Port))
		}
		switch ep.Protocol {
		case "tcp", "udp", "both", "":
			// valid
		default:
			errs = append(errs, fmt.Sprintf("extra_ports[%d].protocol: must be tcp/udp/both, got %q", i, ep.Protocol))
		}
	}

	// 8. Validate NodeType if set.
	if c.NodeType != "" {
		validNodeTypes := map[string]bool{
			"v2ray": true, "vmess": true, "vless": true,
			"trojan": true, "shadowsocks": true,
			"hysteria": true, "hysteria2": true,
			"tuic": true, "anytls": true, "naive": true,
		}
		if !validNodeTypes[c.NodeType] {
			errs = append(errs, fmt.Sprintf("node_type: must be one of v2ray/vmess/vless/trojan/shadowsocks/hysteria/hysteria2/tuic/anytls/naive, got %q", c.NodeType))
		}
	}

	// 9. Validate CoreType if set.
	if c.CoreType != "" {
		validCoreTypes := map[string]bool{"xray": true, "singbox": true, "dual": true}
		if !validCoreTypes[c.CoreType] {
			errs = append(errs, fmt.Sprintf("core_type: must be xray, singbox or dual, got %q", c.CoreType))
		}
	}

	// 10. Validate ALPN mode if set.
	if c.ALPN.Mode != "" {
		validALPN := map[string]bool{"h2_http11": true, "h2_only": true, "http11_only": true, "h3_only": true, "all": true}
		if !validALPN[c.ALPN.Mode] {
			errs = append(errs, fmt.Sprintf("alpn.mode: must be one of h2_http11/h2_only/http11_only/h3_only/all, got %q", c.ALPN.Mode))
		}
	}

	// 10.5 Validate per-protocol install metadata.
	for protoName, mode := range c.ProtocolModes {
		switch mode {
		case "domain", "nodomain", "":
		default:
			errs = append(errs, fmt.Sprintf("protocol_modes[%s]: must be domain or nodomain, got %q", protoName, mode))
		}
	}
	for protoName, port := range c.ProtocolPorts {
		if err := security.ValidatePort(port); err != nil {
			errs = append(errs, fmt.Sprintf("protocol_ports[%s]: %v", protoName, err))
		}
	}
	for protoName, domain := range c.ProtocolDomains {
		if strings.TrimSpace(domain) == "" {
			continue
		}
		if err := security.ValidateDomain(domain); err != nil {
			errs = append(errs, fmt.Sprintf("protocol_domains[%s]: %v", protoName, err))
		}
	}

	// 11. Validate TLS version range if set.
	if c.TLS.MinVersion != "" || c.TLS.MaxVersion != "" {
		tlsVersionOrder := map[string]int{"1.0": 0, "1.1": 1, "1.2": 2, "1.3": 3}
		validTLSVersions := map[string]bool{"1.0": true, "1.1": true, "1.2": true, "1.3": true}
		if c.TLS.MinVersion != "" && !validTLSVersions[c.TLS.MinVersion] {
			errs = append(errs, fmt.Sprintf("tls.min_version: must be one of 1.0/1.1/1.2/1.3, got %q", c.TLS.MinVersion))
		}
		if c.TLS.MaxVersion != "" && !validTLSVersions[c.TLS.MaxVersion] {
			errs = append(errs, fmt.Sprintf("tls.max_version: must be one of 1.0/1.1/1.2/1.3, got %q", c.TLS.MaxVersion))
		}
		if c.TLS.MinVersion != "" && c.TLS.MaxVersion != "" {
			minOrd, minOk := tlsVersionOrder[c.TLS.MinVersion]
			maxOrd, maxOk := tlsVersionOrder[c.TLS.MaxVersion]
			if minOk && maxOk && minOrd > maxOrd {
				errs = append(errs, fmt.Sprintf("tls.min_version %q must not be greater than tls.max_version %q", c.TLS.MinVersion, c.TLS.MaxVersion))
			}
		}
	}

	// 12. Validate subscription DNS mode if set.
	if c.Subscription.DNSMode != "" {
		validDNSModes := map[string]bool{"auto": true, "cn": true, "global": true, "privacy": true, "custom": true}
		if !validDNSModes[c.Subscription.DNSMode] {
			errs = append(errs, fmt.Sprintf("subscription.dns_mode: must be one of auto/cn/global/privacy/custom, got %q", c.Subscription.DNSMode))
		}
		if c.Subscription.DNSMode == "custom" && !hasNonEmptyValue(c.Subscription.DNSCustom) {
			errs = append(errs, "subscription.dns_custom: must not be empty when subscription.dns_mode is custom")
		}
	}
	if c.Subscription.TestURL != "" {
		if err := security.ValidateURL(c.Subscription.TestURL); err != nil {
			errs = append(errs, fmt.Sprintf("subscription.test_url: %v", err))
		}
	}
	if c.Subscription.ServerIP != "" && net.ParseIP(strings.TrimSpace(c.Subscription.ServerIP)) == nil {
		errs = append(errs, fmt.Sprintf("subscription.server_ip: must be a valid IP address, got %q", c.Subscription.ServerIP))
	}

	// 12.5 Validate Reality settings if present.
	if c.Reality.Port != 0 {
		if err := security.ValidatePort(c.Reality.Port); err != nil {
			errs = append(errs, fmt.Sprintf("reality.port: %v", err))
		}
	}
	if c.Reality.ServerName != "" {
		if err := security.ValidateDomain(c.Reality.ServerName); err != nil {
			errs = append(errs, fmt.Sprintf("reality.server_name: %v", err))
		}
	}
	if c.Reality.Dest != "" {
		if _, _, err := security.NormalizeRealityDest(c.Reality.Dest); err != nil {
			errs = append(errs, fmt.Sprintf("reality.dest: %v", err))
		}
	}
	for i, target := range c.Reality.Targets {
		if target.ServerName != "" {
			if err := security.ValidateDomain(target.ServerName); err != nil {
				errs = append(errs, fmt.Sprintf("reality.targets[%d].server_name: %v", i, err))
			}
		}
		if target.Dest != "" {
			if _, _, err := security.NormalizeRealityDest(target.Dest); err != nil {
				errs = append(errs, fmt.Sprintf("reality.targets[%d].dest: %v", i, err))
			}
		}
		if target.Port != 0 {
			if err := security.ValidatePort(target.Port); err != nil {
				errs = append(errs, fmt.Sprintf("reality.targets[%d].port: %v", i, err))
			}
		}
	}

	// 13. Validate server-side core DNS settings if set.
	if c.ServerDNS.Mode != "" {
		validServerDNSModes := map[string]bool{
			ServerDNSModeSystem: true, ServerDNSModeCloudflare: true, ServerDNSModeQuad9: true,
			ServerDNSModeGoogle: true, ServerDNSModeCustom: true,
		}
		if !validServerDNSModes[c.ServerDNS.Mode] {
			errs = append(errs, fmt.Sprintf("server_dns.mode: must be one of system/cloudflare/quad9/google/custom, got %q", c.ServerDNS.Mode))
		}
		if c.ServerDNS.Mode == ServerDNSModeCustom && len(NormalizeServerDNSServers(c.ServerDNS.Servers)) == 0 {
			errs = append(errs, "server_dns.servers: custom mode requires at least one plain DNS IP")
		}
	}
	if c.ServerDNS.Strategy != "" {
		validServerDNSStrategies := map[string]bool{"ipv4_only": true, "prefer_ipv4": true, "prefer_ipv6": true, "ipv6_only": true}
		if !validServerDNSStrategies[c.ServerDNS.Strategy] {
			errs = append(errs, fmt.Sprintf("server_dns.strategy: must be one of ipv4_only/prefer_ipv4/prefer_ipv6/ipv6_only, got %q", c.ServerDNS.Strategy))
		}
	}

	// 14. Validate generated Nginx timeout if set.
	if c.Nginx.LongConnectionTimeout != "" && !nginxDurationRegex.MatchString(c.Nginx.LongConnectionTimeout) {
		errs = append(errs, fmt.Sprintf("nginx.long_connection_timeout: must be a positive duration with unit ms/s/m/h/d, got %q", c.Nginx.LongConnectionTimeout))
	}

	// 15. Validate managed-mode sync safety settings.
	if c.Sync.EmptyUsersApplyThreshold < -1 {
		errs = append(errs, fmt.Sprintf("sync.empty_users_apply_threshold: must be -1 or >= 0, got %d", c.Sync.EmptyUsersApplyThreshold))
	}
	if c.Sync.MinPullIntervalSeconds < 0 {
		errs = append(errs, fmt.Sprintf("sync.min_pull_interval_seconds: must be >= 0, got %d", c.Sync.MinPullIntervalSeconds))
	}
	if c.Sync.MinPushIntervalSeconds < 0 {
		errs = append(errs, fmt.Sprintf("sync.min_push_interval_seconds: must be >= 0, got %d", c.Sync.MinPushIntervalSeconds))
	}

	// 16. Validate low-level connection keepalive settings.
	if c.Connection.KeepAliveMode != "" {
		validKeepAliveModes := map[string]bool{"auto": true, "off": true}
		if !validKeepAliveModes[c.Connection.KeepAliveMode] {
			errs = append(errs, fmt.Sprintf("connection.keepalive_mode: must be auto or off, got %q", c.Connection.KeepAliveMode))
		}
	}
	if c.Connection.KeepAliveIdleSeconds < 0 {
		errs = append(errs, fmt.Sprintf("connection.keepalive_idle_seconds: must be >= 0, got %d", c.Connection.KeepAliveIdleSeconds))
	}
	if c.Connection.KeepAliveIntervalSeconds < 0 {
		errs = append(errs, fmt.Sprintf("connection.keepalive_interval_seconds: must be >= 0, got %d", c.Connection.KeepAliveIntervalSeconds))
	}
	if c.Connection.KeepAliveProbes < 0 {
		errs = append(errs, fmt.Sprintf("connection.keepalive_probes: must be >= 0, got %d", c.Connection.KeepAliveProbes))
	}
	if c.Connection.WebSocketHeartbeatSeconds < 0 {
		errs = append(errs, fmt.Sprintf("connection.websocket_heartbeat_seconds: must be >= 0, got %d", c.Connection.WebSocketHeartbeatSeconds))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

func hasNonEmptyValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
