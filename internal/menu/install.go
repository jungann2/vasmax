package menu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"vasmax/internal/api"
	"vasmax/internal/config"
	"vasmax/internal/core"
	"vasmax/internal/nginx"
	"vasmax/internal/protocol"
	"vasmax/internal/rollback"
	"vasmax/internal/security"
	"vasmax/internal/subscription"
	"vasmax/internal/sysinfo"
	"vasmax/internal/user"
)

// InstallMenu handles protocol installation management.
type InstallMenu struct {
	config      *config.Config
	coreMgr     *core.Manager
	registry    *protocol.Registry
	rollbackMgr *rollback.Manager
	nginxMgr    *nginx.Manager
	userMgr     *user.Manager
	subMgr      *subscription.Manager
	logger      *logrus.Logger
}

// NewInstallMenu creates a new install menu.
func NewInstallMenu(cfg *config.Config, coreMgr *core.Manager, reg *protocol.Registry, rbMgr *rollback.Manager, nginxMgr *nginx.Manager, userMgr *user.Manager, subMgr *subscription.Manager, logger *logrus.Logger) *InstallMenu {
	return &InstallMenu{config: cfg, coreMgr: coreMgr, registry: reg, rollbackMgr: rbMgr, nginxMgr: nginxMgr, userMgr: userMgr, subMgr: subMgr, logger: logger}
}

// Show displays the installation management menu.
func (m *InstallMenu) Show() {
	for {
		PrintTitle("安装管理")
		PrintOption(1, "任意组合安装（需要域名解析）")
		PrintOption(2, "一键 Reality 组合安装（无域名）")
		PrintOption(3, "查看已安装协议")
		PrintOption(4, "卸载协议")
		PrintOption(5, "Reality 管理")
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5"})
		switch choice {
		case "1":
			m.installCombination()
		case "2":
			m.installReality()
		case "3":
			m.showInstalled()
		case "4":
			m.uninstallProtocol()
		case "5":
			m.showRealityMenu()
		case "0":
			return
		}
	}
}

func (m *InstallMenu) installCombination() {
	PrintTitle("任意组合安装（需要域名解析）")

	// 显示所有协议（固定排序：推荐 → 稳定 → 一般 → 不安全）
	domainProtos := m.registry.ListAllOrdered()

	for i, p := range domainProtos {
		installed := ""
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				mode := inferProtocolMode(m.config.ProtocolModes, p.Name())
				switch mode {
				case "nodomain":
					installed = Yellow(" [已安装无域名版本，安装会覆盖]")
				case "domain":
					installed = Green(" [已安装绑定域名版本]")
				}
				break
			}
		}
		PrintOption(i+1, fmt.Sprintf("%-30s %s%s", p.Name(), protocolLabel(p), installed))
	}

	fmt.Println()
	PrintOptionStr("0", "返回上级菜单")
	fmt.Println()
	input := ReadInput("请输入要安装的协议编号（空格/逗号分隔，如 1,3,5）")
	if input == "" || input == "0" {
		return
	}

	// 解析选择
	var selected []protocol.Protocol
	parts := strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' })
	for _, p := range parts {
		var idx int
		if _, err := fmt.Sscanf(p, "%d", &idx); err != nil || idx < 1 || idx > len(domainProtos) {
			PrintError(fmt.Sprintf("无效编号: %s", p))
			return
		}
		selected = append(selected, domainProtos[idx-1])
	}

	if len(selected) == 0 {
		return
	}

	// 安装前检查
	if err := sysinfo.CheckDiskSpace("/", 100); err != nil {
		PrintError(fmt.Sprintf("磁盘空间不足: %v", err))
		return
	}

	// 域名输入
	domain := ReadInput("请输入域名")
	if domain == "" {
		PrintError("此安装方式需要域名，如无域名请使用一键 Reality 组合安装")
		return
	}
	if err := security.ValidateDomain(domain); err != nil {
		PrintError(fmt.Sprintf("域名无效: %v", err))
		return
	}

	// 区分需要 TLS 证书的协议和 Reality 协议
	var needTLSCert bool
	var needReality bool
	for _, p := range selected {
		if strings.Contains(p.Name(), "reality") {
			needReality = true
		} else if p.Name() != "socks5" {
			needTLSCert = true
		}
	}

	// TLS 证书检测与申请
	certFile := m.config.TLS.CertFile
	keyFile := m.config.TLS.KeyFile
	if needTLSCert {
		// 先尝试自动检测已有证书（VasmaX 目录、acme.sh、BT 面板、1Panel）
		m.config.TLS.Domain = domain
		certFile, keyFile = config.DetectCertPath(&m.config.TLS)
		if certFile != "" && keyFile != "" {
			// 如果证书在 acme.sh 目录中，自动 install-cert 到 VasmaX TLS 目录
			if strings.Contains(certFile, ".acme.sh") {
				PrintInfo(fmt.Sprintf("检测到 acme.sh 证书: %s", certFile))
				PrintInfo("正在安装证书到 VasmaX TLS 目录...")
				installed, iKey := m.installAcmeCertToTLS(domain)
				if installed != "" && iKey != "" {
					certFile = installed
					keyFile = iKey
					PrintSuccess(fmt.Sprintf("证书已安装到: %s", certFile))
				} else {
					PrintWarning("证书安装失败，将直接使用 acme.sh 源路径")
				}
			} else {
				PrintSuccess(fmt.Sprintf("已自动检测到 TLS 证书: %s", certFile))
			}
		} else {
			// 未自动检测到，提供多种选择
			PrintWarning("未自动检测到 TLS 证书")
			PrintInfo("域名模式协议需要 TLS 证书才能正常工作")
			fmt.Println()
			PrintOption(1, "申请新证书（acme.sh）")
			PrintOption(2, "手动指定证书路径")
			PrintOption(3, "跳过，稍后在 TLS 证书管理中申请")
			choice := ReadChoice("请选择", []string{"1", "2", "3"})
			switch choice {
			case "1":
				certFile, keyFile = m.inlineIssueCert(domain)
				if certFile == "" || keyFile == "" {
					PrintError("证书申请失败，安装中止")
					return
				}
			case "2":
				certFile = ReadInput("请输入证书文件路径（fullchain.crt 或 .pem）")
				keyFile = ReadInput("请输入私钥文件路径（.key）")
				if certFile == "" || keyFile == "" {
					PrintError("证书路径不能为空")
					return
				}
				if !fileExists(certFile) {
					PrintError(fmt.Sprintf("证书文件不存在: %s", certFile))
					return
				}
				if !fileExists(keyFile) {
					PrintError(fmt.Sprintf("私钥文件不存在: %s", keyFile))
					return
				}
				PrintSuccess(fmt.Sprintf("已指定证书: %s", certFile))
			default:
				PrintWarning("跳过证书申请，非 Reality 协议将无法正常工作直到申请证书并重新安装")
			}
		}
	}

	// Reality 协议需要密钥对
	if needReality {
		if m.config.Reality.PrivateKey == "" {
			PrintInfo("正在生成 X25519 密钥对...")
			keyPair, err := security.GenerateX25519KeyPair()
			if err != nil {
				PrintError(fmt.Sprintf("生成密钥对失败: %v", err))
				return
			}
			m.config.Reality.PrivateKey = keyPair.PrivateKey
			m.config.Reality.PublicKey = keyPair.PublicKey
			PrintSuccess("X25519 密钥对已生成")
		}
		if m.config.Reality.ShortID == "" {
			shortID, err := security.GenerateShortID()
			if err != nil {
				PrintError(fmt.Sprintf("生成 ShortID 失败: %v", err))
				return
			}
			m.config.Reality.ShortID = shortID
		}
		if m.config.Reality.Dest == "" {
			m.config.Reality.Dest = "www.microsoft.com:443"
			m.config.Reality.ServerName = "www.microsoft.com"
		}
		if m.config.Reality.Port == 0 {
			m.config.Reality.Port = 443
		}
	}

	// 创建回滚快照
	snap, err := m.rollbackMgr.CreateSnapshot()
	if err != nil {
		m.logger.WithError(err).Warn("创建回滚快照失败")
	}

	// 自动创建默认用户（如果没有用户）
	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		PrintInfo("正在创建默认用户...")
		uuid := generateUUID()
		email := fmt.Sprintf("user_%s", uuid[:8])
		if err := m.userMgr.AddLocalUser(uuid, email); err != nil {
			PrintError(fmt.Sprintf("创建用户失败: %v", err))
			return
		}
		PrintSuccess(fmt.Sprintf("默认用户已创建: %s", uuid))
		users = m.userMgr.GetAllUsers()
	}

	// 安装核心并记录协议
	ctx := context.Background()
	for _, p := range selected {
		PrintInfo(fmt.Sprintf("正在安装 %s...", p.Name()))

		coreType := p.CoreType()
		status := m.coreMgr.GetStatus()
		if cs, ok := status[coreType]; !ok || !cs.Installed {
			if err := m.coreMgr.InstallCore(ctx, coreType); err != nil {
				PrintError(fmt.Sprintf("安装核心 %s 失败: %v", coreType, err))
				if snap != nil {
					m.rollbackMgr.Rollback(snap)
				}
				return
			}
		}

		// 记录已安装协议
		found := false
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				found = true
				break
			}
		}
		if !found {
			m.config.Protocols = append(m.config.Protocols, p.Name())
		}
		// 记录安装模式为绑定域名
		if m.config.ProtocolModes == nil {
			m.config.ProtocolModes = make(map[string]string)
		}
		m.config.ProtocolModes[p.Name()] = "domain"

		PrintSuccess(fmt.Sprintf("%s 安装完成", p.Name()))
	}

	// 检测选择了哪些核心类型
	hasXrayProto := false
	hasSingBoxProto := false
	for _, p := range selected {
		switch p.CoreType() {
		case "xray":
			hasXrayProto = true
		case "singbox":
			hasSingBoxProto = true
		}
	}

	// 生成 inbound 配置文件
	PrintInfo("正在生成协议配置...")
	apiUsers := make([]*api.User, 0, len(users))
	for _, u := range users {
		apiUsers = append(apiUsers, u.ToAPIUser())
	}

	for _, p := range selected {
		params := &protocol.InboundParams{
			Port:   p.DefaultPort(),
			Users:  apiUsers,
			Tag:    p.Name(),
			Domain: domain,
		}

		// Reality 协议使用 Reality 配置
		if strings.Contains(p.Name(), "reality") {
			params.Reality = &m.config.Reality
			params.Port = m.config.Reality.EffectivePort()
		}

		// 非 Reality、非 socks5 协议使用 TLS 证书
		if !strings.Contains(p.Name(), "reality") && p.Name() != "socks5" {
			params.CertFile = certFile
			params.KeyFile = keyFile
		}

		// WS/HTTPUpgrade 协议设置路径
		transport := p.TransportType()
		if transport == "ws" || transport == "httpupgrade" {
			params.Path = defaultWSPath(p)
		}
		if transport == "grpc" {
			params.ServiceName = defaultGRPCServiceName(p)
		}

		// TLS 版本设置
		params.TLSMinVersion = m.config.TLS.MinVersion
		params.TLSMaxVersion = m.config.TLS.MaxVersion

		inboundJSON, err := p.GenerateInbound(params)
		if err != nil {
			PrintError(fmt.Sprintf("生成 %s 入站配置失败: %v", p.Name(), err))
			continue
		}

		// 根据核心类型包装和写入
		var confPath string
		switch p.CoreType() {
		case "singbox":
			// sing-box 直接写入 confdir（sing-box -C 读取目录下所有 JSON）
			wrapper := map[string]interface{}{
				"inbounds": []json.RawMessage{inboundJSON},
			}
			confPath = filepath.Join(m.config.Paths.SingBoxConf, fmt.Sprintf("10_%s_inbounds.json", p.Name()))
			if err := security.AtomicWriteJSON(confPath, wrapper, 0644); err != nil {
				PrintError(fmt.Sprintf("写入 %s 配置失败: %v", p.Name(), err))
				continue
			}
		default:
			// Xray 写入 confdir
			wrapper := map[string]interface{}{
				"inbounds": []json.RawMessage{inboundJSON},
			}
			confPath = filepath.Join(m.config.Paths.XrayConf, fmt.Sprintf("05_%s_inbounds.json", p.Name()))
			if err := security.AtomicWriteJSON(confPath, wrapper, 0644); err != nil {
				PrintError(fmt.Sprintf("写入 %s 配置失败: %v", p.Name(), err))
				continue
			}
		}
		PrintSuccess(fmt.Sprintf("%s 配置已生成", p.Name()))
	}

	// 生成 Xray 基础配置
	if hasXrayProto {
		confDir := m.config.Paths.XrayConf
		if err := protocol.GenerateStatsAPIConfig(confDir); err != nil {
			m.logger.WithError(err).Warn("生成 Stats API 配置失败")
		}
		if err := protocol.GenerateStatsModuleConfig(confDir); err != nil {
			m.logger.WithError(err).Warn("生成 Stats 模块配置失败")
		}
		if err := protocol.EnsureBaseConfigs(confDir); err != nil {
			m.logger.WithError(err).Warn("生成 Xray 基础配置失败")
		}
	}

	// 生成 sing-box 基础配置
	if hasSingBoxProto {
		if err := protocol.EnsureSingBoxBaseConfigs(m.config.Paths.SingBoxConf); err != nil {
			m.logger.WithError(err).Warn("生成 sing-box 基础配置失败")
		}
	}

	// 保存域名和证书路径到配置
	m.config.TLS.Domain = domain
	if certFile != "" {
		m.config.TLS.CertFile = certFile
	}
	if keyFile != "" {
		m.config.TLS.KeyFile = keyFile
	}

	// 保存配置
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
	}

	// 自动配置 Nginx 反代（ws/grpc/httpupgrade 协议需要）
	m.autoConfigNginx(selected, domain)

	// 启动核心服务
	if hasXrayProto {
		PrintInfo("正在启动 Xray...")
		if err := m.coreMgr.RestartXray(); err != nil {
			PrintWarning(fmt.Sprintf("启动 Xray 失败: %v（可能需要手动启动）", err))
		}
	}
	if hasSingBoxProto {
		PrintInfo("正在合并 sing-box 配置并启动...")
		if err := m.coreMgr.MergeSingBoxConfig(); err != nil {
			PrintError(fmt.Sprintf("sing-box 配置合并失败: %v", err))
		} else if err := m.coreMgr.RestartSingBox(); err != nil {
			PrintWarning(fmt.Sprintf("启动 sing-box 失败: %v（可能需要手动启动）", err))
		}
	}

	// 验证服务启动
	PrintInfo("等待服务启动...")
	time.Sleep(3 * time.Second)
	status := m.coreMgr.GetStatus()
	for name, s := range status {
		if s.Installed && s.Running {
			PrintSuccess(fmt.Sprintf("%s 运行正常", name))
		} else if s.Installed {
			PrintWarning(fmt.Sprintf("%s 未运行", name))
		}
	}

	// 显示证书文件路径
	if certFile != "" || keyFile != "" {
		fmt.Println()
		PrintInfo("TLS 证书文件路径:")
		if certFile != "" {
			PrintInfo(fmt.Sprintf("  证书文件: %s", certFile))
		}
		if keyFile != "" {
			PrintInfo(fmt.Sprintf("  私钥文件: %s", keyFile))
		}
	}

	// 生成订阅
	if m.subMgr != nil {
		_ = m.subMgr.GenerateAll()
	}

	// 显示安装结果和分享链接
	PrintSuccess("域名组合安装完成")
	fmt.Println()
	m.showDomainInfo(users, domain)

	// 清理快照
	if snap != nil {
		m.rollbackMgr.CleanSnapshot(snap)
	}
}

func (m *InstallMenu) installReality() {
	PrintTitle("一键 Reality 组合安装（无域名）")

	// 列出所有支持无域名安装的协议（固定排序：推荐 → 稳定 → 不安全）
	realityProtos := m.registry.ListNoDomainOrdered()

	for i, p := range realityProtos {
		installed := ""
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				mode := inferProtocolMode(m.config.ProtocolModes, p.Name())
				switch mode {
				case "domain":
					installed = Yellow(" [已安装绑定域名版本，安装会覆盖]")
				case "nodomain":
					installed = Green(" [已安装无域名版本]")
				}
				break
			}
		}
		PrintOption(i+1, fmt.Sprintf("%-30s %s%s", p.Name(), protocolLabel(p), installed))
	}

	fmt.Println()
	PrintOptionStr("0", "返回上级菜单")
	fmt.Println()
	input := ReadInput("请输入要安装的协议编号（空格/逗号分隔，如 1,2,3 全选）")
	if input == "" || input == "0" {
		return
	}

	// 解析选择
	var selected []protocol.Protocol
	parts := strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' })
	for _, p := range parts {
		var idx int
		if _, err := fmt.Sscanf(p, "%d", &idx); err != nil || idx < 1 || idx > len(realityProtos) {
			PrintError(fmt.Sprintf("无效编号: %s", p))
			return
		}
		selected = append(selected, realityProtos[idx-1])
	}

	if len(selected) == 0 {
		return
	}

	PrintInfo("将自动生成 X25519 密钥对、ShortID 和默认用户")

	// 0. 设置 Reality 监听端口
	PrintSuccess("  直接回车选择默认 443")
	portInput := ReadInput("请输入 Reality 监听端口（默认 443，阿里云建议 8443）")
	realityPort := 443
	if portInput != "" {
		var p int
		if _, err := fmt.Sscanf(portInput, "%d", &p); err != nil || p < 1 || p > 65535 {
			PrintError("端口无效，使用默认 443")
		} else {
			realityPort = p
		}
	}
	m.config.Reality.Port = realityPort
	PrintInfo(fmt.Sprintf("Reality 端口: %d", realityPort))

	// 1. 安装 Xray-core
	ctx := context.Background()
	status := m.coreMgr.GetStatus()
	if cs, ok := status["xray"]; !ok || !cs.Installed {
		PrintInfo("正在安装 Xray-core...")
		if err := m.coreMgr.InstallCore(ctx, "xray"); err != nil {
			PrintError(fmt.Sprintf("安装 Xray-core 失败: %v", err))
			return
		}
		PrintSuccess("Xray-core 安装完成")
	} else {
		PrintInfo("Xray-core 已安装，跳过")
	}

	// 2. 生成 X25519 密钥对（如果还没有）
	if m.config.Reality.PrivateKey == "" {
		PrintInfo("正在生成 X25519 密钥对...")
		keyPair, err := security.GenerateX25519KeyPair()
		if err != nil {
			PrintError(fmt.Sprintf("生成密钥对失败: %v", err))
			return
		}
		m.config.Reality.PrivateKey = keyPair.PrivateKey
		m.config.Reality.PublicKey = keyPair.PublicKey
		PrintSuccess("X25519 密钥对已生成")
	}

	// 3. 生成 ShortID（如果还没有）
	if m.config.Reality.ShortID == "" {
		shortID, err := security.GenerateShortID()
		if err != nil {
			PrintError(fmt.Sprintf("生成 ShortID 失败: %v", err))
			return
		}
		m.config.Reality.ShortID = shortID
	}

	// 4. 设置默认 Reality dest 和 serverName（如果还没有）
	if m.config.Reality.Dest == "" {
		m.config.Reality.Dest = "www.microsoft.com:443"
		m.config.Reality.ServerName = "www.microsoft.com"
	}

	// 5. 记录协议
	for _, p := range selected {
		found := false
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				found = true
				break
			}
		}
		if !found {
			m.config.Protocols = append(m.config.Protocols, p.Name())
		}
		// 记录安装模式为无域名
		if m.config.ProtocolModes == nil {
			m.config.ProtocolModes = make(map[string]string)
		}
		m.config.ProtocolModes[p.Name()] = "nodomain"
	}

	// 6. 自动创建默认用户（如果没有用户）
	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		PrintInfo("正在创建默认用户...")
		uuid := generateUUID()
		email := fmt.Sprintf("user_%s", uuid[:8])
		if err := m.userMgr.AddLocalUser(uuid, email); err != nil {
			PrintError(fmt.Sprintf("创建用户失败: %v", err))
			return
		}
		PrintSuccess(fmt.Sprintf("默认用户已创建: %s", uuid))
		users = m.userMgr.GetAllUsers()
	}

	// 6.5 安装 sing-box core（如果选择了 sing-box 协议）
	hasSingBoxProto := false
	hasXrayProto := false
	for _, p := range selected {
		switch p.CoreType() {
		case "singbox":
			hasSingBoxProto = true
		case "xray":
			hasXrayProto = true
		}
	}
	if hasSingBoxProto {
		sbStatus := m.coreMgr.GetStatus()
		if cs, ok := sbStatus["singbox"]; !ok || !cs.Installed {
			PrintInfo("正在安装 sing-box...")
			if err := m.coreMgr.InstallCore(ctx, "singbox"); err != nil {
				PrintError(fmt.Sprintf("安装 sing-box 失败: %v", err))
				return
			}
			PrintSuccess("sing-box 安装完成")
		} else {
			PrintInfo("sing-box 已安装，跳过")
		}
	}

	// 6.6 为 sing-box TLS 协议生成自签证书（无域名模式）
	var selfCertFile, selfKeyFile string
	if hasSingBoxProto {
		certPaths, err := security.EnsureSelfSignedCert("/etc/vasmax/tls")
		if err != nil {
			PrintError(fmt.Sprintf("生成自签证书失败: %v", err))
			return
		}
		selfCertFile = certPaths.CertFile
		selfKeyFile = certPaths.KeyFile
		PrintSuccess("自签 TLS 证书已就绪")
	}

	// 7. 生成 inbound 配置文件（区分 Xray 和 sing-box）
	PrintInfo("正在生成配置...")
	apiUsers := make([]*api.User, 0, len(users))
	for _, u := range users {
		apiUsers = append(apiUsers, u.ToAPIUser())
	}

	for _, p := range selected {
		params := &protocol.InboundParams{
			Port:    m.config.Reality.EffectivePort(),
			Users:   apiUsers,
			Tag:     p.Name(),
			Reality: &m.config.Reality,
		}

		// 非 Reality 协议使用自己的默认端口
		if !strings.Contains(p.Name(), "reality") {
			params.Port = p.DefaultPort()
		}

		// sing-box TLS 协议使用自签证书
		if p.CoreType() == "singbox" && p.Name() != "socks5" {
			params.CertFile = selfCertFile
			params.KeyFile = selfKeyFile
		}

		inboundJSON, err := p.GenerateInbound(params)
		if err != nil {
			PrintError(fmt.Sprintf("生成 %s 入站配置失败: %v", p.Name(), err))
			continue
		}

		wrapper := map[string]interface{}{
			"inbounds": []json.RawMessage{inboundJSON},
		}

		// 根据核心类型写入对应目录
		var confPath string
		switch p.CoreType() {
		case "singbox":
			confPath = filepath.Join(m.config.Paths.SingBoxConf, fmt.Sprintf("10_%s_inbounds.json", p.Name()))
		default:
			confPath = filepath.Join(m.config.Paths.XrayConf, fmt.Sprintf("05_%s_inbounds.json", p.Name()))
		}

		if err := security.AtomicWriteJSON(confPath, wrapper, 0644); err != nil {
			PrintError(fmt.Sprintf("写入 %s 配置失败: %v", p.Name(), err))
			continue
		}
		PrintSuccess(fmt.Sprintf("%s 配置已生成", p.Name()))
	}

	// 8. 生成 Xray Stats API 配置（监控功能需要）
	if hasXrayProto {
		if err := protocol.GenerateStatsAPIConfig(m.config.Paths.XrayConf); err != nil {
			m.logger.WithError(err).Warn("生成 Stats API 配置失败")
		}
		if err := protocol.GenerateStatsModuleConfig(m.config.Paths.XrayConf); err != nil {
			m.logger.WithError(err).Warn("生成 Stats 模块配置失败")
		}
		// 生成基础出站和 DNS 配置（Xray 转发流量必需）
		if err := protocol.EnsureBaseConfigs(m.config.Paths.XrayConf); err != nil {
			m.logger.WithError(err).Warn("生成 Xray 基础配置失败")
		}
	}

	// 8.5 生成 sing-box 基础配置（outbound + dns + route）
	if hasSingBoxProto {
		if err := protocol.EnsureSingBoxBaseConfigs(m.config.Paths.SingBoxConf); err != nil {
			m.logger.WithError(err).Warn("生成 sing-box 基础配置失败")
		}
	}

	// 9. 保存配置
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	// 10. 启动核心
	if hasXrayProto {
		PrintInfo("正在启动 Xray...")
		if err := m.coreMgr.RestartXray(); err != nil {
			PrintWarning(fmt.Sprintf("启动 Xray 失败: %v（可能需要手动启动）", err))
		} else {
			time.Sleep(2 * time.Second)
			newStatus := m.coreMgr.GetStatus()
			if xs, ok := newStatus["xray"]; ok && xs.Running {
				PrintSuccess("Xray 运行正常")
			} else {
				PrintWarning("Xray 未运行，请检查日志")
			}
		}
	}
	if hasSingBoxProto {
		PrintInfo("正在合并 sing-box 配置并启动...")
		if err := m.coreMgr.MergeSingBoxConfig(); err != nil {
			PrintError(fmt.Sprintf("sing-box 配置合并失败: %v", err))
		} else if err := m.coreMgr.RestartSingBox(); err != nil {
			PrintWarning(fmt.Sprintf("启动 sing-box 失败: %v（可能需要手动启动）", err))
		} else {
			time.Sleep(2 * time.Second)
			newStatus := m.coreMgr.GetStatus()
			if ss, ok := newStatus["singbox"]; ok && ss.Running {
				PrintSuccess("sing-box 运行正常")
			} else {
				PrintWarning("sing-box 未运行，请检查日志")
			}
		}
	}

	// 11. 生成订阅
	if m.subMgr != nil {
		_ = m.subMgr.GenerateAll()
	}

	// 12. 显示安装结果和分享链接
	PrintSuccess("Reality 组合安装完成")
	fmt.Println()
	m.showRealityInfo(users)
}

// showRealityInfo 显示 Reality 配置信息和分享链接
func (m *InstallMenu) showRealityInfo(users []*user.UserEntry) {
	PrintTitle("连接信息")

	// 获取服务器 IP
	serverIP := getServerIP()

	PrintInfo(fmt.Sprintf("服务器地址: %s", serverIP))
	PrintInfo(fmt.Sprintf("端口: %d", m.config.Reality.EffectivePort()))
	PrintInfo(fmt.Sprintf("伪装域名: %s", m.config.Reality.ServerName))
	PrintInfo(fmt.Sprintf("PublicKey: %s", m.config.Reality.PublicKey))
	PrintInfo(fmt.Sprintf("ShortID: %s", m.config.Reality.ShortID))
	fmt.Println()

	// 显示所有已安装协议的分享链接
	for _, protoName := range m.config.Protocols {
		p, ok := m.registry.Get(protoName)
		if !ok {
			continue
		}

		info := &protocol.ServerInfo{
			Host: serverIP,
			Port: externalPort(p),
		}

		// Reality 协议使用 Reality 配置
		if strings.Contains(protoName, "reality") {
			info.Reality = &m.config.Reality
			info.Port = m.config.Reality.EffectivePort()
		}

		// 无域名模式下 sing-box 协议用 IP 作为 Domain
		mode := inferProtocolMode(m.config.ProtocolModes, protoName)
		if mode == "nodomain" && p.CoreType() == "singbox" {
			info.Domain = serverIP
		}

		// 域名模式协议使用域名
		if mode == "domain" && m.config.TLS.Domain != "" {
			info.Domain = m.config.TLS.Domain
		}

		// WS/HTTPUpgrade/gRPC 路径
		transport := p.TransportType()
		if transport == "ws" || transport == "httpupgrade" {
			info.Path = defaultWSPath(p)
		}
		if transport == "grpc" {
			info.ServiceName = defaultGRPCServiceName(p)
		}

		PrintSeparator()
		PrintInfo(fmt.Sprintf("协议: %s", protoName))

		for _, u := range users {
			apiUser := u.ToAPIUser()
			uri := p.GenerateURI(apiUser, info)
			if uri == "" {
				continue
			}

			PrintInfo(fmt.Sprintf("用户: %s", u.Email))
			PrintInfo(fmt.Sprintf("UUID: %s", u.UUID))
			fmt.Println()
			PrintInfo("分享链接:")
			fmt.Printf("  %s\n", uri)
			fmt.Println()
			PrintInfo("二维码:")
			fmt.Println(subscription.GenerateTerminalQR(uri))
		}
	}
}

// getServerIP 获取服务器公网 IP
func getServerIP() string {
	// 优先通过外部 API 获取公网 IP（阿里云等 NAT 环境下本地 IP 是内网地址）
	apis := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, api := range apis {
		resp, err := client.Get(api)
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

	// 回退：通过出站连接获取本机 IP（可能是内网 IP）
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 3*time.Second)
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return addr.IP.String()
		}
	}
	return "YOUR_SERVER_IP"
}

// showRealityMenu 显示 Reality 管理菜单
func (m *InstallMenu) showRealityMenu() {
	if m.config.Reality.PrivateKey == "" {
		PrintWarning("尚未安装 Reality 协议，请先使用一键 Reality 安装")
		return
	}
	for {
		PrintTitle("Reality 管理")
		PrintInfo(fmt.Sprintf("伪装域名: %s", m.config.Reality.ServerName))
		PrintInfo(fmt.Sprintf("Dest: %s", m.config.Reality.Dest))
		PrintInfo(fmt.Sprintf("端口: %d", m.config.Reality.EffectivePort()))
		PrintSeparator()
		PrintOption(1, "修改伪装域名")
		PrintOption(2, "查看密钥和连接信息")
		PrintOption(3, "重新生成密钥对")
		PrintOptionStr("0", "返回")

		choice := ReadChoice("请选择", []string{"1", "2", "3"})
		switch choice {
		case "1":
			dest := ReadInput("请输入新的伪装域名 (如 www.apple.com)")
			if dest != "" {
				if !strings.Contains(dest, ":") {
					dest = dest + ":443"
				}
				serverName := strings.Split(dest, ":")[0]
				m.config.Reality.Dest = dest
				m.config.Reality.ServerName = serverName
				if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
					PrintError(fmt.Sprintf("保存失败: %v", err))
				} else {
					PrintSuccess(fmt.Sprintf("伪装域名已更新: %s", serverName))
					PrintWarning("修改后需要重启 Xray 才能生效")
				}
			}
		case "2":
			PrintInfo(fmt.Sprintf("PublicKey:   %s", m.config.Reality.PublicKey))
			PrintInfo(fmt.Sprintf("ShortID:     %s", m.config.Reality.ShortID))
			PrintInfo(fmt.Sprintf("ServerName:  %s", m.config.Reality.ServerName))
			PrintInfo(fmt.Sprintf("Dest:        %s", m.config.Reality.Dest))
			fmt.Println()
			// 显示分享链接
			users := m.userMgr.GetAllUsers()
			if len(users) > 0 {
				m.showRealityInfo(users)
			}
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
			if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
				PrintError(fmt.Sprintf("保存失败: %v", err))
			} else {
				PrintSuccess("密钥对和 ShortID 已重新生成")
				PrintWarning("修改后需要重启 Xray 才能生效")
			}
		case "0":
			return
		}
	}
}

func (m *InstallMenu) showInstalled() {
	PrintTitle("已安装协议")
	if len(m.config.Protocols) == 0 {
		PrintInfo("暂无已安装协议")
		return
	}
	for i, p := range m.config.Protocols {
		mode := inferProtocolMode(m.config.ProtocolModes, p)
		modeStr := ""
		switch mode {
		case "domain":
			modeStr = Green(" (绑定域名)")
		case "nodomain":
			modeStr = Cyan(" (无域名)")
		}
		PrintOption(i+1, p+modeStr)
	}

	// 显示所有协议的连接信息
	users := m.userMgr.GetAllUsers()
	if len(users) == 0 {
		fmt.Println()
		PrintWarning("暂无用户，请先在账号管理中添加用户")
		return
	}

	serverIP := getServerIP()

	for _, protoName := range m.config.Protocols {
		p, ok := m.registry.Get(protoName)
		if !ok {
			continue
		}

		mode := inferProtocolMode(m.config.ProtocolModes, protoName)

		// 构建 ServerInfo
		info := &protocol.ServerInfo{
			Host: serverIP,
			Port: externalPort(p),
		}

		// 根据安装模式填充不同字段
		if mode == "domain" && m.config.TLS.Domain != "" {
			info.Domain = m.config.TLS.Domain
			if m.config.CDN.Enabled && m.config.CDN.Address != "" {
				info.CDNHost = m.config.CDN.Address
			}
		}
		if strings.Contains(protoName, "reality") {
			info.Reality = &m.config.Reality
			info.Port = m.config.Reality.EffectivePort()
		}
		// 无域名模式下 sing-box 协议用 IP 作为 Domain
		if mode == "nodomain" && p.CoreType() == "singbox" {
			info.Domain = serverIP
		}

		// WS/HTTPUpgrade/gRPC 路径
		transport := p.TransportType()
		if transport == "ws" || transport == "httpupgrade" {
			info.Path = defaultWSPath(p)
		}
		if transport == "grpc" {
			info.ServiceName = defaultGRPCServiceName(p)
		}

		fmt.Println()
		PrintSeparator()
		PrintInfo(fmt.Sprintf("协议: %s", protoName))

		for _, u := range users {
			apiUser := u.ToAPIUser()
			uri := p.GenerateURI(apiUser, info)
			if uri == "" {
				continue
			}

			PrintInfo(fmt.Sprintf("用户: %s", u.Email))
			PrintInfo("分享链接:")
			fmt.Printf("  %s\n", uri)
			fmt.Println()
			PrintInfo("二维码:")
			fmt.Println(subscription.GenerateTerminalQR(uri))
		}
	}
}

func (m *InstallMenu) uninstallProtocol() {
	PrintTitle("卸载协议")
	if len(m.config.Protocols) == 0 {
		PrintInfo("暂无已安装协议")
		return
	}
	for i, p := range m.config.Protocols {
		mode := inferProtocolMode(m.config.ProtocolModes, p)
		modeStr := ""
		switch mode {
		case "domain":
			modeStr = Green(" (绑定域名)")
		case "nodomain":
			modeStr = Cyan(" (无域名)")
		}
		PrintOption(i+1, p+modeStr)
	}

	input := ReadInput("请输入要卸载的协议编号（空格/逗号分隔，如 1,3,5）")
	if input == "" || input == "0" {
		return
	}

	// 解析选择（支持多选）
	parts := strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' })
	var indices []int
	for _, p := range parts {
		var idx int
		if _, err := fmt.Sscanf(p, "%d", &idx); err != nil || idx < 1 || idx > len(m.config.Protocols) {
			PrintError(fmt.Sprintf("无效编号: %s", p))
			return
		}
		indices = append(indices, idx-1)
	}

	// 显示待卸载列表
	var names []string
	for _, i := range indices {
		names = append(names, m.config.Protocols[i])
	}
	if !Confirm(fmt.Sprintf("确认卸载 %s?", strings.Join(names, ", "))) {
		return
	}

	// 收集需要重启的核心
	needRestartXray := false
	needRestartSingBox := false

	for _, name := range names {
		// 1. 删除 Xray 入站配置文件
		xrayConfFile := filepath.Join(m.config.Paths.XrayConf, fmt.Sprintf("05_%s_inbounds.json", name))
		if err := os.Remove(xrayConfFile); err != nil && !os.IsNotExist(err) {
			m.logger.WithError(err).Warnf("删除 Xray 配置文件失败: %s", xrayConfFile)
		} else if err == nil {
			PrintInfo(fmt.Sprintf("已删除 Xray 配置: %s", xrayConfFile))
			needRestartXray = true
		}

		// 2. 删除 sing-box 入站配置文件（两种命名格式都尝试删除）
		singboxConfFile := filepath.Join(m.config.Paths.SingBoxConf, fmt.Sprintf("10_%s_inbounds.json", name))
		if err := os.Remove(singboxConfFile); err != nil && !os.IsNotExist(err) {
			m.logger.WithError(err).Warnf("删除 sing-box 配置文件失败: %s", singboxConfFile)
		} else if err == nil {
			PrintInfo(fmt.Sprintf("已删除 sing-box 配置: %s", singboxConfFile))
			needRestartSingBox = true
		}
		// 兼容旧版命名格式
		singboxConfFileOld := filepath.Join(m.config.Paths.SingBoxConf, fmt.Sprintf("%s.json", name))
		if err := os.Remove(singboxConfFileOld); err == nil {
			PrintInfo(fmt.Sprintf("已删除 sing-box 配置: %s", singboxConfFileOld))
			needRestartSingBox = true
		}

		// 3. 删除 Nginx 反代 location（如果是需要 Nginx 的协议）
		if p, ok := m.registry.Get(name); ok {
			if needsNginxProxy(p) && m.config.TLS.Domain != "" {
				if err := m.nginxMgr.RemoveLocation(m.config.TLS.Domain, name); err != nil {
					m.logger.WithError(err).Warnf("删除 Nginx location 失败: %s", name)
				} else {
					PrintInfo(fmt.Sprintf("已删除 Nginx 反代配置: %s", name))
				}
			}
			// 标记对应核心需要重启
			if p.CoreType() == "xray" {
				needRestartXray = true
			} else if p.CoreType() == "singbox" {
				needRestartSingBox = true
			}
		}

		// 4. 从配置中移除
		for i, ip := range m.config.Protocols {
			if ip == name {
				m.config.Protocols = append(m.config.Protocols[:i], m.config.Protocols[i+1:]...)
				break
			}
		}
		delete(m.config.ProtocolModes, name)

		PrintSuccess(fmt.Sprintf("%s 已卸载", name))
	}

	// 5. 保存配置
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	// 6. 检查是否还有协议使用对应核心，没有则停止核心
	hasXray := false
	hasSingBox := false
	for _, p := range m.config.Protocols {
		if proto, ok := m.registry.Get(p); ok {
			switch proto.CoreType() {
			case "xray":
				hasXray = true
			case "singbox":
				hasSingBox = true
			}
		}
	}

	// 7. 重启或停止核心服务
	if needRestartXray {
		if hasXray {
			PrintInfo("正在重启 Xray...")
			if err := m.coreMgr.RestartXray(); err != nil {
				PrintWarning(fmt.Sprintf("重启 Xray 失败: %v", err))
			} else {
				PrintSuccess("Xray 已重启")
			}
		} else {
			PrintInfo("已无 Xray 协议，正在停止 Xray...")
			// 清理 Stats API 配置
			_ = protocol.RemoveStatsAPIConfig(m.config.Paths.XrayConf)
			m.coreMgr.StopAll() // StopAll 会安全跳过未安装的
			PrintSuccess("Xray 已停止")
		}
	}
	if needRestartSingBox {
		if hasSingBox {
			PrintInfo("正在重启 sing-box...")
			if err := m.coreMgr.RestartSingBox(); err != nil {
				PrintWarning(fmt.Sprintf("重启 sing-box 失败: %v", err))
			} else {
				PrintSuccess("sing-box 已重启")
			}
		} else {
			PrintInfo("已无 sing-box 协议，正在停止 sing-box...")
			m.coreMgr.StopAll()
			PrintSuccess("sing-box 已停止")
		}
	}

	// 8. 重载 Nginx（如果有改动）
	if m.config.TLS.Domain != "" {
		_ = m.nginxMgr.Reload()
	}
}

// inferProtocolMode 推断协议安装模式，兼容旧版本未记录模式的情况
func inferProtocolMode(modes map[string]string, name string) string {
	if modes != nil {
		if mode, ok := modes[name]; ok && mode != "" {
			return mode
		}
	}
	// 旧版本未记录模式，根据协议名推断
	if strings.Contains(name, "reality") {
		return "nodomain"
	}
	return "domain"
}

// protocolLabel 生成协议的描述标签
func protocolLabel(p protocol.Protocol) string {
	name := p.Name()
	core := p.CoreType()

	switch name {
	case "vless_reality_vision":
		return "(" + core + " Reality Vision 推荐)"
	case "vless_reality_grpc":
		return "(" + core + " Reality gRPC)"
	case "vless_reality_xhttp":
		return "(" + core + " Reality XHTTP)"
	case "vless_ws_tls":
		return "(" + core + " VLESS WebSocket 推荐)"
	case "vless_tcp_tls_vision":
		return "(" + core + " VLESS TCP Vision)"
	case "vless_grpc_tls":
		return "(" + core + " VLESS gRPC)"
	case "vmess_ws_tls":
		return "(" + core + " VMess WebSocket)"
	case "vmess_httpupgrade_tls":
		return "(" + core + " VMess HTTPUpgrade)"
	case "trojan_tcp_tls":
		return "(" + core + " Trojan TCP)"
	case "trojan_grpc_tls":
		return "(" + core + " Trojan gRPC)"
	case "hysteria2":
		return "(" + core + " Hysteria2)"
	case "tuic":
		return "(" + core + " TUIC)"
	case "anytls":
		return "(" + core + " AnyTLS 推荐)"
	case "naive":
		return "(" + core + " NaïveProxy)"
	case "socks5":
		return "(" + core + " SOCKS5)"
	default:
		return "(" + core + ")"
	}
}

// needsNginxProxy 判断协议是否需要 Nginx 反向代理
// ws/grpc/httpupgrade 类型的非 Reality 协议需要 Nginx 做反代
func needsNginxProxy(p protocol.Protocol) bool {
	transport := p.TransportType()
	name := p.Name()
	// Reality 协议自己处理 TLS，不需要 Nginx
	if strings.Contains(name, "reality") {
		return false
	}
	return transport == "ws" || transport == "grpc" || transport == "httpupgrade"
}

// externalPort 返回协议的外部端口（客户端连接用）
// Nginx 反代协议外部端口为 443，其他协议使用自身端口
func externalPort(p protocol.Protocol) int {
	if needsNginxProxy(p) {
		return 443
	}
	return p.DefaultPort()
}

// defaultWSPath 为协议生成默认的 WS/HTTPUpgrade 路径
func defaultWSPath(p protocol.Protocol) string {
	// 每个协议用不同路径避免冲突
	switch p.Name() {
	case "vless_ws_tls":
		return "/vlessws"
	case "vmess_ws_tls":
		return "/vmessws"
	case "vmess_httpupgrade_tls":
		return "/vmesshup"
	default:
		return "/" + strings.ReplaceAll(p.Name(), "_", "")
	}
}

// defaultGRPCServiceName 为 gRPC 协议生成默认 serviceName
func defaultGRPCServiceName(p protocol.Protocol) string {
	switch p.Name() {
	case "vless_grpc_tls":
		return "vlessgrpc"
	case "trojan_grpc_tls":
		return "trojangrpc"
	default:
		return strings.ReplaceAll(p.Name(), "_", "")
	}
}

// inlineIssueCert 在安装流程中内联申请 TLS 证书
func (m *InstallMenu) inlineIssueCert(domain string) (certFile, keyFile string) {
	// 检查 acme.sh 是否安装
	acmePath := filepath.Join(os.Getenv("HOME"), ".acme.sh", "acme.sh")
	if _, err := os.Stat(acmePath); os.IsNotExist(err) {
		PrintWarning("acme.sh 未安装")
		if Confirm("是否安装 acme.sh?") {
			PrintSuccess("  直接回车跳过")
			email := ReadInput("请输入邮箱（用于证书到期提醒）")
			var installCmd string
			if email != "" {
				installCmd = fmt.Sprintf("curl -fsSL https://get.acme.sh | sh -s email=%s", email)
			} else {
				installCmd = "curl -fsSL https://get.acme.sh | sh -s -- --no-email"
			}
			cmd := exec.Command("bash", "-c", installCmd)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				PrintError(fmt.Sprintf("acme.sh 安装失败: %v", err))
				return "", ""
			}
			PrintSuccess("acme.sh 安装成功")
		} else {
			return "", ""
		}
	}

	// 选择验证方式（失败后可重新选择）
	caServer := "letsencrypt"
	for {
		fmt.Println()

		// 预检 80 端口状态
		port80Free := true
		if ln, err := net.Listen("tcp", ":80"); err != nil {
			port80Free = false
			PrintWarning("⚠ 检测到 80 端口已被占用（可能是 Nginx），standalone 方式不可用")
			fmt.Println()
		} else {
			ln.Close()
			PrintSuccess("✓ 80 端口空闲，standalone 方式可用")
			fmt.Println()
		}

		if port80Free {
			PrintOption(1, "standalone（需要 80 端口空闲，申请和续期时临时占用几秒）")
		} else {
			PrintOption(1, "standalone（80 端口被占用，不推荐）")
		}
		PrintOption(2, "Nginx webroot（80 端口已被 Nginx 占用时使用，续期无需停 Nginx）")
		PrintOption(3, "Cloudflare DNS API（无需开放 80 端口，域名 DNS 需托管在 Cloudflare）")
		PrintOption(4, "阿里云 DNS API（无需开放 80 端口，域名 DNS 需托管在阿里云）")
		PrintOption(5, "Cloudflare DNS 通配符证书（申请 *.域名，需 Cloudflare DNS）")
		PrintOptionStr("0", "取消")
		mode := ReadChoice("选择验证方式", []string{"1", "2", "3", "4", "5"})

		var args []string
		switch mode {
		case "1":
			if !port80Free {
				PrintWarning("80 端口被占用，standalone 大概率会失败")
				if !Confirm("仍然尝试?") {
					continue
				}
			}
			args = []string{"--issue", "-d", domain, "--standalone", "--server", caServer}
		case "2":
			PrintSuccess("  直接回车选择默认 /var/www/html")
			webroot := ReadInput("请输入 Nginx webroot 路径")
			if webroot == "" {
				webroot = "/var/www/html"
			}
			args = []string{"--issue", "-d", domain, "--webroot", webroot, "--server", caServer}
		case "3":
			token := ReadInput("请输入 CF_Token")
			if token == "" {
				PrintError("CF_Token 不能为空")
				continue
			}
			os.Setenv("CF_Token", token)
			args = []string{"--issue", "-d", domain, "--dns", "dns_cf", "--server", caServer}
		case "4":
			aliKey := ReadInput("请输入 Ali_Key")
			aliSecret := ReadInput("请输入 Ali_Secret")
			if aliKey == "" || aliSecret == "" {
				PrintError("Ali_Key 和 Ali_Secret 不能为空")
				continue
			}
			os.Setenv("Ali_Key", aliKey)
			os.Setenv("Ali_Secret", aliSecret)
			args = []string{"--issue", "-d", domain, "--dns", "dns_ali", "--server", caServer}
		case "5":
			token := ReadInput("请输入 CF_Token")
			if token == "" {
				PrintError("CF_Token 不能为空")
				continue
			}
			os.Setenv("CF_Token", token)
			args = []string{"--issue", "-d", domain, "-d", "*." + domain, "--dns", "dns_cf", "--server", caServer}
		case "0":
			return "", ""
		}

		PrintInfo("正在申请证书...")
		cmd := exec.Command(acmePath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// acme.sh exit code 2 = cert exists and not expired ("Skipping, next renewal time is ...")
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
				PrintInfo("证书已存在且未过期，直接安装已有证书...")
			} else {
				PrintError(fmt.Sprintf("证书申请失败: %v", err))
				PrintInfo("可以重新选择其他验证方式")
				continue
			}
		}
		break
	}

	// 安装证书到 TLS 目录
	tlsDir := config.DefaultTLSDir
	os.MkdirAll(tlsDir, 0755)
	installArgs := []string{
		"--install-cert", "-d", domain,
		"--cert-file", filepath.Join(tlsDir, domain+".crt"),
		"--key-file", filepath.Join(tlsDir, domain+".key"),
		"--fullchain-file", filepath.Join(tlsDir, domain+".fullchain.crt"),
		"--reloadcmd", "systemctl restart VasmaX",
	}
	installCmd := exec.Command(acmePath, installArgs...)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		PrintError(fmt.Sprintf("证书安装失败: %v", err))
		return "", ""
	}

	// 设置私钥权限
	keyPath := filepath.Join(tlsDir, domain+".key")
	_ = config.EnsureKeyPermissions(keyPath)

	certPath := filepath.Join(tlsDir, domain+".fullchain.crt")

	// 更新配置
	m.config.TLS.Domain = domain
	m.config.TLS.CertFile = certPath
	m.config.TLS.KeyFile = keyPath
	m.config.TLS.Provider = caServer

	PrintSuccess(fmt.Sprintf("证书已申请并安装到 %s", tlsDir))
	return certPath, keyPath
}

// installAcmeCertToTLS 将 acme.sh 证书安装到 VasmaX TLS 目录
func (m *InstallMenu) installAcmeCertToTLS(domain string) (certFile, keyFile string) {
	acmePath := filepath.Join(os.Getenv("HOME"), ".acme.sh", "acme.sh")
	if _, err := os.Stat(acmePath); os.IsNotExist(err) {
		return "", ""
	}

	tlsDir := config.DefaultTLSDir
	os.MkdirAll(tlsDir, 0755)

	installArgs := []string{
		"--install-cert", "-d", domain,
		"--cert-file", filepath.Join(tlsDir, domain+".crt"),
		"--key-file", filepath.Join(tlsDir, domain+".key"),
		"--fullchain-file", filepath.Join(tlsDir, domain+".fullchain.crt"),
		"--reloadcmd", "systemctl restart VasmaX",
	}
	cmd := exec.Command(acmePath, installArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", ""
	}

	keyPath := filepath.Join(tlsDir, domain+".key")
	_ = config.EnsureKeyPermissions(keyPath)

	certPath := filepath.Join(tlsDir, domain+".fullchain.crt")
	return certPath, keyPath
}

// showDomainInfo 显示域名模式安装结果和分享链接
func (m *InstallMenu) showDomainInfo(users []*user.UserEntry, domain string) {
	PrintTitle("连接信息")

	serverIP := getServerIP()
	PrintInfo(fmt.Sprintf("服务器地址: %s", serverIP))
	PrintInfo(fmt.Sprintf("域名: %s", domain))
	fmt.Println()

	for _, protoName := range m.config.Protocols {
		p, ok := m.registry.Get(protoName)
		if !ok {
			continue
		}

		mode := inferProtocolMode(m.config.ProtocolModes, protoName)

		info := &protocol.ServerInfo{
			Host: serverIP,
			Port: externalPort(p),
		}

		// 域名模式协议使用域名
		if mode == "domain" {
			info.Domain = domain
			if m.config.CDN.Enabled && m.config.CDN.Address != "" {
				info.CDNHost = m.config.CDN.Address
			}
		}

		// Reality 协议使用 Reality 配置
		if strings.Contains(protoName, "reality") {
			info.Reality = &m.config.Reality
			info.Port = m.config.Reality.EffectivePort()
		}

		// 无域名模式下 sing-box 协议用 IP 作为 Domain
		if mode == "nodomain" && p.CoreType() == "singbox" {
			info.Domain = serverIP
		}

		// WS/HTTPUpgrade/gRPC 路径
		transport := p.TransportType()
		if transport == "ws" || transport == "httpupgrade" {
			info.Path = defaultWSPath(p)
		}
		if transport == "grpc" {
			info.ServiceName = defaultGRPCServiceName(p)
		}

		PrintSeparator()
		PrintInfo(fmt.Sprintf("协议: %s", protoName))

		for _, u := range users {
			apiUser := u.ToAPIUser()
			uri := p.GenerateURI(apiUser, info)
			if uri == "" {
				continue
			}

			PrintInfo(fmt.Sprintf("用户: %s", u.Email))
			PrintInfo(fmt.Sprintf("UUID: %s", u.UUID))
			fmt.Println()
			PrintInfo("分享链接:")
			fmt.Printf("  %s\n", uri)
			fmt.Println()
			PrintInfo("二维码:")
			fmt.Println(subscription.GenerateTerminalQR(uri))
		}
	}
}

// autoConfigNginx 安装协议后自动配置 Nginx 反向代理
func (m *InstallMenu) autoConfigNginx(installed []protocol.Protocol, domain string) {
	// 收集需要 Nginx 反代的协议
	var locations []nginx.ProtocolLocation
	for _, p := range installed {
		if !needsNginxProxy(p) {
			continue
		}
		transport := p.TransportType()
		loc := nginx.ProtocolLocation{
			Type:        transport,
			BackendPort: p.DefaultPort(),
		}
		if transport == "grpc" {
			loc.Path = defaultGRPCServiceName(p)
		} else {
			loc.Path = defaultWSPath(p)
		}
		locations = append(locations, loc)
	}

	if len(locations) == 0 {
		return
	}

	// 需要域名和证书
	if domain == "" {
		PrintWarning("未配置域名，跳过 Nginx 自动配置")
		return
	}

	certFile := m.config.TLS.CertFile
	keyFile := m.config.TLS.KeyFile
	if certFile == "" || keyFile == "" {
		// 尝试检测证书路径
		certFile, keyFile = config.DetectCertPath(&m.config.TLS)
	}
	if certFile == "" || keyFile == "" {
		PrintWarning("未找到 TLS 证书，跳过 Nginx 自动配置")
		PrintInfo("请先通过 TLS 证书管理申请证书，然后重新安装协议")
		return
	}

	// 检测 Nginx 版本，如果太旧则自动升级
	if nginx.NeedUpgrade() {
		oldVer := nginx.NginxVersionString()
		PrintWarning(fmt.Sprintf("Nginx 版本过低（%s），需要 1.25.1+ 才支持 http2 指令", oldVer))
		PrintInfo("正在自动升级 Nginx 到最新稳定版...")
		if err := nginx.UpgradeNginx(); err != nil {
			PrintError(fmt.Sprintf("Nginx 升级失败: %v", err))
			PrintInfo("请手动升级 Nginx 后重新安装协议")
			return
		}
		newVer := nginx.NginxVersionString()
		PrintSuccess(fmt.Sprintf("Nginx 已升级: %s → %s", oldVer, newVer))
	}

	PrintInfo("正在自动配置 Nginx 反向代理...")

	params := &nginx.NginxParams{
		Domain:    domain,
		CertFile:  certFile,
		KeyFile:   keyFile,
		Protocols: locations,
	}

	if err := m.nginxMgr.GenerateConfig(params); err != nil {
		PrintError(fmt.Sprintf("生成 Nginx 配置失败: %v", err))
		return
	}

	// 配置订阅路径
	if err := m.nginxMgr.SetupSubscribeServer(domain); err != nil {
		m.logger.WithError(err).Warn("配置订阅路径失败")
	}

	// 验证并重载 Nginx
	if err := m.nginxMgr.Reload(); err != nil {
		PrintError(fmt.Sprintf("Nginx 重载失败: %v", err))
		PrintInfo("请手动检查 Nginx 配置: nginx -t")
		return
	}

	PrintSuccess("Nginx 反向代理已自动配置")
	for _, loc := range locations {
		if loc.Type == "grpc" {
			PrintInfo(fmt.Sprintf("  %s → grpc://127.0.0.1:%d/%s", loc.Type, loc.BackendPort, loc.Path))
		} else {
			PrintInfo(fmt.Sprintf("  %s → http://127.0.0.1:%d%s", loc.Type, loc.BackendPort, loc.Path))
		}
	}
}
