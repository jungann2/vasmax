// Package nginx provides Nginx configuration management.
package nginx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"vasmax/internal/security"

	"github.com/sirupsen/logrus"
)

// Default paths for Nginx configuration.
const (
	DefaultConfDir  = "/etc/nginx/conf.d/"
	DefaultHTMLDir  = "/usr/share/nginx/html"
	AllowedNginxDir = "/etc/nginx/"
)

// ProtocolLocation describes a protocol's Nginx location block.
type ProtocolLocation struct {
	Type        string // ws/grpc/httpupgrade
	Path        string
	BackendPort int
}

// NginxParams holds parameters for generating Nginx configuration.
type NginxParams struct {
	Domain    string
	CertFile  string
	KeyFile   string
	Protocols []ProtocolLocation
}

// Manager manages Nginx configuration files.
type Manager struct {
	confDir string
	logger  *logrus.Logger
	mu      sync.Mutex
}

// NewManager creates a new Nginx configuration manager.
func NewManager(confDir string, logger *logrus.Logger) *Manager {
	if confDir == "" {
		confDir = DefaultConfDir
	}
	return &Manager{confDir: confDir, logger: logger}
}

// validateNginxPath ensures the path is within the allowed Nginx directory.
func (m *Manager) validateNginxPath(path string) error {
	return security.ValidatePath(path, []string{AllowedNginxDir})
}

// GenerateConfig generates the main Nginx server configuration based on installed protocols.
func (m *Manager) GenerateConfig(params *NginxParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := security.ValidateDomain(params.Domain); err != nil {
		return fmt.Errorf("invalid domain: %w", err)
	}

	confPath := filepath.Join(m.confDir, params.Domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	conf := generateServerBlock(params)

	if err := security.AtomicWrite(confPath, []byte(conf), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Infof("nginx config generated: %s", confPath)
	return nil
}

// AddLocation adds a location block for a protocol to the domain config.
func (m *Manager) AddLocation(domain, protocol, path string, backendPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	confPath := filepath.Join(m.confDir, domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read nginx config: %w", err)
	}

	locationBlock := generateLocationBlock(protocol, path, backendPort)
	marker := "# --- END LOCATIONS ---"
	content := string(data)

	if !strings.Contains(content, marker) {
		return fmt.Errorf("nginx config missing location marker")
	}

	content = strings.Replace(content, marker, locationBlock+"\n"+marker, 1)

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Infof("added location for protocol %s path %s", protocol, path)
	return nil
}

// RemoveLocation removes a location block for a protocol from the domain config.
func (m *Manager) RemoveLocation(domain, protocol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	confPath := filepath.Join(m.confDir, domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read nginx config: %w", err)
	}

	startMarker := fmt.Sprintf("# --- BEGIN %s ---", protocol)
	endMarker := fmt.Sprintf("# --- END %s ---", protocol)

	content := string(data)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	if startIdx == -1 || endIdx == -1 {
		m.logger.Warnf("location block for protocol %s not found", protocol)
		return nil
	}

	content = content[:startIdx] + content[endIdx+len(endMarker)+1:]

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Infof("removed location for protocol %s", protocol)
	return nil
}

// SetupSubscribeServer configures the subscription server location.
func (m *Manager) SetupSubscribeServer(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := security.ValidateDomain(domain); err != nil {
		return fmt.Errorf("invalid domain: %w", err)
	}

	confPath := filepath.Join(m.confDir, domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read nginx config: %w", err)
	}

	subscribeBlock := generateSubscribeLocation()
	marker := "# --- END LOCATIONS ---"
	content := string(data)

	if !strings.Contains(content, marker) {
		return fmt.Errorf("nginx config missing location marker")
	}

	// Remove existing subscribe block if present.
	content = removeBlock(content, "SUBSCRIBE")
	content = strings.Replace(content, marker, subscribeBlock+"\n"+marker, 1)

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Info("subscribe server location configured")
	return nil
}

// Validate runs nginx -t to validate the configuration.
func (m *Manager) Validate() error {
	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx config validation failed: %s: %w", string(output), err)
	}
	return nil
}

// Reload validates and then reloads Nginx.
// Falls back to systemctl restart if reload fails (e.g. after upgrade, PID file missing).
func (m *Manager) Reload() error {
	if err := m.Validate(); err != nil {
		return err
	}
	cmd := exec.Command("nginx", "-s", "reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// PID file missing after upgrade — fall back to systemctl restart
		m.logger.Warnf("nginx -s reload failed: %s, trying systemctl restart", strings.TrimSpace(string(output)))
		restartCmd := exec.Command("systemctl", "restart", "nginx")
		restartOut, restartErr := restartCmd.CombinedOutput()
		if restartErr != nil {
			return fmt.Errorf("nginx restart failed: %s: %w", string(restartOut), restartErr)
		}
		m.logger.Info("nginx restarted via systemctl")
		return nil
	}
	m.logger.Info("nginx reloaded successfully")
	return nil
}

// NginxVersion returns the installed Nginx version as (major, minor, patch).
// Returns (0,0,0) if nginx is not installed or version cannot be parsed.
func NginxVersion() (major, minor, patch int) {
	cmd := exec.Command("nginx", "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, 0
	}
	// nginx -v outputs to stderr: "nginx version: nginx/1.24.0"
	line := strings.TrimSpace(string(output))
	idx := strings.Index(line, "nginx/")
	if idx == -1 {
		return 0, 0, 0
	}
	verStr := line[idx+6:]
	// strip anything after space (e.g. "(Ubuntu)")
	if sp := strings.IndexByte(verStr, ' '); sp != -1 {
		verStr = verStr[:sp]
	}
	parts := strings.Split(verStr, ".")
	if len(parts) < 2 {
		return 0, 0, 0
	}
	major, _ = strconv.Atoi(parts[0])
	minor, _ = strconv.Atoi(parts[1])
	if len(parts) >= 3 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return major, minor, patch
}

// NginxVersionString returns the version as a string like "1.24.0".
func NginxVersionString() string {
	maj, min, pat := NginxVersion()
	if maj == 0 && min == 0 && pat == 0 {
		return "未安装"
	}
	return fmt.Sprintf("%d.%d.%d", maj, min, pat)
}

// NeedUpgrade checks if Nginx needs upgrade for http2 directive support (requires >= 1.25.1).
func NeedUpgrade() bool {
	maj, min, pat := NginxVersion()
	if maj == 0 && min == 0 && pat == 0 {
		return false // not installed, will be handled elsewhere
	}
	// http2 on; directive requires nginx >= 1.25.1
	if maj > 1 {
		return false
	}
	if maj == 1 && min > 25 {
		return false
	}
	if maj == 1 && min == 25 && pat >= 1 {
		return false
	}
	return true
}

// UpgradeNginx upgrades Nginx to the latest stable version using the OS package manager.
func UpgradeNginx() error {
	// Detect OS and use appropriate upgrade method
	var err error
	if fileExistsNginx("/etc/debian_version") {
		err = upgradeNginxDebian()
	} else if fileExistsNginx("/etc/redhat-release") || fileExistsNginx("/etc/centos-release") {
		err = upgradeNginxRHEL()
	} else {
		return fmt.Errorf("不支持的操作系统，请手动升级 Nginx 到 1.25.1+")
	}
	if err != nil {
		return err
	}

	// Restart nginx after upgrade to pick up new binary and reset PID file
	restartCmd := exec.Command("systemctl", "restart", "nginx")
	restartOut, restartErr := restartCmd.CombinedOutput()
	if restartErr != nil {
		// Try systemctl start if restart fails
		startCmd := exec.Command("systemctl", "start", "nginx")
		startOut, startErr := startCmd.CombinedOutput()
		if startErr != nil {
			return fmt.Errorf("Nginx 升级后启动失败: %s: %w", string(startOut), startErr)
		}
		_ = restartOut // suppress unused
	}
	return nil
}

func upgradeNginxDebian() error {
	// Install prerequisites
	cmds := [][]string{
		{"apt-get", "install", "-y", "curl", "gnupg2", "ca-certificates", "lsb-release"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	// Add nginx official signing key
	keyCmd := exec.Command("bash", "-c", "curl -fsSL https://nginx.org/keys/nginx_signing.key | gpg --dearmor -o /usr/share/keyrings/nginx-archive-keyring.gpg --yes")
	keyCmd.Stdout = os.Stdout
	keyCmd.Stderr = os.Stderr
	if err := keyCmd.Run(); err != nil {
		return fmt.Errorf("添加 Nginx GPG 密钥失败: %w", err)
	}

	// Detect codename
	codeOut, err := exec.Command("bash", "-c", "lsb_release -cs 2>/dev/null || echo jammy").CombinedOutput()
	if err != nil {
		return fmt.Errorf("检测系统版本失败: %w", err)
	}
	codename := strings.TrimSpace(string(codeOut))

	// Add nginx stable repo
	repoLine := fmt.Sprintf("deb [signed-by=/usr/share/keyrings/nginx-archive-keyring.gpg] https://nginx.org/packages/mainline/ubuntu %s nginx", codename)
	// Try ubuntu first, fallback to debian
	distOut, _ := exec.Command("bash", "-c", "cat /etc/os-release | grep ^ID= | cut -d= -f2").CombinedOutput()
	distID := strings.TrimSpace(strings.Trim(string(distOut), "\""))
	if distID == "debian" {
		repoLine = fmt.Sprintf("deb [signed-by=/usr/share/keyrings/nginx-archive-keyring.gpg] https://nginx.org/packages/mainline/debian %s nginx", codename)
	}

	if err := os.WriteFile("/etc/apt/sources.list.d/nginx.list", []byte(repoLine+"\n"), 0644); err != nil {
		return fmt.Errorf("写入 Nginx 源失败: %w", err)
	}

	// Pin nginx packages to prefer official repo
	pinContent := "Package: *\nPin: origin nginx.org\nPin-Priority: 900\n"
	_ = os.WriteFile("/etc/apt/preferences.d/99nginx", []byte(pinContent), 0644)

	// Update and install
	updateCmd := exec.Command("apt-get", "update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("apt-get update 失败: %w", err)
	}

	installCmd := exec.Command("apt-get", "install", "-y", "nginx")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("Nginx 升级失败: %w", err)
	}

	return nil
}

func upgradeNginxRHEL() error {
	// Add nginx official repo
	repoContent := `[nginx-mainline]
name=nginx mainline repo
baseurl=https://nginx.org/packages/mainline/centos/$releasever/$basearch/
gpgcheck=1
enabled=1
gpgkey=https://nginx.org/keys/nginx_signing.key
module_hotfixes=true
`
	if err := os.WriteFile("/etc/yum.repos.d/nginx-mainline.repo", []byte(repoContent), 0644); err != nil {
		return fmt.Errorf("写入 Nginx 源失败: %w", err)
	}

	installCmd := exec.Command("yum", "install", "-y", "nginx")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("Nginx 升级失败: %w", err)
	}

	return nil
}

func fileExistsNginx(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// removeBlock removes a named block delimited by BEGIN/END markers.
func removeBlock(content, name string) string {
	startMarker := fmt.Sprintf("# --- BEGIN %s ---", name)
	endMarker := fmt.Sprintf("# --- END %s ---", name)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)
	if startIdx == -1 || endIdx == -1 {
		return content
	}
	return content[:startIdx] + content[endIdx+len(endMarker)+1:]
}
