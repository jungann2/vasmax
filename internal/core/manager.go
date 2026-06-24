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

// StartAll 启动当前配置实际需要的核心，并停止不再被协议使用的核心。
func (m *Manager) StartAll() error {
	needed := m.neededCores()

	if needed["xray"] && fileExists(m.xray.BinaryPath) {
		// 确保 service 文件存在
		servicePath := "/etc/systemd/system/" + m.xray.ServiceName
		if !fileExists(servicePath) {
			m.installXrayService()
		}
		os.Chmod(m.xray.BinaryPath, 0755)
		if err := systemctl("start", m.xray.ServiceName); err != nil {
			m.logger.WithError(err).Error("启动 Xray 失败")
		}
	} else {
		_ = systemctl("stop", m.xray.ServiceName)
	}
	if needed["singbox"] && fileExists(m.singbox.BinaryPath) {
		servicePath := "/etc/systemd/system/" + m.singbox.ServiceName
		if !fileExists(servicePath) {
			m.installSingBoxService()
		}
		os.Chmod(m.singbox.BinaryPath, 0755)
		if err := systemctl("start", m.singbox.ServiceName); err != nil {
			m.logger.WithError(err).Error("启动 sing-box 失败")
		}
	} else {
		_ = systemctl("stop", m.singbox.ServiceName)
	}
	return nil
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

// RestartAll restarts the cores required by the current protocol set.
func (m *Manager) RestartAll() error {
	needed := m.neededCores()
	var errs []error

	if needed["xray"] {
		if err := m.RestartXray(); err != nil {
			errs = append(errs, fmt.Errorf("restart xray: %w", err))
		}
	}
	if needed["singbox"] {
		if err := m.MergeSingBoxConfig(); err != nil {
			errs = append(errs, fmt.Errorf("merge sing-box config: %w", err))
		} else if err := m.RestartSingBox(); err != nil {
			errs = append(errs, fmt.Errorf("restart sing-box: %w", err))
		}
	}

	return errors.Join(errs...)
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
	// 下载 Xray-core zip 包到临时文件
	zipPath := m.xray.BinaryPath + ".zip"
	tasks := []downloader.DownloadTask{
		{URL: m.xray.DownloadURL(), DestPath: zipPath, Name: "xray-core"},
	}
	if err := downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
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
	// 下载 sing-box tar.gz 包到临时文件
	tgzPath := m.singbox.BinaryPath + ".tar.gz"
	tasks := []downloader.DownloadTask{
		{URL: m.singbox.DownloadURL(), DestPath: tgzPath, Name: "sing-box"},
	}
	if err := downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		m.logger.WithFields(logrus.Fields{"name": name, "progress": pct}).Info("下载进度")
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
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
ExecStart=%s run -C %s
Restart=on-failure
RestartPreventExitStatus=23
RestartSec=10
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

		os.MkdirAll(filepath.Dir(destPath), 0755)
		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("创建目标文件失败: %w", err)
		}

		_, copyErr := io.Copy(out, rc)
		rc.Close()
		closeErr := out.Close()

		if copyErr != nil {
			return fmt.Errorf("写入目标文件失败: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("关闭目标文件失败: %w", closeErr)
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

		os.MkdirAll(filepath.Dir(destPath), 0755)
		out, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("创建目标文件失败: %w", err)
		}

		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()

		if copyErr != nil {
			return fmt.Errorf("写入目标文件失败: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("关闭目标文件失败: %w", closeErr)
		}
		return nil
	}

	return fmt.Errorf("tar.gz 包中未找到 %s", targetName)
}
