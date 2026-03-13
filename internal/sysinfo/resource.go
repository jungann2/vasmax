package sysinfo

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"vasmax/internal/api"
)

// StaticInfo 缓存启动时采集一次的静态系统信息
type StaticInfo struct {
	Hostname string
	CPUModel string
	IPv4     string
	IPv6     string
}

var (
	cachedStaticInfo StaticInfo
	staticInfoOnce   sync.Once
)

// InitStaticInfo 采集静态系统信息（hostname、cpu_model、ipv4、ipv6），仅执行一次
func InitStaticInfo() {
	staticInfoOnce.Do(func() {
		cachedStaticInfo.Hostname, _ = os.Hostname()
		cachedStaticInfo.CPUModel = readCPUModel()
		cachedStaticInfo.IPv4 = detectPublicIP([]string{
			"https://api.ipify.org",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
		})
		cachedStaticInfo.IPv6 = detectPublicIPv6()
		log.Infof("静态信息采集完成: hostname=%s, cpu_model=%s, ipv4=%s, ipv6=%s",
			cachedStaticInfo.Hostname, cachedStaticInfo.CPUModel,
			cachedStaticInfo.IPv4, cachedStaticInfo.IPv6)
	})
}

// CollectStatus 采集节点负载状态（用于 xboard 上报）
// 使用 recover() 保护，防止 panic 导致 goroutine 崩溃
func CollectStatus() (status *api.NodeStatus, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("CollectStatus panic recovered: %v", r)
			status = nil
			err = fmt.Errorf("CollectStatus panic: %v", r)
		}
	}()

	status = &api.NodeStatus{}

	// CPU 使用率
	cpu, cpuErr := readCPUUsage()
	if cpuErr == nil {
		status.CPU = cpu
	}

	// 内存
	mem, memErr := readMemInfo()
	if memErr == nil {
		status.Mem = mem.Mem
		status.Swap = mem.Swap
	}

	// 磁盘
	disk, diskErr := readDiskUsage("/")
	if diskErr == nil {
		status.Disk = disk
	}

	// 网络流量
	status.Network = readNetworkStats()

	// 磁盘 IO
	status.DiskIO = readDiskIOStats()

	// 系统运行时间
	status.Uptime = readUptime()

	// Goroutine 数量
	status.Goroutines = runtime.NumGoroutine()

	// 静态信息（从缓存读取）
	status.Hostname = cachedStaticInfo.Hostname
	status.CPUModel = cachedStaticInfo.CPUModel
	status.IPv4 = cachedStaticInfo.IPv4
	status.IPv6 = cachedStaticInfo.IPv6

	return status, nil
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

// readNetworkStats 从 /proc/net/dev 读取网络流量，汇总物理网卡（排除 lo/docker/veth/br-）
func readNetworkStats() api.NetworkUsage {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		log.Warn("读取 /proc/net/dev 失败: ", err)
		return api.NetworkUsage{}
	}
	defer f.Close()
	return parseNetworkStats(f)
}

// isVirtualInterface 判断网络接口是否为虚拟接口（应排除）
func isVirtualInterface(name string) bool {
	return name == "lo" ||
		strings.HasPrefix(name, "docker") ||
		strings.HasPrefix(name, "veth") ||
		strings.HasPrefix(name, "br-")
}

// parseNetworkStats 从 reader 解析 /proc/net/dev 格式内容，汇总物理网卡流量
func parseNetworkStats(r io.Reader) api.NetworkUsage {
	var result api.NetworkUsage
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		// 跳过头部行（不含 ":"）
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:idx])

		// 排除虚拟接口
		if isVirtualInterface(iface) {
			continue
		}

		fields := strings.Fields(line[idx+1:])
		if len(fields) < 9 {
			continue
		}

		recv, _ := strconv.ParseInt(fields[0], 10, 64)
		sent, _ := strconv.ParseInt(fields[8], 10, 64)
		result.Recv += recv
		result.Sent += sent
	}
	return result
}

// physicalDiskRe 匹配物理磁盘设备名（sda/vda/nvme0n1 等，排除分区如 sda1）
var physicalDiskRe = regexp.MustCompile(`^(sd|vd)[a-z]$|^nvme\d+n\d+$`)

// readDiskIOStats 从 /proc/diskstats 读取磁盘 IO，仅统计物理磁盘
func readDiskIOStats() api.DiskIOUsage {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		log.Warn("读取 /proc/diskstats 失败: ", err)
		return api.DiskIOUsage{}
	}
	defer f.Close()

	var result api.DiskIOUsage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		device := fields[2]
		if !physicalDiskRe.MatchString(device) {
			continue
		}

		// field 6 = sectors read, field 10 = sectors written (512 bytes per sector)
		readSectors, _ := strconv.ParseInt(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseInt(fields[9], 10, 64)
		result.Read += readSectors * 512
		result.Write += writeSectors * 512
	}
	return result
}

// readUptime 从 /proc/uptime 读取系统运行时间（秒）
func readUptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		log.Warn("读取 /proc/uptime 失败: ", err)
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		log.Warn("解析 /proc/uptime 失败: 内容为空")
		return 0
	}
	val, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		log.Warn("解析 /proc/uptime 失败: ", err)
		return 0
	}
	return int64(val)
}

// readCPUModel 从 /proc/cpuinfo 读取 CPU 型号
func readCPUModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		log.Warn("读取 /proc/cpuinfo 失败: ", err)
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			idx := strings.Index(line, ":")
			if idx >= 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	log.Warn("未找到 CPU model name 信息")
	return ""
}
