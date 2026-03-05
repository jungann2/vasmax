package sysinfo

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// SystemInfo 系统信息
type SystemInfo struct {
	OS        string // centos/debian/ubuntu/alpine
	OSVersion string
	Arch      string // amd64/arm64
	IPv4      string
	IPv6      string
	SELinux   bool
}

// DetectSystem 检测系统信息
func DetectSystem() (*SystemInfo, error) {
	info := &SystemInfo{
		Arch: runtime.GOARCH,
	}

	// 检测 OS 类型和版本
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ID=") {
				info.OS = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
			if strings.HasPrefix(line, "VERSION_ID=") {
				info.OSVersion = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}

	// 检测 SELinux
	if output, err := exec.Command("getenforce").Output(); err == nil {
		status := strings.TrimSpace(string(output))
		info.SELinux = status == "Enforcing" || status == "Permissive"
	}

	// 检测公网 IP
	info.IPv4 = detectPublicIP([]string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	})
	info.IPv6 = detectPublicIPv6()

	return info, nil
}

// CheckPortAvailable 检查端口是否可用
func CheckPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// detectPublicIP 通过多个服务检测公网 IPv4
func detectPublicIP(urls []string) string {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			continue
		}
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		resp.Body.Close()
		ip := strings.TrimSpace(string(buf[:n]))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
}

// detectPublicIPv6 检测公网 IPv6
func detectPublicIPv6() string {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://api6.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	ip := strings.TrimSpace(string(buf[:n]))
	if net.ParseIP(ip) != nil {
		return ip
	}
	return ""
}
