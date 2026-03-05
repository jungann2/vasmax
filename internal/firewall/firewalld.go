package firewall

import (
	"fmt"
	"os/exec"
)

// firewalldBackend implements FirewallBackend using firewalld.
type firewalldBackend struct{}

func newFirewalld() *firewalldBackend { return &firewalldBackend{} }

func (f *firewalldBackend) IsActive() bool {
	if !commandExists("firewall-cmd") {
		return false
	}
	err := exec.Command("firewall-cmd", "--state").Run()
	return err == nil
}

func (f *firewalldBackend) AddPort(port int, protocol string) error {
	rule := fmt.Sprintf("%d/%s", port, protocol)
	if err := runCmd("firewall-cmd", "--zone=public", "--add-port="+rule, "--permanent"); err != nil {
		return err
	}
	return runCmd("firewall-cmd", "--reload")
}

func (f *firewalldBackend) RemovePort(port int, protocol string) error {
	rule := fmt.Sprintf("%d/%s", port, protocol)
	if err := runCmd("firewall-cmd", "--zone=public", "--remove-port="+rule, "--permanent"); err != nil {
		return err
	}
	return runCmd("firewall-cmd", "--reload")
}

func (f *firewalldBackend) AddPortRange(start, end int, protocol string) error {
	rule := fmt.Sprintf("%d-%d/%s", start, end, protocol)
	if err := runCmd("firewall-cmd", "--zone=public", "--add-port="+rule, "--permanent"); err != nil {
		return err
	}
	return runCmd("firewall-cmd", "--reload")
}

func (f *firewalldBackend) RemovePortRange(start, end int, protocol string) error {
	rule := fmt.Sprintf("%d-%d/%s", start, end, protocol)
	if err := runCmd("firewall-cmd", "--zone=public", "--remove-port="+rule, "--permanent"); err != nil {
		return err
	}
	return runCmd("firewall-cmd", "--reload")
}

func (f *firewalldBackend) AddPortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	// Use rich rule for port range forwarding (much more efficient than per-port rules).
	rule := fmt.Sprintf(
		`rule family="ipv4" forward-port port="%d-%d" protocol="%s" to-port="%d"`,
		srcStart, srcEnd, protocol, dstPort,
	)
	if err := runCmd("firewall-cmd", "--zone=public", "--add-rich-rule="+rule, "--permanent"); err != nil {
		return err
	}
	return runCmd("firewall-cmd", "--reload")
}

func (f *firewalldBackend) RemovePortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	rule := fmt.Sprintf(
		`rule family="ipv4" forward-port port="%d-%d" protocol="%s" to-port="%d"`,
		srcStart, srcEnd, protocol, dstPort,
	)
	_ = runCmd("firewall-cmd", "--zone=public", "--remove-rich-rule="+rule, "--permanent")
	return runCmd("firewall-cmd", "--reload")
}
