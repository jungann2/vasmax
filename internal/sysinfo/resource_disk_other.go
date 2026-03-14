//go:build !linux

package sysinfo

import (
	"fmt"

	"vasmax/internal/api"
)

// CheckDiskSpace 检查磁盘可用空间（非 Linux 平台 stub）
func CheckDiskSpace(_ string, _ int) error {
	return fmt.Errorf("CheckDiskSpace not supported on this platform")
}

// readDiskUsage 读取磁盘使用情况（非 Linux 平台 stub）
func readDiskUsage(_ string) (api.ResourceUsage, error) {
	return api.ResourceUsage{}, fmt.Errorf("readDiskUsage not supported on this platform")
}
