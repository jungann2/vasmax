package menu

import (
	"fmt"
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
	}
}

// Show displays the main menu and handles user input.
func (m *MainMenu) Show() {
	for {
		m.printHeader()
		m.printOptions()

		choices := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"}
		choice := ReadChoice(i18n.T("menu.select"), choices)

		switch choice {
		case "1":
			m.install.Show()
		case "2":
			m.account.Show()
		case "3":
			m.routing.Show()
		case "4":
			m.routing.ShowBTMenu()
		case "5":
			m.routing.ShowBlacklistMenu()
		case "6":
			m.tools.ShowCDNMenu()
		case "7":
			m.subMenu.Show()
		case "8":
			m.portMenu.Show()
		case "9":
			m.alpnMenu.Show()
		case "10":
			m.coreMenu.Show()
		case "11":
			m.xboard.Show()
		case "12":
			m.tls.Show()
		case "13":
			m.tools.Show()
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
		PrintInfo("运行模式: " + Cyan("[Xboard2 托管模式]"))
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
	PrintOption(1, "安装管理")
	PrintOption(2, "账号管理")
	PrintOption(3, "分流工具")
	PrintOption(4, "BT 下载管理")
	PrintOption(5, "域名黑名单")
	PrintOption(6, "CDN 管理")
	PrintOption(7, "订阅管理")
	PrintOption(8, "额外端口管理")
	PrintOption(9, "ALPN 切换")
	PrintOption(10, "核心管理")
	PrintOption(11, "Xboard2 对接管理")
	PrintOption(12, "TLS 证书管理")
	PrintOption(13, "其他工具（BBR/CDN管理/伪装站管理/健康检查）")
	PrintOptionStr("0", "退出")
	fmt.Println()
}
