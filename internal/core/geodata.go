package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"vasmax/pkg/downloader"
)

const (
	geoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	geoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
)

// UpdateGeoData 从 GitHub Releases 下载 GeoIP/GeoSite 数据
func (m *Manager) UpdateGeoData(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tasks := []downloader.DownloadTask{}

	// Xray-core GeoData
	if fileExists(m.xray.BinaryPath) {
		xrayDataDir := "/usr/local/xray-core/"
		tasks = append(tasks,
			downloader.DownloadTask{URL: geoIPURL, DestPath: filepath.Join(xrayDataDir, "geoip.dat"), Name: "geoip.dat (xray)"},
			downloader.DownloadTask{URL: geoSiteURL, DestPath: filepath.Join(xrayDataDir, "geosite.dat"), Name: "geosite.dat (xray)"},
		)
	}

	// sing-box GeoData
	if fileExists(m.singbox.BinaryPath) {
		singboxDataDir := "/usr/local/sing-box/"
		tasks = append(tasks,
			downloader.DownloadTask{URL: geoIPURL, DestPath: filepath.Join(singboxDataDir, "geoip.dat"), Name: "geoip.dat (singbox)"},
			downloader.DownloadTask{URL: geoSiteURL, DestPath: filepath.Join(singboxDataDir, "geosite.dat"), Name: "geosite.dat (singbox)"},
		)
	}

	if len(tasks) == 0 {
		return fmt.Errorf("未检测到已安装的代理核心")
	}

	if err := downloader.DownloadAll(ctx, tasks, func(name string, pct int) {
		m.logger.WithField("file", name).Info("GeoData 下载完成")
	}); err != nil {
		return fmt.Errorf("更新 GeoData 失败: %w", err)
	}

	// 重载核心
	if fileExists(m.xray.BinaryPath) {
		if err := m.ReloadXray(); err != nil {
			m.logger.WithError(err).Warn("重载 Xray 失败")
		}
	}
	if fileExists(m.singbox.BinaryPath) {
		if err := m.RestartSingBox(); err != nil {
			m.logger.WithError(err).Warn("重启 sing-box 失败")
		}
	}

	return nil
}

// InstallGeoDataCron 安装 GeoData 自动更新定时任务
func InstallGeoDataCron() error {
	cronLine := "0 4 * * * /usr/local/bin/vasmax --update-geodata\n"
	return os.WriteFile("/etc/cron.d/VasmaX-geodata", []byte(cronLine), 0644)
}
