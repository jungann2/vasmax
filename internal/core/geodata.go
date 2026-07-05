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

var (
	geoDataCronPath = "/etc/cron.d/VasmaX-geodata"
	geoDataLogDir   = "/var/log/vasmax"
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
		if m.logger != nil {
			m.logger.WithField("file", name).Info("GeoData 下载完成")
		}
	}); err != nil {
		return fmt.Errorf("更新 GeoData 失败: %w", err)
	}

	// 重载核心
	needed := m.neededCores()
	if needed["xray"] {
		if err := m.ReloadXray(); err != nil {
			m.logWarn(err, "重载 Xray 失败")
		}
	}
	if needed["singbox"] {
		if err := m.RestartSingBox(); err != nil {
			m.logWarn(err, "重启 sing-box 失败")
		}
	}

	return nil
}

// InstallGeoDataCron 安装 GeoData 自动更新定时任务
func InstallGeoDataCron() error {
	if err := os.MkdirAll(geoDataLogDir, 0755); err != nil {
		return fmt.Errorf("create vasmax log dir: %w", err)
	}
	cronLine := `SHELL=/bin/sh
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
0 4 * * * root /usr/local/bin/VasmaX -c /etc/vasmax/config.yaml --update-geodata >> /var/log/vasmax/geodata-update.log 2>&1
`
	return os.WriteFile(geoDataCronPath, []byte(cronLine), 0644)
}
