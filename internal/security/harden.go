package security

import (
	"fmt"
	"os"
	"path/filepath"
)

// SensitiveFilePerm is the permission for sensitive files (config, tokens, keys).
const SensitiveFilePerm os.FileMode = 0600

// SensitivePaths lists paths that should have restricted permissions.
var SensitivePaths = []string{
	"/etc/vasmax/config.yaml",
	"/etc/vasmax/tls/",
	"/var/log/vasmax/audit.log",
}

// HardenFilePermissions ensures all sensitive files have 0600 permissions.
func HardenFilePermissions(paths []string) error {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue // Skip non-existent files.
		}
		if info.IsDir() {
			// For directories, ensure 0700.
			if err := os.Chmod(p, 0700); err != nil {
				return fmt.Errorf("failed to chmod %s: %w", p, err)
			}
			continue
		}
		if err := os.Chmod(p, SensitiveFilePerm); err != nil {
			return fmt.Errorf("failed to chmod %s: %w", p, err)
		}
	}
	return nil
}

// EnsureDirectoryPermissions ensures a directory and its sensitive files
// have appropriate permissions.
func EnsureDirectoryPermissions(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths.
		}
		if info.IsDir() {
			return nil
		}
		// Sensitive file extensions.
		ext := filepath.Ext(path)
		switch ext {
		case ".yaml", ".yml", ".json", ".key", ".pem":
			return os.Chmod(path, SensitiveFilePerm)
		}
		return nil
	})
}

// SafeOrDefault returns the value if non-nil, otherwise the default.
// Used for JSON null handling with pointer types.
func SafeIntOrDefault(ptr *int, defaultVal int) int {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

// SafeStringOrDefault returns the string value if non-empty, otherwise the default.
func SafeStringOrDefault(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}
