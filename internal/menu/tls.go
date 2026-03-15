package menu

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
)

// TLSMenu handles TLS certificate management in the CLI.
type TLSMenu struct {
	config *config.Config
	logger *logrus.Logger
}

// NewTLSMenu creates a new TLS menu.
func NewTLSMenu(cfg *config.Config, logger *logrus.Logger) *TLSMenu {
	return &TLSMenu{config: cfg, logger: logger}
}

// Show displays the TLS certificate management menu.
func (m *TLSMenu) Show() {
	for {
		PrintTitle("TLS 证书管理")
		PrintOption(1, "查看证书状态")
		PrintOption(2, "申请证书（acme.sh）")
		PrintOption(3, "手动续期证书")
		PrintOption(4, "切换证书提供商")
		PrintOption(5, "检测面板证书路径")
		PrintOption(6, fmt.Sprintf("TLS 版本设置（当前: %s ~ %s）", m.currentMinVersion(), m.currentMaxVersion()))
		PrintOptionStr("0", "返回上级菜单")

		choice := ReadChoice("请选择", []string{"1", "2", "3", "4", "5", "6"})
		switch choice {
		case "1":
			m.showCertStatus()
		case "2":
			m.issueCert()
		case "3":
			m.renewCert()
		case "4":
			m.switchProvider()
		case "5":
			m.detectPanelCert()
		case "6":
			m.setTLSVersion()
		case "0":
			return
		}
	}
}

func (m *TLSMenu) showCertStatus() {
	PrintTitle("证书状态")

	domain := m.config.TLS.Domain
	if domain == "" {
		PrintWarning("未配置域名，请先在配置文件中设置 tls.domain")
		return
	}

	PrintInfo(fmt.Sprintf("域名: %s", domain))
	PrintInfo(fmt.Sprintf("提供商: %s", m.providerName()))

	certFile, keyFile := config.DetectCertPath(&m.config.TLS)
	if certFile == "" || keyFile == "" {
		PrintWarning("未找到证书文件")
		PrintInfo(fmt.Sprintf("已检查路径: %s, BT 面板, 1Panel, 默认路径", config.DefaultTLSDir))
		return
	}

	PrintInfo(fmt.Sprintf("证书: %s", certFile))
	PrintInfo(fmt.Sprintf("私钥: %s", keyFile))

	info, err := config.CheckCertificate(certFile)
	if err != nil {
		PrintError(fmt.Sprintf("证书检查失败: %v", err))
		return
	}

	if info.DaysLeft <= 0 {
		PrintError("证书已过期")
	} else if info.DaysLeft <= 7 {
		PrintWarning(fmt.Sprintf("证书将在 %d 天后过期，建议尽快续期", info.DaysLeft))
	} else if info.DaysLeft <= 30 {
		PrintWarning(fmt.Sprintf("证书剩余 %d 天", info.DaysLeft))
	} else {
		PrintSuccess(fmt.Sprintf("证书有效，剩余 %d 天（到期: %s）", info.DaysLeft, info.NotAfter.Format("2006-01-02")))
	}
}

func (m *TLSMenu) issueCert() {
	PrintTitle("申请 TLS 证书")

	// 检查 acme.sh 是否安装
	acmePath := filepath.Join(os.Getenv("HOME"), ".acme.sh", "acme.sh")
	if _, err := os.Stat(acmePath); os.IsNotExist(err) {
		PrintWarning("acme.sh 未安装")
		if Confirm("是否安装 acme.sh?") {
			m.installAcme()
		} else {
			return
		}
	}

	// 输入域名
	domain := ReadInput("请输入域名")
	if domain == "" {
		return
	}

	// 选择 CA 提供商
	fmt.Println()
	PrintOption(1, "Let's Encrypt（推荐）")
	PrintOption(2, "Buypass")
	PrintOption(3, "ZeroSSL")
	provider := ReadChoice("选择证书提供商", []string{"1", "2", "3"})
	var caServer string
	switch provider {
	case "1":
		caServer = "letsencrypt"
	case "2":
		caServer = "buypass"
	case "3":
		caServer = "zerossl"
	case "0":
		return
	}

	// 选择验证方式
	fmt.Println()
	PrintOption(1, "standalone（需要 80 端口空闲）")
	PrintOption(2, "Cloudflare DNS API")
	PrintOption(3, "阿里云 DNS API")
	PrintOption(4, "Cloudflare DNS 通配符证书")
	mode := ReadChoice("选择验证方式", []string{"1", "2", "3", "4"})

	var args []string
	switch mode {
	case "1":
		args = []string{"--issue", "-d", domain, "--standalone", "--server", caServer}
	case "2":
		token := ReadInput("请输入 CF_Token")
		if token == "" {
			PrintError("CF_Token 不能为空")
			return
		}
		os.Setenv("CF_Token", token)
		args = []string{"--issue", "-d", domain, "--dns", "dns_cf", "--server", caServer}
	case "3":
		aliKey := ReadInput("请输入 Ali_Key")
		aliSecret := ReadInput("请输入 Ali_Secret")
		if aliKey == "" || aliSecret == "" {
			PrintError("Ali_Key 和 Ali_Secret 不能为空")
			return
		}
		os.Setenv("Ali_Key", aliKey)
		os.Setenv("Ali_Secret", aliSecret)
		args = []string{"--issue", "-d", domain, "--dns", "dns_ali", "--server", caServer}
	case "4":
		token := ReadInput("请输入 CF_Token")
		if token == "" {
			PrintError("CF_Token 不能为空")
			return
		}
		os.Setenv("CF_Token", token)
		args = []string{"--issue", "-d", domain, "-d", "*." + domain, "--dns", "dns_cf", "--server", caServer}
	case "0":
		return
	}

	PrintInfo("正在申请证书...")
	cmd := exec.Command(acmePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError(fmt.Sprintf("证书申请失败: %v", err))
		return
	}

	// 安装证书到 TLS 目录
	tlsDir := config.DefaultTLSDir
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
		return
	}

	// 设置私钥权限
	keyPath := filepath.Join(tlsDir, domain+".key")
	if err := config.EnsureKeyPermissions(keyPath); err != nil {
		PrintWarning(fmt.Sprintf("设置私钥权限失败: %v", err))
	}

	// 更新配置
	m.config.TLS.Domain = domain
	m.config.TLS.CertFile = filepath.Join(tlsDir, domain+".fullchain.crt")
	m.config.TLS.KeyFile = keyPath
	m.config.TLS.Provider = caServer
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
	}

	PrintSuccess(fmt.Sprintf("证书已申请并安装到 %s", tlsDir))
	fmt.Println()
	PrintInfo("证书文件路径:")
	PrintInfo(fmt.Sprintf("  证书文件:   %s", filepath.Join(tlsDir, domain+".crt")))
	PrintInfo(fmt.Sprintf("  完整链证书: %s", filepath.Join(tlsDir, domain+".fullchain.crt")))
	PrintInfo(fmt.Sprintf("  私钥文件:   %s", keyPath))
	fmt.Println()
	PrintInfo("acme.sh 已配置自动续期（cron job），续期后自动重启服务")
}

func (m *TLSMenu) renewCert() {
	PrintTitle("手动续期证书")

	domain := m.config.TLS.Domain
	if domain == "" {
		PrintWarning("未配置域名")
		return
	}

	acmePath := filepath.Join(os.Getenv("HOME"), ".acme.sh", "acme.sh")
	if _, err := os.Stat(acmePath); os.IsNotExist(err) {
		PrintError("acme.sh 未安装，无法续期")
		return
	}

	PrintInfo(fmt.Sprintf("正在续期 %s 的证书...", domain))
	cmd := exec.Command(acmePath, "--renew", "-d", domain, "--force")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError(fmt.Sprintf("续期失败: %v", err))
		PrintInfo("如果证书未到期，acme.sh 可能拒绝续期。使用 --force 已强制续期。")
		return
	}

	PrintSuccess("证书续期成功")

	// 列出证书文件路径
	tlsDir := config.DefaultTLSDir
	certFile := filepath.Join(tlsDir, domain+".crt")
	fullchainFile := filepath.Join(tlsDir, domain+".fullchain.crt")
	keyFile := filepath.Join(tlsDir, domain+".key")
	fmt.Println()
	PrintInfo("证书文件路径:")
	if fileExists(certFile) {
		PrintInfo(fmt.Sprintf("  证书文件:   %s", certFile))
	}
	if fileExists(fullchainFile) {
		PrintInfo(fmt.Sprintf("  完整链证书: %s", fullchainFile))
	}
	if fileExists(keyFile) {
		PrintInfo(fmt.Sprintf("  私钥文件:   %s", keyFile))
	}
}

func (m *TLSMenu) switchProvider() {
	PrintTitle("切换证书提供商")
	PrintInfo(fmt.Sprintf("当前提供商: %s", m.providerName()))
	fmt.Println()
	PrintOption(1, "Let's Encrypt")
	PrintOption(2, "Buypass")
	PrintOption(3, "ZeroSSL")

	choice := ReadChoice("选择新的提供商", []string{"1", "2", "3"})
	var provider string
	switch choice {
	case "1":
		provider = "letsencrypt"
	case "2":
		provider = "buypass"
	case "3":
		provider = "zerossl"
	case "0":
		return
	}

	acmePath := filepath.Join(os.Getenv("HOME"), ".acme.sh", "acme.sh")
	if _, err := os.Stat(acmePath); os.IsNotExist(err) {
		PrintError("acme.sh 未安装")
		return
	}

	cmd := exec.Command(acmePath, "--set-default-ca", "--server", provider)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError(fmt.Sprintf("切换失败: %v", err))
		return
	}

	m.config.TLS.Provider = provider
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
	}

	PrintSuccess(fmt.Sprintf("已切换到 %s", m.providerName()))
}

func (m *TLSMenu) detectPanelCert() {
	PrintTitle("检测面板证书路径")

	domain := m.config.TLS.Domain
	if domain == "" {
		domain = ReadInput("请输入域名")
		if domain == "" {
			return
		}
	}

	// 检测宝塔面板
	btCert := filepath.Join(config.BTCertDir, domain, "fullchain.pem")
	btKey := filepath.Join(config.BTCertDir, domain, "privkey.pem")
	if fileExists(btCert) && fileExists(btKey) {
		PrintSuccess(fmt.Sprintf("检测到宝塔面板证书: %s", btCert))
		if Confirm("是否使用宝塔面板证书?") {
			m.config.TLS.CertFile = btCert
			m.config.TLS.KeyFile = btKey
			m.config.TLS.Domain = domain
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess("已配置使用宝塔面板证书")
			return
		}
	}

	// 检测 1Panel
	oneCert := filepath.Join(config.OnePanelDir, domain, "ssl", "fullchain.pem")
	oneKey := filepath.Join(config.OnePanelDir, domain, "ssl", "privkey.pem")
	if fileExists(oneCert) && fileExists(oneKey) {
		PrintSuccess(fmt.Sprintf("检测到 1Panel 证书: %s", oneCert))
		if Confirm("是否使用 1Panel 证书?") {
			m.config.TLS.CertFile = oneCert
			m.config.TLS.KeyFile = oneKey
			m.config.TLS.Domain = domain
			_ = config.SaveConfig(config.DefaultConfigPath, m.config)
			PrintSuccess("已配置使用 1Panel 证书")
			return
		}
	}

	// 检测默认路径
	defCert := filepath.Join(config.DefaultTLSDir, domain+".crt")
	defKey := filepath.Join(config.DefaultTLSDir, domain+".key")
	if fileExists(defCert) && fileExists(defKey) {
		PrintSuccess(fmt.Sprintf("检测到默认路径证书: %s", defCert))
		return
	}

	PrintWarning("未检测到任何面板或默认路径的证书")
}

func (m *TLSMenu) installAcme() {
	PrintInfo("正在安装 acme.sh...")
	cmd := exec.Command("bash", "-c", "curl -fsSL https://get.acme.sh | sh -s email=admin@example.com")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError(fmt.Sprintf("acme.sh 安装失败: %v", err))
		return
	}
	PrintSuccess("acme.sh 安装成功")
}

func (m *TLSMenu) providerName() string {
	switch strings.ToLower(m.config.TLS.Provider) {
	case "letsencrypt", "":
		return "Let's Encrypt"
	case "buypass":
		return "Buypass"
	case "zerossl":
		return "ZeroSSL"
	default:
		return m.config.TLS.Provider
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// currentMinVersion 返回当前最低 TLS 版本（显示用）
func (m *TLSMenu) currentMinVersion() string {
	if m.config.TLS.MinVersion == "" {
		return "1.2"
	}
	return m.config.TLS.MinVersion
}

// currentMaxVersion 返回当前最高 TLS 版本（显示用）
func (m *TLSMenu) currentMaxVersion() string {
	if m.config.TLS.MaxVersion == "" {
		return "1.3"
	}
	return m.config.TLS.MaxVersion
}

// setTLSVersion 交互式设置 TLS 版本范围
func (m *TLSMenu) setTLSVersion() {
	PrintTitle("TLS 版本设置")
	PrintInfo(fmt.Sprintf("当前版本范围: TLS %s ~ TLS %s", m.currentMinVersion(), m.currentMaxVersion()))
	fmt.Println()
	PrintInfo("选择最低 TLS 版本（客户端必须支持此版本及以上）:")
	PrintOption(1, "TLS 1.0（兼容性最好，安全性低）")
	PrintOption(2, "TLS 1.1（已废弃，不推荐）")
	PrintOption(3, "TLS 1.2（推荐，安全且兼容）")
	PrintOption(4, "TLS 1.3（最安全，部分旧客户端不支持）")
	PrintOptionStr("0", "取消")

	minChoice := ReadChoice("最低版本", []string{"1", "2", "3", "4"})
	var minVer string
	switch minChoice {
	case "1":
		minVer = "1.0"
	case "2":
		minVer = "1.1"
	case "3":
		minVer = "1.2"
	case "4":
		minVer = "1.3"
	case "0":
		return
	}

	// 最高版本：只有当最低版本不是 1.3 时才询问
	maxVer := "1.3"
	if minVer != "1.3" {
		fmt.Println()
		PrintInfo("选择最高 TLS 版本:")
		PrintOption(1, "TLS 1.2")
		PrintOption(2, "TLS 1.3（推荐）")
		PrintOptionStr("0", "取消")

		maxChoice := ReadChoice("最高版本", []string{"1", "2"})
		switch maxChoice {
		case "1":
			maxVer = "1.2"
		case "2":
			maxVer = "1.3"
		case "0":
			return
		}

		// 校验：最高版本不能低于最低版本
		tlsOrder := map[string]int{"1.0": 0, "1.1": 1, "1.2": 2, "1.3": 3}
		if tlsOrder[minVer] > tlsOrder[maxVer] {
			PrintError("最高版本不能低于最低版本")
			return
		}
	}

	m.config.TLS.MinVersion = minVer
	m.config.TLS.MaxVersion = maxVer
	if err := config.SaveConfig(config.DefaultConfigPath, m.config); err != nil {
		PrintError(fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	PrintSuccess(fmt.Sprintf("TLS 版本范围已设置为: %s ~ %s", minVer, maxVer))
	PrintInfo("重启服务后生效（核心管理 → 重启核心）")
}
