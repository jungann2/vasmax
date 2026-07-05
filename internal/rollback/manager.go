package rollback

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"vasmax/internal/security"

	"github.com/sirupsen/logrus"
)

// Snapshot 安装前状态快照
type Snapshot struct {
	Timestamp     time.Time         `json:"timestamp"`
	CoreVersions  map[string]string `json:"core_versions"`            // {"xray": "1.8.x", "singbox": "1.x.x"}
	CoreBackups   []CoreBackup      `json:"core_backups,omitempty"`   // 新版核心二进制快照
	ConfigBackups []BackupItem      `json:"config_backups,omitempty"` // 新版配置备份清单
	ConfigFiles   []string          `json:"config_files,omitempty"`   // 旧版兼容字段，不再用于新快照命名
	Services      []string          `json:"services"`                 // 需要恢复的 systemd 服务
	ServiceStates []ServiceState    `json:"service_states,omitempty"` // 新版服务状态快照
}

// BackupItem records one file or directory captured in an install snapshot.
type BackupItem struct {
	Path       string `json:"path"`
	BackupPath string `json:"backup_path"`
	IsDir      bool   `json:"is_dir"`
	Existed    bool   `json:"existed"`
	Mode       uint32 `json:"mode,omitempty"`
}

// CoreBackup records whether a proxy core binary existed before the install.
type CoreBackup struct {
	Name       string `json:"name"`
	BinaryPath string `json:"binary_path"`
	BackupPath string `json:"backup_path,omitempty"`
	Existed    bool   `json:"existed"`
}

// ServiceState records the systemd unit state before the install.
type ServiceState struct {
	Name        string `json:"name"`
	ServicePath string `json:"service_path"`
	BackupPath  string `json:"backup_path,omitempty"`
	Existed     bool   `json:"existed"`
	Enabled     bool   `json:"enabled"`
	Active      bool   `json:"active"`
}

// Manager 回滚管理器
type Manager struct {
	snapshotDir string // 快照存储目录
	logger      *logrus.Logger
}

type coreSnapshotTarget struct {
	name       string
	binaryPath string
}

type serviceSnapshotTarget struct {
	name        string
	servicePath string
}

type configSnapshotTarget struct {
	name string
	path string
}

var (
	coreSnapshotTargets = []coreSnapshotTarget{
		{name: "xray", binaryPath: "/usr/local/xray-core/xray"},
		{name: "singbox", binaryPath: "/usr/local/sing-box/sing-box"},
	}
	serviceSnapshotTargets = []serviceSnapshotTarget{
		{name: "xray.service", servicePath: "/etc/systemd/system/xray.service"},
		{name: "sing-box.service", servicePath: "/etc/systemd/system/sing-box.service"},
	}
	configSnapshotTargets = []configSnapshotTarget{
		{name: "config.yaml", path: "/etc/vasmax/config.yaml"},
		{name: "xray_conf", path: "/etc/vasmax/xray/conf/"},
		{name: "singbox_conf", path: "/etc/vasmax/sing-box/conf/"},
		{name: "nginx_conf.d", path: "/etc/nginx/conf.d/"},
		{name: "tls", path: "/etc/vasmax/tls/"},
	}
	rollbackCommandRun = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	rollbackCommandOutput = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

// NewManager 创建回滚管理器
func NewManager(snapshotDir string, logger *logrus.Logger) *Manager {
	os.MkdirAll(snapshotDir, 0755)
	return &Manager{
		snapshotDir: snapshotDir,
		logger:      logger,
	}
}

// CreateSnapshot 记录安装前状态。extraPaths 用于调用方加入自定义 Nginx 配置目录等路径。
func (m *Manager) CreateSnapshot(extraPaths ...string) (*Snapshot, error) {
	snap := &Snapshot{
		Timestamp:     time.Now(),
		CoreVersions:  make(map[string]string),
		CoreBackups:   make([]CoreBackup, 0, len(coreSnapshotTargets)),
		ConfigBackups: make([]BackupItem, 0),
		ConfigFiles:   make([]string, 0),
		Services:      make([]string, 0),
		ServiceStates: make([]ServiceState, 0, len(serviceSnapshotTargets)),
	}

	for _, target := range coreSnapshotTargets {
		item, err := m.snapshotCore(target)
		if err != nil {
			return nil, err
		}
		snap.CoreBackups = append(snap.CoreBackups, item)
		if output, err := rollbackCommandOutput(target.binaryPath, "version"); err == nil {
			ver := strings.TrimSpace(string(output))
			if len(ver) > 50 {
				ver = ver[:50]
			}
			snap.CoreVersions[target.name] = ver
		}
	}

	for _, item := range m.snapshotItems(extraPaths...) {
		info, err := os.Stat(item.Path)
		if err != nil {
			if os.IsNotExist(err) {
				snap.ConfigBackups = append(snap.ConfigBackups, item)
				snap.ConfigFiles = append(snap.ConfigFiles, item.Path)
				continue
			}
			return nil, fmt.Errorf("检查 %s 失败: %w", item.Path, err)
		}
		item.IsDir = info.IsDir()
		item.Existed = true
		if err := os.RemoveAll(item.BackupPath); err != nil {
			return nil, fmt.Errorf("清理旧备份失败 %s: %w", item.BackupPath, err)
		}
		if err := copyPath(item.Path, item.BackupPath); err != nil {
			return nil, fmt.Errorf("备份 %s 失败: %w", item.Path, err)
		}
		snap.ConfigBackups = append(snap.ConfigBackups, item)
		snap.ConfigFiles = append(snap.ConfigFiles, item.Path)
	}

	// 记录需要恢复的服务
	for _, target := range serviceSnapshotTargets {
		state, err := m.snapshotService(target)
		if err != nil {
			return nil, err
		}
		snap.ServiceStates = append(snap.ServiceStates, state)
		if state.Active {
			snap.Services = append(snap.Services, state.Name)
		}
	}
	snapFile := filepath.Join(m.snapshotDir, "snapshot.json")
	if err := security.AtomicWriteJSON(snapFile, snap, 0644); err != nil {
		return nil, fmt.Errorf("保存快照元数据失败: %w", err)
	}

	m.logInfo("安装快照已创建")
	return snap, nil
}

// Rollback 恢复备份配置、核心二进制、重启服务
func (m *Manager) Rollback(snap *Snapshot) error {
	var errs []error

	// 阶段 A：恢复配置文件、核心二进制、服务文件和 enabled 状态。
	// 只有阶段 A 完整成功后，才允许阶段 B 恢复 active 服务状态。
	for _, item := range snap.ConfigBackups {
		if err := restoreBackupItem(item); err != nil {
			m.logErrorf(err, "恢复 %s 失败", item.Path)
			errs = append(errs, err)
		}
	}

	// 恢复核心二进制
	if len(snap.CoreBackups) > 0 {
		for _, item := range snap.CoreBackups {
			if err := restoreCoreBackup(item); err != nil {
				m.logErrorf(err, "恢复 %s 二进制失败", item.Name)
				errs = append(errs, err)
			}
		}
	} else {
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
					m.logErrorf(err, "恢复 %s 二进制失败", core)
					errs = append(errs, err)
				}
			}
		}
	}

	allowServiceStart := len(errs) == 0

	// 恢复服务文件和 systemd 状态
	if len(snap.ServiceStates) > 0 {
		for _, state := range snap.ServiceStates {
			if err := restoreServiceState(state, allowServiceStart); err != nil {
				m.logErrorf(err, "恢复 %s 服务状态失败", state.Name)
				errs = append(errs, err)
			}
		}
	} else {
		for _, svc := range snap.Services {
			if err := rollbackCommandRun("systemctl", "restart", svc); err != nil {
				m.logErrorf(err, "重启 %s 失败", svc)
				errs = append(errs, err)
			}
		}
	}

	if err := errors.Join(errs...); err != nil {
		m.logError("回滚部分失败，请检查日志并手动恢复")
		return fmt.Errorf("回滚部分失败: %w", err)
	}

	m.logInfo("回滚完成")
	return nil
}

// CleanSnapshot 安装成功后清理快照
func (m *Manager) CleanSnapshot(snap *Snapshot) error {
	for _, item := range snap.ConfigBackups {
		os.RemoveAll(item.BackupPath)
	}
	for _, item := range snap.CoreBackups {
		if item.BackupPath != "" {
			os.RemoveAll(item.BackupPath)
		}
	}
	for _, state := range snap.ServiceStates {
		if state.BackupPath != "" {
			os.RemoveAll(state.BackupPath)
		}
	}
	os.Remove(filepath.Join(m.snapshotDir, "snapshot.json"))
	m.logInfo("快照已清理")
	return nil
}

func (m *Manager) snapshotCore(target coreSnapshotTarget) (CoreBackup, error) {
	item := CoreBackup{
		Name:       target.name,
		BinaryPath: target.binaryPath,
		BackupPath: filepath.Join(m.snapshotDir, fmt.Sprintf("core_%s.bak", target.name)),
	}
	if _, err := os.Stat(target.binaryPath); err != nil {
		if os.IsNotExist(err) {
			item.BackupPath = ""
			return item, nil
		}
		return item, fmt.Errorf("检查核心 %s 失败: %w", target.name, err)
	}
	item.Existed = true
	if err := os.RemoveAll(item.BackupPath); err != nil {
		return item, fmt.Errorf("清理核心备份 %s 失败: %w", item.BackupPath, err)
	}
	if err := copyPath(target.binaryPath, item.BackupPath); err != nil {
		return item, fmt.Errorf("备份核心 %s 失败: %w", target.name, err)
	}
	return item, nil
}

func (m *Manager) snapshotService(target serviceSnapshotTarget) (ServiceState, error) {
	state := ServiceState{
		Name:        target.name,
		ServicePath: target.servicePath,
		BackupPath:  filepath.Join(m.snapshotDir, fmt.Sprintf("service_%s.bak", safeBackupName(target.name))),
		Enabled:     rollbackCommandRun("systemctl", "is-enabled", "--quiet", target.name) == nil,
		Active:      rollbackCommandRun("systemctl", "is-active", "--quiet", target.name) == nil,
	}
	if _, err := os.Stat(target.servicePath); err != nil {
		if os.IsNotExist(err) {
			state.BackupPath = ""
			return state, nil
		}
		return state, fmt.Errorf("检查服务文件 %s 失败: %w", target.servicePath, err)
	}
	state.Existed = true
	if err := os.RemoveAll(state.BackupPath); err != nil {
		return state, fmt.Errorf("清理服务备份 %s 失败: %w", state.BackupPath, err)
	}
	if err := copyPath(target.servicePath, state.BackupPath); err != nil {
		return state, fmt.Errorf("备份服务 %s 失败: %w", target.name, err)
	}
	return state, nil
}

func (m *Manager) snapshotItems(extraPaths ...string) []BackupItem {
	paths := make([]configSnapshotTarget, 0, len(configSnapshotTargets)+len(extraPaths))
	paths = append(paths, configSnapshotTargets...)
	for i, path := range extraPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		paths = append(paths, configSnapshotTarget{name: fmt.Sprintf("extra_%d_%s", i+1, safeBackupName(path)), path: path})
	}

	seen := make(map[string]bool)
	items := make([]BackupItem, 0, len(paths))
	for _, p := range paths {
		clean := filepath.Clean(p.path)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		var mode uint32
		if info, err := os.Stat(clean); err == nil {
			mode = uint32(info.Mode())
		}
		items = append(items, BackupItem{
			Path:       clean,
			BackupPath: filepath.Join(m.snapshotDir, p.name+".bak"),
			Mode:       mode,
		})
	}
	return items
}

func safeBackupName(path string) string {
	replacer := strings.NewReplacer(":", "_", "\\", "_", "/", "_", ".", "_", " ", "_")
	name := strings.Trim(replacer.Replace(filepath.Clean(path)), "_")
	if name == "" {
		return "path"
	}
	return name
}

// copyPath 复制文件或目录
func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst, info.Mode())
	}
	return copyFile(src, dst, info.Mode())
}

func copyDir(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(dst, mode); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := copyPath(srcPath, dstPath); err != nil {
			return err
		}
	}
	return os.Chmod(dst, mode)
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return security.AtomicWrite(dst, data, mode)
}

func restorePath(backupPath, targetPath string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); err == nil {
		failedPath := fmt.Sprintf("%s.failed.%d", targetPath, time.Now().UnixNano())
		if err := os.Rename(targetPath, failedPath); err != nil {
			return fmt.Errorf("move current target aside: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return copyPath(backupPath, targetPath)
}

func restoreBackupItem(item BackupItem) error {
	// Backward compatibility: older snapshots did not have Existed. If the
	// backup path is present, restore it even when Existed is false.
	if item.Existed || pathExists(item.BackupPath) {
		return restorePath(item.BackupPath, item.Path)
	}
	if err := os.RemoveAll(item.Path); err != nil {
		return fmt.Errorf("remove newly created target: %w", err)
	}
	return nil
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func restoreCoreBackup(item CoreBackup) error {
	if item.Existed {
		return restorePath(item.BackupPath, item.BinaryPath)
	}
	var errs []error
	if err := os.Remove(item.BinaryPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if err := os.Remove(item.BinaryPath + ".bak"); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func restoreServiceState(state ServiceState, allowStart bool) error {
	var errs []error
	_ = rollbackCommandRun("systemctl", "stop", state.Name)
	if state.Existed {
		if err := restorePath(state.BackupPath, state.ServicePath); err != nil {
			errs = append(errs, err)
		}
	} else if err := os.Remove(state.ServicePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}

	if err := rollbackCommandRun("systemctl", "daemon-reload"); err != nil {
		errs = append(errs, fmt.Errorf("daemon-reload: %w", err))
	}

	if state.Enabled {
		if err := rollbackCommandRun("systemctl", "enable", state.Name); err != nil {
			errs = append(errs, fmt.Errorf("enable %s: %w", state.Name, err))
		}
	} else {
		_ = rollbackCommandRun("systemctl", "disable", state.Name)
	}

	canStart := allowStart && len(errs) == 0
	if state.Active && canStart {
		if err := rollbackCommandRun("systemctl", "restart", state.Name); err != nil {
			errs = append(errs, fmt.Errorf("restart %s: %w", state.Name, err))
		}
	} else if state.Active {
		errs = append(errs, fmt.Errorf("%s was active before rollback but was left stopped because file restore had critical errors", state.Name))
	} else {
		_ = rollbackCommandRun("systemctl", "stop", state.Name)
	}
	return errors.Join(errs...)
}

func (m *Manager) logInfo(msg string) {
	if m.logger != nil {
		m.logger.Info(msg)
	}
}

func (m *Manager) logError(msg string) {
	if m.logger != nil {
		m.logger.Error(msg)
	}
}

func (m *Manager) logErrorf(err error, format string, args ...interface{}) {
	if m.logger != nil {
		m.logger.WithError(err).Errorf(format, args...)
	}
}
