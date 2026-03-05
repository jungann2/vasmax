package security

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

// RealityValidation holds the validation result for a Reality dest domain.
type RealityValidation struct {
	IsCloudflare  bool
	SupportsTLS13 bool
	SupportsH2    bool
	Warnings      []string
}

// CloudflareIPv4Prefixes is a subset of Cloudflare's IPv4 ranges for CDN detection.
var CloudflareIPv4Prefixes = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22",
	"103.31.4.0/22", "141.101.64.0/18", "108.162.192.0/18",
	"190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22",
	"198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
}

// ValidateRealityDest validates a Reality dest domain for security.
// Checks: 1. Cloudflare CDN  2. TLS 1.3 support  3. H2 support
func ValidateRealityDest(domain string) (*RealityValidation, error) {
	if err := ValidateDomain(domain); err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}

	result := &RealityValidation{}

	// Check Cloudflare CDN.
	ips, err := net.LookupIP(domain)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("DNS lookup failed: %v", err))
	} else {
		for _, ip := range ips {
			if isCloudflareIP(ip) {
				result.IsCloudflare = true
				result.Warnings = append(result.Warnings,
					"domain uses Cloudflare CDN proxy, not recommended for Reality dest")
				break
			}
		}
	}

	// Check TLS 1.3 and H2 support.
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp", domain+":443",
		&tls.Config{
			ServerName:         domain,
			NextProtos:         []string{"h2", "http/1.1"},
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("TLS connection failed: %v", err))
		return result, nil
	}
	defer conn.Close()

	state := conn.ConnectionState()
	result.SupportsTLS13 = state.Version == tls.VersionTLS13
	result.SupportsH2 = state.NegotiatedProtocol == "h2"

	if !result.SupportsTLS13 {
		result.Warnings = append(result.Warnings, "domain does not support TLS 1.3")
	}
	if !result.SupportsH2 {
		result.Warnings = append(result.Warnings, "domain does not support H2")
	}

	return result, nil
}

// RecommendedRealityDests returns a list of recommended Reality dest domains.
func RecommendedRealityDests() []string {
	return []string{
		"www.microsoft.com",
		"www.apple.com",
		"www.amazon.com",
		"www.samsung.com",
		"www.lovelive-anime.jp",
		"www.swift.org",
		"www.mozilla.org",
		"www.tesla.com",
		"www.nvidia.com",
	}
}

// isCloudflareIP checks if an IP belongs to Cloudflare's IP ranges.
func isCloudflareIP(ip net.IP) bool {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return false
	}
	for _, prefix := range CloudflareIPv4Prefixes {
		_, cidr, err := net.ParseCIDR(prefix)
		if err != nil {
			continue
		}
		if cidr.Contains(ipv4) {
			return true
		}
	}
	return false
}

// IsValidRealityDest is a convenience function that returns true if the domain
// passes all Reality dest checks (not Cloudflare, supports TLS 1.3 and H2).
func IsValidRealityDest(domain string) bool {
	// Strip port if present.
	host := domain
	if idx := strings.LastIndex(domain, ":"); idx != -1 {
		host = domain[:idx]
	}
	v, err := ValidateRealityDest(host)
	if err != nil {
		return false
	}
	return !v.IsCloudflare && v.SupportsTLS13 && v.SupportsH2
}
