package menu

import (
	"fmt"
	"strings"

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
		PrintTitle("Xboard-Plus 对接管理")
		if !m.config.Standalone && m.config.APIHost != "" {
			PrintInfo("状态: " + Green("已启用"))
			apiInfo := fmt.Sprintf("面板: %s  节点ID: %d", m.config.APIHost, m.config.NodeID)
			if m.config.APIPrefix != "" {
				apiInfo += fmt.Sprintf("  API前缀: %s", m.config.APIPrefix)
			}
			PrintInfo(apiInfo)
		} else {
			PrintInfo("状态: " + Yellow("未启用"))
		}
		PrintSeparator()
		PrintOption(1, "启用 Xboard-Plus 对接")
		PrintOption(2, "禁用 Xboard-Plus 对接")
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
	PrintTitle("启用 Xboard-Plus 对接")

	PrintInfo("请输入完整面板地址，必须带 http:// 或 https://")
	PrintInfo("示例: http://123.45.67.89:7001 或 https://panel.example.com")
	PrintInfo("如果 Xboard-Plus 设置了自定义 API 路径前缀，无需在此填写")
	apiHost := ReadInput("面板地址（必须带 http:// 或 https://，结尾不需要 /）")
	if err := security.ValidateHTTPURL(apiHost); err != nil {
		PrintError(fmt.Sprintf("地址无效: %v", err))
		return
	}
	apiHost = strings.TrimRight(apiHost, "/")

	apiToken := ReadInput("请输入通信密钥")
	if apiToken == "" {
		PrintError("通信密钥不能为空")
		return
	}

	PrintInfo("如果 Xboard-Plus 后台设置了自定义 API 路径前缀，请在此填写")
	PrintInfo("只填路径前缀，不填域名，也不要带 http:// 或 https://")
	PrintInfo("示例: api、custom-api 或 /custom/node；留空使用默认 api")
	apiPrefix := ReadInput("自定义 API 路径前缀（可留空，不填域名）")
	apiPrefix = security.NormalizeAPIPrefix(apiPrefix)
	if err := security.ValidateAPIPrefix(apiPrefix); err != nil {
		PrintError(fmt.Sprintf("API 路径前缀无效: %v", err))
		return
	}

	nodeIDStr := ReadInput("请输入节点 ID")
	var nodeID int
	if _, err := fmt.Sscanf(nodeIDStr, "%d", &nodeID); err != nil || nodeID <= 0 {
		PrintError("节点 ID 无效")
		return
	}

	nodeType := selectNodeType()
	if nodeType == "" {
		return
	}

	// 测试连接
	client := api.NewClient(apiHost, apiToken, nodeID, nodeType, m.logger)
	if apiPrefix != "" {
		client.SetAPIPrefix(apiPrefix)
	}
	if err := client.TestConnection(); err != nil {
		PrintError(fmt.Sprintf("连接测试失败: %v", err))
		if !Confirm("连接失败，是否仍然保存?") {
			return
		}
	} else {
		PrintSuccess("连接测试通过")
	}

	old := *m.config
	m.config.Standalone = false
	m.config.APIHost = apiHost
	m.config.APIToken = apiToken
	m.config.APIPrefix = apiPrefix
	m.config.NodeID = nodeID
	m.config.NodeType = nodeType

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		*m.config = old
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess("Xboard-Plus 对接已启用，请重启 VasmaX 生效")
}

func (m *XboardMenu) disable() {
	if !Confirm("确认禁用 Xboard-Plus 对接? 将切换回独立模式") {
		return
	}

	old := *m.config
	m.config.Standalone = true

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		*m.config = old
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
	if m.config.APIPrefix != "" {
		client.SetAPIPrefix(m.config.APIPrefix)
	}
	if err := client.TestConnection(); err != nil {
		PrintError(fmt.Sprintf("连接测试失败: %v", err))
	} else {
		PrintSuccess("连接测试通过")
	}
}

func (m *XboardMenu) modifyConfig() {
	PrintTitle("修改 Xboard-Plus 配置")
	old := *m.config
	PrintInfo(fmt.Sprintf("当前面板地址: %s", m.config.APIHost))
	if m.config.APIPrefix != "" {
		PrintInfo(fmt.Sprintf("当前 API 前缀: %s", m.config.APIPrefix))
	} else {
		PrintInfo("当前 API 前缀: api (默认)")
	}
	PrintInfo(fmt.Sprintf("当前节点ID: %d", m.config.NodeID))
	PrintInfo(fmt.Sprintf("当前节点类型: %s", m.config.NodeType))

	PrintSuccess("  直接回车不修改")
	apiHost := ReadInput("新面板地址（必须带 http:// 或 https://，结尾不需要 /）")
	if apiHost != "" {
		if err := security.ValidateHTTPURL(apiHost); err != nil {
			PrintError(fmt.Sprintf("地址无效: %v", err))
			return
		}
		m.config.APIHost = strings.TrimRight(apiHost, "/")
	}

	PrintSuccess("  直接回车不修改")
	apiToken := ReadInput("新通信密钥")
	if apiToken != "" {
		m.config.APIToken = apiToken
	}

	PrintSuccess("  直接回车不修改，输入 clear 清除自定义前缀恢复默认")
	PrintInfo("API 前缀只填路径，不填域名，也不要带 http:// 或 https://；示例: api、custom-api 或 /custom/node")
	apiPrefix := strings.TrimSpace(ReadInput("新 API 路径前缀（可留空，不填域名）"))
	if strings.EqualFold(apiPrefix, "clear") {
		m.config.APIPrefix = ""
		PrintInfo("已恢复默认 API 路径")
	} else if apiPrefix != "" {
		apiPrefix = security.NormalizeAPIPrefix(apiPrefix)
		if err := security.ValidateAPIPrefix(apiPrefix); err != nil {
			PrintError(fmt.Sprintf("API 路径前缀无效: %v", err))
			return
		}
		m.config.APIPrefix = apiPrefix
	}

	PrintSuccess("  直接回车不修改")
	nodeIDStr := ReadInput("新节点 ID")
	if nodeIDStr != "" {
		var nodeID int
		if _, err := fmt.Sscanf(nodeIDStr, "%d", &nodeID); err == nil && nodeID > 0 {
			m.config.NodeID = nodeID
		}
	}

	PrintSuccess("  直接回车不修改（输入 0 跳过）")
	nodeType := selectNodeType()
	if nodeType != "" {
		m.config.NodeType = nodeType
	}

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		*m.config = old
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}
	PrintSuccess("配置已更新")
}

// selectNodeType 显示节点类型选择列表，返回选中的类型名称。
// 选择 0 返回空字符串表示跳过/取消。
func selectNodeType() string {
	types := []string{"vless", "vmess", "trojan", "hysteria", "tuic", "anytls"}
	PrintSeparator()
	PrintInfo("请选择节点类型:")
	for i, t := range types {
		PrintOption(i+1, t)
	}
	PrintOptionStr("0", "取消")

	choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5", "6"})
	switch choice {
	case "0":
		return ""
	default:
		var idx int
		if _, err := fmt.Sscanf(choice, "%d", &idx); err == nil && idx >= 1 && idx <= len(types) {
			return types[idx-1]
		}
		PrintError("无效选择")
		return ""
	}
}
