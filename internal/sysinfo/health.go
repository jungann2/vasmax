package sysinfo

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"vasmax/internal/config"
)

// HealthResult 健康检查结果
type HealthResult struct {
	Components []ComponentHealth `json:"components"`
	Overall    string            `json:"overall"` // healthy/unhealthy/warning
}

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // healthy/unhealthy/warning
	Details string `json:"details"`
}

// RunHealthCheck 执行全面健康检查
// 返回 0=healthy, 1=unhealthy
func RunHealthCheck(configPath string) int {
	result := &HealthResult{Overall: "healthy"}

	cfg, _ := config.LoadConfig(configPath)

	// 检查 Xray-core 进程
	result.addCheck(checkProcess("xray", "Xray-core"))
	// 检查 sing-box 进程
	result.addCheck(checkProcess("sing-box", "sing-box"))
	// 检查 Nginx 进程
	result.addCheck(checkProcess("nginx", "Nginx"))
	// 检查磁盘空间
	result.addCheck(checkDisk("/"))
	// 检查 TLS 证书
	if cfg != nil && cfg.TLS.CertFile != "" {
		result.addCheck(checkCertExpiry(cfg.TLS.CertFile))
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	if result.Overall == "unhealthy" {
		return 1
	}
	return 0
}

func (r *HealthResult) addCheck(c ComponentHealth) {
	r.Components = append(r.Components, c)
	if c.Status == "unhealthy" {
		r.Overall = "unhealthy"
	} else if c.Status == "warning" && r.Overall == "healthy" {
		r.Overall = "warning"
	}
}

// checkProcess 检查进程是否运行
func checkProcess(name, displayName string) ComponentHealth {
	cmd := exec.Command("pgrep", "-x", name)
	if err := cmd.Run(); err != nil {
		return ComponentHealth{
			Name:    displayName,
			Status:  "unhealthy",
			Details: "进程未运行",
		}
	}
	return ComponentHealth{
		Name:    displayName,
		Status:  "healthy",
		Details: "进程运行中",
	}
}

// checkDisk 检查磁盘空间
func checkDisk(path string) ComponentHealth {
	err := CheckDiskSpace(path, 100) // 至少 100MB
	if err != nil {
		return ComponentHealth{
			Name:    "磁盘空间",
			Status:  "warning",
			Details: err.Error(),
		}
	}
	return ComponentHealth{
		Name:    "磁盘空间",
		Status:  "healthy",
		Details: "磁盘空间充足",
	}
}

// checkCertExpiry 检查 TLS 证书有效期
func checkCertExpiry(certPath string) ComponentHealth {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: fmt.Sprintf("无法读取证书: %v", err),
		}
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: "证书格式无效",
		}
	}

	// 手动解析第一个证书获取过期时间
	block, _ := pem.Decode(data)
	if block == nil {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: "无法解码 PEM 块",
		}
	}

	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "warning",
			Details: fmt.Sprintf("无法解析证书: %v", err),
		}
	}

	daysLeft := int(time.Until(parsed.NotAfter).Hours() / 24)
	if daysLeft <= 0 {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "unhealthy",
			Details: "证书已过期",
		}
	}
	if daysLeft <= 7 {
		return ComponentHealth{
			Name:    "TLS 证书",
			Status:  "warning",
			Details: fmt.Sprintf("证书将在 %d 天后过期", daysLeft),
		}
	}
	return ComponentHealth{
		Name:    "TLS 证书",
		Status:  "healthy",
		Details: fmt.Sprintf("证书有效，剩余 %d 天", daysLeft),
	}
}

// FormatHealthResult 格式化健康检查结果为彩色文本
func FormatHealthResult(result *HealthResult) string {
	var sb strings.Builder
	sb.WriteString("=== 健康检查结果 ===\n")
	for _, c := range result.Components {
		var icon string
		switch c.Status {
		case "healthy":
			icon = "✓"
		case "warning":
			icon = "⚠"
		case "unhealthy":
			icon = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, c.Name, c.Details))
	}
	sb.WriteString(fmt.Sprintf("\n总体状态: %s\n", result.Overall))
	return sb.String()
}
