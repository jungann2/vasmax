package menu

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/firewall"
)

// PortMenu handles extra port management.
type PortMenu struct {
	config      *config.Config
	firewallMgr *firewall.Manager
	logger      *logrus.Logger
}

// NewPortMenu creates a new port management menu.
func NewPortMenu(cfg *config.Config, fwMgr *firewall.Manager, logger *logrus.Logger) *PortMenu {
	return &PortMenu{config: cfg, firewallMgr: fwMgr, logger: logger}
}

// Show displays the extra port management menu.
func (m *PortMenu) Show() {
	for {
		PrintTitle("额外端口管理")
		ports := m.config.ExtraPorts
		if len(ports) == 0 {
			PrintInfo("暂无额外开放端口")
		} else {
			for i, p := range ports {
				note := p.Note
				if note == "" {
					note = "-"
				}
				PrintOption(i+1, fmt.Sprintf("%-6d  %-5s  %s", p.Port, p.Protocol, note))
			}
		}
		PrintSeparator()
		PrintOption(1, "开放新端口")
		PrintOption(2, "关闭端口")
		PrintOption(3, "查看防火墙状态")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3"})
		switch choice {
		case "1":
			m.openPort()
		case "2":
			m.closePort()
		case "3":
			m.showFirewallStatus()
		case "0":
			return
		}
	}
}

func (m *PortMenu) openPort() {
	PrintTitle("开放新端口")

	portStr := ReadInput("请输入端口号 (1-65535)")
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil || port < 1 || port > 65535 {
		PrintError("端口号无效")
		return
	}

	// 检查是否已存在
	for _, p := range m.config.ExtraPorts {
		if p.Port == port {
			PrintWarning(fmt.Sprintf("端口 %d 已在列表中", port))
			return
		}
	}

	PrintOption(1, "TCP")
	PrintOption(2, "UDP")
	PrintOption(3, "TCP + UDP")
	protoChoice := ReadChoice("选择协议", []string{"1", "2", "3"})
	var proto string
	switch protoChoice {
	case "1":
		proto = "tcp"
	case "2":
		proto = "udp"
	case "3":
		proto = "both"
	case "0":
		return
	}

	note := ReadInput("备注（留空跳过）")

	// 开放防火墙
	var fwErr error
	if proto == "both" {
		fwErr = m.firewallMgr.AddPort(port, "tcp")
		if fwErr == nil {
			fwErr = m.firewallMgr.AddPort(port, "udp")
		}
	} else {
		fwErr = m.firewallMgr.AddPort(port, proto)
	}
	if fwErr != nil {
		PrintWarning(fmt.Sprintf("防火墙规则添加失败（可能无防火墙）: %v", fwErr))
	}

	// 保存到配置
	m.config.ExtraPorts = append(m.config.ExtraPorts, config.ExtraPort{
		Port:     port,
		Protocol: proto,
		Note:     note,
	})
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("端口 %d/%s 已开放", port, proto))
}

func (m *PortMenu) closePort() {
	PrintTitle("关闭端口")
	ports := m.config.ExtraPorts
	if len(ports) == 0 {
		PrintInfo("暂无额外端口")
		return
	}
	for i, p := range ports {
		PrintOption(i+1, fmt.Sprintf("%d/%s  %s", p.Port, p.Protocol, p.Note))
	}

	input := ReadInput("请输入要关闭的编号")
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(ports) {
		PrintError("无效编号")
		return
	}

	p := ports[idx-1]
	if !Confirm(fmt.Sprintf("确认关闭端口 %d/%s?", p.Port, p.Protocol)) {
		return
	}

	// 移除防火墙规则
	if p.Protocol == "both" {
		m.firewallMgr.RemovePort(p.Port, "tcp")
		m.firewallMgr.RemovePort(p.Port, "udp")
	} else {
		m.firewallMgr.RemovePort(p.Port, p.Protocol)
	}

	// 从配置移除
	m.config.ExtraPorts = append(ports[:idx-1], ports[idx:]...)
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("端口 %d/%s 已关闭", p.Port, p.Protocol))
}

func (m *PortMenu) showFirewallStatus() {
	PrintTitle("防火墙状态")
	if m.firewallMgr.Backend() == nil {
		PrintWarning("未检测到防火墙（ufw / firewalld / iptables）")
		return
	}
	PrintSuccess("防火墙已激活")
	PrintInfo(fmt.Sprintf("已管理 %d 个额外端口", len(m.config.ExtraPorts)))
}
