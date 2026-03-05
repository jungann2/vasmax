package route

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"vasmax/internal/security"
)

// BlacklistManager manages domain blacklist routing rules.
type BlacklistManager struct {
	mgr           *Manager
	blacklistFile string // Persistent blacklist file path.
}

// NewBlacklistManager creates a new domain blacklist manager.
func NewBlacklistManager(mgr *Manager) *BlacklistManager {
	blFile := filepath.Join(filepath.Dir(mgr.xrayConfDir), "blacklist.json")
	return &BlacklistManager{mgr: mgr, blacklistFile: blFile}
}

// List returns the current blacklisted domains.
func (b *BlacklistManager) List() ([]string, error) {
	return b.loadBlacklist()
}

// Add adds domains to the blacklist and updates routing rules.
func (b *BlacklistManager) Add(domains ...string) error {
	b.mgr.mu.Lock()
	defer b.mgr.mu.Unlock()

	current, _ := b.loadBlacklist()
	existing := make(map[string]bool)
	for _, d := range current {
		existing[d] = true
	}

	for _, d := range domains {
		if !existing[d] {
			current = append(current, d)
		}
	}

	if err := b.saveBlacklist(current); err != nil {
		return err
	}

	return b.applyBlacklist(current)
}

// Remove removes domains from the blacklist.
func (b *BlacklistManager) Remove(domains ...string) error {
	b.mgr.mu.Lock()
	defer b.mgr.mu.Unlock()

	current, _ := b.loadBlacklist()
	toRemove := make(map[string]bool)
	for _, d := range domains {
		toRemove[d] = true
	}

	var filtered []string
	for _, d := range current {
		if !toRemove[d] {
			filtered = append(filtered, d)
		}
	}

	if err := b.saveBlacklist(filtered); err != nil {
		return err
	}

	if len(filtered) == 0 {
		return b.removeBlacklistRule()
	}
	return b.applyBlacklist(filtered)
}

// BlockChina adds geosite:cn to the blacklist for one-click China domain blocking.
func (b *BlacklistManager) BlockChina() error {
	return b.Add("geosite:cn")
}

// UnblockChina removes geosite:cn from the blacklist.
func (b *BlacklistManager) UnblockChina() error {
	return b.Remove("geosite:cn")
}

// loadBlacklist loads the blacklist from the persistent file.
func (b *BlacklistManager) loadBlacklist() ([]string, error) {
	data, err := os.ReadFile(b.blacklistFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var domains []string
	if err := json.Unmarshal(data, &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// saveBlacklist persists the blacklist to file.
func (b *BlacklistManager) saveBlacklist(domains []string) error {
	return security.AtomicWriteJSON(b.blacklistFile, domains, 0644)
}

// applyBlacklist updates the routing rules with the current blacklist.
func (b *BlacklistManager) applyBlacklist(domains []string) error {
	rule := &RouteRule{
		Type:     "domain_blacklist",
		Domains:  domains,
		Outbound: "blocked",
	}

	// Remove existing blacklist rule first.
	_ = b.mgr.removeXrayRule("domain_blacklist")
	_ = b.mgr.removeSingboxRule("domain_blacklist")

	if err := b.mgr.addXrayRule(rule); err != nil {
		return fmt.Errorf("failed to apply xray blacklist: %w", err)
	}
	if err := b.mgr.addSingboxRule(rule); err != nil {
		return fmt.Errorf("failed to apply singbox blacklist: %w", err)
	}

	b.mgr.logger.Infof("blacklist updated: %d domains", len(domains))
	return nil
}

// removeBlacklistRule removes the blacklist routing rule.
func (b *BlacklistManager) removeBlacklistRule() error {
	_ = b.mgr.removeXrayRule("domain_blacklist")
	_ = b.mgr.removeSingboxRule("domain_blacklist")
	return nil
}
