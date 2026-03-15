package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/sirupsen/logrus"

	"vasmax/internal/config"
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

// NewManager 创建核心管理器
func NewManager(cfg *config.Config, logger *logrus.Logger) *Manager {
	return &Manager{
		xray:    NewXrayCore(),
		singbox: NewSingBox(),
		config:  cfg,
		logger:  logger,
	}
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
			m.logger.WithError(err).Warn("备份 Xray 二进制失败")
		}
		if err := m.installXray(ctx); err != nil {
			return err
		}
		return m.RestartXray()
	case "singbox":
		if err := backupFile(m.singbox.BinaryPath); err != nil {
			m.logger.WithError(err).Warn("备份 sing-box 二进制失败")
		}
		if err := m.installSingBox(ctx); err != nil {
			return err
		}
		return m.RestartSingBox()
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
		return m.RestartXray()
	case "singbox":
		if err := restoreFile(m.singbox.BinaryPath); err != nil {
			return fmt.Errorf("回滚 sing-box 失败: %w", err)
		}
		return m.RestartSingBox()
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
	// 1. 停止服务
	systemctl("stop", serviceName)

	// 2. 禁用服务
	exec.Command("systemctl", "disable", serviceName).Run()

	// 3. 删除 service 文件
	servicePath := "/etc/systemd/system/" + serviceName
	os.Remove(servicePath)

	// 4. daemon-reload
	systemctl("daemon-reload", "")

	// 5. 删除二进制文件
	os.Remove(binaryPath)
	os.Remove(binaryPath + ".bak")

	return nil
}

// StartAll 启动所有已安装的核心
func (m *Manager) StartAll() error {
	if fileExists(m.xray.BinaryPath) {
		// 确保 service 文件存在
		servicePath := "/etc/systemd/system/" + m.xray.ServiceName
		if !fileExists(servicePath) {
			m.installXrayService()
		}
		os.Chmod(m.xray.BinaryPath, 0755)
		if err := systemctl("start", m.xray.ServiceName); err != nil {
			m.logger.WithError(err).Error("启动 Xray 失败")
		}
	}
	if fileExists(m.singbox.BinaryPath) {
		servicePath := "/etc/systemd/system/" + m.singbox.ServiceName
		if !fileExists(servicePath) {
			m.installSingBoxService()
		}
		os.Chmod(m.singbox.BinaryPath, 0755)
		if err := systemctl("start", m.singbox.ServiceName); err != nil {
			m.logger.WithError(err).Error("启动 sing-box 失败")
		}
	}
	return nil
}

// StopAll 停止所有核心
func (m *Manager) StopAll() error {
	systemctl("stop", m.xray.ServiceName)
	systemctl("stop", m.singbox.ServiceName)
	return nil
}

// ReloadXray 热重载 Xray-core（SIGUSR1）
func (m *Manager) ReloadXray() error {
	return exec.Command("killall", "-USR1", "xray").Run()
}

// RestartXray 重启 Xray-core（自动确保 service 文件存在）
func (m *Manager) RestartXray() error {
	servicePath := "/etc/systemd/system/" + m.xray.ServiceName
	if !fileExists(servicePath) {
		if err := m.installXrayService(); err != nil {
			return fmt.Errorf("创建 Xray service 失败: %w", err)
		}
	}
	// 确保二进制有可执行权限
	os.Chmod(m.xray.BinaryPath, 0755)
	return systemctl("restart", m.xray.ServiceName)
}

// MergeSingBoxConfig 合并 sing-box 配置文件到单一 config.json
func (m *Manager) MergeSingBoxConfig() error {
	return m.singbox.MergeConfig()
}

// RestartSingBox 重启 sing-box（自动确保 service 文件存在）
func (m *Manager) RestartSingBox() error {
	servicePath := "/etc/systemd/system/" + m.singbox.ServiceName
	if !fileExists(servicePath) {
		if err := m.installSingBoxService(); err != nil {
			return fmt.Errorf("创建 sing-box service 失败: %w", err)
		}
	}
	// 确保二进制有可执行权限
	os.Chmod(m.singbox.BinaryPath, 0755)
	return systemctl("restart", m.singbox.ServiceName)
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
	// 下载 Xray-core 二进制
	tasks := []downloader.DownloadTask{
		{URL: m.xray.DownloadURL(), DestPath: m.xray.BinaryPath, Name: "xray-core"},
	}
	if err := downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
	}); err != nil {
		return err
	}

	// 设置可执行权限
	if err := os.Chmod(m.xray.BinaryPath, 0755); err != nil {
		m.logger.WithError(err).Warn("设置 Xray 可执行权限失败")
	}

	// 创建配置目录
	os.MkdirAll(m.xray.ConfDir, 0755)

	// 安装 systemd service 文件
	return m.installXrayService()
}

// installXrayService 创建 Xray systemd service 文件
func (m *Manager) installXrayService() error {
	serviceContent := fmt.Sprintf(`[Unit]
Description=Xray Service
Documentation=https://xtls.github.io
After=network.target nss-lookup.target

[Service]
Type=simple
ExecStart=%s run -confdir %s
Restart=on-failure
RestartPreventExitStatus=23
LimitNPROC=10000
LimitNOFILE=1000000

[Install]
WantedBy=multi-user.target
`, m.xray.BinaryPath, m.xray.ConfDir)

	servicePath := "/etc/systemd/system/" + m.xray.ServiceName
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("创建 Xray service 文件失败: %w", err)
	}

	if err := systemctl("daemon-reload", ""); err != nil {
		m.logger.WithError(err).Warn("systemctl daemon-reload 失败")
	}
	if err := exec.Command("systemctl", "enable", m.xray.ServiceName).Run(); err != nil {
		m.logger.WithError(err).Warn("systemctl enable xray 失败")
	}

	return nil
}

func (m *Manager) installSingBox(ctx context.Context) error {
	tasks := []downloader.DownloadTask{
		{URL: m.singbox.DownloadURL(), DestPath: m.singbox.BinaryPath, Name: "sing-box"},
	}
	if err := downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
	}); err != nil {
		return err
	}

	// 设置可执行权限
	if err := os.Chmod(m.singbox.BinaryPath, 0755); err != nil {
		m.logger.WithError(err).Warn("设置 sing-box 可执行权限失败")
	}

	// 创建配置目录
	os.MkdirAll(m.singbox.ConfDir, 0755)

	// 安装 systemd service 文件
	return m.installSingBoxService()
}

// installSingBoxService 创建 sing-box systemd service 文件
func (m *Manager) installSingBoxService() error {
	serviceContent := fmt.Sprintf(`[Unit]
Description=sing-box Service
Documentation=https://sing-box.sagernet.org
After=network.target nss-lookup.target

[Service]
Type=simple
ExecStart=%s run -C %s
Restart=on-failure
RestartPreventExitStatus=23
LimitNPROC=10000
LimitNOFILE=1000000

[Install]
WantedBy=multi-user.target
`, m.singbox.BinaryPath, m.singbox.ConfDir)

	servicePath := "/etc/systemd/system/" + m.singbox.ServiceName
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("创建 sing-box service 文件失败: %w", err)
	}

	if err := systemctl("daemon-reload", ""); err != nil {
		m.logger.WithError(err).Warn("systemctl daemon-reload 失败")
	}
	if err := exec.Command("systemctl", "enable", m.singbox.ServiceName).Run(); err != nil {
		m.logger.WithError(err).Warn("systemctl enable sing-box 失败")
	}

	return nil
}

// 辅助函数

func systemctl(action, service string) error {
	if service == "" {
		return exec.Command("systemctl", action).Run()
	}
	return exec.Command("systemctl", action, service).Run()
}

func isServiceRunning(service string) bool {
	return exec.Command("systemctl", "is-active", "--quiet", service).Run() == nil
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
