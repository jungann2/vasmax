package bbr

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const (
	SysctlOptConf  = "/etc/sysctl.d/99-vasmax-optimize.conf"
	SysctlIPv6Conf = "/etc/sysctl.d/99-vasmax-ipv6.conf"
)

// oldSchemeParams 旧方案 sysctl 参数
var oldSchemeParams = map[string]string{
	"net.ipv4.tcp_fastopen":              "3",
	"net.ipv4.tcp_slow_start_after_idle": "0",
	"net.ipv4.tcp_retries2":              "8",
	"net.ipv4.tcp_fin_timeout":           "30",
	"net.ipv4.tcp_keepalive_time":        "1200",
	"net.ipv4.tcp_keepalive_intvl":       "15",
	"net.ipv4.tcp_keepalive_probes":      "5",
	"net.ipv4.tcp_mtu_probing":           "1",
	"net.core.rmem_max":                  "67108864",
	"net.core.wmem_max":                  "67108864",
	"net.ipv4.tcp_rmem":                  "4096 87380 67108864",
	"net.ipv4.tcp_wmem":                  "4096 65536 67108864",
}

// newSchemeExtra 新方案额外参数（在旧方案基础上追加）
var newSchemeExtra = map[string]string{
	"net.core.netdev_max_backlog":  "250000",
	"net.core.somaxconn":           "4096",
	"net.ipv4.tcp_max_syn_backlog": "8192",
	"net.ipv4.tcp_max_tw_buckets":  "5000",
	"net.ipv4.tcp_tw_reuse":        "1",
	"net.ipv4.ip_local_port_range": "10000 65000",
	"net.ipv4.tcp_syncookies":      "1",
	"net.ipv4.tcp_timestamps":      "1",
	"net.ipv4.tcp_sack":            "1",
	"net.ipv4.tcp_window_scaling":  "1",
	"fs.file-max":                  "1048576",
	"net.core.optmem_max":          "25165824",
}

// ApplyOptimize 应用系统配置优化
func ApplyOptimize(newScheme bool) error {
	params := make(map[string]string)
	for k, v := range oldSchemeParams {
		params[k] = v
	}
	if newScheme {
		for k, v := range newSchemeExtra {
			params[k] = v
		}
	}
	if ecn, ok := readSysctlParam(SysctlOptConf, "net.ipv4.tcp_ecn"); ok {
		params["net.ipv4.tcp_ecn"] = ecn
	}

	// 收集 key 并排序，确保配置文件内容稳定可预测
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("# vasmax 系统配置优化\n")
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, params[k]))
	}

	if err := os.WriteFile(SysctlOptConf, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("写入优化配置失败: %w", err)
	}

	if out, err := exec.Command("sysctl", "-p", SysctlOptConf).CombinedOutput(); err != nil {
		return fmt.Errorf("应用优化配置失败: %s", string(out))
	}

	return nil
}

// SetECN 开启或关闭 ECN（显式拥塞通知）
func SetECN(enable bool) error {
	val := "0"
	if enable {
		val = "1"
	}
	if err := exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv4.tcp_ecn=%s", val)).Run(); err != nil {
		return fmt.Errorf("设置 ECN 失败: %w", err)
	}
	// ECN 持久化到 optimize 配置文件
	// 注意：DisableAll 会删除此文件，ECN 设置也会被一并清除
	return appendSysctlParam(SysctlOptConf, "net.ipv4.tcp_ecn", val)
}

// SetIPv6 开启或禁用 IPv6
func SetIPv6(enable bool) error {
	if enable {
		// 删除禁用配置
		_ = os.Remove(SysctlIPv6Conf)
		// 立即开启
		var errs []string
		if err := exec.Command("sysctl", "-w", "net.ipv6.conf.all.disable_ipv6=0").Run(); err != nil {
			errs = append(errs, fmt.Sprintf("conf.all: %v", err))
		}
		if err := exec.Command("sysctl", "-w", "net.ipv6.conf.default.disable_ipv6=0").Run(); err != nil {
			errs = append(errs, fmt.Sprintf("conf.default: %v", err))
		}
		if err := exec.Command("sysctl", "-w", "net.ipv6.conf.lo.disable_ipv6=0").Run(); err != nil {
			errs = append(errs, fmt.Sprintf("conf.lo: %v", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("开启 IPv6 部分失败: %s", strings.Join(errs, "; "))
		}
	} else {
		content := "net.ipv6.conf.all.disable_ipv6=1\nnet.ipv6.conf.default.disable_ipv6=1\nnet.ipv6.conf.lo.disable_ipv6=1\n"
		if err := os.WriteFile(SysctlIPv6Conf, []byte(content), 0644); err != nil {
			return fmt.Errorf("写入 IPv6 禁用配置失败: %w", err)
		}
		if out, err := exec.Command("sysctl", "-p", SysctlIPv6Conf).CombinedOutput(); err != nil {
			return fmt.Errorf("应用 IPv6 配置失败: %s", string(out))
		}
	}
	return nil
}

// MergeSysctl 手动提交合并所有内核参数
func MergeSysctl() error {
	if out, err := exec.Command("sysctl", "--system").CombinedOutput(); err != nil {
		return fmt.Errorf("sysctl --system 失败: %s", string(out))
	}
	return nil
}

// appendSysctlParam 在配置文件中追加或更新一个参数
func appendSysctlParam(file, key, val string) error {
	data, _ := os.ReadFile(file)
	content := strings.TrimSpace(string(data))

	if content == "" {
		// 文件不存在或为空，直接写入
		return os.WriteFile(file, []byte(fmt.Sprintf("%s=%s\n", key, val)), 0644)
	}

	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") || strings.HasPrefix(line, key+" =") {
			lines[i] = fmt.Sprintf("%s=%s", key, val)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, val))
	}

	return os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func readSysctlParam(file, key string) (string, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) != key {
			continue
		}
		val := strings.TrimSpace(parts[1])
		if val == "0" || val == "1" || val == "2" {
			return val, true
		}
	}
	return "", false
}
