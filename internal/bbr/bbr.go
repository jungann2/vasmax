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

// 预定义的加速模式
var CCModes = []CCMode{
	{Qdisc: "fq", CC: "bbr", Label: "BBR + FQ（推荐）"},
	{Qdisc: "fq_pie", CC: "bbr", Label: "BBR + FQ_PIE"},
	{Qdisc: "cake", CC: "bbr", Label: "BBR + CAKE"},
	{Qdisc: "fq", CC: "bbr2", Label: "BBR2 + FQ"},
	{Qdisc: "fq_pie", CC: "bbr2", Label: "BBR2 + FQ_PIE"},
	{Qdisc: "cake", CC: "bbr2", Label: "BBR2 + CAKE"},
	{Qdisc: "fq", CC: "bbrplus", Label: "BBRplus + FQ"},
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

// AvailableCC 返回当前内核支持的拥塞控制算法列表
func AvailableCC() []string {
	out, err := exec.Command("sysctl", "-n", "net.ipv4.tcp_available_congestion_control").Output()
	if err != nil {
		return nil
	}
	return strings.Fields(strings.TrimSpace(string(out)))
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

	return nil
}

// DisableAll 卸载全部加速配置，恢复默认 cubic
// 注意：此操作会同时删除 BBR 配置和系统优化配置（包括 ECN 设置）
func DisableAll() error {
	// 删除所有 vasmax sysctl 配置文件
	_ = os.Remove(SysctlBBRConf)
	_ = os.Remove(SysctlOptConf)
	_ = os.Remove(SysctlIPv6Conf)

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

// EditSysctlFile 用编辑器打开 BBR sysctl 配置文件，编辑后自动重新加载
func EditSysctlFile() error {
	// 确保文件存在
	if _, err := os.Stat(SysctlBBRConf); os.IsNotExist(err) {
		if err := os.WriteFile(SysctlBBRConf, []byte("# vasmax BBR 配置\n"), 0644); err != nil {
			return err
		}
	}

	// 按优先级查找可用编辑器
	editor := ""
	for _, e := range []string{"nano", "vi", "vim"} {
		if _, err := exec.LookPath(e); err == nil {
			editor = e
			break
		}
	}
	if editor == "" {
		return fmt.Errorf("未找到可用的文本编辑器（需要 nano、vi 或 vim）")
	}

	cmd := exec.Command(editor, SysctlBBRConf)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// 编辑完成后自动重新加载使修改立即生效
	return ReloadSysctl()
}
