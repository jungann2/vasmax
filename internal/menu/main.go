package menu

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"

	"vasmax/internal/bbr"
	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/firewall"
	"vasmax/internal/i18n"
	"vasmax/internal/nginx"
	"vasmax/internal/protocol"
	"vasmax/internal/rollback"
	"vasmax/internal/route"
	"vasmax/internal/subscription"
	"vasmax/internal/user"
)

// MainMenu displays the main interactive menu.
type MainMenu struct {
	config    *config.Config
	coreMgr   *core.Manager
	registry  *protocol.Registry
	logger    *logrus.Logger
	install   *InstallMenu
	account   *AccountMenu
	routing   *RoutingMenu
	tools     *ToolsMenu
	xboard    *XboardMenu
	tls       *TLSMenu
	protocols *ProtocolMenus
	coreMenu  *CoreMenu
	subMenu   *SubscriptionMenu
	portMenu  *PortMenu
	alpnMenu  *ALPNMenu
	monitor   *MonitorMenu
}

// NewMainMenu creates a new main menu with all sub-menus wired up.
func NewMainMenu(
	cfg *config.Config,
	coreMgr *core.Manager,
	reg *protocol.Registry,
	rbMgr *rollback.Manager,
	userMgr *user.Manager,
	subMgr *subscription.Manager,
	routeMgr *route.Manager,
	btMgr *route.BTManager,
	blMgr *route.BlacklistManager,
	warpMgr *route.WARPManager,
	nginxMgr *nginx.Manager,
	fwMgr *firewall.Manager,
	logger *logrus.Logger,
) *MainMenu {
	return &MainMenu{
		config:    cfg,
		coreMgr:   coreMgr,
		registry:  reg,
		logger:    logger,
		install:   NewInstallMenu(cfg, coreMgr, reg, rbMgr, nginxMgr, userMgr, subMgr, logger),
		account:   NewAccountMenu(userMgr, subMgr),
		routing:   NewRoutingMenu(routeMgr, btMgr, blMgr, warpMgr),
		tools:     NewToolsMenu(cfg, nginxMgr, logger),
		xboard:    NewXboardMenu(cfg, logger),
		tls:       NewTLSMenu(cfg, logger),
		protocols: NewProtocolMenus(cfg, fwMgr, logger),
		coreMenu:  NewCoreMenu(coreMgr, logger),
		subMenu:   NewSubscriptionMenu(cfg, subMgr, userMgr, logger),
		portMenu:  NewPortMenu(cfg, fwMgr, logger),
		alpnMenu:  NewALPNMenu(cfg, logger),
		monitor:   NewMonitorMenu(cfg, userMgr, logger),
	}
}

// Show displays the main menu and handles user input.
func (m *MainMenu) Show() {
	for {
		m.printHeader()
		m.printOptions()

		choices := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18"}
		choice := ReadChoice(i18n.T("menu.select"), choices)

		switch choice {
		case "1":
			m.updateSelf()
		case "2":
			m.uninstallSelf()
		case "3":
			m.install.Show()
		case "4":
			m.account.Show()
		case "5":
			m.routing.Show()
		case "6":
			m.routing.ShowBTMenu()
		case "7":
			m.routing.ShowBlacklistMenu()
		case "8":
			m.tools.ShowCDNMenu()
		case "9":
			m.subMenu.Show()
		case "10":
			m.portMenu.Show()
		case "11":
			m.alpnMenu.Show()
		case "12":
			m.coreMenu.Show()
		case "13":
			m.xboard.Show()
		case "14":
			m.tls.Show()
		case "15":
			m.tools.Show()
		case "16":
			m.monitor.Show()
		case "17":
			m.showServiceMenu()
		case "18":
			m.protocols.Show()
		case "0":
			return
		}
	}
}

func (m *MainMenu) printHeader() {
	PrintTitle("VasmaX 管理面板")

	// 显示运行模式
	if m.config.Standalone {
		PrintInfo("运行模式: " + Green("独立模式"))
	} else {
		PrintInfo("运行模式: " + Cyan("[Xboard-Plus 托管模式]"))
	}

	// 显示核心状态（只显示已安装的核心）
	status := m.coreMgr.GetStatus()
	if xs, ok := status["xray"]; ok && xs.Installed {
		state := Red("已停止")
		if xs.Running {
			state = Green("运行中")
		}
		PrintInfo(fmt.Sprintf("Xray: %s v%s", state, xs.Version))
	}
	if ss, ok := status["singbox"]; ok && ss.Installed {
		state := Red("已停止")
		if ss.Running {
			state = Green("运行中")
		}
		PrintInfo(fmt.Sprintf("sing-box: %s v%s", state, ss.Version))
	}

	// 显示已安装协议列表
	if len(m.config.Protocols) > 0 {
		var protoList []string
		for _, pName := range m.config.Protocols {
			label := pName
			if p, ok := m.registry.Get(pName); ok {
				label = fmt.Sprintf("%s %s", pName, protocolLabel(p))
			}
			mode := inferProtocolMode(m.config.ProtocolModes, pName)
			switch mode {
			case "domain":
				label += Green(" [绑定域名]")
			case "nodomain":
				label += Cyan(" [无域名]")
			}
			protoList = append(protoList, label)
		}
		PrintInfo(fmt.Sprintf("已安装协议（%d 个）:", len(m.config.Protocols)))
		for _, p := range protoList {
			PrintInfo(fmt.Sprintf("  · %s", p))
		}
	}

	// 显示 BBR 状态
	cc := bbr.CurrentCC()
	qdisc := bbr.CurrentQdisc()
	if strings.Contains(cc, "bbr") {
		PrintInfo(fmt.Sprintf("BBR: %s（%s + %s）", Green("已启用"), cc, qdisc))
	} else {
		PrintInfo(fmt.Sprintf("BBR: %s（当前: %s + %s）", Yellow("未启用"), cc, qdisc))
	}

	// 显示 TLS 证书状态
	if m.config.TLS.Domain != "" {
		certFile, _ := config.DetectCertPath(&m.config.TLS)
		if certFile != "" {
			if info, err := config.CheckCertificate(certFile); err == nil {
				if info.DaysLeft <= 0 {
					PrintInfo(fmt.Sprintf("TLS: %s (%s)", Red("已过期"), m.config.TLS.Domain))
				} else if info.DaysLeft <= 7 {
					PrintInfo(fmt.Sprintf("TLS: %s 剩余 %d 天 (%s)", Yellow("即将过期"), info.DaysLeft, m.config.TLS.Domain))
				} else {
					PrintInfo(fmt.Sprintf("TLS: %s 剩余 %d 天 (%s)", Green("有效"), info.DaysLeft, m.config.TLS.Domain))
				}
			}
		}
	}

	PrintSeparator()
}

func (m *MainMenu) printOptions() {
	PrintOption(1, "更新 VasmaX")
	PrintOption(2, "卸载 VasmaX")
	PrintSeparator()
	PrintOption(3, "安装管理（安装/卸载协议）")
	PrintOption(4, "账号管理")
	PrintOption(5, "分流工具")
	PrintOption(6, "BT 下载管理")
	PrintOption(7, "域名黑名单")
	PrintOption(8, "CDN 管理")
	PrintOption(9, "订阅管理")
	PrintOption(10, "额外端口管理")
	PrintOption(11, "ALPN 切换")
	PrintOption(12, "核心管理（启动/停止/重启 Xray 和 sing-box）")
	PrintOption(13, "Xboard-Plus 对接管理")
	PrintOption(14, "TLS 证书管理")
	PrintOption(15, "系统工具（BBR/Nginx 伪装站/健康检查）")
	PrintOption(16, "实时监控")
	PrintOption(17, "VasmaX 服务管理（启动/停止/重启/状态/日志）")
	PrintOption(18, "协议参数管理（Reality/Hysteria2/TUIC 专项参数）")
	PrintOptionStr("0", "退出")
	fmt.Println()
}

func (m *MainMenu) showServiceMenu() {
	for {
		PrintTitle("VasmaX 服务管理")
		PrintInfo("管理 VasmaX 主服务；Xray/sing-box 请在「核心管理」中操作")
		m.printServiceState()
		PrintSeparator()
		PrintOption(1, "启动 VasmaX 主服务")
		PrintOption(2, "停止 VasmaX 主服务")
		PrintOption(3, "重启 VasmaX 主服务")
		PrintOption(4, "查看 systemd 状态")
		PrintOption(5, "查看最近 120 行日志")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5"})
		switch choice {
		case "1":
			m.runServiceCommand("start", "已启动 VasmaX 主服务")
		case "2":
			if Confirm("停止主服务会暂停托管同步、流量上报和自动配置更新，确认停止?") {
				m.runServiceCommand("stop", "已停止 VasmaX 主服务")
			}
		case "3":
			m.runServiceCommand("restart", "已重启 VasmaX 主服务")
		case "4":
			m.runInteractiveCommand("systemctl", "status", "VasmaX", "--no-pager")
		case "5":
			m.runInteractiveCommand("journalctl", "-u", "VasmaX", "-n", "120", "--no-pager")
		case "0":
			return
		}
	}
}

func (m *MainMenu) printServiceState() {
	cmd := exec.Command("systemctl", "is-active", "VasmaX")
	out, err := cmd.Output()
	state := strings.TrimSpace(string(out))
	if err != nil || state == "" {
		state = "unknown"
	}
	switch state {
	case "active":
		PrintInfo("主服务状态: " + Green("运行中"))
	case "inactive":
		PrintInfo("主服务状态: " + Yellow("已停止"))
	case "failed":
		PrintInfo("主服务状态: " + Red("失败"))
	default:
		PrintInfo("主服务状态: " + Yellow(state))
	}
}

func (m *MainMenu) runServiceCommand(action, success string) {
	if err := exec.Command("systemctl", action, "VasmaX").Run(); err != nil {
		PrintError(fmt.Sprintf("执行失败: %v", err))
		return
	}
	PrintSuccess(success)
}

func (m *MainMenu) runInteractiveCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		PrintError(fmt.Sprintf("执行失败: %v", err))
	}
}

// updateSelf 更新 VasmaX 自身
func (m *MainMenu) updateSelf() {
	PrintInfo("正在检查 VasmaX 更新...")

	// 通过 install.sh update 执行更新
	installScript := "/usr/local/bin/install_vasmax.sh"
	if _, err := os.Stat(installScript); os.IsNotExist(err) {
		// 尝试从 GitHub 下载最新 install.sh 并执行
		PrintInfo("正在下载最新安装脚本...")
		cmd := exec.Command("bash", "-c",
			"curl -fsSL https://raw.githubusercontent.com/jungann2/vasmax/main/install.sh -o /tmp/vasmax_update.sh && bash /tmp/vasmax_update.sh update")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			PrintError(fmt.Sprintf("更新失败: %v", err))
			return
		}
	} else {
		cmd := exec.Command("bash", installScript, "update")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			PrintError(fmt.Sprintf("更新失败: %v", err))
			return
		}
	}
	PrintSuccess("VasmaX 更新完成，请重新启动菜单")
	os.Exit(0)
}

// uninstallSelf 卸载 VasmaX
func (m *MainMenu) uninstallSelf() {
	PrintWarning("此操作将完全卸载 VasmaX 及所有相关组件")
	if !Confirm("确认卸载?") {
		return
	}

	fmt.Println()
	PrintOption(1, "保留配置卸载")
	PrintOption(2, "完全清除（含配置和数据）")
	choice := ReadChoice("请选择", []string{"1", "2"})

	purge := ""
	if choice == "2" {
		if !Confirm("再次确认: 所有配置和数据将被永久删除") {
			return
		}
		purge = "--purge"
	}

	// 通过 install.sh uninstall 执行卸载
	installScript := "/usr/local/bin/install_vasmax.sh"
	if _, err := os.Stat(installScript); os.IsNotExist(err) {
		PrintInfo("正在下载卸载脚本...")
		cmdStr := "curl -fsSL https://raw.githubusercontent.com/jungann2/vasmax/main/install.sh -o /tmp/vasmax_uninstall.sh && bash /tmp/vasmax_uninstall.sh uninstall"
		if purge != "" {
			cmdStr += " " + purge
		}
		cmd := exec.Command("bash", "-c", cmdStr)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			PrintError(fmt.Sprintf("卸载失败: %v", err))
			return
		}
	} else {
		args := []string{installScript, "uninstall"}
		if purge != "" {
			args = append(args, purge)
		}
		cmd := exec.Command("bash", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			PrintError(fmt.Sprintf("卸载失败: %v", err))
			return
		}
	}
	PrintSuccess("VasmaX 已卸载")
	os.Exit(0)
}
