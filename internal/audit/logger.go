// Package audit provides audit logging with JSON Lines format and log rotation.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Default audit log settings.
const (
	DefaultFilePath = "/var/log/vasmax/audit.log"
	DefaultMaxSize  = 50 * 1024 * 1024 // 50MB
	DefaultMaxFiles = 3
)

// AuditEntry represents a single audit log entry in JSON Lines format.
type AuditEntry struct {
	Timestamp string `json:"timestamp"` // ISO 8601
	Action    string `json:"action"`    // user_add/user_remove/protocol_install/...
	Details   string `json:"details"`
	Result    string `json:"result"` // success/failure
	Source    string `json:"source"` // cli/syncloop/auto
}

// Logger is the audit log recorder with rotation support.
type Logger struct {
	file     *os.File
	mu       sync.Mutex
	maxSize  int64
	maxFiles int
	filePath string
}

// NewLogger creates a new audit logger.
func NewLogger(filePath string, maxSize int64, maxFiles int) (*Logger, error) {
	if filePath == "" {
		filePath = DefaultFilePath
	}
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}

	// Ensure directory exists.
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &Logger{
		file:     f,
		maxSize:  maxSize,
		maxFiles: maxFiles,
		filePath: filePath,
	}, nil
}

// Log writes an audit entry to the log file.
func (l *Logger) Log(entry *AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Check if rotation is needed.
	if err := l.checkRotate(); err != nil {
		// Log rotation failure is non-fatal; continue writing.
		fmt.Fprintf(os.Stderr, "audit log rotation failed: %v\n", err)
	}

	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	return l.file.Sync()
}

// Close closes the audit log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// checkRotate checks if the log file exceeds maxSize and rotates if needed.
func (l *Logger) checkRotate() error {
	info, err := l.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < l.maxSize {
		return nil
	}
	return l.rotate()
}
