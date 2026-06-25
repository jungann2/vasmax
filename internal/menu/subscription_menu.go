package menu

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/security"
	"vasmax/internal/subscription"
	"vasmax/internal/user"
)

// SubscriptionMenu handles subscription management.
type SubscriptionMenu struct {
	config  *config.Config
	subMgr  *subscription.Manager
	userMgr *user.Manager
	logger  *logrus.Logger
}

// NewSubscriptionMenu creates a new subscription menu.
func NewSubscriptionMenu(cfg *config.Config, subMgr *subscription.Manager, userMgr *user.Manager, logger *logrus.Logger) *SubscriptionMenu {
	return &SubscriptionMenu{config: cfg, subMgr: subMgr, userMgr: userMgr, logger: logger}
}

// Show displays the subscription management menu.
func (m *SubscriptionMenu) Show() {
	for {
		PrintTitle("订阅管理")
		subDomain := m.config.Subscription.Domain
		if subDomain == "" {
			subDomain = m.config.TLS.Domain
		}
		if subDomain != "" {
			PrintInfo(fmt.Sprintf("订阅域名: %s", subDomain))
		} else {
			PrintWarning("未配置订阅域名")
		}
		PrintSeparator()
		PrintInfo("订阅文件为静态生成，以下情况需手动执行「重新生成订阅」:")
		PrintInfo("  · 新增/删除用户后")
		PrintInfo("  · 安装/卸载协议后")
		PrintInfo("  · 修改域名、证书、CDN、Reality 配置后")
		PrintInfo("  · 添加/删除远程订阅后")
		PrintInfo("  · 修改订阅 DNS/测速配置后")
		PrintSeparator()
		PrintOption(1, "查看订阅链接")
		PrintOption(2, "重新生成订阅")
		PrintOption(3, "设置订阅域名")
		PrintOption(4, "远程订阅管理")
		PrintOption(5, "订阅 DNS/测速配置")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5"})
		switch choice {
		case "1":
			m.showLinks()
		case "2":
			m.regenerate()
		case "3":
			m.setDomain()
		case "4":
			m.remoteSubMenu()
		case "5":
			m.dnsSettingsMenu()
		case "0":
			return
		}
	}
}

func (m *SubscriptionMenu) showLinks() {
	PrintTitle("订阅链接")
	subDomain := m.config.Subscription.Domain
	if subDomain == "" {
		subDomain = m.config.TLS.Domain
	}
	if subDomain == "" {
		PrintWarning("未配置订阅域名，无法生成链接")
		return
	}

	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		PrintInfo("暂无用户")
		return
	}

	// 使用与 Manager 相同的 salt 逻辑：优先配置文件，否则从文件读取
	salt := m.config.Subscription.Salt
	if salt == "" {
		if s, err := subscription.LoadOrCreateSalt("/etc/vasmax"); err == nil {
			salt = s
		}
	}

	for _, u := range users {
		emailMd5 := subscription.GenerateSubscribePath(u.Email, salt)
		PrintInfo(fmt.Sprintf("用户: %s", u.Email))
		PrintInfo(fmt.Sprintf("  通用(v2ray/clash):  https://%s/s/%s/default", subDomain, emailMd5))
		PrintInfo(fmt.Sprintf("  Clash:              https://%s/s/%s/clash", subDomain, emailMd5))
		PrintInfo(fmt.Sprintf("  SingBox:            https://%s/s/%s/singbox", subDomain, emailMd5))
		fmt.Println()
	}
}

func (m *SubscriptionMenu) regenerate() {
	PrintInfo("正在重新生成订阅...")
	if err := m.subMgr.GenerateAll(); err != nil {
		PrintError(fmt.Sprintf("生成失败: %v", err))
		return
	}
	PrintSuccess("订阅已重新生成")
}

func (m *SubscriptionMenu) setDomain() {
	PrintTitle("设置订阅域名")
	PrintInfo(fmt.Sprintf("当前域名: %s", m.config.Subscription.Domain))
	PrintInfo("只填域名，不要带 http:// 或 https://；生成订阅链接时会自动使用 https://")
	PrintSuccess("  直接回车取消")
	domain := ReadInput("请输入新的订阅域名（如 sub.example.com）")
	if domain == "" {
		return
	}
	if err := security.ValidateDomain(domain); err != nil {
		PrintError(fmt.Sprintf("域名无效: %v", err))
		return
	}
	m.config.Subscription.Domain = domain
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存失败: %v", err))
		return
	}
	PrintSuccess(fmt.Sprintf("订阅域名已设置为: %s", domain))
}

func (m *SubscriptionMenu) remoteSubMenu() {
	baseDir := filepath.Dir(m.config.Paths.Subscribe)
	for {
		PrintTitle("远程订阅管理")
		PrintInfo("将其他 VasmaX 服务器（或任意 Base64 订阅链接）合并到本机订阅中")
		PrintInfo("用户拉取订阅时，远程节点会自动追加到本机节点后面")
		PrintSeparator()
		subs, _ := subscription.LoadRemoteSubscriptions(baseDir)
		if len(subs) == 0 {
			PrintInfo("暂无远程订阅")
		} else {
			for i, s := range subs {
				PrintOption(i+1, fmt.Sprintf("[%s]  %s", s.Alias, s.URL))
			}
		}
		PrintSeparator()
		PrintOption(len(subs)+1, "添加远程订阅")
		PrintOption(len(subs)+2, "删除远程订阅")
		PrintOptionStr("0", "返回")

		choices := []string{"0"}
		for i := 1; i <= len(subs)+2; i++ {
			choices = append(choices, fmt.Sprintf("%d", i))
		}
		choice := ReadChoice("请选择", choices)
		switch {
		case choice == "0":
			return
		case choice == fmt.Sprintf("%d", len(subs)+1):
			PrintInfo("请输入格式: 完整订阅 URL:别名")
			PrintInfo("示例: https://hk.example.com/s/abc123def456/default:香港节点")
			PrintInfo("说明: URL 必须带 http:// 或 https://；别名用于区分节点来源")
			input := ReadInput("请输入完整订阅 URL:别名")
			if input == "" {
				continue
			}
			sub, err := subscription.ParseRemoteSubscription(input)
			if err != nil {
				PrintError(fmt.Sprintf("格式错误: %v", err))
				continue
			}
			subs = append(subs, *sub)
			if err := subscription.SaveRemoteSubscriptions(baseDir, subs); err != nil {
				PrintError(fmt.Sprintf("保存失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("已添加: [%s] %s", sub.Alias, sub.URL))
			}
		case choice == fmt.Sprintf("%d", len(subs)+2):
			if len(subs) == 0 {
				PrintInfo("暂无远程订阅")
				continue
			}
			idx := ReadInput("请输入要删除的编号")
			var n int
			if _, err := fmt.Sscanf(idx, "%d", &n); err != nil || n < 1 || n > len(subs) {
				PrintError("无效编号")
				continue
			}
			alias := subs[n-1].Alias
			subs = append(subs[:n-1], subs[n:]...)
			if err := subscription.SaveRemoteSubscriptions(baseDir, subs); err != nil {
				PrintError(fmt.Sprintf("保存失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("已删除: %s", alias))
			}
		}
	}
}

func (m *SubscriptionMenu) dnsSettingsMenu() {
	for {
		PrintTitle("订阅 DNS/测速配置")
		PrintInfo("此处只影响 Clash 和 sing-box 客户端订阅，不改变服务器系统 DNS")
		PrintInfo(fmt.Sprintf("当前 DNS 模式: %s", currentDNSMode(m.config.Subscription.DNSMode)))
		PrintInfo(fmt.Sprintf("当前测速 URL: %s", currentTestURL(m.config.Subscription.TestURL)))
		if len(m.config.Subscription.DNSCustom) > 0 {
			PrintInfo(fmt.Sprintf("当前自定义 DNS: %s", strings.Join(m.config.Subscription.DNSCustom, ", ")))
		}
		PrintSeparator()
		PrintOption(1, "auto 自动模式（国内 DNS 直连，国外 DNS 走代理，推荐）")
		PrintOption(2, "cn 国内模式（全部使用国内 DNS）")
		PrintOption(3, "global 全球模式（全部使用海外 DNS）")
		PrintOption(4, "privacy 隐私模式（使用 Quad9/Cloudflare/AdGuard）")
		PrintOption(5, "custom 自定义模式")
		PrintOption(6, "设置自定义 DNS 服务器")
		PrintOption(7, "设置订阅测速 URL")
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5", "6", "7"})
		switch choice {
		case "1":
			m.saveSubscriptionDNSMode("auto")
		case "2":
			m.saveSubscriptionDNSMode("cn")
		case "3":
			m.saveSubscriptionDNSMode("global")
		case "4":
			m.saveSubscriptionDNSMode("privacy")
		case "5":
			if len(nonEmptyList(m.config.Subscription.DNSCustom)) == 0 {
				PrintWarning("custom 模式需要先设置至少一个自定义 DNS")
				if !m.setCustomDNS() {
					continue
				}
			}
			m.saveSubscriptionDNSMode("custom")
		case "6":
			m.setCustomDNS()
		case "7":
			m.setSubscriptionTestURL()
		case "0":
			return
		}
	}
}

func (m *SubscriptionMenu) saveSubscriptionDNSMode(mode string) {
	m.config.Subscription.DNSMode = mode
	if err := m.saveSubscriptionConfig(); err != nil {
		PrintError(fmt.Sprintf("保存失败: %v", err))
		return
	}
	PrintSuccess(fmt.Sprintf("订阅 DNS 模式已设置为: %s", mode))
	PrintInfo("请执行「重新生成订阅」让客户端配置文件更新")
}

func (m *SubscriptionMenu) setCustomDNS() bool {
	PrintTitle("设置自定义 DNS")
	PrintInfo("可填写多个 DNS，用逗号或空格分隔")
	PrintInfo("DoH 地址必须带 https://，如 https://dns.example/dns-query；普通 IP 不要带协议")
	PrintSuccess("  直接回车取消")
	input := ReadInput("自定义 DNS 服务器")
	if input == "" {
		return false
	}

	servers := splitListInput(input)
	for _, server := range servers {
		if strings.HasPrefix(strings.ToLower(server), "http://") {
			PrintError("DNS DoH 地址请使用 https://，不要使用 http://")
			return false
		}
	}
	if len(servers) == 0 {
		PrintError("自定义 DNS 不能为空")
		return false
	}

	m.config.Subscription.DNSCustom = servers
	if err := m.saveSubscriptionConfig(); err != nil {
		PrintError(fmt.Sprintf("保存失败: %v", err))
		return false
	}
	PrintSuccess(fmt.Sprintf("自定义 DNS 已保存: %s", strings.Join(servers, ", ")))
	PrintInfo("如需启用，请选择 custom 自定义模式并重新生成订阅")
	return true
}

func (m *SubscriptionMenu) setSubscriptionTestURL() {
	PrintTitle("设置订阅测速 URL")
	PrintInfo("该 URL 用于 Clash/sing-box 的自动测速，必须是完整 https:// 地址")
	PrintInfo("示例: https://www.gstatic.com/generate_204 或 https://cp.cloudflare.com/generate_204")
	PrintSuccess("  直接回车取消")
	testURL := ReadInput("测速 URL（必须带 https://）")
	if testURL == "" {
		return
	}
	if err := security.ValidateURL(testURL); err != nil {
		PrintError(fmt.Sprintf("测速 URL 无效: %v", err))
		return
	}

	m.config.Subscription.TestURL = testURL
	if err := m.saveSubscriptionConfig(); err != nil {
		PrintError(fmt.Sprintf("保存失败: %v", err))
		return
	}
	PrintSuccess(fmt.Sprintf("订阅测速 URL 已设置为: %s", testURL))
	PrintInfo("请执行「重新生成订阅」让客户端配置文件更新")
}

func (m *SubscriptionMenu) saveSubscriptionConfig() error {
	return config.SaveConfig(config.DefaultConfigPath, m.config)
}

func currentDNSMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "cn", "global", "privacy", "custom":
		return mode
	default:
		return "auto"
	}
}

func currentTestURL(testURL string) string {
	testURL = strings.TrimSpace(testURL)
	if testURL == "" {
		return "https://www.gstatic.com/generate_204"
	}
	return testURL
}

func splitListInput(input string) []string {
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return result
}

func nonEmptyList(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
