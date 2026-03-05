package menu

import (
	"fmt"

	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/i18n"
)

// MainMenu displays the main interactive menu.
type MainMenu struct {
	config  *config.Config
	coreMgr *core.Manager
}

// NewMainMenu creates a new main menu.
func NewMainMenu(cfg *config.Config, coreMgr *core.Manager) *MainMenu {
	return &MainMenu{config: cfg, coreMgr: coreMgr}
}

// Show displays the main menu and handles user input.
func (m *MainMenu) Show() {
	for {
		m.printHeader()
		m.printOptions()

		choices := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"}
		choice := ReadChoice(i18n.T("menu.choose"), choices)

		switch choice {
		case "1":
			PrintInfo("安装管理 - TODO")
		case "2":
			PrintInfo("账号管理 - TODO")
		case "3":
			PrintInfo("分流工具 - TODO")
		case "4":
			PrintInfo("BT 下载管理 - TODO")
		case "5":
			PrintInfo("域名黑名单 - TODO")
		case "6":
			PrintInfo("CDN 管理 - TODO")
		case "7":
			PrintInfo("订阅管理 - TODO")
		case "8":
			PrintInfo("额外端口管理 - TODO")
		case "9":
			PrintInfo("ALPN 切换 - TODO")
		case "10":
			PrintInfo("核心管理 - TODO")
		case "11":
			PrintInfo("xboard 对接管理 - TODO")
		case "12":
			PrintInfo("TLS 证书管理 - TODO")
		case "13":
			PrintInfo("其他工具 - TODO")
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
		PrintInfo("运行模式: " + Cyan("[xboard 托管模式]"))
	}

	// 显示核心状态
	status := m.coreMgr.GetStatus()
	for name, s := range status {
		if s.Installed {
			state := Red("已停止")
			if s.Running {
				state = Green("运行中")
			}
			PrintInfo(fmt.Sprintf("%s: %s v%s", name, state, s.Version))
		}
	}

	// 显示已安装协议
	if len(m.config.Protocols) > 0 {
		PrintInfo(fmt.Sprintf("已安装协议: %d 个", len(m.config.Protocols)))
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
	PrintOption(11, "xboard 对接管理")
	PrintOption(12, "TLS 证书管理")
	PrintOption(13, "其他工具")
	PrintOptionStr("0", "退出")
	fmt.Println()
}
