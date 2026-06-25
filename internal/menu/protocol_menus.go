package menu

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/firewall"
	"vasmax/internal/security"
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

// Show displays installed protocol-specific management entries.
func (m *ProtocolMenus) Show() {
	for {
		PrintTitle("协议专项管理")
		PrintInfo("这里只显示已安装且有专项参数可调的协议")
		PrintInfo("安装/卸载协议请返回主菜单进入「安装管理」")
		PrintSeparator()

		handlers := make(map[string]func())
		idx := 1
		if m.hasRealityProtocol() {
			key := fmt.Sprintf("%d", idx)
			PrintOption(idx, "Reality 管理（伪装域名、Reality 密钥）")
			handlers[key] = m.ShowReality
			idx++
		}
		if m.protocolInstalled("hysteria2") {
			key := fmt.Sprintf("%d", idx)
			PrintOption(idx, "Hysteria2 管理（端口跳跃、上下行速率）")
			handlers[key] = m.ShowHysteria2
			idx++
		}
		if m.protocolInstalled("tuic") {
			key := fmt.Sprintf("%d", idx)
			PrintOption(idx, "TUIC 管理（拥塞控制算法）")
			handlers[key] = m.ShowTuic
			idx++
		}

		if len(handlers) == 0 {
			PrintWarning("当前没有已安装的专项管理协议")
			PrintInfo("可管理协议: Reality、Hysteria2、TUIC")
			ReadInput("按 Enter 返回")
			return
		}

		PrintOptionStr("0", "返回上级菜单")
		choices := make([]string, 0, len(handlers))
		for i := 1; i < idx; i++ {
			choices = append(choices, fmt.Sprintf("%d", i))
		}
		choice := ReadChoice("请选择", choices)
		if choice == "0" {
			return
		}
		if handler := handlers[choice]; handler != nil {
			handler()
		}
	}
}

func (m *ProtocolMenus) protocolInstalled(name string) bool {
	for _, protoName := range m.config.Protocols {
		if protoName == name {
			return true
		}
	}
	return false
}

func (m *ProtocolMenus) hasRealityProtocol() bool {
	if m.config.Reality.PrivateKey != "" {
		return true
	}
	for _, protoName := range m.config.Protocols {
		if strings.HasPrefix(protoName, "vless_reality_") {
			return true
		}
	}
	return false
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
	PrintSuccess("  直接回车不修改")
	downStr := ReadInput("下行速度 (Mbps)")
	upStr := ReadInput("上行速度 (Mbps)")

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
		PrintInfo(fmt.Sprintf("ServerName: %s", m.config.Reality.ServerName))
		PrintSeparator()
		PrintOption(1, "修改伪装域名")
		PrintOption(2, "查看密钥信息")
		PrintOption(3, "重新生成密钥对")
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1", "2", "3"})
		switch choice {
		case "1":
			dest := ReadInput("请输入新的伪装域名（只填域名或 域名:端口，不要带 http:// 或 https://，如 www.apple.com）")
			if dest != "" {
				if strings.Contains(dest, "://") {
					PrintError("伪装域名不要带 http:// 或 https://")
					continue
				}
				if !strings.Contains(dest, ":") {
					dest = dest + ":443"
				}
				// 提取域名部分作为 ServerName
				serverName := strings.Split(dest, ":")[0]
				m.config.Reality.Dest = dest
				m.config.Reality.ServerName = serverName
				_ = config.SaveConfig(config.DefaultConfigPath, m.config)
				PrintSuccess(fmt.Sprintf("伪装域名已更新: %s (ServerName: %s)", dest, serverName))
			}
		case "2":
			PrintInfo(fmt.Sprintf("PublicKey:   %s", m.config.Reality.PublicKey))
			PrintInfo(fmt.Sprintf("PrivateKey:  %s", m.config.Reality.PrivateKey))
			PrintInfo(fmt.Sprintf("ShortID:     %s", m.config.Reality.ShortID))
			PrintInfo(fmt.Sprintf("ServerName:  %s", m.config.Reality.ServerName))
			PrintInfo(fmt.Sprintf("Dest:        %s", m.config.Reality.Dest))
		case "3":
			if !Confirm("重新生成密钥对将导致所有客户端需要更新配置，确认?") {
				continue
			}
			keyPair, err := security.GenerateX25519KeyPair()
			if err != nil {
				PrintError(fmt.Sprintf("生成密钥对失败: %v", err))
				continue
			}
			shortID, err := security.GenerateShortID()
			if err != nil {
				PrintError(fmt.Sprintf("生成 ShortID 失败: %v", err))
				continue
			}
			m.config.Reality.PrivateKey = keyPair.PrivateKey
			m.config.Reality.PublicKey = keyPair.PublicKey
			m.config.Reality.ShortID = shortID
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess("密钥对和 ShortID 已重新生成")
			PrintInfo(fmt.Sprintf("新 PublicKey: %s", keyPair.PublicKey))
			PrintInfo(fmt.Sprintf("新 ShortID:  %s", shortID))
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
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1"})
		switch choice {
		case "1":
			PrintOption(1, "bbr")
			PrintOption(2, "cubic")
			PrintOption(3, "new_reno")
			PrintOptionStr("0", "返回")
			cc := ReadChoice("选择算法", []string{"1", "2", "3"})
			switch cc {
			case "1":
				m.config.Tuic.CongestionControl = "bbr"
			case "2":
				m.config.Tuic.CongestionControl = "cubic"
			case "3":
				m.config.Tuic.CongestionControl = "new_reno"
			case "0":
				continue
			}
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess("拥塞控制算法已更新")
		case "0":
			return
		}
	}
}
