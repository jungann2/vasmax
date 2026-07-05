package menu

import (
	"fmt"

	"vasmax/internal/core"
	"vasmax/internal/route"
)

// RoutingMenu handles routing, BT, blacklist, and socks5 management.
type RoutingMenu struct {
	routeMgr *route.Manager
	btMgr    *route.BTManager
	blMgr    *route.BlacklistManager
	warpMgr  *route.WARPManager
	coreMgr  *core.Manager
}

// NewRoutingMenu creates a new routing menu.
func NewRoutingMenu(routeMgr *route.Manager, btMgr *route.BTManager, blMgr *route.BlacklistManager, warpMgr *route.WARPManager, coreMgr *core.Manager) *RoutingMenu {
	return &RoutingMenu{routeMgr: routeMgr, btMgr: btMgr, blMgr: blMgr, warpMgr: warpMgr, coreMgr: coreMgr}
}

// ShowBTMenu directly shows the BT download management sub-menu.
func (m *RoutingMenu) ShowBTMenu() {
	m.btMenu()
}

// ShowBlacklistMenu directly shows the domain blacklist management sub-menu.
func (m *RoutingMenu) ShowBlacklistMenu() {
	m.blacklistMenu()
}

// Show displays the routing tools menu.
func (m *RoutingMenu) Show() {
	for {
		PrintTitle("分流工具")
		PrintOption(1, "WARP 分流管理")
		PrintOption(2, "BT 下载管理")
		PrintOption(3, "域名黑名单管理")
		PrintOption(4, "查看路由规则")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
		switch choice {
		case "1":
			m.warpMenu()
		case "2":
			m.btMenu()
		case "3":
			m.blacklistMenu()
		case "4":
			m.listRules()
		case "0":
			return
		}
	}
}

func (m *RoutingMenu) warpMenu() {
	PrintTitle("WARP 分流管理")
	if m.warpMgr.IsInstalled() {
		PrintInfo("WARP 状态: " + Green("已安装"))
	} else {
		PrintInfo("WARP 状态: " + Yellow("未安装"))
	}
	PrintOption(1, "安装 WARP")
	PrintOption(2, "配置 WARP")
	PrintOption(3, "测试连接")
	PrintOption(4, "卸载 WARP")
	PrintOptionStr("0", "返回")

	choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
	switch choice {
	case "1":
		if err := m.warpMgr.Install(); err != nil {
			PrintError(fmt.Sprintf("安装失败: %v", err))
		} else {
			PrintSuccess("WARP 安装完成")
		}
	case "2":
		if err := m.warpMgr.Setup(); err != nil {
			PrintError(fmt.Sprintf("配置失败: %v", err))
		} else {
			PrintSuccess("WARP 配置完成")
		}
	case "3":
		if err := m.warpMgr.TestConnection(); err != nil {
			PrintError(fmt.Sprintf("连接测试失败: %v", err))
		} else {
			PrintSuccess("WARP 连接正常")
		}
	case "4":
		if Confirm("确认卸载 WARP?") {
			if err := m.warpMgr.Uninstall(); err != nil {
				PrintError(fmt.Sprintf("卸载失败: %v", err))
			} else {
				PrintSuccess("WARP 已卸载")
			}
		}
	}
}

func (m *RoutingMenu) btMenu() {
	PrintTitle("BT 下载管理")
	if m.btMgr.IsBlocked() {
		PrintInfo("BT 状态: " + Red("已阻断"))
	} else {
		PrintInfo("BT 状态: " + Green("已允许"))
	}
	PrintOption(1, "阻断 BT 下载")
	PrintOption(2, "允许 BT 下载")
	PrintOptionStr("0", "返回")

	choice := ReadChoice("请选择", []string{"1", "2"})
	switch choice {
	case "1":
		if err := m.btMgr.Block(); err != nil {
			PrintError(fmt.Sprintf("阻断失败: %v", err))
		} else {
			PrintSuccess("BT 下载已阻断")
			m.applyRoutingChanges()
		}
	case "2":
		if err := m.btMgr.Allow(); err != nil {
			PrintError(fmt.Sprintf("允许失败: %v", err))
		} else {
			PrintSuccess("BT 下载已允许")
			m.applyRoutingChanges()
		}
	}
}

func (m *RoutingMenu) blacklistMenu() {
	PrintTitle("域名黑名单管理")
	PrintOption(1, "查看黑名单")
	PrintOption(2, "添加域名")
	PrintOption(3, "删除域名")
	PrintOption(4, "一键阻断中国大陆域名")
	PrintOption(5, "取消阻断中国大陆域名")
	PrintOptionStr("0", "返回")

	choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5"})
	switch choice {
	case "1":
		domains, err := m.blMgr.List()
		if err != nil {
			PrintError(fmt.Sprintf("获取黑名单失败: %v", err))
			return
		}
		if len(domains) == 0 {
			PrintInfo("黑名单为空")
		} else {
			for i, d := range domains {
				PrintOption(i+1, d)
			}
		}
	case "2":
		domain := ReadInput("请输入要阻断的域名（只填域名，不要带 http:// 或 https://）")
		if domain != "" {
			if err := m.blMgr.Add(domain); err != nil {
				PrintError(fmt.Sprintf("添加失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("已添加: %s", domain))
				m.applyRoutingChanges()
			}
		}
	case "3":
		domain := ReadInput("请输入要移除的域名（只填域名，不要带 http:// 或 https://）")
		if domain != "" {
			if err := m.blMgr.Remove(domain); err != nil {
				PrintError(fmt.Sprintf("删除失败: %v", err))
			} else {
				PrintSuccess(fmt.Sprintf("已移除: %s", domain))
				m.applyRoutingChanges()
			}
		}
	case "4":
		if err := m.blMgr.BlockChina(); err != nil {
			PrintError(fmt.Sprintf("阻断失败: %v", err))
		} else {
			PrintSuccess("已阻断中国大陆域名(geosite:cn)")
			m.applyRoutingChanges()
		}
	case "5":
		if err := m.blMgr.UnblockChina(); err != nil {
			PrintError(fmt.Sprintf("取消失败: %v", err))
		} else {
			PrintSuccess("已取消阻断中国大陆域名")
			m.applyRoutingChanges()
		}
	}
}

func (m *RoutingMenu) applyRoutingChanges() {
	if m == nil || m.coreMgr == nil {
		PrintWarning("路由配置已写入；未绑定核心管理器，请手动重启当前使用的核心后生效")
		return
	}
	PrintInfo("正在应用路由配置...")
	if err := m.coreMgr.RestartAll(); err != nil {
		PrintWarning(fmt.Sprintf("路由配置已写入，但重启核心失败: %v", err))
		PrintWarning("请检查核心配置后手动重启 Xray/sing-box")
		return
	}
	PrintSuccess("路由配置已应用")
}

func (m *RoutingMenu) listRules() {
	rules, err := m.routeMgr.ListRules()
	if err != nil {
		PrintError(fmt.Sprintf("获取规则失败: %v", err))
		return
	}
	if len(rules) == 0 {
		PrintInfo("暂无自定义路由规则")
		return
	}
	PrintTitle("路由规则列表")
	for i, r := range rules {
		PrintOption(i+1, fmt.Sprintf("类型: %-15s 出站: %s", r.Type, r.Outbound))
	}
}
