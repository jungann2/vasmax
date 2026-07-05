package firewall

import (
	"errors"
	"fmt"
)

// PortHopConfig holds port hopping configuration.
type PortHopConfig struct {
	StartPort  int    // Range start (e.g., 30000)
	EndPort    int    // Range end (e.g., 40000)
	TargetPort int    // Actual service port
	Protocol   string // udp (typically for Hysteria2/Tuic)
}

// DefaultPortHopRange returns the default port hopping range.
func DefaultPortHopRange() (int, int) {
	return 30000, 40000
}

// SetupPortHopping configures port hopping using the firewall manager.
// It opens the port range and sets up NAT forwarding to the target port.
func (m *Manager) SetupPortHopping(cfg *PortHopConfig) error {
	if m == nil {
		return fmt.Errorf("firewall manager is nil")
	}
	if m.backend == nil {
		if m.logger != nil {
			m.logger.Warn("no firewall backend, cannot setup port hopping")
		}
		return fmt.Errorf("no active firewall backend; port hopping requires local NAT/port-forward support")
	}

	if cfg.StartPort < 1 || cfg.EndPort > 65535 || cfg.StartPort >= cfg.EndPort {
		return fmt.Errorf("invalid port range: %d-%d", cfg.StartPort, cfg.EndPort)
	}
	if cfg.TargetPort < 1 || cfg.TargetPort > 65535 {
		return fmt.Errorf("invalid target port: %d", cfg.TargetPort)
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "udp"
	}

	// Open the port range in the firewall.
	if err := m.backend.AddPortRange(cfg.StartPort, cfg.EndPort, cfg.Protocol); err != nil {
		return fmt.Errorf("failed to open port range: %w", err)
	}

	// Set up NAT forwarding from range to target port.
	if err := m.backend.AddPortForward(cfg.StartPort, cfg.EndPort, cfg.TargetPort, cfg.Protocol); err != nil {
		_ = m.backend.RemovePortRange(cfg.StartPort, cfg.EndPort, cfg.Protocol)
		return fmt.Errorf("failed to setup port forwarding: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("port hopping configured: %d-%d/%s -> %d",
			cfg.StartPort, cfg.EndPort, cfg.Protocol, cfg.TargetPort)
	}
	return nil
}

// RemovePortHopping removes port hopping configuration.
func (m *Manager) RemovePortHopping(cfg *PortHopConfig) error {
	if m == nil {
		return nil
	}
	if m.backend == nil {
		return nil
	}

	if cfg.Protocol == "" {
		cfg.Protocol = "udp"
	}

	// Remove forwarding first, then close the port range.
	forwardErr := m.backend.RemovePortForward(cfg.StartPort, cfg.EndPort, cfg.TargetPort, cfg.Protocol)
	rangeErr := m.backend.RemovePortRange(cfg.StartPort, cfg.EndPort, cfg.Protocol)
	if err := errors.Join(forwardErr, rangeErr); err != nil {
		return err
	}

	if m.logger != nil {
		m.logger.Infof("port hopping removed: %d-%d/%s", cfg.StartPort, cfg.EndPort, cfg.Protocol)
	}
	return nil
}
