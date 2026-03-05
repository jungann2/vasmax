package menu

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"vasmax/internal/api"
	"vasmax/internal/config"
	"vasmax/internal/security"
)

// XboardMenu handles xboard integration management.
type XboardMenu struct {
	config *config.Config
	logger *logrus.Logger
}

// NewXboardMenu creates a new xboard menu.
func NewXboardMenu(cfg *config.Config, logger *logrus.Logger) *XboardMenu {
	return &XboardMenu{config: cfg, logger: logger}
}

// Show displays the xboard management menu.
func (m *XboardMenu) Show() {
	for {
		PrintTitle("xboard 对接管理")
		if !m.config.Standalone && m.config.APIHost != "" {
			PrintInfo("状态: " + Green("已启用"))
			PrintInfo(fmt.Sprintf("API: %s  NodeID: %d", m.config.APIHost, m.config.NodeID))
		} else {
			PrintInfo("状态: " + Yellow("未启用"))
		}
		PrintSeparator()
		PrintOption(1, "启用 xboard 对接")
		PrintOption(2, "禁用 xboard 对接")
		PrintOption(3, "测试连接")
		PrintOption(4, "修改配置")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4"})
		switch choice {
		case "1":
			m.enable()
		case "2":
			m.disable()
		case "3":
			m.testConnection()
		case "4":
			m.modifyConfig()
		case "0":
			return
		}
	}
}

func (m *XboardMenu) enable() {
	PrintTitle("启用 xboard 对接")

	apiHost := ReadInput("请输入 API 地址 (https://...)")
	if err := security.ValidateURL(apiHost); err != nil {
		PrintError(fmt.Sprintf("API 地址无效: %v", err))
		return
	}

	apiToken := ReadInput("请输入 API Token")
	if apiToken == "" {
		PrintError("Token 不能为空")
		return
	}

	nodeIDStr := ReadInput("请输入 Node ID")
	var nodeID int
	if _, err := fmt.Sscanf(nodeIDStr, "%d", &nodeID); err != nil || nodeID <= 0 {
		PrintError("Node ID 无效")
		return
	}

	nodeType := ReadInput("请输入 Node Type (vless/vmess/trojan/hysteria/tuic/anytls)")
	if nodeType == "" {
		PrintError("Node Type 不能为空")
		return
	}

	// 测试连接
	client := api.NewClient(apiHost, apiToken, nodeID, nodeType, m.logger)
	if err := client.TestConnection(); err != nil {
		PrintError(fmt.Sprintf("连接测试失败: %v", err))
		if !Confirm("连接失败，是否仍然保存?") {
			return
		}
	} else {
		PrintSuccess("连接测试通过")
	}

	m.config.Standalone = false
	m.config.APIHost = apiHost
	m.config.APIToken = apiToken
	m.config.NodeID = nodeID
	m.config.NodeType = nodeType

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess("xboard 对接已启用，请重启 VasmaX 生效")
}

func (m *XboardMenu) disable() {
	if !Confirm("确认禁用 xboard 对接? 将切换回独立模式") {
		return
	}

	m.config.Standalone = true

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess("已切换回独立模式，请重启 VasmaX 生效")
}

func (m *XboardMenu) testConnection() {
	if m.config.APIHost == "" {
		PrintError("未配置 API 地址")
		return
	}

	client := api.NewClient(m.config.APIHost, m.config.APIToken, m.config.NodeID, m.config.NodeType, m.logger)
	if err := client.TestConnection(); err != nil {
		PrintError(fmt.Sprintf("连接测试失败: %v", err))
	} else {
		PrintSuccess("连接测试通过")
	}
}

func (m *XboardMenu) modifyConfig() {
	PrintTitle("修改 xboard 配置")
	PrintInfo(fmt.Sprintf("当前 API: %s", m.config.APIHost))
	PrintInfo(fmt.Sprintf("当前 NodeID: %d", m.config.NodeID))
	PrintInfo(fmt.Sprintf("当前 NodeType: %s", m.config.NodeType))

	apiHost := ReadInput("新 API 地址（留空不修改）")
	if apiHost != "" {
		if err := security.ValidateURL(apiHost); err != nil {
			PrintError(fmt.Sprintf("API 地址无效: %v", err))
			return
		}
		m.config.APIHost = apiHost
	}

	apiToken := ReadInput("新 API Token（留空不修改）")
	if apiToken != "" {
		m.config.APIToken = apiToken
	}

	nodeIDStr := ReadInput("新 Node ID（留空不修改）")
	if nodeIDStr != "" {
		var nodeID int
		if _, err := fmt.Sscanf(nodeIDStr, "%d", &nodeID); err == nil && nodeID > 0 {
			m.config.NodeID = nodeID
		}
	}

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}
	PrintSuccess("配置已更新")
}
