package firewall

import (
	"fmt"
	"os/exec"
	"strings"
)

// iptablesBackend implements FirewallBackend using iptables.
type iptablesBackend struct{}

func newIptables() *iptablesBackend { return &iptablesBackend{} }

func (i *iptablesBackend) IsActive() bool {
	return commandExists("iptables")
}

func (i *iptablesBackend) AddPort(port int, protocol string) error {
	return runCmd("iptables", "-I", "INPUT", "-p", protocol, "--dport",
		fmt.Sprintf("%d", port), "-j", "ACCEPT")
}

func (i *iptablesBackend) RemovePort(port int, protocol string) error {
	return runCmd("iptables", "-D", "INPUT", "-p", protocol, "--dport",
		fmt.Sprintf("%d", port), "-j", "ACCEPT")
}

func (i *iptablesBackend) AddPortRange(start, end int, protocol string) error {
	return runCmd("iptables", "-I", "INPUT", "-p", protocol, "--dport",
		fmt.Sprintf("%d:%d", start, end), "-j", "ACCEPT")
}

func (i *iptablesBackend) RemovePortRange(start, end int, protocol string) error {
	return runCmd("iptables", "-D", "INPUT", "-p", protocol, "--dport",
		fmt.Sprintf("%d:%d", start, end), "-j", "ACCEPT")
}

func (i *iptablesBackend) AddPortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	return addIptablesForward(srcStart, srcEnd, dstPort, protocol)
}

func (i *iptablesBackend) RemovePortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	return removeIptablesForward(srcStart, srcEnd, dstPort, protocol)
}

// addIptablesForward adds iptables NAT PREROUTING rules for port forwarding.
func addIptablesForward(srcStart, srcEnd, dstPort int, protocol string) error {
	srcRange := fmt.Sprintf("%d:%d", srcStart, srcEnd)
	dst := fmt.Sprintf("%d", dstPort)

	// Check if rule already exists.
	out, _ := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n").CombinedOutput()
	if strings.Contains(string(out), srcRange) && strings.Contains(string(out), dst) {
		return nil // Already exists.
	}

	return runCmd("iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", protocol, "--dport", srcRange,
		"-j", "REDIRECT", "--to-port", dst)
}

// removeIptablesForward removes iptables NAT PREROUTING rules for port forwarding.
func removeIptablesForward(srcStart, srcEnd, dstPort int, protocol string) error {
	return runCmd("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", protocol, "--dport", fmt.Sprintf("%d:%d", srcStart, srcEnd),
		"-j", "REDIRECT", "--to-port", fmt.Sprintf("%d", dstPort))
}

// runCmd executes a command with arguments and returns any error.
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %s: %w", name, strings.Join(args, " "), string(output), err)
	}
	return nil
}

// contains checks if s contains substr (case-insensitive not needed here).
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
