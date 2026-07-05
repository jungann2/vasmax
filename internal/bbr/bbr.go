package bbr

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	SysctlBBRConf = "/etc/sysctl.d/99-vasmax-bbr.conf"
)

// CCMode 描述拥塞控制算法 + 队列调度组合
type CCMode struct {
	Qdisc string // fq / fq_pie / cake / ""（空表示不设置）
	CC    string // bbr / bbr2 / bbrplus / cubic
	Label string // 显示名称
}

// AutoEnableResult describes the outcome of automatic BBR+FQ enablement.
type AutoEnableResult struct {
	Applied       bool
	AlreadyActive bool
	Supported     bool
	Message       string
}

// RuntimeStatus describes both sysctl defaults and the current default device qdisc.
type RuntimeStatus struct {
	CC                 string
	DefaultQdisc       string
	DefaultInterface   string
	DeviceQdisc        string
	DeviceQdiscError   string
	RecommendedSysctl  bool
	RecommendedRuntime bool
}

// 预定义的加速模式
var CCModes = []CCMode{
	{Qdisc: "fq", CC: "bbr", Label: "BBR + FQ（推荐*）"},
}

// RecommendedMode returns the default conservative acceleration mode.
func RecommendedMode() CCMode {
	return CCModes[0]
}

// IsBBRCC reports whether cc is one of the BBR-family algorithms.
func IsBBRCC(cc string) bool {
	return strings.TrimSpace(cc) == "bbr"
}

// IsModeActive reports whether both congestion control and qdisc match.
func IsModeActive(mode CCMode) bool {
	cc := CurrentCC()
	qdisc := CurrentQdisc()
	if strings.TrimSpace(cc) != mode.CC {
		return false
	}
	if mode.Qdisc == "" {
		return true
	}
	return strings.TrimSpace(qdisc) == mode.Qdisc
}

// CurrentCC 返回当前拥塞控制算法
func CurrentCC() string {
	out, err := exec.Command("sysctl", "-n", "net.ipv4.tcp_congestion_control").Output()
	if err != nil {
		return "未知"
	}
	return strings.TrimSpace(string(out))
}

// CurrentQdisc 返回当前队列调度算法
func CurrentQdisc() string {
	out, err := exec.Command("sysctl", "-n", "net.core.default_qdisc").Output()
	if err != nil {
		return "未知"
	}
	return strings.TrimSpace(string(out))
}

// DefaultInterface returns the default outbound network interface when detectable.
func DefaultInterface() string {
	if out, err := exec.Command("ip", "route", "get", "1.1.1.1").Output(); err == nil {
		if iface := interfaceFromRouteOutput(string(out)); iface != "" {
			return iface
		}
	}
	if out, err := exec.Command("ip", "route", "show", "default").Output(); err == nil {
		if iface := interfaceFromRouteOutput(string(out)); iface != "" {
			return iface
		}
	}
	return ""
}

// CurrentDeviceQdisc returns the root qdisc for one interface.
func CurrentDeviceQdisc(iface string) (string, error) {
	iface = strings.TrimSpace(iface)
	if iface == "" {
		return "", fmt.Errorf("default interface not found")
	}
	out, err := exec.Command("tc", "qdisc", "show", "dev", iface).Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	for i, field := range fields {
		if field == "qdisc" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("qdisc not found for %s", iface)
}

// RecommendedRuntimeStatus reports whether BBR+FQ is active in sysctl and on the default interface.
func RecommendedRuntimeStatus() RuntimeStatus {
	mode := RecommendedMode()
	status := RuntimeStatus{
		CC:                CurrentCC(),
		DefaultQdisc:      CurrentQdisc(),
		DefaultInterface:  DefaultInterface(),
		RecommendedSysctl: IsModeActive(mode),
	}
	qdisc, err := CurrentDeviceQdisc(status.DefaultInterface)
	if err != nil {
		status.DeviceQdiscError = err.Error()
		return status
	}
	status.DeviceQdisc = qdisc
	status.RecommendedRuntime = status.RecommendedSysctl && qdisc == mode.Qdisc
	return status
}

// ApplyDeviceQdisc applies qdisc to the current default outbound interface.
func ApplyDeviceQdisc(qdisc string) error {
	qdisc = strings.TrimSpace(qdisc)
	if qdisc == "" {
		return nil
	}
	iface := DefaultInterface()
	if iface == "" {
		return fmt.Errorf("default interface not found")
	}
	if _, err := exec.LookPath("tc"); err != nil {
		return fmt.Errorf("tc command not found: %w", err)
	}
	out, err := exec.Command("tc", "qdisc", "replace", "dev", iface, "root", qdisc).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tc qdisc replace dev %s root %s failed: %s: %w", iface, qdisc, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func interfaceFromRouteOutput(output string) string {
	fields := strings.Fields(output)
	for i, field := range fields {
		if field == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// AvailableCC 返回当前内核支持的拥塞控制算法列表
func AvailableCC() []string {
	out, err := exec.Command("sysctl", "-n", "net.ipv4.tcp_available_congestion_control").Output()
	if err != nil {
		return nil
	}
	return strings.Fields(strings.TrimSpace(string(out)))
}

// IsCCAvailable reports whether the current kernel supports a congestion control algorithm.
func IsCCAvailable(cc string) bool {
	cc = strings.TrimSpace(cc)
	if cc == "" {
		return false
	}
	_ = exec.Command("modprobe", "tcp_"+cc).Run()
	for _, available := range AvailableCC() {
		if available == cc {
			return true
		}
	}
	return false
}

// AutoEnableRecommended enables BBR+FQ only when the running kernel already supports BBR.
// It never installs or switches kernels and never requires a reboot.
func AutoEnableRecommended() (AutoEnableResult, error) {
	mode := RecommendedMode()
	status := RecommendedRuntimeStatus()
	if status.RecommendedRuntime {
		return AutoEnableResult{
			AlreadyActive: true,
			Supported:     true,
			Message:       fmt.Sprintf("%s 已经启用", mode.Label),
		}, nil
	}
	if !IsRoot() {
		return AutoEnableResult{
			Supported: false,
			Message:   "自动启用 BBR+FQ 需要 root 权限",
		}, fmt.Errorf("需要 root 权限")
	}
	if !IsCCAvailable(mode.CC) {
		return AutoEnableResult{
			Supported: false,
			Message: fmt.Sprintf("当前内核不支持 %s，未自动启用；可用算法: %s",
				mode.CC, strings.Join(AvailableCC(), ", ")),
		}, nil
	}
	if err := SetCC(mode); err != nil {
		return AutoEnableResult{Supported: true, Message: err.Error()}, err
	}
	status = RecommendedRuntimeStatus()
	if !status.RecommendedRuntime {
		detail := fmt.Sprintf("当前状态为 %s + %s", status.CC, status.DefaultQdisc)
		if status.DeviceQdisc != "" {
			detail += fmt.Sprintf("，网卡 qdisc=%s", status.DeviceQdisc)
		}
		if status.DeviceQdiscError != "" {
			detail += fmt.Sprintf("，无法确认网卡 qdisc: %s", status.DeviceQdiscError)
		}
		return AutoEnableResult{
			Applied:   true,
			Supported: true,
			Message:   fmt.Sprintf("已尝试启用 %s，但%s", mode.Label, detail),
		}, fmt.Errorf("BBR+FQ 状态未确认")
	}
	return AutoEnableResult{
		Applied:   true,
		Supported: true,
		Message:   fmt.Sprintf("%s 已自动启用，并会在重启后持续生效", mode.Label),
	}, nil
}

// SetCC 设置拥塞控制算法和队列调度，持久化到配置文件并立即生效
func SetCC(mode CCMode) error {
	// 尝试加载模块
	_ = exec.Command("modprobe", "tcp_"+mode.CC).Run()
	if mode.Qdisc != "" {
		_ = exec.Command("modprobe", "sch_"+mode.Qdisc).Run()
	}

	// 检查目标 CC 算法是否可用
	available := AvailableCC()
	found := false
	for _, cc := range available {
		if cc == mode.CC {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("当前内核不支持 %s 算法（可用: %s），可能需要先安装对应内核",
			mode.CC, strings.Join(available, ", "))
	}

	// 构建配置内容
	var lines []string
	if mode.Qdisc != "" {
		lines = append(lines, fmt.Sprintf("net.core.default_qdisc=%s", mode.Qdisc))
	}
	lines = append(lines, fmt.Sprintf("net.ipv4.tcp_congestion_control=%s", mode.CC))

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(SysctlBBRConf, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 sysctl 配置失败: %w", err)
	}

	// 立即生效
	if out, err := exec.Command("sysctl", "-p", SysctlBBRConf).CombinedOutput(); err != nil {
		return fmt.Errorf("应用 sysctl 配置失败: %s", string(out))
	}
	if err := ApplyDeviceQdisc(mode.Qdisc); err != nil {
		return fmt.Errorf("应用网卡 qdisc 失败: %w", err)
	}

	return nil
}

// DisableAll 卸载 BBR/系统优化配置，恢复默认 cubic。
// IPv6 是独立网络开关，不在此处修改。
func DisableAll() error {
	// 删除 vasmax BBR/优化 sysctl 配置文件。
	_ = os.Remove(SysctlBBRConf)
	_ = os.Remove(SysctlOptConf)

	// 立即恢复 cubic 和默认 qdisc
	if err := exec.Command("sysctl", "-w", "net.ipv4.tcp_congestion_control=cubic").Run(); err != nil {
		return fmt.Errorf("恢复 cubic 失败: %w", err)
	}
	// fq_codel 是大多数现代 Linux 发行版的默认 qdisc
	_ = exec.Command("sysctl", "-w", "net.core.default_qdisc=fq_codel").Run()

	// 重新加载所有 sysctl
	return ReloadSysctl()
}

// ReloadSysctl 重新加载所有 sysctl 配置文件
func ReloadSysctl() error {
	if out, err := exec.Command("sysctl", "--system").CombinedOutput(); err != nil {
		return fmt.Errorf("sysctl --system 失败: %s", string(out))
	}
	return nil
}
