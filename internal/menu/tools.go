package menu

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"vasmax/internal/bbr"
	"vasmax/internal/config"
	"vasmax/internal/nginx"
	"vasmax/internal/sysinfo"
)

// ToolsMenu handles miscellaneous tools.
type ToolsMenu struct {
	config   *config.Config
	nginxMgr *nginx.Manager
	logger   *logrus.Logger
}

// NewToolsMenu creates a new tools menu.
func NewToolsMenu(cfg *config.Config, nginxMgr *nginx.Manager, logger *logrus.Logger) *ToolsMenu {
	return &ToolsMenu{config: cfg, nginxMgr: nginxMgr, logger: logger}
}

// ShowCDNMenu directly shows the CDN management sub-menu.
func (m *ToolsMenu) ShowCDNMenu() {
	m.cdnMenu()
}

// Show displays the tools menu.
func (m *ToolsMenu) Show() {
	for {
		PrintTitle("其他工具")
		PrintOption(1, "CDN 管理")
		PrintOption(2, "伪装站管理")
		PrintOption(3, "健康检查")
		PrintOption(4, fmt.Sprintf("BBR 加速管理（当前: %s）", bbrStatus()))
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
		switch choice {
		case "1":
			m.cdnMenu()
		case "2":
			m.fakeSiteMenu()
		case "3":
			m.healthCheck()
		case "4":
			m.bbrMenu()
		case "0":
			return
		}
	}
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
		addr := ReadInput("请输入 CDN 域名/IP")
		if addr != "" {
			m.config.CDN.Enabled = true
			m.config.CDN.Address = addr
			if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
				PrintError(fmt.Sprintf("保存失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("CDN 已设置: %s", addr))
			}
		}
	case "2":
		m.config.CDN.Enabled = false
		_ = config.SaveConfig(config.DefaultConfigPath, m.config)
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
			m.config.CDN.Enabled = true
			m.config.CDN.Address = presets[n-1]
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess(fmt.Sprintf("CDN 已设置: %s", presets[n-1]))
		}
	}
}

func (m *ToolsMenu) fakeSiteMenu() {
	PrintTitle("伪装站管理")
	PrintOption(1, "部署预设模板")
	PrintOption(2, "自定义模板 URL")
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
		url := ReadInput("请输入模板 URL")
		if url != "" {
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
	cc := bbr.CurrentCC()
	if cc == "bbr" || cc == "bbr2" || cc == "bbrplus" {
		return Green(cc + " 已启用")
	}
	return Yellow(cc)
}

func (m *ToolsMenu) bbrMenu() {
	for {
		// ── 标题与状态 ──────────────────────────────────────────
		PrintTitle("BBR 加速管理")
		PrintInfo(fmt.Sprintf("内核版本  : %s", bbr.KernelVersion()))
		cc := bbr.CurrentCC()
		if cc == "bbr" || cc == "bbr2" || cc == "bbrplus" {
			PrintInfo(fmt.Sprintf("拥塞控制  : %s", Green(cc+" [已启用]")))
		} else {
			PrintInfo(fmt.Sprintf("拥塞控制  : %s", Yellow(cc)))
		}
		PrintInfo(fmt.Sprintf("队列调度  : %s", bbr.CurrentQdisc()))
		PrintInfo(fmt.Sprintf("可用算法  : %s", strings.Join(bbr.AvailableCC(), " ")))

		// ── 内核安装类 ──────────────────────────────────────────
		fmt.Println()
		PrintSectionTitle("内核安装类（需要重启）")
		PrintOption(1, "安装 BBR 原版内核")
		PrintOption(2, "安装 BBRplus 版内核")
		PrintOption(3, "安装 Lotserver（锐速）内核")
		PrintOption(4, "安装 BBRplus 新版内核")
		PrintOption(5, "安装 Zen 官方版内核")
		PrintOption(6, "安装官方 cloud 内核")
		PrintOption(7, "安装官方稳定内核")
		PrintOption(8, "安装官方最新内核")
		PrintOption(9, "安装 XANMOD-main 内核")
		PrintOption(10, "安装 XANMOD-LTS 内核")
		PrintOption(11, "安装 XANMOD-EDGE 内核")
		PrintOption(12, "安装 XANMOD-RT 内核")

		// ── 加速启用类 ──────────────────────────────────────────
		fmt.Println()
		PrintSectionTitle("加速启用类（无需重启）")
		PrintOption(13, "BBR + FQ（推荐）")
		PrintOption(14, "BBR + FQ_PIE")
		PrintOption(15, "BBR + CAKE")
		PrintOption(16, "BBR2 + FQ")
		PrintOption(17, "BBR2 + FQ_PIE")
		PrintOption(18, "BBR2 + CAKE")
		PrintOption(19, "BBRplus + FQ")
		PrintOption(20, "Lotserver（锐速）加速")
		PrintOption(21, "编译安装 brutal 模块")

		// ── 系统配置类 ──────────────────────────────────────────
		fmt.Println()
		PrintSectionTitle("系统配置类")
		PrintOption(22, "开启 ECN")
		PrintOption(23, "关闭 ECN")
		PrintOption(24, "系统配置优化（旧方案）")
		PrintOption(25, "系统配置优化（新方案，含更多 sysctl 调优）")
		PrintOption(26, "禁用 IPv6")
		PrintOption(27, "开启 IPv6")
		PrintOption(28, "手动提交合并内核参数")
		PrintOption(29, "手动编辑内核参数")

		// ── 内核管理类 ──────────────────────────────────────────
		fmt.Println()
		PrintSectionTitle("内核管理类")
		PrintOption(30, "查看已安装内核列表")
		PrintOption(31, "删除/保留指定内核")
		PrintOption(32, "卸载全部加速配置")

		fmt.Println()
		PrintOptionStr("0", "返回上级菜单")

		choices := []string{
			"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12",
			"13", "14", "15", "16", "17", "18", "19", "20", "21",
			"22", "23", "24", "25", "26", "27", "28", "29",
			"30", "31", "32",
		}
		choice := ReadChoice("请选择", choices)
		if choice == "0" {
			return
		}
		m.handleBBRChoice(choice)
	}
}

func (m *ToolsMenu) handleBBRChoice(choice string) {
	// 选项 30（查看内核列表）是只读操作，不需要 root
	if choice != "30" && !bbr.IsRoot() {
		PrintError("此操作需要 root 权限")
		return
	}

	distro, err := bbr.DetectDistro()
	if err != nil {
		PrintWarning(fmt.Sprintf("发行版检测失败，将尝试继续: %v", err))
		distro = &bbr.Distro{}
	}

	switch choice {
	// ── 内核安装类 1-12 ──────────────────────────────────────
	case "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12":
		targetMap := map[string]bbr.KernelTarget{
			"1": bbr.KernelBBR, "2": bbr.KernelBBRPlus, "3": bbr.KernelLotserver,
			"4": bbr.KernelBBRPlusNew, "5": bbr.KernelZen, "6": bbr.KernelCloud,
			"7": bbr.KernelStable, "8": bbr.KernelLatest,
			"9": bbr.KernelXANMODMain, "10": bbr.KernelXANMODLTS,
			"11": bbr.KernelXANMODEdge, "12": bbr.KernelXANMODRT,
		}
		target := targetMap[choice]
		label := bbr.GetKernelLabel(target)
		PrintWarning(fmt.Sprintf("即将安装 %s，安装完成后需要重启系统", label))
		if !Confirm("确认继续?") {
			return
		}
		PrintInfo(fmt.Sprintf("正在安装 %s，请稍候...", label))
		if err := bbr.InstallKernel(target, distro); err != nil {
			PrintError(fmt.Sprintf("安装失败: %v", err))
			return
		}
		if err := bbr.UpdateGrub(distro); err != nil {
			PrintWarning(fmt.Sprintf("更新 grub 失败: %v", err))
		}
		PrintSuccess(fmt.Sprintf("%s 安装完成，请重启系统生效", label))
		PrintInfo("重启命令: reboot")

	// ── 加速启用类 13-21 ─────────────────────────────────────
	case "13", "14", "15", "16", "17", "18", "19":
		idx := map[string]int{"13": 0, "14": 1, "15": 2, "16": 3, "17": 4, "18": 5, "19": 6}
		mode := bbr.CCModes[idx[choice]]
		PrintInfo(fmt.Sprintf("正在启用 %s...", mode.Label))
		if err := bbr.SetCC(mode); err != nil {
			PrintError(fmt.Sprintf("启用失败: %v", err))
			return
		}
		PrintSuccess(fmt.Sprintf("%s 已启用，重启后持续生效", mode.Label))

	case "20":
		PrintInfo("正在安装 Lotserver（锐速）加速...")
		if err := bbr.InstallLotserverAccel(); err != nil {
			PrintError(fmt.Sprintf("安装失败: %v", err))
			return
		}
		PrintSuccess("Lotserver 加速已安装")

	case "21":
		PrintInfo("正在编译安装 brutal 模块，这可能需要几分钟...")
		if err := bbr.InstallBrutal(); err != nil {
			PrintError(fmt.Sprintf("安装失败: %v", err))
			return
		}
		PrintSuccess("brutal 模块已安装")

	// ── 系统配置类 22-29 ─────────────────────────────────────
	case "22":
		if err := bbr.SetECN(true); err != nil {
			PrintError(fmt.Sprintf("开启 ECN 失败: %v", err))
		} else {
			PrintSuccess("ECN 已开启")
		}

	case "23":
		if err := bbr.SetECN(false); err != nil {
			PrintError(fmt.Sprintf("关闭 ECN 失败: %v", err))
		} else {
			PrintSuccess("ECN 已关闭")
		}

	case "24":
		PrintInfo("正在应用系统配置优化（旧方案）...")
		if err := bbr.ApplyOptimize(false); err != nil {
			PrintError(fmt.Sprintf("优化失败: %v", err))
		} else {
			PrintSuccess("系统配置优化（旧方案）已应用")
		}

	case "25":
		PrintInfo("正在应用系统配置优化（新方案）...")
		if err := bbr.ApplyOptimize(true); err != nil {
			PrintError(fmt.Sprintf("优化失败: %v", err))
		} else {
			PrintSuccess("系统配置优化（新方案）已应用")
		}

	case "26":
		if !Confirm("确认禁用 IPv6?") {
			return
		}
		if err := bbr.SetIPv6(false); err != nil {
			PrintError(fmt.Sprintf("禁用 IPv6 失败: %v", err))
		} else {
			PrintSuccess("IPv6 已禁用")
		}

	case "27":
		if err := bbr.SetIPv6(true); err != nil {
			PrintError(fmt.Sprintf("开启 IPv6 失败: %v", err))
		} else {
			PrintSuccess("IPv6 已开启")
		}

	case "28":
		PrintInfo("正在合并提交所有内核参数...")
		if err := bbr.MergeSysctl(); err != nil {
			PrintError(fmt.Sprintf("失败: %v", err))
		} else {
			PrintSuccess("所有内核参数已重新加载")
		}

	case "29":
		if err := bbr.EditSysctlFile(); err != nil {
			PrintError(fmt.Sprintf("打开编辑器失败: %v", err))
		}

	// ── 内核管理类 30-32 ─────────────────────────────────────
	case "30":
		kernels, err := bbr.ListInstalledKernels(distro)
		if err != nil {
			PrintError(fmt.Sprintf("获取内核列表失败: %v", err))
			return
		}
		PrintTitle("已安装内核列表")
		current := bbr.KernelVersion()
		for i, k := range kernels {
			if strings.Contains(k, current) {
				PrintInfo(fmt.Sprintf("  %d. %s %s", i+1, k, Green("[当前运行]")))
			} else {
				PrintInfo(fmt.Sprintf("  %d. %s", i+1, k))
			}
		}
		ReadInput("按 Enter 返回")

	case "31":
		kernels, err := bbr.ListInstalledKernels(distro)
		if err != nil {
			PrintError(fmt.Sprintf("获取内核列表失败: %v", err))
			return
		}
		if len(kernels) == 0 {
			PrintInfo("没有找到已安装的内核")
			return
		}
		current := bbr.KernelVersion()
		PrintTitle("选择要保留的内核（其余将被删除）")
		for i, k := range kernels {
			if strings.Contains(k, current) {
				PrintInfo(fmt.Sprintf("  %d. %s %s", i+1, k, Green("[当前，不可删除]")))
			} else {
				PrintInfo(fmt.Sprintf("  %d. %s", i+1, k))
			}
		}
		input := ReadInput("输入要保留的内核编号（多个用逗号分隔，如 1,3）")
		keepSet := map[int]bool{}
		for _, s := range strings.Split(input, ",") {
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err == nil && n >= 1 && n <= len(kernels) {
				keepSet[n-1] = true
			}
		}
		// 当前内核强制保留
		for i, k := range kernels {
			if strings.Contains(k, current) {
				keepSet[i] = true
			}
		}
		var toDelete []string
		for i, k := range kernels {
			if !keepSet[i] {
				toDelete = append(toDelete, k)
			}
		}
		if len(toDelete) == 0 {
			PrintInfo("没有需要删除的内核")
			return
		}
		PrintWarning(fmt.Sprintf("将删除以下内核: %s", strings.Join(toDelete, ", ")))
		if !Confirm("确认删除?") {
			return
		}
		if err := bbr.DeleteKernels(toDelete, distro); err != nil {
			PrintError(fmt.Sprintf("删除失败: %v", err))
		} else {
			PrintSuccess("指定内核已删除")
		}

	case "32":
		PrintWarning("此操作将删除所有 vasmax BBR/优化配置，恢复默认 cubic")
		if !Confirm("确认卸载全部加速配置?") {
			return
		}
		if err := bbr.DisableAll(); err != nil {
			PrintError(fmt.Sprintf("卸载失败: %v", err))
		} else {
			PrintSuccess("全部加速配置已卸载，已恢复 cubic")
		}
	}
}
