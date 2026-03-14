package bbr

import (
	"os"
	"os/exec"
	"strings"
)

// Distro 描述当前系统发行版信息
type Distro struct {
	ID      string // debian / ubuntu / centos / rhel / fedora
	Version string // 22.04 / 20.04 / 7 / 8
	Arch    string // x86_64 / aarch64
	PkgMgr  string // apt / yum / dnf
}

// DetectDistro 检测当前发行版
func DetectDistro() (*Distro, error) {
	d := &Distro{}

	// 架构
	out, err := exec.Command("uname", "-m").Output()
	if err == nil {
		d.Arch = strings.TrimSpace(string(out))
	}

	// 读取 /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return d, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		switch key {
		case "ID":
			d.ID = strings.ToLower(val)
		case "VERSION_ID":
			d.Version = val
		}
	}

	// 包管理器
	switch d.ID {
	case "debian", "ubuntu", "linuxmint", "pop":
		d.PkgMgr = "apt"
	case "centos", "rhel", "rocky", "almalinux", "ol", "scientific":
		if _, err := exec.LookPath("dnf"); err == nil {
			d.PkgMgr = "dnf"
		} else {
			d.PkgMgr = "yum"
		}
	case "fedora":
		d.PkgMgr = "dnf"
	default:
		// 尝试自动检测
		if _, err := exec.LookPath("apt"); err == nil {
			d.PkgMgr = "apt"
		} else if _, err := exec.LookPath("dnf"); err == nil {
			d.PkgMgr = "dnf"
		} else if _, err := exec.LookPath("yum"); err == nil {
			d.PkgMgr = "yum"
		}
	}

	return d, nil
}

// IsRoot 检查当前进程是否以 root 运行
func IsRoot() bool {
	return os.Getuid() == 0
}

// KernelVersion 返回当前内核版本字符串
func KernelVersion() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "未知"
	}
	return strings.TrimSpace(string(out))
}

// IsDebian 判断是否为 Debian 系
func (d *Distro) IsDebian() bool {
	return d.PkgMgr == "apt"
}

// IsRHEL 判断是否为 RHEL 系
func (d *Distro) IsRHEL() bool {
	return d.PkgMgr == "yum" || d.PkgMgr == "dnf"
}
