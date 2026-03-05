package nginx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"vasmax/internal/security"
)

// AddRedirect adds a 302 redirect rule to the Nginx config.
// sourcePath must start with "/" and contain no "..".
// targetURL must start with "http://" or "https://".
func (m *Manager) AddRedirect(domain, sourcePath, targetURL string) error {
	if err := validateRedirectInput(sourcePath, targetURL); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	confPath := filepath.Join(m.confDir, domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read nginx config: %w", err)
	}

	tag := redirectTag(sourcePath)
	block := generateRedirectBlock(tag, sourcePath, targetURL)
	marker := "# --- END LOCATIONS ---"
	content := string(data)

	if !strings.Contains(content, marker) {
		return fmt.Errorf("nginx config missing location marker")
	}

	// Remove existing redirect for this path if present.
	content = removeBlock(content, tag)
	content = strings.Replace(content, marker, block+"\n"+marker, 1)

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Validate and reload.
	if err := m.Validate(); err != nil {
		// Restore original config on validation failure.
		_ = security.AtomicWrite(confPath, data, 0644)
		return fmt.Errorf("redirect config invalid, reverted: %w", err)
	}

	if err := m.Reload(); err != nil {
		return err
	}

	m.logger.Infof("added redirect: %s -> %s", sourcePath, targetURL)
	return nil
}

// RemoveRedirect removes a redirect rule from the Nginx config.
func (m *Manager) RemoveRedirect(domain, sourcePath string) error {
	if !strings.HasPrefix(sourcePath, "/") {
		return fmt.Errorf("source path must start with /")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	confPath := filepath.Join(m.confDir, domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read nginx config: %w", err)
	}

	tag := redirectTag(sourcePath)
	content := removeBlock(string(data), tag)

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	if err := m.Validate(); err != nil {
		_ = security.AtomicWrite(confPath, data, 0644)
		return fmt.Errorf("config invalid after removing redirect, reverted: %w", err)
	}

	if err := m.Reload(); err != nil {
		return err
	}

	m.logger.Infof("removed redirect: %s", sourcePath)
	return nil
}

// validateRedirectInput validates redirect source path and target URL.
func validateRedirectInput(sourcePath, targetURL string) error {
	if !strings.HasPrefix(sourcePath, "/") {
		return fmt.Errorf("source path must start with /")
	}
	if strings.Contains(sourcePath, "..") {
		return fmt.Errorf("source path must not contain ..")
	}
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		return fmt.Errorf("target URL must start with http:// or https://")
	}
	return nil
}

// redirectTag generates a unique tag for a redirect block.
func redirectTag(sourcePath string) string {
	safe := strings.ReplaceAll(sourcePath, "/", "_")
	return "REDIRECT" + safe
}

// generateRedirectBlock generates a redirect location block.
func generateRedirectBlock(tag, sourcePath, targetURL string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("    # --- BEGIN %s ---\n", tag))
	b.WriteString(fmt.Sprintf("    location = %s {\n", sourcePath))
	b.WriteString(fmt.Sprintf("        return 302 %s;\n", targetURL))
	b.WriteString("    }\n")
	b.WriteString(fmt.Sprintf("    # --- END %s ---\n", tag))
	return b.String()
}
