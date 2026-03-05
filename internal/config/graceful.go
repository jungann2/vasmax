package config

import (
	"encoding/json"
	"fmt"
	"os"

	"vasmax/internal/security"
)

// BackupConfig creates a .bak backup of a config file before modification.
// Only keeps the most recent backup.
func BackupConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to backup.
		}
		return fmt.Errorf("failed to read config for backup: %w", err)
	}

	bakPath := path + ".bak"
	return security.AtomicWrite(bakPath, data, 0600)
}

// RestoreConfig restores a config file from its .bak backup.
// Validates JSON syntax before restoring.
func RestoreConfig(path string) error {
	bakPath := path + ".bak"
	data, err := os.ReadFile(bakPath)
	if err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Validate JSON syntax if it's a JSON file.
	if len(data) > 0 && (data[0] == '{' || data[0] == '[') {
		if !json.Valid(data) {
			return fmt.Errorf("backup file contains invalid JSON")
		}
	}

	return security.AtomicWrite(path, data, 0644)
}

// SafeWriteConfig writes a config file with automatic backup.
// Creates a .bak backup before writing, and restores on failure.
func SafeWriteConfig(path string, data []byte, perm os.FileMode) error {
	// Backup existing file.
	if err := BackupConfig(path); err != nil {
		// Non-fatal: continue with write.
		fmt.Fprintf(os.Stderr, "warning: backup failed: %v\n", err)
	}

	if err := security.AtomicWrite(path, data, perm); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// SafeWriteJSON writes a JSON config file with automatic backup.
func SafeWriteJSON(path string, v interface{}, perm os.FileMode) error {
	if err := BackupConfig(path); err != nil {
		fmt.Fprintf(os.Stderr, "warning: backup failed: %v\n", err)
	}

	return security.AtomicWriteJSON(path, v, perm)
}
