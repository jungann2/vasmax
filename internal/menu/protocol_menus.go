package menu

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/firewall"
)

// ProtocolMenus handles protocol-specific management menus.
type ProtocolMenus struct {
	config      *config.Config
	firewallMgr *firewall.Manager
	logger      *logrus.Logger
}

// NewProtocolMenus creates protocol-specific menus.
func NewProtocolMenus(cfg *config.Config, fwMgr *firewall.Manager, logger *logrus.Logger) *ProtocolMenus {
	return &ProtocolMenus{config: cfg, firewallMgr: fwMgr, logger: logger}
}

// ShowHysteria2 displays the Hysteria2 management menu.
func (m *ProtocolMenus) ShowHysteria2() {
	for {
		PrintTitle("Hysteria2 管理")
		PrintInfo(fmt.Sprintf("端口: %d", m.config.Hysteria2.Port))
		if m.config.Hysteria2.HopStart > 0 {
			PrintInfo(fmt.Sprintf("端口跳跃: %d-%d", m.config.Hysteria2.HopStart, m.config.Hysteria2.HopEnd))
		}
		PrintSeparator()
		PrintOption(1, "端口跳跃管理")
		PrintOption(2, "网络速度配置")
		PrintOption(3, "查看账号")
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1", "2", "3"})
		switch choice {
		case "1":
			m.hysteria2PortHop()
		case "2":
			m.hysteria2Speed()
		case "3":
			PrintInfo("查看账号 - 请使用账号管理菜单")
		case "0":
			return
		}
	}
}

func (m *ProtocolMenus) hysteria2PortHop() {
	PrintTitle("Hysteria2 端口跳跃")
	PrintOption(1, "启用端口跳跃")
	PrintOption(2, "禁用端口跳跃")
	PrintOptionStr("0", "返回")

	choice := ReadChoice("请选择", []string{"1", "2"})
	switch choice {
	case "1":
		start, end := firewall.DefaultPortHopRange()
		cfg := &firewall.PortHopConfig{
			StartPort:  start,
			EndPort:    end,
			TargetPort: m.config.Hysteria2.Port,
			Protocol:   "udp",
		}
		if err := m.firewallMgr.SetupPortHopping(cfg); err != nil {
			PrintError(fmt.Sprintf("启用端口跳跃失败: %v", err))
		} else {
			m.config.Hysteria2.HopStart = start
			m.config.Hysteria2.HopEnd = end
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess(fmt.Sprintf("端口跳跃已启用: %d-%d -> %d", start, end, m.config.Hysteria2.Port))
		}
	case "2":
		if m.config.Hysteria2.HopStart > 0 {
			cfg := &firewall.PortHopConfig{
				StartPort:  m.config.Hysteria2.HopStart,
				EndPort:    m.config.Hysteria2.HopEnd,
				TargetPort: m.config.Hysteria2.Port,
				Protocol:   "udp",
			}
			_ = m.firewallMgr.RemovePortHopping(cfg)
			m.config.Hysteria2.HopStart = 0
			m.config.Hysteria2.HopEnd = 0
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess("端口跳跃已禁用")
		} else {
			PrintInfo("端口跳跃未启用")
		}
	}
}

func (m *ProtocolMenus) hysteria2Speed() {
	PrintInfo(fmt.Sprintf("当前下行: %d Mbps  上行: %d Mbps", m.config.Hysteria2.DownMbps, m.config.Hysteria2.UpMbps))
	downStr := ReadInput("下行速度 (Mbps，留空不修改)")
	upStr := ReadInput("上行速度 (Mbps，留空不修改)")

	if downStr != "" {
		var v int
		if _, err := fmt.Sscanf(downStr, "%d", &v); err == nil && v > 0 {
			m.config.Hysteria2.DownMbps = v
		}
	}
	if upStr != "" {
		var v int
		if _, err := fmt.Sscanf(upStr, "%d", &v); err == nil && v > 0 {
			m.config.Hysteria2.UpMbps = v
		}
	}

	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存失败: %v", err))
	} else {
		PrintSuccess("速度配置已更新")
	}
}

// ShowReality displays the Reality management menu.
func (m *ProtocolMenus) ShowReality() {
	for {
		PrintTitle("Reality 管理")
		PrintInfo(fmt.Sprintf("Dest: %s", m.config.Reality.Dest))
		PrintSeparator()
		PrintOption(1, "修改 dest 域名")
		PrintOption(2, "查看密钥信息")
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1", "2"})
		switch choice {
		case "1":
			dest := ReadInput("请输入新的 dest 域名 (如 www.microsoft.com:443)")
			if dest != "" {
				m.config.Reality.Dest = dest
				_ = config.SaveConfig(config.DefaultConfigPath, m.config)
				PrintSuccess("dest 域名已更新")
			}
		case "2":
			PrintInfo(fmt.Sprintf("PublicKey: %s", m.config.Reality.PublicKey))
			PrintInfo(fmt.Sprintf("ShortID: %s", m.config.Reality.ShortID))
			PrintInfo(fmt.Sprintf("ServerName: %s", m.config.Reality.ServerName))
		case "0":
			return
		}
	}
}

// ShowTuic displays the Tuic management menu.
func (m *ProtocolMenus) ShowTuic() {
	for {
		PrintTitle("Tuic 管理")
		PrintInfo(fmt.Sprintf("端口: %d  拥塞控制: %s", m.config.Tuic.Port, m.config.Tuic.CongestionControl))
		PrintSeparator()
		PrintOption(1, "修改拥塞控制算法")
		PrintOption(2, "端口跳跃管理")
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1", "2"})
		switch choice {
		case "1":
			PrintOption(1, "bbr")
			PrintOption(2, "cubic")
			PrintOption(3, "new_reno")
			cc := ReadChoice("选择算法", []string{"1", "2", "3"})
			switch cc {
			case "1":
				m.config.Tuic.CongestionControl = "bbr"
			case "2":
				m.config.Tuic.CongestionControl = "cubic"
			case "3":
				m.config.Tuic.CongestionControl = "new_reno"
			}
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess("拥塞控制算法已更新")
		case "2":
			PrintInfo("Tuic 端口跳跃 - 与 Hysteria2 类似")
		case "0":
			return
		}
	}
}
