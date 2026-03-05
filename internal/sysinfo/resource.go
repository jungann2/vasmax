package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"vasmax/internal/api"
)

// CollectStatus 采集节点负载状态（用于 xboard 上报）
func CollectStatus() (*api.NodeStatus, error) {
	status := &api.NodeStatus{}

	// CPU 使用率
	cpu, err := readCPUUsage()
	if err == nil {
		status.CPU = cpu
	}

	// 内存
	mem, err := readMemInfo()
	if err == nil {
		status.Mem = mem.Mem
		status.Swap = mem.Swap
	}

	// 磁盘
	disk, err := readDiskUsage("/")
	if err == nil {
		status.Disk = disk
	}

	return status, nil
}

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

type memResult struct {
	Mem  api.ResourceUsage
	Swap api.ResourceUsage
}

// readMemInfo 从 /proc/meminfo 读取内存信息
func readMemInfo() (*memResult, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := &memResult{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(parts[1], 10, 64)
		val *= 1024 // kB -> bytes

		switch parts[0] {
		case "MemTotal:":
			result.Mem.Total = val
		case "MemAvailable:":
			result.Mem.Used = result.Mem.Total - val
		case "SwapTotal:":
			result.Swap.Total = val
		case "SwapFree:":
			result.Swap.Used = result.Swap.Total - val
		}
	}
	return result, nil
}

// readCPUUsage 从 /proc/stat 读取 CPU 使用率（两次采样取差值）
func readCPUUsage() (float64, error) {
	read := func() (total, idle int64, err error) {
		f, err := os.Open("/proc/stat")
		if err != nil {
			return 0, 0, err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		if !scanner.Scan() {
			return 0, 0, fmt.Errorf("读取 /proc/stat 失败")
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 || fields[0] != "cpu" {
			return 0, 0, fmt.Errorf("解析 /proc/stat 失败")
		}
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseInt(fields[i], 10, 64)
			total += v
			if i == 4 {
				idle = v
			}
		}
		return total, idle, nil
	}

	total1, idle1, err := read()
	if err != nil {
		return 0, err
	}

	time.Sleep(500 * time.Millisecond)

	total2, idle2, err := read()
	if err != nil {
		return 0, err
	}

	totalDelta := float64(total2 - total1)
	idleDelta := float64(idle2 - idle1)
	if totalDelta <= 0 {
		return 0, nil
	}
	return (totalDelta - idleDelta) / totalDelta * 100, nil
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
