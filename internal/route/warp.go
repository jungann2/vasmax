package route

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// WARPPort is the default WARP socks5 proxy port.
const WARPPort = 31303

// WARPManager manages Cloudflare WARP client installation and configuration.
type WARPManager struct {
	logger *logrus.Logger
}

// NewWARPManager creates a new WARP manager.
func NewWARPManager(logger *logrus.Logger) *WARPManager {
	return &WARPManager{logger: logger}
}

// Install installs the WARP client based on the OS type.
func (w *WARPManager) Install() error {
	if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
		return fmt.Errorf("WARP is not supported on ARM architecture")
	}

	osType, err := detectOSType()
	if err != nil {
		return fmt.Errorf("failed to detect OS: %w", err)
	}

	switch osType {
	case "debian", "ubuntu":
		return w.installDebian()
	case "centos", "rhel", "fedora":
		return w.installRHEL()
	default:
		return fmt.Errorf("unsupported OS for WARP: %s", osType)
	}
}

// Setup registers and configures WARP in proxy mode.
func (w *WARPManager) Setup() error {
	// Register WARP.
	if err := runWarpCmd("register"); err != nil {
		// May already be registered.
		w.logger.Warn("WARP register returned error (may already be registered)")
	}

	// Set proxy mode.
	if err := runWarpCmd("set-mode", "proxy"); err != nil {
		return fmt.Errorf("failed to set WARP proxy mode: %w", err)
	}

	// Set proxy port.
	if err := runWarpCmd("set-proxy-port", fmt.Sprintf("%d", WARPPort)); err != nil {
		return fmt.Errorf("failed to set WARP proxy port: %w", err)
	}

	// Enable always-on.
	if err := runWarpCmd("enable-always-on"); err != nil {
		w.logger.Warn("failed to enable WARP always-on")
	}

	// Connect.
	if err := runWarpCmd("connect"); err != nil {
		return fmt.Errorf("failed to connect WARP: %w", err)
	}

	w.logger.Info("WARP configured and connected")
	return nil
}

// TestConnection tests the WARP connection by checking if the socks5 proxy port is reachable.
func (w *WARPManager) TestConnection() error {
	proxyAddr := fmt.Sprintf("127.0.0.1:%d", WARPPort)

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", proxyAddr)
	if err != nil {
		return fmt.Errorf("WARP proxy not reachable at %s: %w", proxyAddr, err)
	}
	conn.Close()

	// Verify WARP status via warp-cli.
	output, err := exec.Command("warp-cli", "status").CombinedOutput()
	if err != nil {
		return fmt.Errorf("warp-cli status failed: %w", err)
	}
	if !strings.Contains(string(output), "Connected") {
		return fmt.Errorf("WARP not connected: %s", strings.TrimSpace(string(output)))
	}

	w.logger.Info("WARP connection test passed")
	return nil
}

// Uninstall removes the WARP client.
func (w *WARPManager) Uninstall() error {
	_ = runWarpCmd("disconnect")
	_ = runWarpCmd("disable-always-on")

	osType, _ := detectOSType()
	switch osType {
	case "debian", "ubuntu":
		return runSystemCmd("apt-get", "remove", "-y", "cloudflare-warp")
	case "centos", "rhel", "fedora":
		return runSystemCmd("yum", "remove", "-y", "cloudflare-warp")
	}
	return nil
}

// IsInstalled checks if WARP client is installed.
func (w *WARPManager) IsInstalled() bool {
	_, err := exec.LookPath("warp-cli")
	return err == nil
}

// installDebian installs WARP on Debian/Ubuntu.
func (w *WARPManager) installDebian() error {
	cmds := [][]string{
		{"apt-get", "update"},
		{"apt-get", "install", "-y", "curl", "gnupg"},
	}
	for _, c := range cmds {
		if err := runSystemCmd(c[0], c[1:]...); err != nil {
			return err
		}
	}

	// Add Cloudflare GPG key and repo.
	if err := runSystemCmd("bash", "-c",
		"curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor -o /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg"); err != nil {
		return fmt.Errorf("failed to add WARP GPG key: %w", err)
	}

	if err := runSystemCmd("bash", "-c",
		`echo "deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/cloudflare-client.list`); err != nil {
		return fmt.Errorf("failed to add WARP repo: %w", err)
	}

	if err := runSystemCmd("apt-get", "update"); err != nil {
		return err
	}
	return runSystemCmd("apt-get", "install", "-y", "cloudflare-warp")
}

// installRHEL installs WARP on CentOS/RHEL/Fedora.
func (w *WARPManager) installRHEL() error {
	if err := runSystemCmd("bash", "-c",
		"curl -fsSl https://pkg.cloudflareclient.com/cloudflare-warp-ascii.repo > /etc/yum.repos.d/cloudflare-warp.repo"); err != nil {
		return fmt.Errorf("failed to add WARP repo: %w", err)
	}
	return runSystemCmd("yum", "install", "-y", "cloudflare-warp")
}

// runWarpCmd runs a warp-cli command.
func runWarpCmd(args ...string) error {
	cmd := exec.Command("warp-cli", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("warp-cli %s failed: %s: %w", strings.Join(args, " "), string(output), err)
	}
	return nil
}

// runSystemCmd runs a system command.
func runSystemCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %s: %w", name, strings.Join(args, " "), string(output), err)
	}
	return nil
}

// detectOSType detects the Linux distribution type.
func detectOSType() (string, error) {
	// Check /etc/os-release.
	cmd := exec.Command("bash", "-c", "source /etc/os-release 2>/dev/null && echo $ID")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect OS type: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
