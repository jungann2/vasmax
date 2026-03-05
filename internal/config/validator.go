package config

import (
	"fmt"
	"strings"

	"vasmax/internal/security"
)

// validLogLevels defines the allowed log level values.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Validate checks the Config for completeness and correctness.
// In standalone mode only local fields are validated.
// In managed mode api_host, api_token, and node_id are additionally required.
func (c *Config) Validate() error {
	var errs []string

	// 1. Always validate log level.
	if !validLogLevels[c.Log.Level] {
		errs = append(errs, fmt.Sprintf("log.level: must be one of debug/info/warn/error, got %q", c.Log.Level))
	}

	// 2. Managed mode: validate xboard API fields.
	if !c.Standalone {
		if c.APIHost == "" {
			errs = append(errs, "api_host: must not be empty in managed mode")
		} else if err := security.ValidateURL(c.APIHost); err != nil {
			errs = append(errs, fmt.Sprintf("api_host: %v", err))
		}

		if c.APIToken == "" {
			errs = append(errs, "api_token: must not be empty in managed mode")
		}

		if c.NodeID <= 0 {
			errs = append(errs, fmt.Sprintf("node_id: must be > 0 in managed mode, got %d", c.NodeID))
		}
	}

	// 3. Validate TLS domain if set.
	if c.TLS.Domain != "" {
		if err := security.ValidateDomain(c.TLS.Domain); err != nil {
			errs = append(errs, fmt.Sprintf("tls.domain: %v", err))
		}
	}

	// 4. Validate Hysteria2 port if set.
	if c.Hysteria2.Port != 0 {
		if err := security.ValidatePort(c.Hysteria2.Port); err != nil {
			errs = append(errs, fmt.Sprintf("hysteria2.port: %v", err))
		}
	}

	// 5. Validate Tuic port if set.
	if c.Tuic.Port != 0 {
		if err := security.ValidatePort(c.Tuic.Port); err != nil {
			errs = append(errs, fmt.Sprintf("tuic.port: %v", err))
		}
	}

	// 6. Validate CDN address if CDN is enabled.
	if c.CDN.Enabled && c.CDN.Address == "" {
		errs = append(errs, "cdn.address: must not be empty when cdn is enabled")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}
