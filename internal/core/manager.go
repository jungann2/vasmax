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

// StartAll 启动所有已安装的核心
func (m *Manager) StartAll() error {
	if fileExists(m.xray.BinaryPath) {
		if err := systemctl("start", m.xray.ServiceName); err != nil {
			m.logger.WithError(err).Error("启动 Xray 失败")
		}
	}
	if fileExists(m.singbox.BinaryPath) {
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

// RestartXray 重启 Xray-core
func (m *Manager) RestartXray() error {
	return systemctl("restart", m.xray.ServiceName)
}

// MergeSingBoxConfig 合并 sing-box 配置文件到单一 config.json
func (m *Manager) MergeSingBoxConfig() error {
	return m.singbox.MergeConfig()
}

// RestartSingBox 重启 sing-box
func (m *Manager) RestartSingBox() error {
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
	// 实际安装逻辑由 CLI 菜单调用时提供下载 URL
	tasks := []downloader.DownloadTask{
		{URL: m.xray.DownloadURL(), DestPath: m.xray.BinaryPath, Name: "xray-core"},
	}
	return downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
	})
}

func (m *Manager) installSingBox(ctx context.Context) error {
	tasks := []downloader.DownloadTask{
		{URL: m.singbox.DownloadURL(), DestPath: m.singbox.BinaryPath, Name: "sing-box"},
	}
	return downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
	})
}

// 辅助函数

func systemctl(action, service string) error {
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
	return os.WriteFile(path+".bak", data, 0755)
}

func restoreFile(path string) error {
	bakPath := path + ".bak"
	if !fileExists(bakPath) {
		return fmt.Errorf("备份文件不存在: %s", bakPath)
	}
	return os.Rename(bakPath, path)
}
