// Package nginx provides Nginx configuration management.
package nginx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"vasmax/internal/security"

	"github.com/sirupsen/logrus"
)

// Default paths for Nginx configuration.
const (
	DefaultConfDir  = "/etc/nginx/conf.d/"
	DefaultHTMLDir  = "/usr/share/nginx/html"
	AllowedNginxDir = "/etc/nginx/"
)

// ProtocolLocation describes a protocol's Nginx location block.
type ProtocolLocation struct {
	Type        string // ws/grpc/httpupgrade
	Path        string
	BackendPort int
}

// NginxParams holds parameters for generating Nginx configuration.
type NginxParams struct {
	Domain    string
	CertFile  string
	KeyFile   string
	Protocols []ProtocolLocation
}

// Manager manages Nginx configuration files.
type Manager struct {
	confDir string
	logger  *logrus.Logger
	mu      sync.Mutex
}

// NewManager creates a new Nginx configuration manager.
func NewManager(confDir string, logger *logrus.Logger) *Manager {
	if confDir == "" {
		confDir = DefaultConfDir
	}
	return &Manager{confDir: confDir, logger: logger}
}

// validateNginxPath ensures the path is within the allowed Nginx directory.
func (m *Manager) validateNginxPath(path string) error {
	return security.ValidatePath(path, []string{AllowedNginxDir})
}

// GenerateConfig generates the main Nginx server configuration based on installed protocols.
func (m *Manager) GenerateConfig(params *NginxParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := security.ValidateDomain(params.Domain); err != nil {
		return fmt.Errorf("invalid domain: %w", err)
	}

	confPath := filepath.Join(m.confDir, params.Domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	conf := generateServerBlock(params)

	if err := security.AtomicWrite(confPath, []byte(conf), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Infof("nginx config generated: %s", confPath)
	return nil
}

// AddLocation adds a location block for a protocol to the domain config.
func (m *Manager) AddLocation(domain, protocol, path string, backendPort int) error {
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

	locationBlock := generateLocationBlock(protocol, path, backendPort)
	marker := "# --- END LOCATIONS ---"
	content := string(data)

	if !strings.Contains(content, marker) {
		return fmt.Errorf("nginx config missing location marker")
	}

	content = strings.Replace(content, marker, locationBlock+"\n"+marker, 1)

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Infof("added location for protocol %s path %s", protocol, path)
	return nil
}

// RemoveLocation removes a location block for a protocol from the domain config.
func (m *Manager) RemoveLocation(domain, protocol string) error {
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

	startMarker := fmt.Sprintf("# --- BEGIN %s ---", protocol)
	endMarker := fmt.Sprintf("# --- END %s ---", protocol)

	content := string(data)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	if startIdx == -1 || endIdx == -1 {
		m.logger.Warnf("location block for protocol %s not found", protocol)
		return nil
	}

	content = content[:startIdx] + content[endIdx+len(endMarker)+1:]

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Infof("removed location for protocol %s", protocol)
	return nil
}

// SetupSubscribeServer configures the subscription server location.
func (m *Manager) SetupSubscribeServer(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := security.ValidateDomain(domain); err != nil {
		return fmt.Errorf("invalid domain: %w", err)
	}

	confPath := filepath.Join(m.confDir, domain+".conf")
	if err := m.validateNginxPath(confPath); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read nginx config: %w", err)
	}

	subscribeBlock := generateSubscribeLocation()
	marker := "# --- END LOCATIONS ---"
	content := string(data)

	if !strings.Contains(content, marker) {
		return fmt.Errorf("nginx config missing location marker")
	}

	// Remove existing subscribe block if present.
	content = removeBlock(content, "SUBSCRIBE")
	content = strings.Replace(content, marker, subscribeBlock+"\n"+marker, 1)

	if err := security.AtomicWrite(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	m.logger.Info("subscribe server location configured")
	return nil
}

// Validate runs nginx -t to validate the configuration.
func (m *Manager) Validate() error {
	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx config validation failed: %s: %w", string(output), err)
	}
	return nil
}

// Reload validates and then reloads Nginx.
func (m *Manager) Reload() error {
	if err := m.Validate(); err != nil {
		return err
	}
	cmd := exec.Command("nginx", "-s", "reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx reload failed: %s: %w", string(output), err)
	}
	m.logger.Info("nginx reloaded successfully")
	return nil
}

// removeBlock removes a named block delimited by BEGIN/END markers.
func removeBlock(content, name string) string {
	startMarker := fmt.Sprintf("# --- BEGIN %s ---", name)
	endMarker := fmt.Sprintf("# --- END %s ---", name)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)
	if startIdx == -1 || endIdx == -1 {
		return content
	}
	return content[:startIdx] + content[endIdx+len(endMarker)+1:]
}
