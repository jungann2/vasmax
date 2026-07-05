package menu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vasmax/internal/api"
	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/protocol"
	"vasmax/internal/security"
	"vasmax/internal/user"
)

func regenerateInstalledProtocolRuntime(cfg *config.Config, coreMgr *core.Manager, registry *protocol.Registry, userMgr *user.Manager) error {
	if err := writeInstalledProtocolRuntimeFiles(cfg, registry, userMgr); err != nil {
		return err
	}
	if cfg == nil || len(cfg.Protocols) == 0 {
		return nil
	}
	if coreMgr == nil {
		return fmt.Errorf("核心管理器不可用")
	}
	return coreMgr.RestartAll()
}

func writeInstalledProtocolRuntimeFiles(cfg *config.Config, registry *protocol.Registry, userMgr *user.Manager) error {
	if cfg == nil {
		return fmt.Errorf("配置为空")
	}
	if len(cfg.Protocols) == 0 {
		return nil
	}
	if registry == nil || userMgr == nil {
		return fmt.Errorf("协议或用户管理器不可用")
	}

	apiUsers := make([]*api.User, 0, len(userMgr.GetAllUsers()))
	for _, u := range userMgr.GetAllUsers() {
		apiUsers = append(apiUsers, u.ToAPIUser())
	}

	for _, protoName := range cfg.Protocols {
		p, ok := registry.Get(protoName)
		if !ok {
			continue
		}
		params, err := runtimeParamsForProtocolConfig(cfg, p, protoName, apiUsers)
		if err != nil {
			return err
		}
		inbounds, err := protocol.GenerateInboundMessages(p, params)
		if err != nil {
			return fmt.Errorf("生成 %s 入站配置失败: %w", protoName, err)
		}
		if err := writeRuntimeInboundConfig(cfg, p, protoName, inbounds); err != nil {
			return err
		}
	}
	return nil
}

func runtimeParamsForProtocolConfig(cfg *config.Config, p protocol.Protocol, protoName string, users []*api.User) (*protocol.InboundParams, error) {
	domain := cfg.GetProtocolDomain(protoName)
	if domain == "" {
		domain = cfg.TLS.Domain
	}

	certFile := ""
	keyFile := ""
	if !strings.Contains(protoName, "reality") && protoName != "socks5" {
		certFile, keyFile = config.DetectCertPath(&config.TLSConfig{
			Domain:   domain,
			CertFile: cfg.TLS.CertFile,
			KeyFile:  cfg.TLS.KeyFile,
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
		Port:          protocol.EffectiveInboundPort(p, cfg),
		Domain:        domain,
		CertFile:      certFile,
		KeyFile:       keyFile,
		Users:         users,
		Tag:           protoName,
		Path:          protocol.DefaultWSPath(p),
		ServiceName:   protocol.DefaultGRPCServiceName(p),
		TLSMinVersion: cfg.TLS.MinVersion,
		TLSMaxVersion: cfg.TLS.MaxVersion,
		ALPN:          cfg.ALPN.ALPNList(),
		KeepAlive:     cfg.Connection,
	}
	if strings.Contains(protoName, "reality") {
		params.Reality = &cfg.Reality
	}
	applyProtocolSpecificInboundParams(params, cfg, protoName)
	return params, nil
}

func writeRuntimeInboundConfig(cfg *config.Config, p protocol.Protocol, protoName string, inbounds []json.RawMessage) error {
	wrapper := map[string]interface{}{"inbounds": inbounds}
	var confDir, fileName string
	switch p.CoreType() {
	case "singbox":
		confDir = cfg.Paths.SingBoxConf
		fileName = fmt.Sprintf("10_%s_inbounds.json", protoName)
	default:
		confDir = cfg.Paths.XrayConf
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
