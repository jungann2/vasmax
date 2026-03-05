package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"

	"vasmax/internal/security"
)

const (
	// StatsAPIAddr Xray Stats API 默认监听地址
	StatsAPIAddr = "127.0.0.1"
	// StatsAPIPort Xray Stats API 默认监听端口
	StatsAPIPort = 10085
)

// GenerateStatsAPIConfig 生成 Xray Stats API 配置文件（1_api.json）
// 托管模式下自动生成，包含 StatsService 和 dokodemo-door 入站
func GenerateStatsAPIConfig(confDir string) error {
	apiConfig := map[string]interface{}{
		"api": map[string]interface{}{
			"tag": "api",
			"services": []string{
				"StatsService",
			},
		},
		"inbounds": []map[string]interface{}{
			{
				"tag":      "api",
				"listen":   StatsAPIAddr,
				"port":     StatsAPIPort,
				"protocol": "dokodemo-door",
				"settings": map[string]interface{}{
					"address": StatsAPIAddr,
				},
			},
		},
		"routing": map[string]interface{}{
			"rules": []map[string]interface{}{
				{
					"inboundTag":  []string{"api"},
					"outboundTag": "api",
					"type":        "field",
				},
			},
		},
	}
	data, err := json.MarshalIndent(apiConfig, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(filepath.Join(confDir, "01_api.json"), data, 0644)
}

// GenerateStatsModuleConfig 生成 Xray stats 模块配置文件（6_stats.json）
func GenerateStatsModuleConfig(confDir string) error {
	statsConfig := map[string]interface{}{
		"stats": map[string]interface{}{},
		"policy": map[string]interface{}{
			"system": map[string]interface{}{
				"statsInboundUplink":   true,
				"statsInboundDownlink": true,
			},
		},
	}
	data, err := json.MarshalIndent(statsConfig, "", "  ")
	if err != nil {
		return err
	}
	return security.AtomicWrite(filepath.Join(confDir, "06_stats.json"), data, 0644)
}

// RemoveStatsAPIConfig 移除 Stats API 配置（切换回独立模式时调用）
func RemoveStatsAPIConfig(confDir string) error {
	files := []string{"01_api.json", "06_stats.json"}
	for _, f := range files {
		path := filepath.Join(confDir, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// EnsureStatsConfig 确保托管模式下 Stats API 配置存在
func EnsureStatsConfig(confDir string, managed bool) error {
	if managed {
		if err := GenerateStatsAPIConfig(confDir); err != nil {
			return err
		}
		return GenerateStatsModuleConfig(confDir)
	}
	return RemoveStatsAPIConfig(confDir)
}
