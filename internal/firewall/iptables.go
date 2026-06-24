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
	return addIptablesRule("", "INPUT", "-I", "-p", protocol, "--dport",
		fmt.Sprintf("%d", port), "-j", "ACCEPT")
}

func (i *iptablesBackend) RemovePort(port int, protocol string) error {
	return removeIptablesRules("", "INPUT", "-p", protocol, "--dport",
		fmt.Sprintf("%d", port), "-j", "ACCEPT")
}

func (i *iptablesBackend) AddPortRange(start, end int, protocol string) error {
	return addIptablesRule("", "INPUT", "-I", "-p", protocol, "--dport",
		fmt.Sprintf("%d:%d", start, end), "-j", "ACCEPT")
}

func (i *iptablesBackend) RemovePortRange(start, end int, protocol string) error {
	return removeIptablesRules("", "INPUT", "-p", protocol, "--dport",
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

	return addIptablesRule("nat", "PREROUTING", "-A",
		"-p", protocol, "--dport", srcRange,
		"-j", "REDIRECT", "--to-port", dst)
}

// removeIptablesForward removes iptables NAT PREROUTING rules for port forwarding.
func removeIptablesForward(srcStart, srcEnd, dstPort int, protocol string) error {
	return removeIptablesRules("nat", "PREROUTING",
		"-p", protocol, "--dport", fmt.Sprintf("%d:%d", srcStart, srcEnd),
		"-j", "REDIRECT", "--to-port", fmt.Sprintf("%d", dstPort))
}

func addIptablesRule(table, chain, op string, rule ...string) error {
	if iptablesRuleExists(table, chain, rule...) {
		return nil
	}
	args := iptablesBaseArgs(table)
	args = append(args, op, chain)
	args = append(args, rule...)
	return runCmd("iptables", args...)
}

func removeIptablesRules(table, chain string, rule ...string) error {
	for iptablesRuleExists(table, chain, rule...) {
		args := iptablesBaseArgs(table)
		args = append(args, "-D", chain)
		args = append(args, rule...)
		if err := runCmd("iptables", args...); err != nil {
			return err
		}
	}
	return nil
}

func iptablesRuleExists(table, chain string, rule ...string) bool {
	args := iptablesBaseArgs(table)
	args = append(args, "-C", chain)
	args = append(args, rule...)
	return exec.Command("iptables", args...).Run() == nil
}

func iptablesBaseArgs(table string) []string {
	if table == "" {
		return nil
	}
	return []string{"-t", table}
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
