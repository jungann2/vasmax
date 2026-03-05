// Package firewall provides firewall management with auto-detection of ufw/firewalld/iptables.
package firewall

import (
	"os/exec"

	"github.com/sirupsen/logrus"
)

// FirewallBackend defines the interface for firewall operations.
type FirewallBackend interface {
	AddPort(port int, protocol string) error
	RemovePort(port int, protocol string) error
	AddPortRange(start, end int, protocol string) error
	RemovePortRange(start, end int, protocol string) error
	AddPortForward(srcStart, srcEnd, dstPort int, protocol string) error
	RemovePortForward(srcStart, srcEnd, dstPort int, protocol string) error
	IsActive() bool
}

// Manager manages firewall rules using the detected backend.
type Manager struct {
	backend FirewallBackend
	logger  *logrus.Logger
}

// NewManager creates a firewall manager with auto-detected backend.
func NewManager(logger *logrus.Logger) *Manager {
	backend := Detect(logger)
	return &Manager{backend: backend, logger: logger}
}

// Detect auto-detects the active firewall backend.
// Priority: ufw > firewalld > iptables. Returns nil if none found.
func Detect(logger *logrus.Logger) FirewallBackend {
	if b := newUFW(); b.IsActive() {
		logger.Info("detected firewall: ufw")
		return b
	}
	if b := newFirewalld(); b.IsActive() {
		logger.Info("detected firewall: firewalld")
		return b
	}
	if b := newIptables(); b.IsActive() {
		logger.Info("detected firewall: iptables")
		return b
	}
	logger.Warn("no active firewall detected, firewall operations will be skipped")
	return nil
}

// Backend returns the detected firewall backend (may be nil).
func (m *Manager) Backend() FirewallBackend { return m.backend }

// AddPort adds a port rule. Skips if no backend detected.
func (m *Manager) AddPort(port int, protocol string) error {
	if m.backend == nil {
		m.logger.Warn("no firewall backend, skipping AddPort")
		return nil
	}
	return m.backend.AddPort(port, protocol)
}

// RemovePort removes a port rule.
func (m *Manager) RemovePort(port int, protocol string) error {
	if m.backend == nil {
		return nil
	}
	return m.backend.RemovePort(port, protocol)
}

// AddPortRange adds a port range rule.
func (m *Manager) AddPortRange(start, end int, protocol string) error {
	if m.backend == nil {
		return nil
	}
	return m.backend.AddPortRange(start, end, protocol)
}

// RemovePortRange removes a port range rule.
func (m *Manager) RemovePortRange(start, end int, protocol string) error {
	if m.backend == nil {
		return nil
	}
	return m.backend.RemovePortRange(start, end, protocol)
}

// commandExists checks if a command is available in PATH.
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
