package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"vasmax/internal/security"
)

// SingBox sing-box 进程管理
type SingBox struct {
	BinaryPath  string // /usr/local/sing-box/sing-box
	ConfDir     string // /etc/vasmax/sing-box/conf/config/
	ConfigFile  string // /etc/vasmax/sing-box/conf/config.json
	ServiceName string // sing-box.service
}

// NewSingBox 创建 SingBox 实例
func NewSingBox() *SingBox {
	return &SingBox{
		BinaryPath:  "/usr/local/sing-box/sing-box",
		ConfDir:     "/etc/vasmax/sing-box/conf/config/",
		ConfigFile:  "/etc/vasmax/sing-box/conf/config.json",
		ServiceName: "sing-box.service",
	}
}

// GetVersion 获取当前安装版本
func (s *SingBox) GetVersion() (string, error) {
	output, err := exec.Command(s.BinaryPath, "version").Output()
	if err != nil {
		return "", fmt.Errorf("获取 sing-box 版本失败: %w", err)
	}
	// 输出格式: "sing-box version 1.x.x"
	parts := strings.Fields(string(output))
	for i, p := range parts {
		if p == "version" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return strings.TrimSpace(string(output)), nil
}

// MergeConfig 合并 confDir 下的配置文件到 configFile
// 正确处理 inbounds、outbounds、route.rules、route.rule_set 等数组类型的合并
func (s *SingBox) MergeConfig() error {
	entries, err := os.ReadDir(s.ConfDir)
	if err != nil {
		return fmt.Errorf("读取配置目录失败: %w", err)
	}

	merged := make(map[string]json.RawMessage)
	// arrayKeys 是需要数组合并而非覆盖的顶层 key
	arrayKeys := map[string]bool{
		"inbounds":  true,
		"outbounds": true,
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.ConfDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", entry.Name(), err)
		}
		var partial map[string]json.RawMessage
		if err := json.Unmarshal(data, &partial); err != nil {
			return fmt.Errorf("解析 %s 失败: %w", entry.Name(), err)
		}
		for k, v := range partial {
			if arrayKeys[k] {
				// 数组类型：合并而非覆盖
				if existing, ok := merged[k]; ok {
					mergedArr, mergeErr := mergeJSONArrays(existing, v)
					if mergeErr != nil {
						return fmt.Errorf("合并 %s.%s 失败: %w", entry.Name(), k, mergeErr)
					}
					merged[k] = mergedArr
				} else {
					merged[k] = v
				}
			} else if k == "route" {
				// route 需要特殊处理：合并 rules 和 rule_set 数组
				if existing, ok := merged[k]; ok {
					mergedRoute, mergeErr := mergeRouteObjects(existing, v)
					if mergeErr != nil {
						return fmt.Errorf("合并 %s.route 失败: %w", entry.Name(), mergeErr)
					}
					merged[k] = mergedRoute
				} else {
					merged[k] = v
				}
			} else {
				merged[k] = v
			}
		}
	}

	output, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化合并配置失败: %w", err)
	}

	return security.AtomicWrite(s.ConfigFile, output, 0644)
}

// mergeJSONArrays 合并两个 JSON 数组
func mergeJSONArrays(a, b json.RawMessage) (json.RawMessage, error) {
	var arrA, arrB []json.RawMessage
	if err := json.Unmarshal(a, &arrA); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &arrB); err != nil {
		return nil, err
	}
	arrA = append(arrA, arrB...)
	return json.Marshal(arrA)
}

// mergeRouteObjects 合并两个 route 对象，其中 rules 和 rule_set 数组合并
func mergeRouteObjects(a, b json.RawMessage) (json.RawMessage, error) {
	var objA, objB map[string]json.RawMessage
	if err := json.Unmarshal(a, &objA); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &objB); err != nil {
		return nil, err
	}

	routeArrayKeys := map[string]bool{"rules": true, "rule_set": true}
	for k, v := range objB {
		if routeArrayKeys[k] {
			if existing, ok := objA[k]; ok {
				merged, err := mergeJSONArrays(existing, v)
				if err != nil {
					return nil, err
				}
				objA[k] = merged
			} else {
				objA[k] = v
			}
		} else {
			objA[k] = v
		}
	}
	return json.Marshal(objA)
}

// DownloadURL 获取下载地址
func (s *SingBox) DownloadURL() string {
	arch := runtime.GOARCH
	return fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/latest/download/sing-box-linux-%s.tar.gz", arch)
}
