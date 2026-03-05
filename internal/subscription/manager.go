package subscription

import (
	"fmt"
	"os"
	"path/filepath"

	"vasmax/internal/config"
	"vasmax/internal/protocol"
	"vasmax/internal/security"
	"vasmax/internal/user"

	"github.com/sirupsen/logrus"
)

// Manager 订阅管理器
type Manager struct {
	config   *config.Config
	registry *protocol.Registry
	users    *user.Manager
	salt     string
	logger   *logrus.Logger
}

// NewManager 创建订阅管理器
func NewManager(cfg *config.Config, reg *protocol.Registry, um *user.Manager, logger *logrus.Logger) (*Manager, error) {
	salt, err := LoadOrCreateSalt("/etc/VasmaX")
	if err != nil {
		logger.Warnf("failed to load/create salt: %v, using generated salt", err)
	}
	if cfg.Subscription.Salt != "" {
		salt = cfg.Subscription.Salt
	}
	return &Manager{
		config:   cfg,
		registry: reg,
		users:    um,
		salt:     salt,
		logger:   logger,
	}, nil
}

// GenerateAll 为所有用户生成所有格式订阅文件
func (m *Manager) GenerateAll() error {
	allUsers := m.users.GetAllUsers()
	if len(allUsers) == 0 {
		m.logger.Info("no users, skipping subscription generation")
		return nil
	}

	for _, u := range allUsers {
		if err := m.GenerateForUser(u); err != nil {
			m.logger.Warnf("failed to generate subscription for user %d: %v", u.ID, err)
			// 单个用户失败不影响其他用户
		}
	}
	return nil
}

// GenerateForUser 为单个用户生成所有格式订阅文件
func (m *Manager) GenerateForUser(u *user.UserEntry) error {
	info := m.buildServerInfo()
	protocols := m.getInstalledProtocols()
	apiUser := u.ToAPIUser()

	emailMd5 := GenerateSubscribePath(u.Email, m.salt)
	subDir := filepath.Join(m.config.Paths.Subscribe, emailMd5)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return fmt.Errorf("failed to create subscribe dir: %w", err)
	}

	// 生成 Base64 URI 订阅
	uris := GenerateURIs(protocols, apiUser, info)
	base64Content := EncodeBase64Subscription(uris)
	if err := security.AtomicWrite(filepath.Join(subDir, "default"), []byte(base64Content), 0644); err != nil {
		m.logger.Warnf("failed to write base64 subscription: %v", err)
	}

	// 生成 ClashMeta 订阅
	clashProxies := protocol.GenerateClashProxiesForUser(protocols, apiUser, info)
	if clashData, err := GenerateClashFullProfile(clashProxies, m.config.Subscription.Domain); err == nil {
		if writeErr := security.AtomicWrite(filepath.Join(subDir, "clash"), clashData, 0644); writeErr != nil {
			m.logger.Warnf("failed to write clash subscription: %v", writeErr)
		}
	} else {
		m.logger.Warnf("failed to generate clash profile: %v", err)
	}

	// 生成 sing-box 订阅
	sbOutbounds := protocol.GenerateSingBoxOutboundsForUser(protocols, apiUser, info)
	if sbData, err := GenerateSingBoxFullProfile(sbOutbounds); err == nil {
		if writeErr := security.AtomicWrite(filepath.Join(subDir, "singbox"), sbData, 0644); writeErr != nil {
			m.logger.Warnf("failed to write singbox subscription: %v", writeErr)
		}
	} else {
		m.logger.Warnf("failed to generate singbox profile: %v", err)
	}

	return nil
}

// buildServerInfo 构建服务器连接信息
func (m *Manager) buildServerInfo() *protocol.ServerInfo {
	info := &protocol.ServerInfo{
		Host:   m.config.TLS.Domain,
		Port:   443,
		Domain: m.config.TLS.Domain,
	}
	if m.config.CDN.Enabled && m.config.CDN.Address != "" {
		info.CDNHost = m.config.CDN.Address
	}
	if m.config.Reality.PrivateKey != "" {
		info.Reality = &m.config.Reality
	}
	return info
}

// getInstalledProtocols 获取已安装的协议列表
func (m *Manager) getInstalledProtocols() []protocol.Protocol {
	var protocols []protocol.Protocol
	for _, name := range m.config.Protocols {
		if p, ok := m.registry.Get(name); ok {
			protocols = append(protocols, p)
		}
	}
	return protocols
}
