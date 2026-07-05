package menu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"vasmax/internal/api"
	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/firewall"
	"vasmax/internal/protocol"
	"vasmax/internal/security"
	"vasmax/internal/subscription"
	"vasmax/internal/user"
)

// ProtocolMenus handles protocol-specific management menus.
type ProtocolMenus struct {
	config      *config.Config
	coreMgr     *core.Manager
	registry    *protocol.Registry
	subMgr      *subscription.Manager
	userMgr     *user.Manager
	firewallMgr *firewall.Manager
	logger      *logrus.Logger
}

// NewProtocolMenus creates protocol-specific menus.
func NewProtocolMenus(cfg *config.Config, coreMgr *core.Manager, registry *protocol.Registry, subMgr *subscription.Manager, userMgr *user.Manager, fwMgr *firewall.Manager, logger *logrus.Logger) *ProtocolMenus {
	return &ProtocolMenus{config: cfg, coreMgr: coreMgr, registry: registry, subMgr: subMgr, userMgr: userMgr, firewallMgr: fwMgr, logger: logger}
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
			PrintOption(idx, "Reality 参数（伪装目标/SNI、Reality 密钥）")
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
		hysteriaPort := configuredProtocolPort(m.config, "hysteria2", defaultProtocolPort("hysteria2"))
		PrintInfo(fmt.Sprintf("端口: %d", hysteriaPort))
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
		targetPort := configuredProtocolPort(m.config, "hysteria2", defaultProtocolPort("hysteria2"))
		cfg := &firewall.PortHopConfig{
			StartPort:  start,
			EndPort:    end,
			TargetPort: targetPort,
			Protocol:   "udp",
		}
		if err := m.firewallMgr.SetupPortHopping(cfg); err != nil {
			PrintError(fmt.Sprintf("启用端口跳跃失败: %v", err))
		} else {
			oldStart, oldEnd := m.config.Hysteria2.HopStart, m.config.Hysteria2.HopEnd
			m.config.Hysteria2.HopStart = start
			m.config.Hysteria2.HopEnd = end
			if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
				m.config.Hysteria2.HopStart = oldStart
				m.config.Hysteria2.HopEnd = oldEnd
				_ = m.firewallMgr.RemovePortHopping(cfg)
				PrintError(fmt.Sprintf("保存配置失败，已回滚端口跳跃规则: %v", err))
				return
			}
			PrintSuccess(fmt.Sprintf("端口跳跃已启用: %d-%d -> %d", start, end, targetPort))
		}
	case "2":
		if m.config.Hysteria2.HopStart > 0 {
			oldStart, oldEnd := m.config.Hysteria2.HopStart, m.config.Hysteria2.HopEnd
			targetPort := configuredProtocolPort(m.config, "hysteria2", defaultProtocolPort("hysteria2"))
			cfg := &firewall.PortHopConfig{
				StartPort:  m.config.Hysteria2.HopStart,
				EndPort:    m.config.Hysteria2.HopEnd,
				TargetPort: targetPort,
				Protocol:   "udp",
			}
			if err := m.firewallMgr.RemovePortHopping(cfg); err != nil {
				PrintError(fmt.Sprintf("移除端口跳跃规则失败，配置未修改: %v", err))
				return
			}
			m.config.Hysteria2.HopStart = 0
			m.config.Hysteria2.HopEnd = 0
			if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
				m.config.Hysteria2.HopStart = oldStart
				m.config.Hysteria2.HopEnd = oldEnd
				_ = m.firewallMgr.SetupPortHopping(cfg)
				PrintError(fmt.Sprintf("保存配置失败，已尝试恢复端口跳跃规则: %v", err))
				return
			}
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

	oldDown, oldUp := m.config.Hysteria2.DownMbps, m.config.Hysteria2.UpMbps
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
		m.config.Hysteria2.DownMbps = oldDown
		m.config.Hysteria2.UpMbps = oldUp
		PrintError(fmt.Sprintf("保存失败: %v", err))
	} else {
		if err := m.applySingleProtocolRuntime("hysteria2"); err != nil {
			m.config.Hysteria2.DownMbps = oldDown
			m.config.Hysteria2.UpMbps = oldUp
			if saveErr := config.SaveConfig(config.DefaultConfigPath, m.config); saveErr != nil {
				PrintWarning(fmt.Sprintf("恢复旧 Hysteria2 配置文件失败，请手动检查 config.yaml: %v", saveErr))
			} else if rollbackErr := m.applySingleProtocolRuntime("hysteria2"); rollbackErr != nil {
				PrintWarning(fmt.Sprintf("旧 Hysteria2 运行配置恢复失败，请手动重启或重新安装协议: %v", rollbackErr))
			}
			PrintError(fmt.Sprintf("速度配置应用失败，已恢复旧配置: %v", err))
			return
		}
		m.regenerateSubscriptionsOrWarn()
		PrintSuccess("速度配置已更新并应用")
	}
}

// ShowReality displays the Reality management menu.
func (m *ProtocolMenus) ShowReality() {
	for {
		PrintTitle("Reality 参数管理")
		hasVision := m.realityTargetPoolSupported()
		PrintInfo(fmt.Sprintf("伪装目标 Dest: %s", m.config.Reality.Dest))
		PrintInfo(fmt.Sprintf("SNI ServerName: %s", m.config.Reality.ServerName))
		PrintInfo(fmt.Sprintf("监听端口: %s", formatRealityPorts(m.config)))
		if hasVision && len(m.config.Reality.Targets) > 0 {
			printRealityTargetPoolDetails(m.config)
		}
		if security.IsKnownProblematicRealityDest(m.config.Reality.ServerName) {
			PrintWarning("当前 Reality 伪装目标属于高风险目标，建议改为推荐目标池中的非 Apple/Microsoft 目标")
		}
		PrintSeparator()
		PrintOption(1, "切换为单伪装目标（会关闭 5 目标池；只填域名或 域名:端口，不要带 http://）")
		PrintOption(2, "查看密钥和连接信息")
		PrintOption(3, "重新生成密钥对")
		if hasVision {
			PrintOption(4, "查看 Reality Vision 目标池")
			PrintOption(5, "重置默认 5 目标池（NVIDIA/Samsung/Tesla/Amazon/Mozilla）")
			PrintOption(6, "一键检测推荐伪站池（TLS/H2/延时）")
			PrintOption(7, "一键匹配最低延时伪站并刷新订阅/二维码（切换为单伪站）")
		} else {
			PrintInfo("Reality 目标池仅支持 vless_reality_vision；当前协议仍可使用单伪站切换")
		}
		PrintOptionStr("0", "返回")

		choices := []string{"1", "2", "3", "0"}
		if hasVision {
			choices = append(choices, "4", "5", "6", "7")
		}
		choice := ReadChoice("请选择", choices)
		switch choice {
		case "1":
			dest, serverName, ok := readRealityDestInput()
			if !ok {
				continue
			}
			if applyAndSyncRealityRuntime(m.coreMgr, m.config, m.subMgr, func(next *config.Config) {
				setSingleRealityTarget(next, dest, serverName)
			}) {
				PrintSuccess(fmt.Sprintf("伪装域名已更新: %s (ServerName: %s)", dest, serverName))
			}
		case "2":
			PrintInfo(fmt.Sprintf("PublicKey:   %s", m.config.Reality.PublicKey))
			PrintInfo(fmt.Sprintf("PrivateKey:  %s", m.config.Reality.PrivateKey))
			PrintInfo(fmt.Sprintf("ShortID:     %s", m.config.Reality.ShortID))
			PrintInfo(fmt.Sprintf("ServerName:  %s", m.config.Reality.ServerName))
			PrintInfo(fmt.Sprintf("Dest:        %s", m.config.Reality.Dest))
			fmt.Println()
			m.showRealityShareInfo()
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
			if applyAndSyncRealityRuntime(m.coreMgr, m.config, m.subMgr, func(next *config.Config) {
				next.Reality.PrivateKey = keyPair.PrivateKey
				next.Reality.PublicKey = keyPair.PublicKey
				next.Reality.ShortID = shortID
			}) {
				PrintSuccess("密钥对和 ShortID 已重新生成")
				PrintInfo(fmt.Sprintf("新 PublicKey: %s", keyPair.PublicKey))
				PrintInfo(fmt.Sprintf("新 ShortID:  %s", shortID))
			}
		case "4":
			printRealityTargetPoolDetails(m.config)
		case "5":
			if applyAndSyncRealityRuntime(m.coreMgr, m.config, m.subMgr, func(next *config.Config) {
				resetDefaultRealityTargetPool(next)
			}) {
				PrintSuccess("默认 5 目标池已重置")
				printRealityTargetPoolDetails(m.config)
			}
		case "6":
			detectRealityTargetPool(m.config)
		case "7":
			if _, ok := applyBestRealityTarget(m.coreMgr, m.config, m.subMgr); ok {
				m.showRealityShareInfo()
			}
		case "0":
			return
		}
	}
}

func (m *ProtocolMenus) realityTargetPoolSupported() bool {
	return m.protocolInstalled("vless_reality_vision")
}

func (m *ProtocolMenus) showRealityShareInfo() {
	if m.userMgr == nil {
		PrintWarning("缺少用户上下文，无法显示分享链接和二维码")
		return
	}
	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		PrintWarning("当前没有用户，无法显示分享链接和二维码")
		return
	}
	showRealityInfoForUsers(m.config, m.registry, users)
}

// ShowTuic displays the Tuic management menu.
func (m *ProtocolMenus) ShowTuic() {
	for {
		PrintTitle("Tuic 管理")
		tuicPort := configuredProtocolPort(m.config, "tuic", defaultProtocolPort("tuic"))
		cc := m.config.Tuic.CongestionControl
		if cc == "" {
			cc = "bbr"
		}
		PrintInfo(fmt.Sprintf("端口: %d  拥塞控制: %s", tuicPort, cc))
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
			oldCC := m.config.Tuic.CongestionControl
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
			if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
				m.config.Tuic.CongestionControl = oldCC
				PrintError(fmt.Sprintf("保存失败: %v", err))
				continue
			}
			if err := m.applySingleProtocolRuntime("tuic"); err != nil {
				m.config.Tuic.CongestionControl = oldCC
				if saveErr := config.SaveConfig(config.DefaultConfigPath, m.config); saveErr != nil {
					PrintWarning(fmt.Sprintf("恢复旧 TUIC 配置文件失败，请手动检查 config.yaml: %v", saveErr))
				} else if rollbackErr := m.applySingleProtocolRuntime("tuic"); rollbackErr != nil {
					PrintWarning(fmt.Sprintf("旧 TUIC 运行配置恢复失败，请手动重启或重新安装协议: %v", rollbackErr))
				}
				PrintError(fmt.Sprintf("拥塞控制算法应用失败，已恢复旧配置: %v", err))
				continue
			}
			m.regenerateSubscriptionsOrWarn()
			PrintSuccess("拥塞控制算法已更新并应用")
		case "0":
			return
		}
	}
}

func (m *ProtocolMenus) regenerateSubscriptionsOrWarn() {
	if m != nil && m.subMgr != nil {
		if err := m.subMgr.GenerateAll(); err != nil {
			PrintWarning(fmt.Sprintf("运行配置已应用，但订阅刷新失败，请不要复用旧订阅: %v", err))
		}
	}
}

func (m *ProtocolMenus) applySingleProtocolRuntime(protoName string) error {
	if m == nil || m.config == nil {
		return fmt.Errorf("协议菜单配置为空")
	}
	if m.coreMgr == nil || m.registry == nil || m.userMgr == nil {
		return fmt.Errorf("核心、协议或用户管理器不可用")
	}
	p, ok := m.registry.Get(protoName)
	if !ok {
		return fmt.Errorf("协议未注册: %s", protoName)
	}
	if !m.protocolInstalled(protoName) {
		return nil
	}

	apiUsers := make([]*api.User, 0, len(m.userMgr.GetAllUsers()))
	for _, u := range m.userMgr.GetAllUsers() {
		apiUsers = append(apiUsers, u.ToAPIUser())
	}

	params, err := m.runtimeParamsForProtocol(p, protoName, apiUsers)
	if err != nil {
		return err
	}
	inbounds, err := protocol.GenerateInboundMessages(p, params)
	if err != nil {
		return fmt.Errorf("生成 %s 入站配置失败: %w", protoName, err)
	}
	if err := m.writeRuntimeInbound(p, protoName, inbounds); err != nil {
		return err
	}
	return m.coreMgr.RestartAll()
}

func (m *ProtocolMenus) runtimeParamsForProtocol(p protocol.Protocol, protoName string, users []*api.User) (*protocol.InboundParams, error) {
	domain := m.config.GetProtocolDomain(protoName)
	if domain == "" {
		domain = m.config.TLS.Domain
	}

	certFile := ""
	keyFile := ""
	if !strings.Contains(protoName, "reality") && protoName != "socks5" {
		certFile, keyFile = config.DetectCertPath(&config.TLSConfig{
			Domain:   domain,
			CertFile: m.config.TLS.CertFile,
			KeyFile:  m.config.TLS.KeyFile,
		})
		if p.CoreType() == "singbox" && (certFile == "" || keyFile == "") {
			certPaths, err := security.EnsureSelfSignedCert(config.DefaultTLSDir)
			if err != nil {
				return nil, fmt.Errorf("%s 自签证书生成失败: %w", protoName, err)
			}
			certFile = certPaths.CertFile
			keyFile = certPaths.KeyFile
		}
	}

	params := &protocol.InboundParams{
		Port:          protocol.EffectiveInboundPort(p, m.config),
		Domain:        domain,
		CertFile:      certFile,
		KeyFile:       keyFile,
		Users:         users,
		Tag:           protoName,
		Path:          protocol.DefaultWSPath(p),
		ServiceName:   protocol.DefaultGRPCServiceName(p),
		TLSMinVersion: m.config.TLS.MinVersion,
		TLSMaxVersion: m.config.TLS.MaxVersion,
		ALPN:          m.config.ALPN.ALPNList(),
		KeepAlive:     m.config.Connection,
	}
	if strings.Contains(protoName, "reality") {
		params.Reality = &m.config.Reality
	}
	applyProtocolSpecificInboundParams(params, m.config, protoName)
	return params, nil
}

func (m *ProtocolMenus) writeRuntimeInbound(p protocol.Protocol, protoName string, inbounds []json.RawMessage) error {
	wrapper := map[string]interface{}{"inbounds": inbounds}
	var confDir, fileName string
	switch p.CoreType() {
	case "singbox":
		confDir = m.config.Paths.SingBoxConf
		fileName = fmt.Sprintf("10_%s_inbounds.json", protoName)
	default:
		confDir = m.config.Paths.XrayConf
		fileName = fmt.Sprintf("05_%s_inbounds.json", protoName)
	}
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return fmt.Errorf("创建 %s 配置目录失败: %w", protoName, err)
	}
	confPath := filepath.Join(confDir, fileName)
	if err := security.AtomicWriteJSON(confPath, wrapper, 0644); err != nil {
		return fmt.Errorf("写入 %s 运行配置失败: %w", protoName, err)
	}
	return nil
}
