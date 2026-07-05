package menu

import (
	"errors"
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
			PrintInfo("已开放端口:")
			for _, p := range ports {
				note := p.Note
				if note == "" {
					note = "-"
				}
				PrintInfo(fmt.Sprintf("  %-6d  %-5s  %s", p.Port, p.Protocol, note))
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
	PrintOptionStr("0", "返回")
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

	PrintSuccess("  直接回车跳过")
	note := ReadInput("备注")

	if err := m.addFirewallPortRules(port, proto); err != nil {
		PrintError(fmt.Sprintf("防火墙规则添加失败，端口未保存到配置: %v", err))
		return
	}

	// 保存到配置
	oldPorts := append([]config.ExtraPort(nil), m.config.ExtraPorts...)
	m.config.ExtraPorts = append(m.config.ExtraPorts, config.ExtraPort{
		Port:     port,
		Protocol: proto,
		Note:     note,
	})
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		m.config.ExtraPorts = oldPorts
		_ = m.removeFirewallPortRules(port, proto)
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
	PrintOptionStr("0", "返回")

	input := ReadInput("请输入要关闭的编号")
	if input == "" || input == "0" {
		return
	}
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(ports) {
		PrintError("无效编号")
		return
	}

	p := ports[idx-1]
	if !Confirm(fmt.Sprintf("确认关闭端口 %d/%s?", p.Port, p.Protocol)) {
		return
	}

	if err := m.removeFirewallPortRules(p.Port, p.Protocol); err != nil {
		PrintError(fmt.Sprintf("防火墙规则移除失败，配置未修改: %v", err))
		return
	}

	// 从配置移除
	oldPorts := append([]config.ExtraPort(nil), m.config.ExtraPorts...)
	m.config.ExtraPorts = append(m.config.ExtraPorts[:idx-1], m.config.ExtraPorts[idx:]...)
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		m.config.ExtraPorts = oldPorts
		_ = m.addFirewallPortRules(p.Port, p.Protocol)
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("端口 %d/%s 已关闭", p.Port, p.Protocol))
}

func (m *PortMenu) showFirewallStatus() {
	PrintTitle("防火墙状态")
	if m.firewallMgr == nil || m.firewallMgr.Backend() == nil {
		PrintWarning("未检测到防火墙（ufw / firewalld / iptables）")
		return
	}
	PrintSuccess("防火墙已激活")
	PrintInfo(fmt.Sprintf("已管理 %d 个额外端口", len(m.config.ExtraPorts)))
}

func (m *PortMenu) addFirewallPortRules(port int, proto string) error {
	if m.firewallMgr == nil {
		return nil
	}
	if proto != "both" {
		return m.firewallMgr.AddPort(port, proto)
	}
	if err := m.firewallMgr.AddPort(port, "tcp"); err != nil {
		return err
	}
	if err := m.firewallMgr.AddPort(port, "udp"); err != nil {
		_ = m.firewallMgr.RemovePort(port, "tcp")
		return err
	}
	return nil
}

func (m *PortMenu) removeFirewallPortRules(port int, proto string) error {
	if m.firewallMgr == nil {
		return nil
	}
	if proto != "both" {
		return m.firewallMgr.RemovePort(port, proto)
	}
	tcpErr := m.firewallMgr.RemovePort(port, "tcp")
	udpErr := m.firewallMgr.RemovePort(port, "udp")
	return errors.Join(tcpErr, udpErr)
}
