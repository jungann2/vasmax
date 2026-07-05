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
	"vasmax/internal/protocol"
	"vasmax/internal/security"
	"vasmax/internal/subscription"
	"vasmax/internal/user"
)

// ALPNMenu handles ALPN switching for TLS protocols.
type ALPNMenu struct {
	config   *config.Config
	coreMgr  *core.Manager
	registry *protocol.Registry
	users    *user.Manager
	subMgr   *subscription.Manager
	logger   *logrus.Logger
}

// NewALPNMenu creates a new ALPN menu.
func NewALPNMenu(cfg *config.Config, coreMgr *core.Manager, reg *protocol.Registry, userMgr *user.Manager, subMgr *subscription.Manager, logger *logrus.Logger) *ALPNMenu {
	return &ALPNMenu{config: cfg, coreMgr: coreMgr, registry: reg, users: userMgr, subMgr: subMgr, logger: logger}
}

// Show displays the ALPN switching menu.
func (m *ALPNMenu) Show() {
	for {
		PrintTitle("ALPN 切换")
		PrintInfo(fmt.Sprintf("当前模式: %s", m.currentModeLabel()))
		PrintInfo("ALPN 影响直连 TLS 协议的握手协商；WS/gRPC/HTTPUpgrade 由 Nginx 终结 TLS")
		PrintSeparator()
		PrintOption(1, "h2 + http/1.1（默认，兼容性最好）")
		PrintOption(2, "仅 h2（HTTP/2 专用，性能更好）")
		PrintOption(3, "仅 http/1.1（旧客户端兼容）")
		PrintOption(4, "仅 h3（QUIC 协议专用，如 Hysteria2/TUIC）")
		PrintOption(5, "h2 + http/1.1 + h3（全开，客户端自动协商最优）")
		PrintInfo("")
		PrintInfo("注: h3 仅对 QUIC 类协议有效；Nginx 反代协议固定由 Nginx 提供 h2/http/1.1")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5"})
		switch choice {
		case "1":
			m.setMode("h2_http11")
		case "2":
			m.setMode("h2_only")
		case "3":
			m.setMode("http11_only")
		case "4":
			m.setMode("h3_only")
		case "5":
			m.setMode("all")
		case "0":
			return
		}
	}
}

func (m *ALPNMenu) setMode(mode string) {
	if m.config.ALPN.Mode == mode {
		PrintInfo("已是当前模式，无需修改")
		return
	}
	oldMode := m.config.ALPN.Mode
	m.config.ALPN.Mode = mode
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		m.config.ALPN.Mode = oldMode
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}
	if err := m.regenerateRuntimeConfigs(); err != nil {
		PrintWarning(fmt.Sprintf("ALPN 运行配置应用失败，正在恢复旧配置: %v", err))
		m.config.ALPN.Mode = oldMode
		if saveErr := config.SaveConfig(config.DefaultConfigPath, m.config); saveErr != nil {
			PrintError(fmt.Sprintf("恢复旧 ALPN 配置失败，请手动检查 config.yaml: %v", saveErr))
			return
		}
		if rollbackErr := m.regenerateRuntimeConfigs(); rollbackErr != nil {
			PrintError(fmt.Sprintf("旧运行配置恢复失败，请手动重启或重新安装协议: %v", rollbackErr))
			return
		}
		PrintError("ALPN 切换失败，已恢复旧配置")
		return
	}
	if m.subMgr != nil {
		if err := m.subMgr.GenerateAll(); err != nil {
			PrintWarning(fmt.Sprintf("ALPN 已应用，但订阅刷新失败，请不要复用旧订阅: %v", err))
			return
		}
	}
	PrintSuccess(fmt.Sprintf("ALPN 已切换为: %s", m.currentModeLabel()))
	if len(m.config.Protocols) == 0 {
		PrintInfo("当前未安装协议，新 ALPN 会在后续安装时生效")
		return
	}
	PrintInfo("已重新生成运行配置并重启当前使用的核心")
}

func (m *ALPNMenu) currentModeLabel() string {
	switch m.config.ALPN.Mode {
	case "h2_only":
		return "仅 h2"
	case "http11_only":
		return "仅 http/1.1"
	case "h3_only":
		return "仅 h3"
	case "all":
		return "全部支持（h2 + http/1.1 + h3）"
	default:
		return "h2 + http/1.1（默认）"
	}
}

// ALPNList 根据配置返回当前 ALPN 列表（供协议生成时使用）
// Deprecated: 使用 config.ALPNConfig.ALPNList() 代替
func ALPNList(cfg *config.Config) []string {
	return cfg.ALPN.ALPNList()
}

func (m *ALPNMenu) regenerateRuntimeConfigs() error {
	if m == nil || m.config == nil {
		return fmt.Errorf("ALPN 菜单配置为空")
	}
	if len(m.config.Protocols) == 0 {
		return nil
	}
	if m.coreMgr == nil || m.registry == nil || m.users == nil {
		return fmt.Errorf("核心、协议或用户管理器不可用")
	}

	apiUsers := make([]*api.User, 0, len(m.users.GetAllUsers()))
	for _, u := range m.users.GetAllUsers() {
		apiUsers = append(apiUsers, u.ToAPIUser())
	}

	for _, protoName := range m.config.Protocols {
		p, ok := m.registry.Get(protoName)
		if !ok {
			continue
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
	}
	return m.coreMgr.RestartAll()
}

func (m *ALPNMenu) runtimeParamsForProtocol(p protocol.Protocol, protoName string, users []*api.User) (*protocol.InboundParams, error) {
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

func (m *ALPNMenu) writeRuntimeInbound(p protocol.Protocol, protoName string, inbounds []json.RawMessage) error {
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
