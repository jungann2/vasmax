package bbr

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// KernelTarget 内核安装目标类型
type KernelTarget int

const (
	KernelBBR        KernelTarget = iota // BBR 原版内核
	KernelBBRPlus                        // BBRplus 版内核
	KernelLotserver                      // Lotserver（锐速）内核
	KernelBBRPlusNew                     // BBRplus 新版内核
	KernelZen                            // Zen 官方版内核
	KernelCloud                          // 官方 cloud 内核
	KernelStable                         // 官方稳定内核
	KernelLatest                         // 官方最新内核
	KernelXANMODMain                     // XANMOD-main
	KernelXANMODLTS                      // XANMOD-LTS
	KernelXANMODEdge                     // XANMOD-EDGE
	KernelXANMODRT                       // XANMOD-RT
)

// KernelInfo 内核安装信息
type KernelInfo struct {
	Label      string
	DebianOnly bool
	RHELOnly   bool
	NeedReboot bool
}

var kernelInfoMap = map[KernelTarget]KernelInfo{
	KernelBBR:        {Label: "BBR 原版内核", NeedReboot: true},
	KernelBBRPlus:    {Label: "BBRplus 版内核", DebianOnly: true, NeedReboot: true},
	KernelLotserver:  {Label: "Lotserver（锐速）内核", NeedReboot: true},
	KernelBBRPlusNew: {Label: "BBRplus 新版内核", DebianOnly: true, NeedReboot: true},
	KernelZen:        {Label: "Zen 官方版内核", DebianOnly: true, NeedReboot: true},
	KernelCloud:      {Label: "官方 cloud 内核", NeedReboot: true},
	KernelStable:     {Label: "官方稳定内核", NeedReboot: true},
	KernelLatest:     {Label: "官方最新内核", NeedReboot: true},
	KernelXANMODMain: {Label: "XANMOD-main 内核", DebianOnly: true, NeedReboot: true},
	KernelXANMODLTS:  {Label: "XANMOD-LTS 内核", DebianOnly: true, NeedReboot: true},
	KernelXANMODEdge: {Label: "XANMOD-EDGE 内核", DebianOnly: true, NeedReboot: true},
	KernelXANMODRT:   {Label: "XANMOD-RT 内核", DebianOnly: true, NeedReboot: true},
}

// GetKernelLabel 返回内核类型的显示名称
func GetKernelLabel(t KernelTarget) string {
	if info, ok := kernelInfoMap[t]; ok {
		return info.Label
	}
	return "未知内核"
}

// InstallKernel 安装指定内核
func InstallKernel(target KernelTarget, distro *Distro) error {
	info := kernelInfoMap[target]

	if info.DebianOnly && !distro.IsDebian() {
		return fmt.Errorf("%s 仅支持 Debian/Ubuntu 系统", info.Label)
	}
	if info.RHELOnly && !distro.IsRHEL() {
		return fmt.Errorf("%s 仅支持 CentOS/RHEL 系统", info.Label)
	}

	switch target {
	case KernelBBR:
		return installBBRVanilla(distro)
	case KernelBBRPlus:
		return installBBRPlus(distro, false)
	case KernelBBRPlusNew:
		return installBBRPlus(distro, true)
	case KernelLotserver:
		return installLotserverKernel(distro)
	case KernelZen:
		return installZenKernel(distro)
	case KernelCloud:
		return installMainlineKernel(distro, "cloud")
	case KernelStable:
		return installMainlineKernel(distro, "stable")
	case KernelLatest:
		return installMainlineKernel(distro, "latest")
	case KernelXANMODMain:
		return installXANMOD(distro, "linux-xanmod")
	case KernelXANMODLTS:
		return installXANMOD(distro, "linux-xanmod-lts")
	case KernelXANMODEdge:
		return installXANMOD(distro, "linux-xanmod-edge")
	case KernelXANMODRT:
		return installXANMOD(distro, "linux-xanmod-rt-lts")
	}
	return fmt.Errorf("未知内核类型")
}

func installBBRVanilla(distro *Distro) error {
	if distro.IsDebian() {
		// 用 bash -c 确保 shell 展开 $(lsb_release -rs)
		runApt("update")
		cmd := exec.Command("bash", "-c", "apt-get install -y linux-generic-hwe-$(lsb_release -rs)")
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if distro.IsRHEL() {
		// RHEL 系：安装 elrepo 后安装 mainline 内核
		if err := installELRepo(); err != nil {
			return err
		}
		return runCmd(distro.PkgMgr, "install", "-y", "--enablerepo=elrepo-kernel", "kernel-ml")
	}
	return fmt.Errorf("不支持的发行版: %s", distro.ID)
}

func installBBRPlus(distro *Distro, newVersion bool) error {
	// 从 GitHub cx9208/bbrplus 下载预编译包
	arch := distro.Arch
	if arch == "" || arch == "x86_64" {
		arch = "amd64"
	}
	var url string
	if newVersion {
		url = fmt.Sprintf("https://github.com/cx9208/bbrplus/releases/latest/download/bbrplus_%s.deb", arch)
	} else {
		url = fmt.Sprintf("https://github.com/cx9208/bbrplus/releases/download/v1.0/bbrplus_%s.deb", arch)
	}
	if distro.IsDebian() {
		return downloadAndInstallDeb(url)
	}
	return fmt.Errorf("BBRplus 预编译包暂不支持 RHEL 系，请手动编译")
}

func installLotserverKernel(_ *Distro) error {
	// 使用 appotry/LotServer 安装脚本
	return runBashScript("https://raw.githubusercontent.com/appotry/LotServer/master/lotServer.sh", "install")
}

func installZenKernel(distro *Distro) error {
	if !distro.IsDebian() {
		return fmt.Errorf("Zen 内核仅支持 Debian/Ubuntu")
	}
	runApt("update")
	return runApt("install", "-y", "linux-image-zen")
}

func installMainlineKernel(distro *Distro, variant string) error {
	if distro.IsDebian() {
		script := "https://raw.githubusercontent.com/pimlie/ubuntu-mainline-kernel.sh/master/ubuntu-mainline-kernel.sh"
		switch variant {
		case "latest":
			return runBashScript(script, "--install", "latest")
		case "stable":
			return runBashScript(script, "--install", "stable")
		case "cloud":
			runApt("update")
			arch := distro.Arch
			pkg := "linux-image-cloud-amd64"
			if arch == "aarch64" || arch == "arm64" {
				pkg = "linux-image-cloud-arm64"
			}
			return runApt("install", "-y", pkg)
		default:
			return fmt.Errorf("未知内核变体: %s", variant)
		}
	}
	if distro.IsRHEL() {
		if variant == "cloud" {
			return fmt.Errorf("官方 cloud 内核不支持 CentOS/RHEL 系统")
		}
		if err := installELRepo(); err != nil {
			return err
		}
		// stable → kernel-lt（长期支持），latest/其他 → kernel-ml（主线）
		pkg := "kernel-ml"
		if variant == "stable" {
			pkg = "kernel-lt"
		}
		return runCmd(distro.PkgMgr, "install", "-y", "--enablerepo=elrepo-kernel", pkg)
	}
	return fmt.Errorf("不支持的发行版: %s", distro.ID)
}

func installXANMOD(distro *Distro, pkg string) error {
	if !distro.IsDebian() {
		return fmt.Errorf("XANMOD 内核仅支持 Debian/Ubuntu")
	}
	// 添加 XANMOD APT 仓库
	steps := [][]string{
		{"bash", "-c", "wget -qO - https://dl.xanmod.org/archive.key | gpg --dearmor -o /usr/share/keyrings/xanmod-archive-keyring.gpg"},
		{"bash", "-c", `echo 'deb [signed-by=/usr/share/keyrings/xanmod-archive-keyring.gpg] http://deb.xanmod.org releases main' | tee /etc/apt/sources.list.d/xanmod-release.list`},
	}
	for _, step := range steps {
		if out, err := exec.Command(step[0], step[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("添加 XANMOD 仓库失败: %s", string(out))
		}
	}
	runApt("update")
	return runApt("install", "-y", pkg)
}

// InstallLotserverAccel 安装 Lotserver 加速（不换内核）
func InstallLotserverAccel() error {
	return runBashScript("https://raw.githubusercontent.com/appotry/LotServer/master/lotServer.sh", "install")
}

// InstallBrutal 编译安装 brutal 模块
func InstallBrutal() error {
	// 安装编译依赖（用 bash -c 确保 shell 展开 $(uname -r)）
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("bash", "-c", "apt-get install -y linux-headers-$(uname -r) build-essential git")
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装编译依赖失败: %w", err)
		}
	} else if _, err := exec.LookPath("yum"); err == nil {
		// RHEL 系
		cmd := exec.Command("bash", "-c", "yum install -y kernel-devel-$(uname -r) gcc make git")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装编译依赖失败: %w", err)
		}
	} else if _, err := exec.LookPath("dnf"); err == nil {
		cmd := exec.Command("bash", "-c", "dnf install -y kernel-devel-$(uname -r) gcc make git")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装编译依赖失败: %w", err)
		}
	}
	// 清理可能残留的上次安装目录
	_ = os.RemoveAll("/tmp/tcp-brutal")

	steps := [][]string{
		{"git", "clone", "--depth=1", "https://github.com/apernet/tcp-brutal.git", "/tmp/tcp-brutal"},
		{"bash", "-c", "cd /tmp/tcp-brutal && make && make install"},
	}
	for _, step := range steps {
		if out, err := exec.Command(step[0], step[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("编译 brutal 失败: %s", string(out))
		}
	}
	_ = os.RemoveAll("/tmp/tcp-brutal")
	return nil
}

// ListInstalledKernels 列出已安装的内核，返回排序后的列表
func ListInstalledKernels(distro *Distro) ([]string, error) {
	var out []byte
	var err error

	if distro.IsDebian() {
		out, err = exec.Command("bash", "-c", `dpkg --list | grep linux-image | awk '{print $2}'`).Output()
	} else {
		out, err = exec.Command("bash", "-c", `rpm -qa | grep "^kernel" | grep -v "kernel-headers\|kernel-devel\|kernel-tools"`).Output()
	}
	if err != nil {
		return nil, fmt.Errorf("获取内核列表失败: %w", err)
	}

	var kernels []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			kernels = append(kernels, line)
		}
	}
	sort.Strings(kernels)
	return kernels, nil
}

// DeleteKernels 删除指定内核列表
func DeleteKernels(toDelete []string, distro *Distro) error {
	if len(toDelete) == 0 {
		return nil
	}
	if distro.IsDebian() {
		args := append([]string{"purge", "-y"}, toDelete...)
		return runApt(args...)
	}
	args := append([]string{"-e"}, toDelete...)
	return runCmd("rpm", args...)
}

// UpdateGrub 更新 grub 引导
func UpdateGrub(distro *Distro) error {
	if distro.IsDebian() {
		if out, err := exec.Command("update-grub").CombinedOutput(); err != nil {
			return fmt.Errorf("update-grub 失败: %s", string(out))
		}
		return nil
	}
	// RHEL 系
	if out, err := exec.Command("grub2-mkconfig", "-o", "/boot/grub2/grub.cfg").CombinedOutput(); err != nil {
		return fmt.Errorf("grub2-mkconfig 失败: %s", string(out))
	}
	return nil
}

// ---- 辅助函数 ----

func runApt(args ...string) error {
	cmd := exec.Command("apt-get", args...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadAndInstallDeb(url string) error {
	tmpFile := "/tmp/vasmax-kernel.deb"
	if out, err := exec.Command("wget", "-O", tmpFile, url).CombinedOutput(); err != nil {
		return fmt.Errorf("下载失败: %s", string(out))
	}
	defer os.Remove(tmpFile)
	if out, err := exec.Command("dpkg", "-i", tmpFile).CombinedOutput(); err != nil {
		return fmt.Errorf("安装 deb 包失败: %s", string(out))
	}
	return nil
}

func runBashScript(url string, args ...string) error {
	// 下载脚本到临时文件后执行
	tmpScript := "/tmp/vasmax-install-script.sh"
	if out, err := exec.Command("wget", "-O", tmpScript, url).CombinedOutput(); err != nil {
		return fmt.Errorf("下载脚本失败: %s", string(out))
	}
	defer os.Remove(tmpScript)
	_ = os.Chmod(tmpScript, 0755)

	cmdArgs := append([]string{tmpScript}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installELRepo() error {
	// 检查是否已安装
	out, _ := exec.Command("rpm", "-q", "elrepo-release").Output()
	if len(out) > 0 && !strings.Contains(string(out), "not installed") {
		return nil
	}
	// 导入 GPG key
	if err := runCmd("rpm", "--import", "https://www.elrepo.org/RPM-GPG-KEY-elrepo.org"); err != nil {
		return fmt.Errorf("导入 ELRepo GPG key 失败: %w", err)
	}
	// 安装 ELRepo release 包（根据 RHEL 版本选择）
	repoURL := "https://www.elrepo.org/elrepo-release-8.el8.elrepo.noarch.rpm"
	if out, _ := exec.Command("rpm", "-E", "%{rhel}").Output(); strings.TrimSpace(string(out)) == "7" {
		repoURL = "https://www.elrepo.org/elrepo-release-7.el7.elrepo.noarch.rpm"
	}
	return runCmd("rpm", "-Uvh", repoURL)
}
