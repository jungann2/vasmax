//go:build linux

package sysinfo

import (
	"fmt"
	"syscall"

	"vasmax/internal/api"
)

// CheckDiskSpace 检查磁盘可用空间
func CheckDiskSpace(path string, requiredMB int) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return fmt.Errorf("获取磁盘信息失败: %w", err)
	}
	availMB := int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024)
	if availMB < int64(requiredMB) {
		return fmt.Errorf("磁盘空间不足: 需要 %dMB, 可用 %dMB", requiredMB, availMB)
	}
	return nil
}

// readDiskUsage 读取磁盘使用情况
func readDiskUsage(path string) (api.ResourceUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return api.ResourceUsage{}, err
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bfree) * int64(stat.Bsize)
	return api.ResourceUsage{
		Total: total,
		Used:  total - free,
	}, nil
}
