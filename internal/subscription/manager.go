package subscription

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	salt, err := LoadOrCreateSalt("/etc/vasmax")
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
	protocols := m.getInstalledProtocols()
	apiUser := u.ToAPIUser()

	emailMd5 := GenerateSubscribePath(u.Email, m.salt)
	subDir := filepath.Join(m.config.Paths.Subscribe, emailMd5)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return fmt.Errorf("failed to create subscribe dir: %w", err)
	}

	// 为每个协议单独构建 ServerInfo（端口、路径各不相同）
	// 无域名模式协议需要服务器 IP，懒加载一次
	serverIP := ""
	getServerIP := func() string {
		if serverIP != "" {
			return serverIP
		}
		serverIP = m.resolveServerIP()
		return serverIP
	}

	var uris []string
	var clashProxies []map[string]interface{}
	var sbOutbounds []map[string]interface{}
	for _, p := range protocols {
		info := m.buildServerInfoForProtocol(p, getServerIP)

		uri := p.GenerateURI(apiUser, info)
		if uri != "" {
			uris = append(uris, uri)
		}
		if proxy := p.GenerateClashProxy(apiUser, info); proxy != nil {
			clashProxies = append(clashProxies, proxy)
		}
		if ob := p.GenerateSingBoxOutbound(apiUser, info); ob != nil {
			sbOutbounds = append(sbOutbounds, ob)
		}
	}

	// 生成 Base64 URI 订阅
	base64Content := EncodeBase64Subscription(uris)

	// 合并远程订阅（如果有配置）
	baseDir := filepath.Dir(m.config.Paths.Subscribe)
	remoteSubs, _ := LoadRemoteSubscriptions(baseDir)
	if len(remoteSubs) > 0 {
		var remoteContents [][]byte
		var aliases []string
		for _, rs := range remoteSubs {
			data, err := FetchRemote(&rs)
			if err != nil {
				m.logger.Warnf("拉取远程订阅失败 [%s]: %v", rs.Alias, err)
				continue
			}
			remoteContents = append(remoteContents, data)
			aliases = append(aliases, rs.Alias)
		}
		if len(remoteContents) > 0 {
			merged, err := MergeRemote([]byte(base64Content), remoteContents, aliases)
			if err != nil {
				m.logger.Warnf("合并远程订阅失败: %v", err)
			} else {
				base64Content = string(merged)
			}
		}
	}

	if err := security.AtomicWrite(filepath.Join(subDir, "default"), []byte(base64Content), 0644); err != nil {
		m.logger.Warnf("failed to write base64 subscription: %v", err)
	}

	// 生成 ClashMeta 订阅
	subDomain := m.config.Subscription.Domain
	if subDomain == "" {
		subDomain = m.config.TLS.Domain
	}
	profileOptions := ProfileOptionsFromConfig(m.config.Subscription)
	if clashData, err := GenerateClashFullProfileWithOptions(clashProxies, subDomain, profileOptions); err == nil {
		if writeErr := security.AtomicWrite(filepath.Join(subDir, "clash"), clashData, 0644); writeErr != nil {
			m.logger.Warnf("failed to write clash subscription: %v", writeErr)
		}
	} else {
		m.logger.Warnf("failed to generate clash profile: %v", err)
	}

	// 生成 sing-box 订阅
	if sbData, err := GenerateSingBoxFullProfileWithOptions(sbOutbounds, profileOptions); err == nil {
		if writeErr := security.AtomicWrite(filepath.Join(subDir, "singbox"), sbData, 0644); writeErr != nil {
			m.logger.Warnf("failed to write singbox subscription: %v", writeErr)
		}
	} else {
		m.logger.Warnf("failed to generate singbox profile: %v", err)
	}

	return nil
}

// buildServerInfoForProtocol 为指定协议构建正确的 ServerInfo
func (m *Manager) buildServerInfoForProtocol(p protocol.Protocol, getIP func() string) *protocol.ServerInfo {
	// 外部端口：Nginx 反代协议用 443，直连协议用配置端口或默认端口。
	port := protocol.ExternalPort(p, m.config)

	// 域名：优先使用协议独立域名，否则全局域名
	domain := m.config.GetProtocolDomain(p.Name())
	if domain == "" {
		domain = m.config.TLS.Domain
	}

	// 无域名模式（Reality / sing-box nodomain）
	mode := ""
	if m.config.ProtocolModes != nil {
		mode = m.config.ProtocolModes[p.Name()]
	}
	if mode == "" && strings.Contains(p.Name(), "reality") {
		mode = "nodomain"
	}

	host := domain
	if mode == "nodomain" {
		// 无域名模式：用服务器公网 IP 作为连接地址
		ip := getIP()
		host = ip
		domain = ip
	}

	// WS/HTTPUpgrade 路径
	path := ""
	switch p.TransportType() {
	case "ws", "httpupgrade", "xhttp":
		path = protocol.DefaultWSPath(p)
	}

	// gRPC serviceName
	serviceName := ""
	if p.TransportType() == "grpc" {
		serviceName = protocol.DefaultGRPCServiceName(p)
	}

	info := &protocol.ServerInfo{
		Host:        host,
		Port:        port,
		Domain:      domain,
		Path:        path,
		ServiceName: serviceName,
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

// resolveServerIP 获取服务器公网 IP（用于无域名模式订阅链接）
func (m *Manager) resolveServerIP() string {
	apis := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range apis {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
}
