package rollback

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"vasmax/internal/security"

	"github.com/sirupsen/logrus"
)

// Snapshot 安装前状态快照
type Snapshot struct {
	Timestamp    time.Time         `json:"timestamp"`
	CoreVersions map[string]string `json:"core_versions"` // {"xray": "1.8.x", "singbox": "1.x.x"}
	ConfigFiles  []string          `json:"config_files"`  // 已备份的配置文件路径
	Services     []string          `json:"services"`      // 需要恢复的 systemd 服务
}

// Manager 回滚管理器
type Manager struct {
	snapshotDir string // 快照存储目录
	logger      *logrus.Logger
}

// NewManager 创建回滚管理器
func NewManager(snapshotDir string, logger *logrus.Logger) *Manager {
	os.MkdirAll(snapshotDir, 0755)
	return &Manager{
		snapshotDir: snapshotDir,
		logger:      logger,
	}
}

// CreateSnapshot 记录安装前状态
func (m *Manager) CreateSnapshot() (*Snapshot, error) {
	snap := &Snapshot{
		Timestamp:    time.Now(),
		CoreVersions: make(map[string]string),
		ConfigFiles:  make([]string, 0),
		Services:     make([]string, 0),
	}

	// 记录核心版本
	if output, err := exec.Command("/usr/local/xray-core/xray", "version").Output(); err == nil {
		snap.CoreVersions["xray"] = string(output[:min(len(output), 50)])
	}
	if output, err := exec.Command("/usr/local/sing-box/sing-box", "version").Output(); err == nil {
		snap.CoreVersions["singbox"] = string(output[:min(len(output), 50)])
	}

	// 备份配置文件
	configPaths := []string{
		"/etc/vasmax/config.yaml",
		"/etc/vasmax/xray/conf/",
		"/etc/vasmax/sing-box/conf/",
	}
	for _, p := range configPaths {
		if _, err := os.Stat(p); err == nil {
			bakPath := filepath.Join(m.snapshotDir, filepath.Base(p)+".bak")
			if err := copyPath(p, bakPath); err == nil {
				snap.ConfigFiles = append(snap.ConfigFiles, p)
			}
		}
	}

	// 保存快照元数据
	snapFile := filepath.Join(m.snapshotDir, "snapshot.json")
	if err := security.AtomicWriteJSON(snapFile, snap, 0644); err != nil {
		return nil, fmt.Errorf("保存快照元数据失败: %w", err)
	}

	m.logger.Info("安装快照已创建")
	return snap, nil
}

// Rollback 恢复备份配置、核心二进制、重启服务
func (m *Manager) Rollback(snap *Snapshot) error {
	var lastErr error

	// 恢复配置文件
	for _, p := range snap.ConfigFiles {
		bakPath := filepath.Join(m.snapshotDir, filepath.Base(p)+".bak")
		if err := copyPath(bakPath, p); err != nil {
			m.logger.WithError(err).Errorf("恢复 %s 失败", p)
			lastErr = err
		}
	}

	// 恢复核心二进制
	for core := range snap.CoreVersions {
		var binPath string
		switch core {
		case "xray":
			binPath = "/usr/local/xray-core/xray"
		case "singbox":
			binPath = "/usr/local/sing-box/sing-box"
		}
		bakPath := binPath + ".bak"
		if _, err := os.Stat(bakPath); err == nil {
			if err := os.Rename(bakPath, binPath); err != nil {
				m.logger.WithError(err).Errorf("恢复 %s 二进制失败", core)
				lastErr = err
			}
		}
	}

	// 重启服务
	for _, svc := range snap.Services {
		if err := exec.Command("systemctl", "restart", svc).Run(); err != nil {
			m.logger.WithError(err).Errorf("重启 %s 失败", svc)
			lastErr = err
		}
	}

	if lastErr != nil {
		m.logger.Error("回滚部分失败，请检查日志并手动恢复")
		return fmt.Errorf("回滚部分失败: %w", lastErr)
	}

	m.logger.Info("回滚完成")
	return nil
}

// CleanSnapshot 安装成功后清理快照
func (m *Manager) CleanSnapshot(snap *Snapshot) error {
	for _, p := range snap.ConfigFiles {
		bakPath := filepath.Join(m.snapshotDir, filepath.Base(p)+".bak")
		os.Remove(bakPath)
	}
	os.Remove(filepath.Join(m.snapshotDir, "snapshot.json"))
	m.logger.Info("快照已清理")
	return nil
}

// copyPath 复制文件或目录
func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return exec.Command("cp", "-r", src, dst).Run()
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return security.AtomicWrite(dst, data, info.Mode())
}
