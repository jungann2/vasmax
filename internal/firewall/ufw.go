package firewall

import (
	"fmt"
	"os/exec"
)

// ufwBackend implements FirewallBackend using ufw.
type ufwBackend struct{}

func newUFW() *ufwBackend { return &ufwBackend{} }

func (u *ufwBackend) IsActive() bool {
	if !commandExists("ufw") {
		return false
	}
	out, err := exec.Command("ufw", "status").CombinedOutput()
	if err != nil {
		return false
	}
	return len(out) > 0 && contains(string(out), "Status: active")
}

func (u *ufwBackend) AddPort(port int, protocol string) error {
	return runCmd("ufw", "allow", fmt.Sprintf("%d/%s", port, protocol))
}

func (u *ufwBackend) RemovePort(port int, protocol string) error {
	return runCmd("ufw", "delete", "allow", fmt.Sprintf("%d/%s", port, protocol))
}

func (u *ufwBackend) AddPortRange(start, end int, protocol string) error {
	return runCmd("ufw", "allow", fmt.Sprintf("%d:%d/%s", start, end, protocol))
}

func (u *ufwBackend) RemovePortRange(start, end int, protocol string) error {
	return runCmd("ufw", "delete", "allow", fmt.Sprintf("%d:%d/%s", start, end, protocol))
}

func (u *ufwBackend) AddPortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	// UFW doesn't natively support port forwarding; use iptables NAT rules.
	return addIptablesForward(srcStart, srcEnd, dstPort, protocol)
}

func (u *ufwBackend) RemovePortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	return removeIptablesForward(srcStart, srcEnd, dstPort, protocol)
}
