package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AtomicWrite writes data to a file atomically using the pattern:
// create temp file in same dir → write data → fsync → close → chmod → rename.
// The temp file is created in the same directory as the target to ensure
// os.Rename is atomic (same filesystem).
// On any error, the temp file is cleaned up.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, ".tmp-atomic-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := f.Name()

	// Ensure cleanup on any failure path.
	success := false
	defer func() {
		if !success {
			_ = f.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to fsync temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	success = true
	return nil
}

// AtomicWriteJSON marshals v to JSON, validates the output, then atomically
// writes it to path. If v cannot be marshaled to valid JSON, an error is
// returned and no file is written.
func AtomicWriteJSON(path string, v interface{}, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Validate the marshaled JSON is syntactically correct.
	if !json.Valid(data) {
		return fmt.Errorf("marshaled data is not valid JSON")
	}

	return AtomicWrite(path, data, perm)
}

// AtomicWriteYAML marshals v to YAML, validates the output by round-tripping
// through unmarshal, then atomically writes it to path.
func AtomicWriteYAML(path string, v interface{}, perm os.FileMode) (retErr error) {
	// yaml.Marshal may panic on unsupported types (e.g. channels).
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("failed to marshal YAML: %v", r)
		}
	}()

	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Validate YAML by attempting to unmarshal it back.
	var check interface{}
	if err := yaml.Unmarshal(data, &check); err != nil {
		return fmt.Errorf("marshaled data is not valid YAML: %w", err)
	}

	return AtomicWrite(path, data, perm)
}
