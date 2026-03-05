package menu

import (
	"fmt"

	"github.com/sirupsen/logrus"

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

// Show displays the tools menu.
func (m *ToolsMenu) Show() {
	for {
		PrintTitle("其他工具")
		PrintOption(1, "CDN 管理")
		PrintOption(2, "伪装站管理")
		PrintOption(3, "健康检查")
		PrintOption(4, "卸载 VasmaX")
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
			m.uninstall()
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
		idx := ReadInput("选择预设")
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
		idx := ReadInput("选择模板")
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

func (m *ToolsMenu) uninstall() {
	PrintWarning("此操作将完全卸载 VasmaX 及所有相关组件")
	if !Confirm("确认卸载?") {
		return
	}
	if !Confirm("再次确认: 所有配置和数据将被删除") {
		return
	}
	PrintInfo("卸载功能需要通过 install.sh 执行")
	PrintInfo("请运行: bash install.sh uninstall")
	PrintWarning("注意: acme.sh 证书配置不会被自动移除")
	PrintInfo("如需清理请手动执行: ~/.acme.sh/acme.sh --uninstall")
}
