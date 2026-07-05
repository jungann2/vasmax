package bbr

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const (
	SysctlOptConf = "/etc/sysctl.d/99-vasmax-optimize.conf"
)

// baseOptimizeParams contains conservative TCP tuning shared by the current optimize profile.
var baseOptimizeParams = map[string]string{
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

// extendedOptimizeParams contains additional server-side queue and connection limits.
var extendedOptimizeParams = map[string]string{
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

// ApplyOptimize applies the current conservative sysctl tuning set.
func ApplyOptimize() error {
	params := make(map[string]string)
	for k, v := range baseOptimizeParams {
		params[k] = v
	}
	for k, v := range extendedOptimizeParams {
		params[k] = v
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
