package security

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RealityValidation holds the validation result for a Reality dest domain.
type RealityValidation struct {
	IsCloudflare  bool
	SupportsTLS13 bool
	SupportsH2    bool
	Warnings      []string
}

// RealityProbeResult is the active probe result for one REALITY camouflage target.
type RealityProbeResult struct {
	ServerName    string
	Dest          string
	Latency       time.Duration
	IsCloudflare  bool
	SupportsTLS13 bool
	SupportsH2    bool
	Warnings      []string
	Error         string
}

// Available reports whether the target is suitable for automatic selection.
func (r RealityProbeResult) Available() bool {
	return r.Error == "" &&
		r.ServerName != "" &&
		!r.IsCloudflare &&
		r.SupportsTLS13 &&
		r.SupportsH2 &&
		!IsKnownProblematicRealityDest(r.ServerName)
}

const (
	// DefaultRealityServerName avoids Apple/iCloud/Microsoft-family targets that
	// have shown higher operational risk with REALITY deployments.
	DefaultRealityServerName = "www.nvidia.com"
	DefaultRealityDest       = DefaultRealityServerName + ":443"

	// DefaultRealityExtendedProbeSampleSize limits the random extended pool
	// probe size. The menu also probes the fixed recommended pool, so 12 keeps
	// one run around 20 domains without making the interactive menu feel stuck.
	DefaultRealityExtendedProbeSampleSize = 12
)

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
	probe := ProbeRealityDest(domain, 10*time.Second)
	if probe.ServerName == "" {
		return nil, fmt.Errorf("invalid domain: %s", probe.Error)
	}

	warnings := append([]string{}, probe.Warnings...)
	if probe.Error != "" {
		warnings = append(warnings, probe.Error)
	}
	return &RealityValidation{
		IsCloudflare:  probe.IsCloudflare,
		SupportsTLS13: probe.SupportsTLS13,
		SupportsH2:    probe.SupportsH2,
		Warnings:      warnings,
	}, nil
}

// NormalizeRealityDest normalizes a user supplied REALITY target into
// "host:port" Dest plus the SNI ServerName. REALITY camouflage targets are
// intentionally domain based, not IP based.
func NormalizeRealityDest(input string) (dest, serverName string, err error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", "", fmt.Errorf("target must not be empty")
	}
	if strings.Contains(raw, "://") {
		return "", "", fmt.Errorf("target must not include http:// or https://")
	}
	if strings.ContainsAny(raw, "/ \t\r\n") {
		return "", "", fmt.Errorf("target must be a domain or domain:port")
	}

	host := raw
	port := "443"
	if strings.Count(raw, ":") > 1 {
		return "", "", fmt.Errorf("target must be a domain or domain:port")
	}
	if strings.Contains(raw, ":") {
		parts := strings.Split(raw, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("target must be a domain or domain:port")
		}
		host = parts[0]
		port = parts[1]
	}
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if err := ValidateDomain(host); err != nil {
		return "", "", fmt.Errorf("invalid domain: %w", err)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return "", "", fmt.Errorf("invalid port: %s", port)
	}
	return fmt.Sprintf("%s:%d", host, portNum), host, nil
}

// ProbeRealityDests probes candidates concurrently and returns them sorted with
// suitable low-latency targets first.
func ProbeRealityDests(candidates []string, timeout time.Duration) []RealityProbeResult {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	normalized := normalizeRealityProbeCandidates(candidates)
	results := make([]RealityProbeResult, len(normalized))

	var wg sync.WaitGroup
	for i, candidate := range normalized {
		wg.Add(1)
		go func(i int, candidate string) {
			defer wg.Done()
			results[i] = ProbeRealityDest(candidate, timeout)
		}(i, candidate)
	}
	wg.Wait()

	sort.SliceStable(results, func(i, j int) bool {
		return realityProbeLess(results[i], results[j])
	})
	return results
}

// ProbeRealityDest actively checks DNS, TLS handshake, TLS 1.3 and H2 support.
func ProbeRealityDest(candidate string, timeout time.Duration) RealityProbeResult {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	dest, serverName, err := NormalizeRealityDest(candidate)
	result := RealityProbeResult{
		ServerName: serverName,
		Dest:       dest,
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Warnings = append(result.Warnings, knownRealityDestWarnings(serverName)...)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", serverName)
	cancel()
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

	start := time.Now()
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: timeout},
		"tcp", dest,
		&tls.Config{
			ServerName:         serverName,
			NextProtos:         []string{"h2", "http/1.1"},
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	)
	result.Latency = time.Since(start)
	if err != nil {
		result.Error = fmt.Sprintf("TLS connection failed: %v", err)
		return result
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

	return result
}

// BestRealityProbeResult returns the lowest latency target that passes the
// automatic selection rules.
func BestRealityProbeResult(results []RealityProbeResult) (*RealityProbeResult, bool) {
	best := -1
	for i := range results {
		if !results[i].Available() {
			continue
		}
		if best == -1 || results[i].Latency < results[best].Latency {
			best = i
		}
	}
	if best == -1 {
		return nil, false
	}
	return &results[best], true
}

func knownRealityDestWarnings(domain string) []string {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	warnings := make([]string, 0, 4)
	if containsAny(normalized, []string{"apple.com", "icloud.com", "itunes.apple.com", "mzstatic.com"}) {
		warnings = append(warnings,
			"Apple/iCloud targets are not recommended for REALITY because they are commonly scrutinized and may degrade or fail over time",
		)
	}
	if containsAny(normalized, []string{"microsoft.com", "s-microsoft.com", "bing.com", "xbox.com", "xboxlive.com", "gamepass.com", "msftauth.net", "office.net", "vsassets.io", "azure.microsoft.com"}) {
		warnings = append(warnings,
			"Microsoft/Bing targets are not recommended for REALITY because recent Xray builds or network filtering can cause handshake failures",
		)
	}
	if containsAny(normalized, []string{"cloudfront.net", "akamaihd.net", "awsstatic.com", "arc-cdn.net", "arcpublishing.com", "gtv-cdn.com", "lpsnmedia.net", "scene7.com"}) ||
		strings.Contains(normalized, ".cdn.") ||
		strings.HasPrefix(normalized, "cdn.") ||
		strings.HasPrefix(normalized, "cdn-") ||
		strings.HasPrefix(normalized, "cdn77.") ||
		strings.HasPrefix(normalized, "cdnssl.") {
		warnings = append(warnings,
			"CDN/static asset targets can change edge behavior frequently and are not used for automatic REALITY selection",
		)
	}
	if containsAny(normalized, []string{"adobedtm.com", "marketo.net", "optimizely.com", "demandbase.com", "bizible.com", "6sc.co", "trustarc.com", "oracleinfinity.io", "clicktale.net", "userway.org", "marsflag.com", "digitaleast.mobi", "cloud.coveo.com"}) {
		warnings = append(warnings,
			"marketing/telemetry targets are not stable enough for automatic REALITY selection",
		)
	}
	if strings.Contains(normalized, "company-target.com") {
		warnings = append(warnings,
			"placeholder/generated targets are not suitable for automatic REALITY selection",
		)
	}
	return warnings
}

func IsKnownProblematicRealityDest(domain string) bool {
	return len(knownRealityDestWarnings(domain)) > 0
}

// RecommendedRealityDests returns a list of recommended Reality dest domains.
func RecommendedRealityDests() []string {
	return []string{
		DefaultRealityServerName,
		"www.samsung.com",
		"www.tesla.com",
		"www.amazon.com",
		"www.mozilla.org",
		"www.paypal.com",
		"www.adobe.com",
		"www.oracle.com",
	}
}

// ExtendedRealityDests returns a larger candidate source for randomized probes.
// These domains are not all equally good: SampleExtendedRealityDests filters out
// known risky families before choosing the per-run sample.
func ExtendedRealityDests() []string {
	return []string{
		"a.b.cdn.console.awsstatic.com",
		"a0.awsstatic.com",
		"aadcdn.msftauth.net",
		"acctcdn.msftauth.net",
		"amd.com",
		"amp-api-edge.apps.apple.com",
		"api.company-target.com",
		"apps.mzstatic.com",
		"assets-www.xbox.com",
		"assets-xbxweb.xbox.com",
		"assets.adobedtm.com",
		"aws.amazon.com",
		"aws.com",
		"azure.microsoft.com",
		"b.6sc.co",
		"beacon.gtv-pub.com",
		"c.6sc.co",
		"c.s-microsoft.com",
		"catalog.gamepass.com",
		"cdn-dynmedia-1.microsoft.com",
		"cdn.bizible.com",
		"cdn.userway.org",
		"cdn77.api.userway.org",
		"cdnssl.clicktale.net",
		"ce.mf.marsflag.com",
		"consent.trustarc.com",
		"cua-chat-ui.tesla.com",
		"d.oracleinfinity.io",
		"d0.m.awsstatic.com",
		"d1.awsstatic.com",
		"d2c.aws.amazon.com",
		"d3agakyjgjv5i8.cloudfront.net",
		"devblogs.microsoft.com",
		"digitalassets.tesla.com",
		"download.amd.com",
		"downloaddispatch.itunes.apple.com",
		"downloadmirror.intel.com",
		"drivers.amd.com",
		"ds-aksb-a.akamaihd.net",
		"electronics.sony.com",
		"fpinit.itunes.apple.com",
		"github.gallerycdn.vsassets.io",
		"gray-config-prod.api.arc-cdn.net",
		"gray-config-prod.api.cdn.arcpublishing.com",
		"gray-wowt-prod.gtv-cdn.com",
		"gray.video-player.arcpublishing.com",
		"gsp-ssl.ls.apple.com",
		"i7158c100-ds-aksb-a.akamaihd.net",
		"images.nvidia.com",
		"intel.com",
		"iosapps.itunes.apple.com",
		"is1-ssl.mzstatic.com",
		"j.6sc.co",
		"location-services-prd.tesla.com",
		"logx.optimizely.com",
		"lpcdn.lpsnmedia.net",
		"ms-python.gallerycdn.vsassets.io",
		"ms-vscode.gallerycdn.vsassets.io",
		"munchkin.marketo.net",
		"prod.log.shortbread.aws.dev",
		"prod.pa.cdn.uis.awsstatic.com",
		"r.bing.com",
		"res-1.cdn.office.net",
		"rum.hlx.page",
		"s.company-target.com",
		"s.mp.marsflag.com",
		"s0.awsstatic.com",
		"s7mbrstream.scene7.com",
		"se-edge.itunes.apple.com",
		"services.digitaleast.mobi",
		"sisu.xboxlive.com",
		"static.cloud.coveo.com",
		"store-images.s-microsoft.com",
		"t0.m.awsstatic.com",
		"tag-logger.demandbase.com",
		"tag.demandbase.com",
		"th.bing.com",
		"ts1.tc.mm.bing.net",
		"ts3.tc.mm.bing.net",
		"ts4.tc.mm.bing.net",
		"www.aws.com",
		"www.intel.com",
		"www.nvidia.com",
		"www.oracle.com",
		"www.sony.com",
		"www.tesla.com",
		"www.wowt.com",
		"www.xbox.com",
		"www.xilinx.com",
	}
}

// SampleExtendedRealityDests returns a randomized subset from the extended
// source pool after removing known risky families.
func SampleExtendedRealityDests(limit int) []string {
	pool := normalizeRealityProbeCandidates(ExtendedRealityDests())
	filtered := make([]string, 0, len(pool))
	for _, candidate := range pool {
		_, serverName, err := NormalizeRealityDest(candidate)
		if err != nil || IsKnownProblematicRealityDest(serverName) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	if limit <= 0 || limit >= len(filtered) {
		return filtered
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(filtered), func(i, j int) {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	})
	return append([]string(nil), filtered[:limit]...)
}

// DefaultRealityTargetServerNames returns the default multi-target pool used by
// Reality Vision. Each target is generated as an independent inbound on its own
// port so clients can automatically fall back when one target breaks.
func DefaultRealityTargetServerNames() []string {
	return []string{
		"www.nvidia.com",
		"www.samsung.com",
		"www.tesla.com",
		"www.amazon.com",
		"www.mozilla.org",
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

func containsAny(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func normalizeRealityProbeCandidates(candidates []string) []string {
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		raw := strings.TrimSpace(candidate)
		if raw == "" {
			continue
		}
		key := strings.ToLower(raw)
		if dest, _, err := NormalizeRealityDest(raw); err == nil {
			key = dest
			raw = dest
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, raw)
	}
	return normalized
}

func realityProbeLess(a, b RealityProbeResult) bool {
	if a.Available() != b.Available() {
		return a.Available()
	}
	if (a.Error == "") != (b.Error == "") {
		return a.Error == ""
	}
	if len(a.Warnings) != len(b.Warnings) {
		return len(a.Warnings) < len(b.Warnings)
	}
	if a.Latency > 0 && b.Latency > 0 && a.Latency != b.Latency {
		return a.Latency < b.Latency
	}
	return a.ServerName < b.ServerName
}

// IsValidRealityDest is a convenience function that returns true if the domain
// passes all Reality dest checks (not Cloudflare, supports TLS 1.3 and H2).
func IsValidRealityDest(domain string) bool {
	v, err := ValidateRealityDest(domain)
	if err != nil {
		return false
	}
	return !v.IsCloudflare && v.SupportsTLS13 && v.SupportsH2
}
