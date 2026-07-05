package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
	"vasmax/internal/protocol"
	"vasmax/pkg/downloader"
)

// CoreStatus 核心运行状态
type CoreStatus struct {
	Installed bool
	Running   bool
	Version   string
}

// Manager 核心管理器
type Manager struct {
	xray    *XrayCore
	singbox *SingBox
	config  *config.Config
	logger  *logrus.Logger
	mu      sync.Mutex
}

var coreCommandRun = func(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// NewManager 创建核心管理器
func NewManager(cfg *config.Config, logger *logrus.Logger) *Manager {
	mgr := &Manager{
		xray:    NewXrayCore(),
		singbox: NewSingBox(),
		config:  cfg,
		logger:  logger,
	}
	mgr.syncConfiguredPaths()
	return mgr
}

// InstallCore 安装指定核心（并发下载 + SHA256 校验）
func (m *Manager) InstallCore(ctx context.Context, coreType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch coreType {
	case "xray":
		return m.installXray(ctx)
	case "singbox":
		return m.installSingBox(ctx)
	default:
		return fmt.Errorf("未知核心类型: %s", coreType)
	}
}

// UpdateCore 更新核心（备份 → 下载 → 校验 → 替换 → 重启）
func (m *Manager) UpdateCore(ctx context.Context, coreType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch coreType {
	case "xray":
		// 备份旧版本
		if err := backupFile(m.xray.BinaryPath); err != nil {
			m.logWarn(err, "备份 Xray 二进制失败")
		}
		if err := m.installXray(ctx); err != nil {
			return err
		}
		return m.restartCoreIfNeeded("xray")
	case "singbox":
		if err := backupFile(m.singbox.BinaryPath); err != nil {
			m.logWarn(err, "备份 sing-box 二进制失败")
		}
		if err := m.installSingBox(ctx); err != nil {
			return err
		}
		return m.restartCoreIfNeeded("singbox")
	default:
		return fmt.Errorf("未知核心类型: %s", coreType)
	}
}

// RollbackCore 回滚到上一版本
func (m *Manager) RollbackCore(coreType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch coreType {
	case "xray":
		if err := restoreFile(m.xray.BinaryPath); err != nil {
			return fmt.Errorf("回滚 Xray 失败: %w", err)
		}
		return m.restartCoreIfNeeded("xray")
	case "singbox":
		if err := restoreFile(m.singbox.BinaryPath); err != nil {
			return fmt.Errorf("回滚 sing-box 失败: %w", err)
		}
		return m.restartCoreIfNeeded("singbox")
	default:
		return fmt.Errorf("未知核心类型: %s", coreType)
	}
}

// UninstallCore 卸载指定核心（停止服务 → 删除 service → 删除二进制）
func (m *Manager) UninstallCore(coreType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch coreType {
	case "xray":
		return m.uninstallCore(m.xray.ServiceName, m.xray.BinaryPath)
	case "singbox":
		return m.uninstallCore(m.singbox.ServiceName, m.singbox.BinaryPath)
	default:
		return fmt.Errorf("未知核心类型: %s", coreType)
	}
}

func (m *Manager) uninstallCore(serviceName, binaryPath string) error {
	var errs []error
	// 1. 停止服务
	if err := systemctl("stop", serviceName); err != nil {
		m.logWarn(err, "停止服务失败，继续卸载")
	}

	// 2. 禁用服务
	if err := coreCommandRun("systemctl", "disable", serviceName); err != nil {
		m.logWarn(err, "禁用服务失败，继续卸载")
	}

	// 3. 删除 service 文件
	servicePath := "/etc/systemd/system/" + serviceName
	if err := os.Remove(servicePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove service file: %w", err))
	}

	// 4. daemon-reload
	if err := systemctl("daemon-reload", ""); err != nil {
		errs = append(errs, fmt.Errorf("daemon-reload: %w", err))
	}

	// 5. 删除二进制文件
	if err := os.Remove(binaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove binary: %w", err))
	}
	if err := os.Remove(binaryPath + ".bak"); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove backup binary: %w", err))
	}

	return errors.Join(errs...)
}

// StartAll 启动当前配置实际需要的核心，并停止不再被协议使用的核心。
func (m *Manager) StartAll() error {
	needed := m.neededCores()
	if err := m.prepareNeededRuntimes(needed); err != nil {
		return err
	}

	var errs []error
	if needed["xray"] {
		if err := m.startPreparedCore("xray"); err != nil {
			errs = append(errs, err)
		}
	}
	if needed["singbox"] {
		if err := m.startPreparedCore("singbox"); err != nil {
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}

	m.stopUnusedCoreServices(needed)
	return nil
}

func (m *Manager) startPreparedCore(coreType string) error {
	switch coreType {
	case "xray":
		if err := m.ensureXrayService(); err != nil {
			return fmt.Errorf("install xray service: %w", err)
		}
		if err := os.Chmod(m.xray.BinaryPath, 0755); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("chmod xray: %w", err)
		}
		if err := systemctl("start", m.xray.ServiceName); err != nil {
			m.logError(err, "启动 Xray 失败")
			return fmt.Errorf("start xray: %w", err)
		}
		return nil
	case "singbox":
		if err := m.ensureSingBoxService(); err != nil {
			return fmt.Errorf("install sing-box service: %w", err)
		}
		if err := os.Chmod(m.singbox.BinaryPath, 0755); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("chmod sing-box: %w", err)
		}
		if err := systemctl("start", m.singbox.ServiceName); err != nil {
			m.logError(err, "启动 sing-box 失败")
			return fmt.Errorf("start sing-box: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown core type: %s", coreType)
	}
}

func (m *Manager) neededCores() map[string]bool {
	needed := map[string]bool{}
	if m.config == nil {
		return needed
	}
	reg := protocol.DefaultRegistry()
	for _, protoName := range m.config.Protocols {
		if p, ok := reg.Get(protoName); ok {
			needed[p.CoreType()] = true
		}
	}
	return needed
}

func (m *Manager) coreNeeded(coreType string) bool {
	return m.neededCores()[coreType]
}

func (m *Manager) restartCoreIfNeeded(coreType string) error {
	if !m.coreNeeded(coreType) {
		return nil
	}
	switch coreType {
	case "xray":
		return m.RestartXray()
	case "singbox":
		return m.RestartSingBox()
	default:
		return fmt.Errorf("unknown core type: %s", coreType)
	}
}

// EnsureRuntimeBaseConfigs regenerates DNS/outbound base config files for the
// cores required by the current protocol set. This keeps manual config edits,
// DNS menu changes, daemon startup, and managed sync on the same path.
func (m *Manager) EnsureRuntimeBaseConfigs() error {
	m.syncConfiguredPaths()
	needed := m.neededCores()
	var errs []error

	if needed["xray"] {
		if err := os.MkdirAll(m.xray.ConfDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("create xray config dir: %w", err))
		} else if err := protocol.EnsureBaseConfigs(m.xray.ConfDir, m.config); err != nil {
			errs = append(errs, fmt.Errorf("ensure xray base configs: %w", err))
		}
	}

	if needed["singbox"] {
		if err := os.MkdirAll(m.singbox.ConfDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("create sing-box config dir: %w", err))
		} else if err := protocol.EnsureSingBoxBaseConfigs(m.singbox.ConfDir, m.config); err != nil {
			errs = append(errs, fmt.Errorf("ensure sing-box base configs: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) prepareNeededRuntimes(needed map[string]bool) error {
	var errs []error
	if needed["xray"] {
		if err := m.prepareXrayRuntime(); err != nil {
			errs = append(errs, fmt.Errorf("prepare xray runtime: %w", err))
		}
	}
	if needed["singbox"] {
		if err := m.prepareSingBoxRuntime(); err != nil {
			errs = append(errs, fmt.Errorf("prepare sing-box runtime: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) prepareXrayRuntime() error {
	m.syncConfiguredPaths()
	if err := os.MkdirAll(m.xray.ConfDir, 0755); err != nil {
		return fmt.Errorf("create xray config dir: %w", err)
	}
	if err := protocol.EnsureBaseConfigs(m.xray.ConfDir, m.config); err != nil {
		return fmt.Errorf("ensure xray base configs: %w", err)
	}
	if err := os.Chmod(m.xray.BinaryPath, 0755); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("chmod xray binary: %w", err)
	}
	return m.TestXrayConfig()
}

func (m *Manager) prepareSingBoxRuntime() error {
	m.syncConfiguredPaths()
	if err := os.MkdirAll(m.singbox.ConfDir, 0755); err != nil {
		return fmt.Errorf("create sing-box config dir: %w", err)
	}
	if err := protocol.EnsureSingBoxBaseConfigs(m.singbox.ConfDir, m.config); err != nil {
		return fmt.Errorf("ensure sing-box base configs: %w", err)
	}
	if err := m.MergeSingBoxConfig(); err != nil {
		return fmt.Errorf("merge sing-box config: %w", err)
	}
	if err := os.Chmod(m.singbox.BinaryPath, 0755); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("chmod sing-box binary: %w", err)
	}
	return m.TestSingBoxConfig()
}

// StopAll 停止所有核心
func (m *Manager) StopAll() error {
	_ = m.StopXray()
	_ = m.StopSingBox()
	return nil
}

// StopXray stops only the Xray service.
func (m *Manager) StopXray() error {
	return systemctl("stop", m.xray.ServiceName)
}

// StopSingBox stops only the sing-box service.
func (m *Manager) StopSingBox() error {
	return systemctl("stop", m.singbox.ServiceName)
}

// RestartAll restarts the cores required by the current protocol set, then
// stops cores no longer referenced by the active protocols. Runtime preparation
// must pass before any service is touched, so a bad config cannot stop the
// currently working core.
func (m *Manager) RestartAll() error {
	needed := m.neededCores()
	if err := m.prepareNeededRuntimes(needed); err != nil {
		return err
	}
	var errs []error
	if needed["xray"] {
		if err := m.restartXrayPrepared(); err != nil {
			errs = append(errs, fmt.Errorf("restart xray: %w", err))
		}
	}
	if needed["singbox"] {
		if err := m.restartSingBoxPrepared(); err != nil {
			errs = append(errs, fmt.Errorf("restart sing-box: %w", err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}

	m.stopUnusedCoreServices(needed)

	return nil
}

func (m *Manager) stopUnusedCoreServices(needed map[string]bool) {
	for _, service := range unusedCoreServices(needed, m.xray.ServiceName, m.singbox.ServiceName) {
		if err := systemctl("stop", service); err != nil && m.logger != nil {
			m.logger.WithError(err).Warnf("停止不再需要的核心服务失败: %s", service)
		}
	}
}

func (m *Manager) logWarn(err error, msg string) {
	if m.logger != nil {
		m.logger.WithError(err).Warn(msg)
	}
}

func (m *Manager) logError(err error, msg string) {
	if m.logger != nil {
		m.logger.WithError(err).Error(msg)
	}
}

func unusedCoreServices(needed map[string]bool, xrayService, singboxService string) []string {
	var services []string
	if !needed["xray"] && xrayService != "" {
		services = append(services, xrayService)
	}
	if !needed["singbox"] && singboxService != "" {
		services = append(services, singboxService)
	}
	return services
}

// ReloadXray 热重载 Xray-core（SIGUSR1）
func (m *Manager) ReloadXray() error {
	if err := m.TestXrayConfig(); err != nil {
		return err
	}
	return exec.Command("killall", "-USR1", "xray").Run()
}

// RestartXray 重启 Xray-core（自动确保 service 文件存在）
func (m *Manager) RestartXray() error {
	if err := m.prepareXrayRuntime(); err != nil {
		return err
	}
	return m.restartXrayPrepared()
}

func (m *Manager) restartXrayPrepared() error {
	if err := m.ensureXrayService(); err != nil {
		return fmt.Errorf("创建 Xray service 失败: %w", err)
	}
	return systemctl("restart", m.xray.ServiceName)
}

// MergeSingBoxConfig 合并 sing-box 配置文件到单一 config.json
func (m *Manager) MergeSingBoxConfig() error {
	m.syncConfiguredPaths()
	return m.singbox.MergeConfig()
}

// TestXrayConfig validates the generated Xray config directory before reload
// or restart so a bad write cannot immediately replace the running service.
func (m *Manager) TestXrayConfig() error {
	m.syncConfiguredPaths()
	if !fileExists(m.xray.BinaryPath) {
		return fmt.Errorf("xray binary not found: %s", m.xray.BinaryPath)
	}
	output, err := exec.Command(m.xray.BinaryPath, "test", "-confdir", m.xray.ConfDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("xray config test failed: %w: %s", err, commandOutputSnippet(output))
	}
	return nil
}

// TestSingBoxConfig validates the merged sing-box config before restart.
func (m *Manager) TestSingBoxConfig() error {
	m.syncConfiguredPaths()
	if !fileExists(m.singbox.BinaryPath) {
		return fmt.Errorf("sing-box binary not found: %s", m.singbox.BinaryPath)
	}
	if !fileExists(m.singbox.ConfigFile) {
		return fmt.Errorf("sing-box config file not found: %s", m.singbox.ConfigFile)
	}
	output, err := exec.Command(m.singbox.BinaryPath, "check", "-c", m.singbox.ConfigFile).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box config check failed: %w: %s", err, commandOutputSnippet(output))
	}
	return nil
}

// RestartSingBox 重启 sing-box（自动确保 service 文件存在）
func (m *Manager) RestartSingBox() error {
	if err := m.prepareSingBoxRuntime(); err != nil {
		return err
	}
	return m.restartSingBoxPrepared()
}

func (m *Manager) restartSingBoxPrepared() error {
	if err := m.ensureSingBoxService(); err != nil {
		return fmt.Errorf("创建 sing-box service 失败: %w", err)
	}
	return systemctl("restart", m.singbox.ServiceName)
}

func (m *Manager) syncConfiguredPaths() {
	if m.config == nil {
		return
	}
	if confDir := strings.TrimSpace(m.config.Paths.XrayConf); confDir != "" {
		m.xray.ConfDir = confDir
	}
	if confDir := strings.TrimSpace(m.config.Paths.SingBoxConf); confDir != "" {
		m.singbox.ConfDir = confDir
		m.singbox.ConfigFile = singBoxConfigFileForConfDir(confDir)
	}
}

func singBoxConfigFileForConfDir(confDir string) string {
	confDir = strings.TrimSpace(confDir)
	if confDir == "" {
		confDir = "/etc/vasmax/sing-box/conf/config/"
	}
	clean := filepath.Clean(confDir)
	return filepath.Join(filepath.Dir(clean), "config.json")
}

func commandOutputSnippet(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "<no output>"
	}
	if len(text) > 2000 {
		return text[:2000] + "...(truncated)"
	}
	return text
}

// GetStatus 获取核心运行状态
func (m *Manager) GetStatus() map[string]CoreStatus {
	status := make(map[string]CoreStatus)

	xrayStatus := CoreStatus{Installed: fileExists(m.xray.BinaryPath)}
	if xrayStatus.Installed {
		xrayStatus.Version, _ = m.xray.GetVersion()
		xrayStatus.Running = isServiceRunning(m.xray.ServiceName)
	}
	status["xray"] = xrayStatus

	singboxStatus := CoreStatus{Installed: fileExists(m.singbox.BinaryPath)}
	if singboxStatus.Installed {
		singboxStatus.Version, _ = m.singbox.GetVersion()
		singboxStatus.Running = isServiceRunning(m.singbox.ServiceName)
	}
	status["singbox"] = singboxStatus

	return status
}

func (m *Manager) installXray(ctx context.Context) error {
	// 下载 Xray-core zip 包到临时文件
	zipPath := m.xray.BinaryPath + ".zip"
	tasks := []downloader.DownloadTask{
		{URL: m.xray.DownloadURL(), DestPath: zipPath, Name: "xray-core"},
	}
	if err := downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		if m.logger != nil {
			m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
		}
	}); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	// 解压 zip 包，提取 xray 二进制
	if err := extractFromZip(zipPath, "xray", m.xray.BinaryPath); err != nil {
		return fmt.Errorf("解压 Xray 失败: %w", err)
	}

	// 设置可执行权限
	if err := os.Chmod(m.xray.BinaryPath, 0755); err != nil {
		m.logWarn(err, "设置 Xray 可执行权限失败")
	}

	// 创建配置目录
	os.MkdirAll(m.xray.ConfDir, 0755)

	// 安装 systemd service 文件
	return m.installXrayService()
}

// installXrayService 创建 Xray systemd service 文件
func (m *Manager) installXrayService() error {
	m.syncConfiguredPaths()
	return m.writeSystemdService("/etc/systemd/system/"+m.xray.ServiceName, m.xray.ServiceName, m.xrayServiceContent())
}

func (m *Manager) ensureXrayService() error {
	m.syncConfiguredPaths()
	return m.ensureSystemdService("/etc/systemd/system/"+m.xray.ServiceName, m.xray.ServiceName, m.xrayServiceContent())
}

func (m *Manager) xrayServiceContent() string {
	return fmt.Sprintf(`[Unit]
Description=Xray Service
Documentation=https://xtls.github.io
After=network.target nss-lookup.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
ExecStart=%s run -confdir %s
Restart=on-failure
RestartPreventExitStatus=23
RestartSec=10
LimitNPROC=10000
LimitNOFILE=1000000

[Install]
WantedBy=multi-user.target
`, m.xray.BinaryPath, m.xray.ConfDir)
}

func (m *Manager) installSingBox(ctx context.Context) error {
	// 下载 sing-box tar.gz 包到临时文件
	tgzPath := m.singbox.BinaryPath + ".tar.gz"
	tasks := []downloader.DownloadTask{
		{URL: m.singbox.DownloadURL(), DestPath: tgzPath, Name: "sing-box"},
	}
	if err := downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		if m.logger != nil {
			m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
		}
	}); err != nil {
		return err
	}
	defer os.Remove(tgzPath)

	// 解压 tar.gz 包，提取 sing-box 二进制
	if err := extractFromTarGz(tgzPath, "sing-box", m.singbox.BinaryPath); err != nil {
		return fmt.Errorf("解压 sing-box 失败: %w", err)
	}

	// 设置可执行权限
	if err := os.Chmod(m.singbox.BinaryPath, 0755); err != nil {
		m.logWarn(err, "设置 sing-box 可执行权限失败")
	}

	// 创建配置目录
	os.MkdirAll(m.singbox.ConfDir, 0755)

	// 安装 systemd service 文件
	return m.installSingBoxService()
}

// installSingBoxService 创建 sing-box systemd service 文件
func (m *Manager) installSingBoxService() error {
	m.syncConfiguredPaths()
	return m.writeSystemdService("/etc/systemd/system/"+m.singbox.ServiceName, m.singbox.ServiceName, m.singBoxServiceContent())
}

func (m *Manager) ensureSingBoxService() error {
	m.syncConfiguredPaths()
	return m.ensureSystemdService("/etc/systemd/system/"+m.singbox.ServiceName, m.singbox.ServiceName, m.singBoxServiceContent())
}

func (m *Manager) singBoxServiceContent() string {
	return fmt.Sprintf(`[Unit]
Description=sing-box Service
Documentation=https://sing-box.sagernet.org
After=network.target nss-lookup.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
ExecStart=%s run -c %s
Restart=on-failure
RestartPreventExitStatus=23
RestartSec=10
LimitNPROC=10000
LimitNOFILE=1000000

[Install]
WantedBy=multi-user.target
`, m.singbox.BinaryPath, m.singbox.ConfigFile)
}

func (m *Manager) ensureSystemdService(servicePath, serviceName, content string) error {
	current, err := os.ReadFile(servicePath)
	if err == nil && string(current) == content {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取 service 文件失败: %w", err)
	}
	return m.writeSystemdService(servicePath, serviceName, content)
}

func (m *Manager) writeSystemdService(servicePath, serviceName, content string) error {
	if err := os.WriteFile(servicePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("创建 service 文件失败: %w", err)
	}

	if err := systemctl("daemon-reload", ""); err != nil {
		if m.logger != nil {
			m.logger.WithError(err).Warn("systemctl daemon-reload 失败")
		}
	}
	if err := coreCommandRun("systemctl", "enable", serviceName); err != nil {
		if m.logger != nil {
			m.logger.WithError(err).Warnf("systemctl enable %s 失败", serviceName)
		}
	}

	return nil
}

// 辅助函数

func systemctl(action, service string) error {
	if service == "" {
		return coreCommandRun("systemctl", action)
	}
	return coreCommandRun("systemctl", action, service)
}

func isServiceRunning(service string) bool {
	return coreCommandRun("systemctl", "is-active", "--quiet", service) == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func backupFile(path string) error {
	if !fileExists(path) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".bak", data, 0600)
}

func restoreFile(path string) error {
	bakPath := path + ".bak"
	if !fileExists(bakPath) {
		return fmt.Errorf("备份文件不存在: %s", bakPath)
	}
	return os.Rename(bakPath, path)
}

// extractFromZip 从 zip 包中提取指定文件名的文件
func extractFromZip(zipPath, targetName, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name != targetName {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("读取 zip 条目失败: %w", err)
		}

		copyErr := copyReaderAtomic(rc, destPath)
		rc.Close()

		if copyErr != nil {
			return fmt.Errorf("写入目标文件失败: %w", copyErr)
		}
		return nil
	}

	return fmt.Errorf("zip 包中未找到 %s", targetName)
}

// extractFromTarGz 从 tar.gz 包中提取指定文件名的文件
func extractFromTarGz(tgzPath, targetName, destPath string) error {
	f, err := os.Open(tgzPath)
	if err != nil {
		return fmt.Errorf("打开 tar.gz 失败: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("解压 gzip 失败: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 条目失败: %w", err)
		}

		name := filepath.Base(hdr.Name)
		if name != targetName || hdr.Typeflag != tar.TypeReg {
			continue
		}

		if copyErr := copyReaderAtomic(tr, destPath); copyErr != nil {
			return fmt.Errorf("写入目标文件失败: %w", copyErr)
		}
		return nil
	}

	return fmt.Errorf("tar.gz 包中未找到 %s", targetName)
}

func copyReaderAtomic(r io.Reader, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".extract-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	_, copyErr := io.Copy(tmp, r)
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("替换目标文件失败: %w", err)
	}
	return nil
}
