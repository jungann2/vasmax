package bbr

import (
	"os"
	"os/exec"
	"strings"
)

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
