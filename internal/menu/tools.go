package menu

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"vasmax/internal/bbr"
	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/nginx"
	"vasmax/internal/protocol"
	"vasmax/internal/sysinfo"
)

// ToolsMenu handles miscellaneous tools.
type ToolsMenu struct {
	config   *config.Config
	coreMgr  *core.Manager
	nginxMgr *nginx.Manager
	logger   *logrus.Logger
}

// NewToolsMenu creates a new tools menu.
func NewToolsMenu(cfg *config.Config, coreMgr *core.Manager, nginxMgr *nginx.Manager, logger *logrus.Logger) *ToolsMenu {
	return &ToolsMenu{config: cfg, coreMgr: coreMgr, nginxMgr: nginxMgr, logger: logger}
}

// ShowCDNMenu directly shows the CDN management sub-menu.
func (m *ToolsMenu) ShowCDNMenu() {
	m.cdnMenu()
}

// Show displays the tools menu.
func (m *ToolsMenu) Show() {
	for {
		PrintTitle("其他工具")
		PrintOption(1, "Nginx 伪装站管理（部署假网页防止代理特征识别）")
		PrintOption(2, "健康检查")
		PrintOption(3, fmt.Sprintf("BBR 加速管理（当前: %s）", bbrStatus()))
		PrintOption(4, "服务端 DNS 配置（Xray/sing-box 出站解析）")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
		switch choice {
		case "1":
			m.fakeSiteMenu()
		case "2":
			m.healthCheck()
		case "3":
			m.bbrMenu()
		case "4":
			m.serverDNSMenu()
		case "0":
			return
		}
	}
}

func (m *ToolsMenu) serverDNSMenu() {
	for {
		PrintTitle("服务端 DNS 配置")
		PrintInfo("此处只影响服务器上的 Xray/sing-box 出站解析，不影响 Clash/sing-box 客户端订阅 DNS")
		PrintInfo(fmt.Sprintf("当前模式: %s", m.config.ServerDNS.EffectiveMode()))
		PrintInfo(fmt.Sprintf("当前策略: %s", m.config.ServerDNS.EffectiveStrategy()))
		servers := m.config.ServerDNS.EffectiveServers()
		if len(servers) > 0 {
			PrintInfo(fmt.Sprintf("当前 DNS: %s", strings.Join(servers, ", ")))
		} else {
			PrintInfo("当前 DNS: system resolver")
		}
		PrintSeparator()
		PrintOption(1, "system 默认（删除显式 core DNS，走系统 resolver）")
		PrintOption(2, "Cloudflare DNS（1.1.1.1 / 1.0.0.1）")
		PrintOption(3, "Quad9 DNS（9.9.9.9 / 149.112.112.112）")
		PrintOption(4, "Google DNS（8.8.8.8 / 8.8.4.4，仅手动选择）")
		PrintOption(5, "自定义 DNS IP")
		PrintOption(6, "切换 IPv4/IPv6 策略")
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5", "6"})
		switch choice {
		case "1":
			m.setServerDNS(config.ServerDNSModeSystem, nil)
		case "2":
			m.setServerDNS(config.ServerDNSModeCloudflare, nil)
		case "3":
			m.setServerDNS(config.ServerDNSModeQuad9, nil)
		case "4":
			m.setServerDNS(config.ServerDNSModeGoogle, nil)
		case "5":
			m.setCustomServerDNS()
		case "6":
			m.serverDNSStrategyMenu()
		case "0":
			return
		}
	}
}

func (m *ToolsMenu) setServerDNS(mode string, servers []string) {
	oldDNS := m.config.ServerDNS
	m.config.ServerDNS.Mode = config.NormalizeServerDNSMode(mode)
	if m.config.ServerDNS.Mode == config.ServerDNSModeCustom {
		m.config.ServerDNS.Servers = config.NormalizeServerDNSServers(servers)
	} else {
		m.config.ServerDNS.Servers = nil
	}
	if m.config.ServerDNS.Strategy == "" {
		m.config.ServerDNS.Strategy = "ipv4_only"
	}
	m.applyServerDNSConfig(oldDNS)
}

func (m *ToolsMenu) setCustomServerDNS() {
	PrintTitle("自定义服务端 DNS")
	PrintInfo("填写普通 DNS IP，多个用逗号或空格分隔；不要填写 http://、https://、DoH/DoT 地址")
	input := ReadInput("DNS IP")
	servers := config.NormalizeServerDNSServers(splitListInput(input))
	if len(servers) == 0 {
		PrintError("未识别到合法 DNS IP")
		return
	}
	m.setServerDNS(config.ServerDNSModeCustom, servers)
}

func (m *ToolsMenu) serverDNSStrategyMenu() {
	PrintTitle("服务端 DNS IPv4/IPv6 策略")
	PrintOption(1, "ipv4_only（推荐：只解析/使用 IPv4，规避 IPv6 半通）")
	PrintOption(2, "prefer_ipv4（优先 IPv4）")
	PrintOption(3, "prefer_ipv6（优先 IPv6）")
	PrintOption(4, "ipv6_only（只解析/使用 IPv6）")
	PrintOptionStr("0", "返回")

	choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
	switch choice {
	case "1":
		oldDNS := m.config.ServerDNS
		m.config.ServerDNS.Strategy = "ipv4_only"
		m.applyServerDNSConfig(oldDNS)
	case "2":
		oldDNS := m.config.ServerDNS
		m.config.ServerDNS.Strategy = "prefer_ipv4"
		m.applyServerDNSConfig(oldDNS)
	case "3":
		oldDNS := m.config.ServerDNS
		m.config.ServerDNS.Strategy = "prefer_ipv6"
		m.applyServerDNSConfig(oldDNS)
	case "4":
		oldDNS := m.config.ServerDNS
		m.config.ServerDNS.Strategy = "ipv6_only"
		m.applyServerDNSConfig(oldDNS)
	case "0":
		return
	}
}

func (m *ToolsMenu) applyServerDNSConfig(oldDNS config.ServerDNSConfig) {
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		m.config.ServerDNS = oldDNS
		PrintError(fmt.Sprintf("保存失败: %v", err))
		return
	}

	if !m.usesCore("xray") && !m.usesCore("singbox") {
		PrintInfo("当前未安装需要更新的核心协议，配置会在下次安装/生成配置时生效")
		return
	}
	if m.coreMgr == nil {
		PrintWarning("核心管理器不可用，配置已保存，将在下次服务启动时生效")
		return
	}
	if err := m.coreMgr.RestartAll(); err != nil {
		m.config.ServerDNS = oldDNS
		if saveErr := config.SaveConfig(config.DefaultConfigPath, m.config); saveErr != nil {
			PrintWarning(fmt.Sprintf("恢复旧 DNS 配置失败，请手动检查 config.yaml: %v", saveErr))
		} else if retryErr := m.coreMgr.RestartAll(); retryErr != nil {
			PrintWarning(fmt.Sprintf("旧 DNS 配置已恢复，但核心恢复重启失败，请手动检查: %v", retryErr))
		}
		PrintWarning(fmt.Sprintf("重启核心失败，请手动检查: %v", err))
	} else {
		PrintSuccess("已更新 DNS/基础配置并重启当前使用的核心")
	}
}

func (m *ToolsMenu) usesCore(coreType string) bool {
	reg := protocol.DefaultRegistry()
	for _, protoName := range m.config.Protocols {
		if p, ok := reg.Get(protoName); ok && p.CoreType() == coreType {
			return true
		}
	}
	return false
}

func (m *ToolsMenu) cdnMenu() {
	PrintTitle("CDN 管理")
	if m.config.CDN.Enabled {
		PrintInfo(fmt.Sprintf("CDN 状态: %s  地址: %s", Green("已启用"), m.config.CDN.Address))
	} else {
		PrintInfo("CDN 状态: " + Yellow("未启用"))
	}
	PrintSeparator()
	PrintOption(1, "启用/修改 CDN")
	PrintOption(2, "禁用 CDN")
	PrintOption(3, "预设 CDN 列表")
	PrintOptionStr("0", "返回")

	choice := ReadChoice("请选择", []string{"1", "2", "3"})
	switch choice {
	case "1":
		addr := ReadInput("请输入 CDN 域名/IP（不要带 http:// 或 https://）")
		if addr != "" {
			if strings.Contains(addr, "://") {
				PrintError("CDN 地址不要带 http:// 或 https://")
				return
			}
			oldCDN := m.config.CDN
			m.config.CDN.Enabled = true
			m.config.CDN.Address = addr
			if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
				m.config.CDN = oldCDN
				PrintError(fmt.Sprintf("保存失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("CDN 已设置: %s", addr))
			}
		}
	case "2":
		oldCDN := m.config.CDN
		m.config.CDN.Enabled = false
		if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
			m.config.CDN = oldCDN
			PrintError(fmt.Sprintf("保存失败: %v", err))
			return
		}
		PrintSuccess("CDN 已禁用")
	case "3":
		presets := []string{"who.int", "icao.int", "cdn.who.int", "www.visa.com.sg"}
		for i, p := range presets {
			PrintOption(i+1, p)
		}
		PrintOptionStr("0", "返回")
		idx := ReadInput("选择预设")
		if idx == "" || idx == "0" {
			return
		}
		var n int
		if _, err := fmt.Sscanf(idx, "%d", &n); err == nil && n >= 1 && n <= len(presets) {
			oldCDN := m.config.CDN
			m.config.CDN.Enabled = true
			m.config.CDN.Address = presets[n-1]
			if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
				m.config.CDN = oldCDN
				PrintError(fmt.Sprintf("保存失败: %v", err))
				return
			}
			PrintSuccess(fmt.Sprintf("CDN 已设置: %s", presets[n-1]))
		}
	}
}

func (m *ToolsMenu) fakeSiteMenu() {
	PrintTitle("Nginx 伪装站管理")
	PrintInfo("通过 Nginx 在 80/443 端口部署一个真实网页，")
	PrintInfo("使服务器对外看起来像普通网站，防止被识别为代理服务器。")
	PrintInfo("与 Reality 伪装域名无关，Reality 无需配置此项。")
	PrintSeparator()
	PrintOption(1, "部署预设模板（从远程下载 HTML 模板）")
	PrintOption(2, "自定义模板 URL（指定任意 HTML 模板地址）")
	PrintOptionStr("0", "返回")

	choice := ReadChoice("请选择", []string{"1", "2"})
	switch choice {
	case "1":
		for i, url := range nginx.PresetFakeSites {
			PrintOption(i+1, url)
		}
		PrintOptionStr("0", "返回")
		idx := ReadInput("选择模板")
		if idx == "" || idx == "0" {
			return
		}
		var n int
		if _, err := fmt.Sscanf(idx, "%d", &n); err == nil && n >= 1 && n <= len(nginx.PresetFakeSites) {
			PrintInfo("正在部署...")
			if err := m.nginxMgr.DeployFakeSite(nginx.PresetFakeSites[n-1]); err != nil {
				PrintError(fmt.Sprintf("部署失败: %v", err))
			} else {
				PrintSuccess("伪装站已部署")
			}
		}
	case "2":
		url := ReadInput("请输入模板 URL（必须带 http:// 或 https://）")
		if url != "" {
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				PrintError("模板 URL 必须带 http:// 或 https://")
				return
			}
			if err := m.nginxMgr.DeployFakeSite(url); err != nil {
				PrintError(fmt.Sprintf("部署失败: %v", err))
			} else {
				PrintSuccess("伪装站已部署")
			}
		}
	}
}

func (m *ToolsMenu) healthCheck() {
	PrintTitle("健康检查")
	code := sysinfo.RunHealthCheck(config.DefaultConfigPath)
	if code == 0 {
		PrintSuccess("所有组件健康")
	} else {
		PrintWarning("存在不健康组件，请检查日志")
	}
}

// bbrStatus 返回当前 BBR 状态描述（用于主菜单显示）
func bbrStatus() string {
	status := bbr.RecommendedRuntimeStatus()
	detail := fmt.Sprintf("%s + %s", status.CC, status.DefaultQdisc)
	if status.DeviceQdisc != "" {
		detail += fmt.Sprintf("，网卡 %s=%s", status.DefaultInterface, status.DeviceQdisc)
	}
	if status.RecommendedRuntime {
		return Green("已启用") + fmt.Sprintf("（%s）", detail)
	}
	if status.RecommendedSysctl {
		if status.DeviceQdiscError != "" {
			return Yellow(fmt.Sprintf("sysctl 已启用，网卡未确认（%s；%s）", detail, status.DeviceQdiscError))
		}
		return Yellow(fmt.Sprintf("sysctl 已启用，网卡未使用 fq（%s）", detail))
	}
	if bbr.IsBBRCC(status.CC) {
		return Yellow(fmt.Sprintf("部分启用（%s，推荐 fq）", detail))
	}
	return Yellow(fmt.Sprintf("未启用（%s）", detail))
}

func (m *ToolsMenu) bbrMenu() {
	for {
		// ── 标题与状态 ──────────────────────────────────────────
		PrintTitle("BBR 加速管理")
		PrintInfo(fmt.Sprintf("内核版本  : %s", bbr.KernelVersion()))
		status := bbr.RecommendedRuntimeStatus()
		if status.RecommendedRuntime {
			PrintInfo(fmt.Sprintf("推荐组合  : %s", Green("BBR + FQ [已启用]")))
		} else if status.RecommendedSysctl {
			PrintInfo(fmt.Sprintf("推荐组合  : %s", Yellow("sysctl 已启用，网卡 qdisc 未完整确认")))
		} else if bbr.IsBBRCC(status.CC) {
			PrintInfo(fmt.Sprintf("推荐组合  : %s", Yellow(fmt.Sprintf("未完整启用（当前 %s + %s）", status.CC, status.DefaultQdisc))))
		} else {
			PrintInfo(fmt.Sprintf("推荐组合  : %s", Yellow(fmt.Sprintf("未启用（当前 %s + %s）", status.CC, status.DefaultQdisc))))
		}
		PrintInfo(fmt.Sprintf("拥塞控制  : %s", status.CC))
		PrintInfo(fmt.Sprintf("队列调度  : %s", status.DefaultQdisc))
		if status.DefaultInterface != "" {
			deviceQdisc := status.DeviceQdisc
			if deviceQdisc == "" {
				deviceQdisc = "未知"
			}
			PrintInfo(fmt.Sprintf("默认网卡  : %s qdisc=%s", status.DefaultInterface, deviceQdisc))
		}
		if status.DeviceQdiscError != "" {
			PrintWarning(fmt.Sprintf("无法读取默认网卡 qdisc: %s", status.DeviceQdiscError))
		}
		availableCC := strings.Join(bbr.AvailableCC(), " ")
		if availableCC == "" {
			availableCC = "未知"
		}
		PrintInfo(fmt.Sprintf("可用算法  : %s", availableCC))

		fmt.Println()
		PrintSectionTitle("推荐操作")
		PrintOption(1, "启用/重载 BBR + FQ（推荐）")
		PrintOption(2, "重新应用默认网卡队列调度 FQ")
		PrintOption(3, "应用系统配置优化（新方案，谨慎）")
		PrintOption(4, "关闭 BBR 并恢复默认 cubic")

		fmt.Println()
		PrintOptionStr("0", "返回上级菜单")

		choices := []string{"1", "2", "3", "4"}
		choice := ReadChoice("请选择", choices)
		if choice == "0" {
			return
		}
		m.handleBBRChoice(choice)
	}
}

func (m *ToolsMenu) handleBBRChoice(choice string) {
	if !bbr.IsRoot() {
		PrintError("此操作需要 root 权限")
		return
	}

	switch choice {
	case "1":
		mode := bbr.RecommendedMode()
		PrintInfo(fmt.Sprintf("正在启用/重载 %s...", mode.Label))
		if err := bbr.SetCC(mode); err != nil {
			PrintError(fmt.Sprintf("启用失败: %v", err))
			return
		}
		PrintSuccess(fmt.Sprintf("%s 已立即应用，并会在重启后持续生效", mode.Label))

	case "2":
		PrintInfo("正在重新应用默认网卡队列调度 FQ...")
		if err := bbr.ApplyDeviceQdisc("fq"); err != nil {
			PrintError(fmt.Sprintf("应用失败: %v", err))
			return
		}
		PrintSuccess("默认网卡队列调度 FQ 已重新应用")

	case "3":
		PrintWarning("系统配置优化会修改全局 sysctl 参数；小内存或特殊用途服务器请谨慎使用")
		if !Confirm("确认应用系统配置优化?") {
			return
		}
		PrintInfo("正在应用系统配置优化（新方案）...")
		if err := bbr.ApplyOptimize(); err != nil {
			PrintError(fmt.Sprintf("优化失败: %v", err))
		} else {
			PrintSuccess("系统配置优化（新方案）已应用")
		}

	case "4":
		PrintWarning("此操作将删除 vasmax BBR/系统优化配置，关闭 BBR 并恢复默认 cubic；不会修改 IPv6 开关")
		if !Confirm("确认关闭 BBR 并恢复默认 cubic?") {
			return
		}
		if err := bbr.DisableAll(); err != nil {
			PrintError(fmt.Sprintf("关闭失败: %v", err))
		} else {
			PrintSuccess("BBR 已关闭，全部加速配置已卸载，已恢复 cubic")
		}
	}
}
