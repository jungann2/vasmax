package route

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"vasmax/internal/security"
)

// BTManager manages BT protocol blocking/allowing.
type BTManager struct {
	mgr *Manager
}

// NewBTManager creates a new BT download manager.
func NewBTManager(mgr *Manager) *BTManager {
	return &BTManager{mgr: mgr}
}

// Block enables BT protocol blocking by adding sniffing and block rules.
func (b *BTManager) Block() error {
	b.mgr.mu.Lock()
	defer b.mgr.mu.Unlock()

	// Enable sniffing on Xray inbounds to detect BT protocol.
	if err := b.enableXraySniffing(); err != nil {
		return fmt.Errorf("failed to enable sniffing: %w", err)
	}

	// Add BT block rule.
	rule := &RouteRule{
		Type:     "bt_block",
		Protocol: []string{"bittorrent"},
		Outbound: "blocked",
	}

	if err := b.mgr.addXrayRule(rule); err != nil {
		return fmt.Errorf("failed to add BT block rule: %w", err)
	}

	b.mgr.logger.Info("BT protocol blocking enabled")
	return nil
}

// Allow disables BT protocol blocking.
func (b *BTManager) Allow() error {
	b.mgr.mu.Lock()
	defer b.mgr.mu.Unlock()

	if err := b.mgr.removeXrayRule("bt_block"); err != nil {
		return fmt.Errorf("failed to remove BT block rule: %w", err)
	}

	b.mgr.logger.Info("BT protocol blocking disabled")
	return nil
}

// IsBlocked returns whether BT is currently blocked.
func (b *BTManager) IsBlocked() bool {
	rules, err := b.mgr.loadCustomRules()
	if err != nil {
		return false
	}
	for _, r := range rules {
		if r.Type == "bt_block" {
			return true
		}
	}
	return false
}

// enableXraySniffing enables protocol sniffing on all Xray inbounds.
func (b *BTManager) enableXraySniffing() error {
	// Read all inbound config files and add sniffing if not present.
	entries, err := os.ReadDir(b.mgr.xrayConfDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only process inbound config files (05_*_inbounds.json pattern).
		if len(name) < 3 || name[:2] != "05" {
			continue
		}

		path := filepath.Join(b.mgr.xrayConfDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}

		// Add sniffing to inbounds.
		if inboundsRaw, ok := cfg["inbounds"]; ok {
			if inbounds, ok := inboundsRaw.([]interface{}); ok {
				modified := false
				for i, ib := range inbounds {
					if ibMap, ok := ib.(map[string]interface{}); ok {
						if _, hasSniffing := ibMap["sniffing"]; !hasSniffing {
							ibMap["sniffing"] = map[string]interface{}{
								"enabled":      true,
								"destOverride": []string{"http", "tls", "quic"},
							}
							inbounds[i] = ibMap
							modified = true
						}
					}
				}
				if modified {
					cfg["inbounds"] = inbounds
					_ = security.AtomicWriteJSON(path, cfg, 0644)
				}
			}
		}
	}
	return nil
}
