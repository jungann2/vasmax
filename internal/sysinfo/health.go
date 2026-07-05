package sysinfo

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"vasmax/internal/bbr"
	"vasmax/internal/config"
	"vasmax/internal/protocol"
	"vasmax/internal/security"
)

// HealthResult 健康检查结果
type HealthResult struct {
	Components []ComponentHealth `json:"components"`
	Overall    string            `json:"overall"` // healthy/unhealthy/warning
}

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // healthy/unhealthy/warning
	Details string `json:"details"`
}

// RunHealthCheck 执行全面健康检查
// 返回 0=healthy, 1=unhealthy, 2=warning
func RunHealthCheck(configPath string) int {
	result := &HealthResult{Overall: "healthy"}

	cfg, cfgErr := config.LoadConfig(configPath)
	if cfgErr != nil {
		result.addCheck(ComponentHealth{
			Name:    "配置文件",
			Status:  "warning",
			Details: fmt.Sprintf("无法读取配置，core 进程检查将跳过: %v", cfgErr),
		})
	}

	if cfg != nil {
		needed := neededHealthProcesses(cfg)
		if needed["xray"] {
			result.addCheck(checkProcess("xray", "Xray-core"))
		}
		if needed["singbox"] {
			result.addCheck(checkProcess("sing-box", "sing-box"))
		}
		if needed["nginx"] {
			result.addCheck(checkProcess("nginx", "Nginx"))
		}
		result.addCheck(checkProtocolListenPorts(cfg))
	}
	// 检查磁盘空间
	result.addCheck(checkDisk("/"))
	// 检查 BBR + FQ 状态
	result.addCheck(checkBBRFQ())
	// 检查 TLS 证书
	if cfg != nil && cfg.TLS.CertFile != "" {
		result.addCheck(checkCertExpiry(cfg.TLS.CertFile))
	}
	// 检查服务端 DNS 和 IPv6 状态
	if cfg != nil {
		result.addCheck(checkServerDNS(cfg))
		result.addCheck(checkIPv6(cfg))
		if hasNoDomainProtocol(cfg) {
			result.addCheck(checkSubscriptionServerIP(cfg))
		}
		if cfg.Reality.ServerName != "" || len(cfg.Reality.Targets) > 0 {
			result.addCheck(checkRealityTLS(cfg))
		}
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	if result.Overall == "unhealthy" {
		return 1
	}
	if result.Overall == "warning" {
		return 2
	}
	return 0
}

func checkProtocolListenPorts(cfg *config.Config) ComponentHealth {
	if cfg == nil || len(cfg.Protocols) == 0 {
		return ComponentHealth{Name: "协议监听端口", Status: "healthy", Details: "未安装协议"}
	}
	listeners, err := listeningPorts()
	if err != nil {
		return ComponentHealth{Name: "协议监听端口", Status: "warning", Details: fmt.Sprintf("无法读取监听端口: %v", err)}
	}

	reg := protocol.DefaultRegistry()
	var okParts []string
	var missingParts []string
	for _, protoName := range cfg.Protocols {
		p, ok := reg.Get(protoName)
		if !ok {
			continue
		}
		network := expectedListenNetwork(p)
		for _, port := range expectedListenPorts(cfg, p) {
			key := fmt.Sprintf("%s/%d", network, port)
			if listeners[key] {
				if network == "tcp" {
					if err := checkTCP4Loopback(port); err != nil {
						missingParts = append(missingParts, fmt.Sprintf("%s:%s tcp4不可连:%v", protoName, key, err))
						continue
					}
				}
				okParts = append(okParts, fmt.Sprintf("%s:%s", protoName, key))
			} else {
				missingParts = append(missingParts, fmt.Sprintf("%s:%s", protoName, key))
			}
		}
	}
	if len(okParts) == 0 && len(missingParts) == 0 {
		return ComponentHealth{Name: "协议监听端口", Status: "warning", Details: "没有可识别的协议端口"}
	}
	detail := "正常: " + strings.Join(okParts, ", ")
	if len(missingParts) > 0 {
		detail += "；未监听: " + strings.Join(missingParts, ", ")
		return ComponentHealth{Name: "协议监听端口", Status: "unhealthy", Details: detail}
	}
	return ComponentHealth{Name: "协议监听端口", Status: "healthy", Details: detail}
}

func checkTCP4Loopback(port int) error {
	conn, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)), 500*time.Millisecond)
	if conn != nil {
		_ = conn.Close()
	}
	return err
}

func expectedListenPorts(cfg *config.Config, p protocol.Protocol) []int {
	basePort := protocol.EffectiveInboundPort(p, cfg)
	if p.Name() == "vless_reality_vision" && len(cfg.Reality.Targets) > 0 {
		targets := cfg.Reality.EffectiveTargets(basePort)
		ports := make([]int, 0, len(targets))
		for _, target := range targets {
			ports = append(ports, target.Port)
		}
		return ports
	}
	return []int{basePort}
}

func expectedListenNetwork(p protocol.Protocol) string {
	switch p.Name() {
	case "hysteria2", "tuic":
		return "udp"
	default:
		return "tcp"
	}
}

func listeningPorts() (map[string]bool, error) {
	output, err := exec.Command("ss", "-lntu").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ss -lntu failed: %w", err)
	}
	result := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		network := fields[0]
		if network != "tcp" && network != "udp" {
			continue
		}
		port, ok := parseListenPort(fields[4])
		if !ok {
			continue
		}
		result[fmt.Sprintf("%s/%d", network, port)] = true
	}
	return result, nil
}

func parseListenPort(addr string) (int, bool) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return 0, false
	}
	if i := strings.LastIndex(addr, ":"); i >= 0 && i+1 < len(addr) {
		addr = addr[i+1:]
	}
	addr = strings.Trim(addr, "[]")
	var port int
	if _, err := fmt.Sscanf(addr, "%d", &port); err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

func neededHealthProcesses(cfg *config.Config) map[string]bool {
	needed := map[string]bool{}
	if cfg == nil {
		return needed
	}
	reg := protocol.DefaultRegistry()
	for _, protoName := range cfg.Protocols {
		p, ok := reg.Get(protoName)
		if !ok {
			continue
		}
		switch p.CoreType() {
		case "xray":
			needed["xray"] = true
		case "singbox":
			needed["singbox"] = true
		}
		if protocol.NeedsNginxProxy(p) {
			needed["nginx"] = true
		}
	}
	if cfg.Subscription.Domain != "" {
		needed["nginx"] = true
	}
	return needed
}

func checkBBRFQ() ComponentHealth {
	status := bbr.RecommendedRuntimeStatus()
	detail := fmt.Sprintf("sysctl=%s + %s", status.CC, status.DefaultQdisc)
	if status.DefaultInterface != "" {
		detail += fmt.Sprintf("，默认网卡=%s", status.DefaultInterface)
	}
	if status.DeviceQdisc != "" {
		detail += fmt.Sprintf("，网卡qdisc=%s", status.DeviceQdisc)
	}
	if status.RecommendedRuntime {
		return ComponentHealth{Name: "BBR + FQ", Status: "healthy", Details: detail + "，推荐组合已启用"}
	}
	if status.RecommendedSysctl && status.DeviceQdiscError != "" {
		return ComponentHealth{Name: "BBR + FQ", Status: "warning", Details: detail + "，sysctl 已启用推荐组合，但无法读取网卡 qdisc: " + status.DeviceQdiscError}
	}
	if status.RecommendedSysctl {
		return ComponentHealth{Name: "BBR + FQ", Status: "warning", Details: detail + "，sysctl 已启用推荐组合，但网卡 qdisc 不是 fq"}
	}
	if bbr.IsBBRCC(status.CC) {
		return ComponentHealth{Name: "BBR + FQ", Status: "warning", Details: detail + "，BBR 已启用但 qdisc 不是 fq"}
	}
	for _, available := range bbr.AvailableCC() {
		if available == "bbr" {
			return ComponentHealth{Name: "BBR + FQ", Status: "warning", Details: detail + "，内核支持 bbr 但未启用推荐组合"}
		}
	}
	return ComponentHealth{Name: "BBR + FQ", Status: "warning", Details: detail + "，当前内核未暴露 bbr 支持"}
}

func checkServerDNS(cfg *config.Config) ComponentHealth {
	dnsCfg := cfg.ServerDNS
	mode := dnsCfg.EffectiveMode()
	servers := dnsCfg.EffectiveServers()

	start := time.Now()
	var ips []net.IPAddr
	var err error
	target := "www.gstatic.com"
	if mode == config.ServerDNSModeSystem {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		ips, err = net.DefaultResolver.LookupIPAddr(ctx, target)
		cancel()
	} else {
		return checkExplicitServerDNS(dnsCfg, target, servers)
	}
	elapsed := time.Since(start)
	if err != nil {
		return ComponentHealth{
			Name:    "服务端 DNS",
			Status:  "unhealthy",
			Details: fmt.Sprintf("mode=%s 解析 %s 失败: %v", mode, target, err),
		}
	}
	if len(ips) == 0 {
		return ComponentHealth{
			Name:    "服务端 DNS",
			Status:  "warning",
			Details: fmt.Sprintf("mode=%s 解析 %s 无结果", mode, target),
		}
	}
	detail := fmt.Sprintf("mode=%s strategy=%s 解析 %s 成功，耗时 %dms，结果 %d 条",
		mode, dnsCfg.EffectiveStrategy(), target, elapsed.Milliseconds(), len(ips))
	if elapsed > 1500*time.Millisecond {
		return ComponentHealth{Name: "服务端 DNS", Status: "warning", Details: detail + "，解析偏慢"}
	}
	return ComponentHealth{Name: "服务端 DNS", Status: "healthy", Details: detail}
}

func checkExplicitServerDNS(dnsCfg config.ServerDNSConfig, target string, servers []string) ComponentHealth {
	mode := dnsCfg.EffectiveMode()
	if len(servers) == 0 {
		return ComponentHealth{
			Name:    "服务端 DNS",
			Status:  "warning",
			Details: fmt.Sprintf("mode=%s 但没有可用 DNS 服务器", mode),
		}
	}
	var okParts []string
	var failParts []string
	for _, server := range servers {
		start := time.Now()
		ips, err := lookupWithDNSServer(target, server, 3*time.Second)
		elapsed := time.Since(start)
		if err != nil {
			failParts = append(failParts, fmt.Sprintf("%s: %v", server, err))
			continue
		}
		if len(ips) == 0 {
			failParts = append(failParts, fmt.Sprintf("%s: 无结果", server))
			continue
		}
		okParts = append(okParts, fmt.Sprintf("%s=%dms/%d条", server, elapsed.Milliseconds(), len(ips)))
	}
	detail := fmt.Sprintf("mode=%s strategy=%s %s；成功: %s",
		mode, dnsCfg.EffectiveStrategy(), target, strings.Join(okParts, ", "))
	if len(failParts) > 0 {
		detail += "；失败: " + strings.Join(failParts, "；")
	}
	switch {
	case len(okParts) == len(servers):
		return ComponentHealth{Name: "服务端 DNS", Status: "healthy", Details: detail}
	case len(okParts) > 0:
		return ComponentHealth{Name: "服务端 DNS", Status: "warning", Details: detail}
	default:
		return ComponentHealth{Name: "服务端 DNS", Status: "unhealthy", Details: detail}
	}
}

func checkSubscriptionServerIP(cfg *config.Config) ComponentHealth {
	if cfg == nil {
		return ComponentHealth{Name: "订阅公网 IP", Status: "warning", Details: "配置为空，无法检查"}
	}
	configured := strings.TrimSpace(cfg.Subscription.ServerIP)
	if configured != "" {
		if net.ParseIP(configured) == nil {
			return ComponentHealth{
				Name:    "订阅公网 IP",
				Status:  "unhealthy",
				Details: fmt.Sprintf("subscription.server_ip 无效: %s", configured),
			}
		}
		return ComponentHealth{Name: "订阅公网 IP", Status: "healthy", Details: fmt.Sprintf("使用手动配置公网 IP: %s", configured)}
	}
	ip, err := detectPublicIPForHealth()
	if err != nil {
		return ComponentHealth{Name: "订阅公网 IP", Status: "unhealthy", Details: fmt.Sprintf("自动探测公网 IP 失败: %v；请设置 subscription.server_ip", err)}
	}
	return ComponentHealth{Name: "订阅公网 IP", Status: "healthy", Details: fmt.Sprintf("自动探测公网 IP: %s", ip)}
}

func lookupWithDNSServer(host, server string, timeout time.Duration) ([]net.IPAddr, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return resolver.LookupIPAddr(ctx, host)
}

func checkIPv6(cfg *config.Config) ComponentHealth {
	target := cfg.Reality.ServerName
	if target == "" {
		target = cfg.TLS.Domain
	}
	if target == "" {
		target = "www.google.com"
	}

	hasAAAA := false
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	ips, lookupErr := net.DefaultResolver.LookupIP(ctx, "ip", target)
	cancel()
	for _, ip := range ips {
		if ip.To4() == nil && ip.To16() != nil {
			hasAAAA = true
			break
		}
	}

	conn, dialErr := net.DialTimeout("udp6", "[2606:4700:4700::1111]:53", 2*time.Second)
	if conn != nil {
		conn.Close()
	}
	return ipv6HealthFromProbe(cfg.ServerDNS.EffectiveStrategy(), target, hasAAAA, lookupErr, dialErr)
}

func ipv6HealthFromProbe(strategy, target string, hasAAAA bool, lookupErr, dialErr error) ComponentHealth {
	if dialErr != nil {
		detail := fmt.Sprintf("IPv6 出站不可用: %v", dialErr)
		switch config.NormalizeServerDNSStrategy(strategy) {
		case "ipv6_only":
			return ComponentHealth{
				Name:    "IPv6 连通性",
				Status:  "unhealthy",
				Details: detail + "；当前 server_dns.strategy=ipv6_only，代理核心可能无法连接 IPv6 目标",
			}
		case "prefer_ipv6":
			return ComponentHealth{
				Name:    "IPv6 连通性",
				Status:  "warning",
				Details: detail + "；当前 server_dns.strategy=prefer_ipv6，建议改为 ipv4_only 或 prefer_ipv4",
			}
		}
		if lookupErr == nil && hasAAAA {
			return ComponentHealth{
				Name:    "IPv6 连通性",
				Status:  "warning",
				Details: detail + fmt.Sprintf("；%s 返回 AAAA，建议 server_dns.strategy 使用 ipv4_only 或关闭系统 IPv6", target),
			}
		}
		return ComponentHealth{Name: "IPv6 连通性", Status: "healthy", Details: detail + "；未发现需要 IPv6 的解析结果"}
	}
	if lookupErr != nil {
		return ComponentHealth{Name: "IPv6 连通性", Status: "warning", Details: fmt.Sprintf("IPv6 出站可用，但解析 %s 失败: %v", target, lookupErr)}
	}
	return ComponentHealth{Name: "IPv6 连通性", Status: "healthy", Details: fmt.Sprintf("IPv6 出站可用；%s AAAA=%t", target, hasAAAA)}
}

func checkRealityTLS(cfg *config.Config) ComponentHealth {
	targets := realityHealthTargets(cfg)
	if len(targets) == 0 {
		return ComponentHealth{Name: "Reality 伪站 TLS", Status: "warning", Details: "未配置 Reality 伪站目标"}
	}
	var okParts []string
	var warnParts []string
	var failParts []string
	for _, target := range targets {
		probe := security.ProbeRealityDest(target, 5*time.Second)
		detail := fmt.Sprintf("%s %dms TLS1.3=%t H2=%t", target, probe.Latency.Milliseconds(), probe.SupportsTLS13, probe.SupportsH2)
		if probe.Error != "" {
			failParts = append(failParts, fmt.Sprintf("%s: %s", target, probe.Error))
			continue
		}
		if len(probe.Warnings) > 0 || !probe.Available() {
			warnParts = append(warnParts, detail+" "+strings.Join(probe.Warnings, "；"))
			continue
		}
		okParts = append(okParts, detail)
	}
	parts := []string{}
	if len(okParts) > 0 {
		parts = append(parts, "正常: "+strings.Join(okParts, "；"))
	}
	if len(warnParts) > 0 {
		parts = append(parts, "告警: "+strings.Join(warnParts, "；"))
	}
	if len(failParts) > 0 {
		parts = append(parts, "失败: "+strings.Join(failParts, "；"))
	}
	switch {
	case len(failParts) > 0:
		return ComponentHealth{Name: "Reality 伪站 TLS", Status: "unhealthy", Details: strings.Join(parts, "；")}
	case len(warnParts) > 0:
		return ComponentHealth{Name: "Reality 伪站 TLS", Status: "warning", Details: strings.Join(parts, "；")}
	default:
		return ComponentHealth{Name: "Reality 伪站 TLS", Status: "healthy", Details: strings.Join(parts, "；")}
	}
}

func realityHealthTargets(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	seen := make(map[string]bool)
	var targets []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		targets = append(targets, name)
	}
	if len(cfg.Reality.Targets) > 0 {
		basePort := cfg.Reality.EffectivePort()
		for _, target := range cfg.Reality.EffectiveTargets(basePort) {
			if target.Dest != "" {
				add(target.Dest)
			} else {
				add(target.ServerName)
			}
		}
		return targets
	}
	if cfg.Reality.Dest != "" {
		add(cfg.Reality.Dest)
	} else {
		add(cfg.Reality.ServerName)
	}
	return targets
}

func hasNoDomainProtocol(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	for _, protoName := range cfg.Protocols {
		if config.EffectiveProtocolMode(cfg, protoName) == "nodomain" {
			return true
		}
	}
	return false
}

func detectPublicIPForHealth() (string, error) {
	apis := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, api := range apis {
		resp, err := client.Get(api)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) != nil {
			return ip, nil
		}
	}
	return "", fmt.Errorf("all public IP APIs failed")
}

func (r *HealthResult) addCheck(c ComponentHealth) {
	r.Components = append(r.Components, c)
	if c.Status == "unhealthy" {
		r.Overall = "unhealthy"
	} else if c.Status == "warning" && r.Overall == "healthy" {
		r.Overall = "warning"
	}
}

// checkProcess 检查进程是否运行
func checkProcess(name, displayName string) ComponentHealth {
	cmd := exec.Command("pgrep", "-x", name)
	if err := cmd.Run(); err != nil {
		return ComponentHealth{
			Name:    displayName,
			Status:  "unhealthy",
			Details: "进程未运行",
		}
	}
	return ComponentHealth{
		Name:    displayName,
		Status:  "healthy",
		Details: "进程运行中",
	}
}

// checkDisk 检查磁盘空间
func checkDisk(path string) ComponentHealth {
	err := CheckDiskSpace(path, 100) // 至少 100MB
	if err != nil {
		return ComponentHealth{
			Name:    "磁盘空间",
			Status:  "warning",
			Details: err.Error(),
		}
	}
	return ComponentHealth{
		Name:    "磁盘空间",
		Status:  "healthy",
		Details: "磁盘空间充足",
	}
}

// checkCertExpiry 检查 TLS 证书有效期
func checkCertExpiry(certPath string) ComponentHealth {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: fmt.Sprintf("无法读取证书: %v", err),
		}
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: "证书格式无效",
		}
	}

	// 手动解析第一个证书获取过期时间
	block, _ := pem.Decode(data)
	if block == nil {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: "无法解码 PEM 块",
		}
	}

	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "warning",
			Details: fmt.Sprintf("无法解析证书: %v", err),
		}
	}

	daysLeft := int(time.Until(parsed.NotAfter).Hours() / 24)
	if daysLeft <= 0 {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: "证书已过期",
		}
	}
	if daysLeft <= 7 {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "warning",
			Details: fmt.Sprintf("证书将在 %d 天后过期", daysLeft),
		}
	}
	return ComponentHealth{
		Name:    "TLS 证书",
		Status:  "healthy",
		Details: fmt.Sprintf("证书有效，剩余 %d 天", daysLeft),
	}
}

// FormatHealthResult 格式化健康检查结果为彩色文本
func FormatHealthResult(result *HealthResult) string {
	var sb strings.Builder
	sb.WriteString("=== 健康检查结果 ===\n")
	for _, c := range result.Components {
		var icon string
		switch c.Status {
		case "healthy":
			icon = "✓"
		case "warning":
			icon = "⚠"
		case "unhealthy":
			icon = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, c.Name, c.Details))
	}
	sb.WriteString(fmt.Sprintf("\n总体状态: %s\n", result.Overall))
	return sb.String()
}
