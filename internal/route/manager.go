// Package route provides routing rule management for Xray-core and sing-box.
package route

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"vasmax/internal/security"

	"github.com/sirupsen/logrus"
)

// RouteRule represents a routing rule.
type RouteRule struct {
	Type     string   `json:"type"` // warp_ipv4/warp_ipv6/ipv6/socks5/dns/sni/block
	Domains  []string `json:"domains,omitempty"`
	IPs      []string `json:"ips,omitempty"`
	Outbound string   `json:"outbound"`
	Inbound  string   `json:"inbound,omitempty"`
	Protocol []string `json:"protocol,omitempty"`
}

// Manager manages routing rules for Xray-core and sing-box.
type Manager struct {
	xrayConfDir    string
	singboxConfDir string
	logger         *logrus.Logger
	mu             sync.Mutex
}

// NewManager creates a new route manager.
func NewManager(xrayConfDir, singboxConfDir string, logger *logrus.Logger) *Manager {
	return &Manager{
		xrayConfDir:    xrayConfDir,
		singboxConfDir: singboxConfDir,
		logger:         logger,
	}
}

// xrayRoutePath returns the path to the Xray routing config file.
func (m *Manager) xrayRoutePath() string {
	return filepath.Join(m.xrayConfDir, "04_routing.json")
}

// singboxRoutePath returns the path to the sing-box route config file.
func (m *Manager) singboxRoutePath() string {
	return filepath.Join(m.singboxConfDir, "route.json")
}

// AddRule adds a routing rule to both Xray and sing-box configs.
func (m *Manager) AddRule(rule *RouteRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.addXrayRule(rule); err != nil {
		return fmt.Errorf("failed to add xray route rule: %w", err)
	}
	if err := m.addSingboxRule(rule); err != nil {
		return fmt.Errorf("failed to add singbox route rule: %w", err)
	}

	m.logger.Infof("added route rule: type=%s outbound=%s", rule.Type, rule.Outbound)
	return nil
}

// RemoveRule removes a routing rule by type from both configs.
func (m *Manager) RemoveRule(ruleType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.removeXrayRule(ruleType); err != nil {
		return fmt.Errorf("failed to remove xray route rule: %w", err)
	}
	if err := m.removeSingboxRule(ruleType); err != nil {
		return fmt.Errorf("failed to remove singbox route rule: %w", err)
	}

	m.logger.Infof("removed route rule: type=%s", ruleType)
	return nil
}

// ListRules returns all custom routing rules.
func (m *Manager) ListRules() ([]RouteRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.loadCustomRules()
}

// customRulesPath returns the path to the custom rules file.
func (m *Manager) customRulesPath() string {
	return filepath.Join(m.xrayConfDir, "..", "custom_routes.json")
}

// loadCustomRules loads custom rules from the persistent file.
func (m *Manager) loadCustomRules() ([]RouteRule, error) {
	data, err := os.ReadFile(m.customRulesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rules []RouteRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// saveCustomRules persists custom rules to file.
func (m *Manager) saveCustomRules(rules []RouteRule) error {
	return security.AtomicWriteJSON(m.customRulesPath(), rules, 0644)
}

// addXrayRule adds a rule to the Xray routing config.
func (m *Manager) addXrayRule(rule *RouteRule) error {
	routePath := m.xrayRoutePath()
	cfg := m.loadXrayRouting(routePath)

	xrayRule := buildXrayRule(rule)
	cfg["rules"] = appendRule(cfg["rules"], xrayRule)

	if err := security.AtomicWriteJSON(routePath, map[string]interface{}{"routing": cfg}, 0644); err != nil {
		return err
	}

	// Persist to custom rules.
	rules, _ := m.loadCustomRules()
	rules = appendCustomRule(rules, rule)
	return m.saveCustomRules(rules)
}

// removeXrayRule removes a rule from the Xray routing config.
func (m *Manager) removeXrayRule(ruleType string) error {
	routePath := m.xrayRoutePath()
	cfg := m.loadXrayRouting(routePath)

	if rulesRaw, ok := cfg["rules"]; ok {
		if rules, ok := rulesRaw.([]interface{}); ok {
			var filtered []interface{}
			tag := "custom_" + ruleType
			for _, r := range rules {
				if rm, ok := r.(map[string]interface{}); ok {
					if rm["tag"] == tag {
						continue
					}
				}
				filtered = append(filtered, r)
			}
			cfg["rules"] = filtered
		}
	}

	if err := security.AtomicWriteJSON(routePath, map[string]interface{}{"routing": cfg}, 0644); err != nil {
		return err
	}

	rules, _ := m.loadCustomRules()
	rules = removeCustomRule(rules, ruleType)
	return m.saveCustomRules(rules)
}

// addSingboxRule adds a rule to the sing-box route config.
func (m *Manager) addSingboxRule(rule *RouteRule) error {
	routePath := m.singboxRoutePath()
	cfg := m.loadSingboxRoute(routePath)

	sbRule := buildSingboxRule(rule)
	if rulesRaw, ok := cfg["rules"]; ok {
		if rules, ok := rulesRaw.([]interface{}); ok {
			cfg["rules"] = append(rules, sbRule)
		}
	} else {
		cfg["rules"] = []interface{}{sbRule}
	}

	return security.AtomicWriteJSON(routePath, map[string]interface{}{"route": cfg}, 0644)
}

// removeSingboxRule removes a rule from the sing-box route config.
func (m *Manager) removeSingboxRule(ruleType string) error {
	routePath := m.singboxRoutePath()
	cfg := m.loadSingboxRoute(routePath)

	if rulesRaw, ok := cfg["rules"]; ok {
		if rules, ok := rulesRaw.([]interface{}); ok {
			var filtered []interface{}
			tag := "custom_" + ruleType
			for _, r := range rules {
				if rm, ok := r.(map[string]interface{}); ok {
					if rm["tag"] == tag {
						continue
					}
				}
				filtered = append(filtered, r)
			}
			cfg["rules"] = filtered
		}
	}

	return security.AtomicWriteJSON(routePath, map[string]interface{}{"route": cfg}, 0644)
}

// loadXrayRouting loads the Xray routing config, returning the routing object.
func (m *Manager) loadXrayRouting(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{"domainStrategy": "AsIs", "rules": []interface{}{}}
	}
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return map[string]interface{}{"domainStrategy": "AsIs", "rules": []interface{}{}}
	}
	if routing, ok := wrapper["routing"].(map[string]interface{}); ok {
		return routing
	}
	return map[string]interface{}{"domainStrategy": "AsIs", "rules": []interface{}{}}
}

// loadSingboxRoute loads the sing-box route config.
func (m *Manager) loadSingboxRoute(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{"rules": []interface{}{}}
	}
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return map[string]interface{}{"rules": []interface{}{}}
	}
	if route, ok := wrapper["route"].(map[string]interface{}); ok {
		return route
	}
	return map[string]interface{}{"rules": []interface{}{}}
}

// buildXrayRule converts a RouteRule to an Xray routing rule map.
func buildXrayRule(rule *RouteRule) map[string]interface{} {
	r := map[string]interface{}{
		"type":        "field",
		"outboundTag": rule.Outbound,
		"tag":         "custom_" + rule.Type,
	}
	if len(rule.Domains) > 0 {
		r["domain"] = rule.Domains
	}
	if len(rule.IPs) > 0 {
		r["ip"] = rule.IPs
	}
	if len(rule.Protocol) > 0 {
		r["protocol"] = rule.Protocol
	}
	if rule.Inbound != "" {
		r["inboundTag"] = []string{rule.Inbound}
	}
	return r
}

// buildSingboxRule converts a RouteRule to a sing-box route rule map.
func buildSingboxRule(rule *RouteRule) map[string]interface{} {
	r := map[string]interface{}{
		"outbound": rule.Outbound,
		"tag":      "custom_" + rule.Type,
	}
	if len(rule.Domains) > 0 {
		r["domain"] = rule.Domains
	}
	if len(rule.IPs) > 0 {
		r["ip_cidr"] = rule.IPs
	}
	if len(rule.Protocol) > 0 {
		r["protocol"] = rule.Protocol
	}
	if rule.Inbound != "" {
		r["inbound"] = []string{rule.Inbound}
	}
	return r
}

// appendRule appends a rule to the rules list.
func appendRule(rulesRaw interface{}, rule map[string]interface{}) []interface{} {
	if rules, ok := rulesRaw.([]interface{}); ok {
		return append(rules, rule)
	}
	return []interface{}{rule}
}

// appendCustomRule adds a rule to the custom rules list, replacing if same type exists.
func appendCustomRule(rules []RouteRule, rule *RouteRule) []RouteRule {
	var result []RouteRule
	for _, r := range rules {
		if r.Type != rule.Type {
			result = append(result, r)
		}
	}
	return append(result, *rule)
}

// removeCustomRule removes a rule by type from the custom rules list.
func removeCustomRule(rules []RouteRule, ruleType string) []RouteRule {
	var result []RouteRule
	for _, r := range rules {
		if r.Type != ruleType {
			result = append(result, r)
		}
	}
	return result
}
