package menu

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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

	// 只列出需要域名的协议（非 Reality）
	allProtos := m.registry.ListAll()
	var domainProtos []protocol.Protocol
	for _, p := range allProtos {
		if strings.Contains(p.Name(), "reality") {
			continue
		}
		domainProtos = append(domainProtos, p)
	}

	for i, p := range domainProtos {
		installed := ""
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				mode := m.config.ProtocolModes[p.Name()]
				if mode == "nodomain" {
					installed = Yellow(" [已安装无域名版本]")
				} else if mode == "domain" {
					installed = Green(" [已安装绑定域名版本]")
				} else {
					installed = Green(" [已安装]")
				}
				break
			}
		}
		PrintOption(i+1, fmt.Sprintf("%-30s (%s)%s", p.Name(), p.CoreType(), installed))
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

	// 域名输入（此列表所有协议都需要域名）
	domain := ReadInput("请输入域名")
	if domain == "" {
		PrintError("此安装方式需要域名，如无域名请使用一键 Reality 组合安装")
		return
	}
	if err := security.ValidateDomain(domain); err != nil {
		PrintError(fmt.Sprintf("域名无效: %v", err))
		return
	}

	// 创建回滚快照
	snap, err := m.rollbackMgr.CreateSnapshot()
	if err != nil {
		m.logger.WithError(err).Warn("创建回滚快照失败")
	}

	// 安装核心
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

		_ = domain // Used in config generation
		PrintSuccess(fmt.Sprintf("%s 安装完成", p.Name()))
	}

	// 保存配置
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
	}

	// 自动配置 Nginx 反代（ws/grpc/httpupgrade 协议需要）
	m.autoConfigNginx(selected, domain)

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

	// 清理快照
	if snap != nil {
		m.rollbackMgr.CleanSnapshot(snap)
	}
}

func (m *InstallMenu) installReality() {
	PrintTitle("一键 Reality 组合安装（无域名）")

	// 列出所有 Reality 协议
	allProtos := m.registry.ListAll()
	var realityProtos []protocol.Protocol
	for _, p := range allProtos {
		if strings.Contains(p.Name(), "reality") {
			realityProtos = append(realityProtos, p)
		}
	}

	for i, p := range realityProtos {
		installed := ""
		for _, ip := range m.config.Protocols {
			if ip == p.Name() {
				mode := m.config.ProtocolModes[p.Name()]
				if mode == "domain" {
					installed = Yellow(" [已安装绑定域名版本]")
				} else if mode == "nodomain" {
					installed = Green(" [已安装无域名版本]")
				} else {
					installed = Green(" [已安装]")
				}
				break
			}
		}
		PrintOption(i+1, fmt.Sprintf("%-30s (%s)%s", p.Name(), p.CoreType(), installed))
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

	// 7. 生成 Xray inbound 配置文件
	PrintInfo("正在生成 Xray 配置...")
	apiUsers := make([]*api.User, 0, len(users))
	for _, u := range users {
		apiUsers = append(apiUsers, u.ToAPIUser())
	}

	for _, p := range selected {
		params := &protocol.InboundParams{
			Port:    p.DefaultPort(),
			Users:   apiUsers,
			Tag:     p.Name(),
			Reality: &m.config.Reality,
		}

		inboundJSON, err := p.GenerateInbound(params)
		if err != nil {
			PrintError(fmt.Sprintf("生成 %s 入站配置失败: %v", p.Name(), err))
			continue
		}

		wrapper := map[string]interface{}{
			"inbounds": []json.RawMessage{inboundJSON},
		}
		confPath := filepath.Join(m.config.Paths.XrayConf, fmt.Sprintf("05_%s_inbounds.json", p.Name()))
		if err := security.AtomicWriteJSON(confPath, wrapper, 0644); err != nil {
			PrintError(fmt.Sprintf("写入 %s 配置失败: %v", p.Name(), err))
			continue
		}
		PrintSuccess(fmt.Sprintf("%s 配置已生成", p.Name()))
	}

	// 8. 保存配置
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	// 9. 启动 Xray
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

	// 10. 生成订阅
	if m.subMgr != nil {
		_ = m.subMgr.GenerateAll()
	}

	// 11. 显示安装结果和分享链接
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
	PrintInfo(fmt.Sprintf("端口: 443"))
	PrintInfo(fmt.Sprintf("伪装域名: %s", m.config.Reality.ServerName))
	PrintInfo(fmt.Sprintf("PublicKey: %s", m.config.Reality.PublicKey))
	PrintInfo(fmt.Sprintf("ShortID: %s", m.config.Reality.ShortID))
	fmt.Println()

	p, ok := m.registry.Get("vless_reality_vision")
	if !ok {
		return
	}

	serverInfo := &protocol.ServerInfo{
		Host:    serverIP,
		Port:    443,
		Reality: &m.config.Reality,
	}

	for _, u := range users {
		apiUser := u.ToAPIUser()
		uri := p.GenerateURI(apiUser, serverInfo)

		PrintSeparator()
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

// getServerIP 获取服务器公网 IP
func getServerIP() string {
	// 尝试通过出站连接获取本机 IP
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
		mode := m.config.ProtocolModes[p]
		modeStr := ""
		if mode == "domain" {
			modeStr = Green(" (绑定域名)")
		} else if mode == "nodomain" {
			modeStr = Cyan(" (无域名)")
		}
		PrintOption(i+1, p+modeStr)
	}
}

func (m *InstallMenu) uninstallProtocol() {
	PrintTitle("卸载协议")
	if len(m.config.Protocols) == 0 {
		PrintInfo("暂无已安装协议")
		return
	}
	for i, p := range m.config.Protocols {
		PrintOption(i+1, p)
	}

	input := ReadInput("请输入要卸载的协议编号")
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(m.config.Protocols) {
		PrintError("无效编号")
		return
	}

	name := m.config.Protocols[idx-1]
	if !Confirm(fmt.Sprintf("确认卸载 %s?", name)) {
		return
	}

	// 移除协议
	m.config.Protocols = append(m.config.Protocols[:idx-1], m.config.Protocols[idx:]...)
	delete(m.config.ProtocolModes, name)
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("%s 已卸载", name))
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
