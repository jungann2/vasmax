package core

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// XrayCore Xray-core 进程管理
type XrayCore struct {
	BinaryPath  string // /usr/local/xray-core/xray
	ConfDir     string // /etc/vasmax/xray/conf/
	ServiceName string // xray.service
}

// NewXrayCore 创建 XrayCore 实例
func NewXrayCore() *XrayCore {
	return &XrayCore{
		BinaryPath:  "/usr/local/xray-core/xray",
		ConfDir:     "/etc/vasmax/xray/conf/",
		ServiceName: "xray.service",
	}
}

// GetVersion 获取当前安装版本
func (x *XrayCore) GetVersion() (string, error) {
	output, err := exec.Command(x.BinaryPath, "version").Output()
	if err != nil {
		return "", fmt.Errorf("获取 Xray 版本失败: %w", err)
	}
	// 输出格式: "Xray 1.8.x (Xray, Penetrates Everything.) ..."
	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		return parts[1], nil
	}
	return strings.TrimSpace(string(output)), nil
}

// DownloadURL 获取下载地址
func (x *XrayCore) DownloadURL() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "64"
	} else if arch == "arm64" {
		arch = "arm64-v8a"
	}
	return fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-%s.zip", arch)
}
