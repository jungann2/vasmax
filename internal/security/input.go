// Package security provides input validation and security utilities.
package security

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// Length limits to prevent buffer overflow attacks.
const (
	MaxDomainLength = 253
	MaxUUIDLength   = 36
	MaxURLLength    = 2048
	MaxPathLength   = 4096
	MaxInputLength  = 8192
)

var (
	// domainLabelRegex matches a single DNS label per RFC 1035:
	// starts and ends with alphanumeric, may contain hyphens in the middle, 1-63 chars.
	domainLabelRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

	// uuidRegex matches RFC 4122 UUID format: 8-4-4-4-12 hex digits.
	uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// ValidateDomain validates a domain name per RFC 1035.
// Allowed characters: letters, digits, hyphens, and dots. Max 253 characters.
func ValidateDomain(domain string) error {
	if len(domain) == 0 {
		return errors.New("domain must not be empty")
	}
	if len(domain) > MaxDomainLength {
		return fmt.Errorf("domain length %d exceeds maximum %d", len(domain), MaxDomainLength)
	}

	// Remove trailing dot (FQDN format is valid).
	d := strings.TrimSuffix(domain, ".")
	if len(d) == 0 {
		return errors.New("domain must not be empty")
	}

	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return errors.New("domain must have at least two labels")
	}

	for _, label := range labels {
		if len(label) == 0 {
			return errors.New("domain contains empty label")
		}
		if len(label) > 63 {
			return fmt.Errorf("domain label %q exceeds 63 characters", label)
		}
		if !domainLabelRegex.MatchString(label) {
			return fmt.Errorf("domain label %q contains invalid characters", label)
		}
	}

	return nil
}

// ValidatePort validates that a port number is in the range 1-65535.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d is out of valid range 1-65535", port)
	}
	return nil
}

// ValidateUUID validates a UUID string per RFC 4122 format (8-4-4-4-12 hex).
func ValidateUUID(uuid string) error {
	if len(uuid) == 0 {
		return errors.New("uuid must not be empty")
	}
	if len(uuid) != MaxUUIDLength {
		return fmt.Errorf("uuid length %d does not match expected %d", len(uuid), MaxUUIDLength)
	}
	if !uuidRegex.MatchString(uuid) {
		return errors.New("uuid format is invalid, expected RFC 4122 format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)")
	}
	return nil
}

// ValidatePath validates a file path against path traversal attacks.
// It rejects any path containing ".." segments and verifies the path
// is within one of the allowed directories.
func ValidatePath(path string, allowedDirs []string) error {
	if len(path) == 0 {
		return errors.New("path must not be empty")
	}
	if len(path) > MaxPathLength {
		return fmt.Errorf("path length %d exceeds maximum %d", len(path), MaxPathLength)
	}

	// Reject path traversal: check for ".." in any segment.
	cleaned := filepath.Clean(path)
	for _, segment := range strings.Split(cleaned, string(filepath.Separator)) {
		if segment == ".." {
			return errors.New("path contains path traversal sequence (..)")
		}
	}
	// Also reject raw ".." in the original path for extra safety.
	if strings.Contains(path, "..") {
		return errors.New("path contains path traversal sequence (..)")
	}

	// Verify path is within one of the allowed directories.
	if len(allowedDirs) > 0 {
		absPath, err := filepath.Abs(cleaned)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		allowed := false
		for _, dir := range allowedDirs {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			// Ensure the directory path ends with separator for prefix matching.
			if !strings.HasSuffix(absDir, string(filepath.Separator)) {
				absDir += string(filepath.Separator)
			}
			if strings.HasPrefix(absPath, absDir) || absPath == strings.TrimSuffix(absDir, string(filepath.Separator)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path %q is not within any allowed directory", path)
		}
	}

	return nil
}

// ValidateURL validates a URL string. It must use the HTTPS scheme.
func ValidateURL(rawURL string) error {
	if len(rawURL) == 0 {
		return errors.New("url must not be empty")
	}
	if len(rawURL) > MaxURLLength {
		return fmt.Errorf("url length %d exceeds maximum %d", len(rawURL), MaxURLLength)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("url format is invalid: %w", err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("url scheme must be https, got %q", parsed.Scheme)
	}

	if parsed.Host == "" {
		return errors.New("url must have a host")
	}

	return nil
}
