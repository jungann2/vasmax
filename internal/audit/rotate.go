package audit

import (
	"fmt"
	"os"
)

// rotate performs log rotation:
// audit.log.3 → deleted
// audit.log.2 → audit.log.3
// audit.log.1 → audit.log.2
// audit.log   → audit.log.1
// Then creates a new audit.log.
func (l *Logger) rotate() error {
	// Close current file.
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file for rotation: %w", err)
	}

	// Rotate history files.
	for i := l.maxFiles; i >= 1; i-- {
		src := l.rotatedPath(i - 1)
		dst := l.rotatedPath(i)

		if i == l.maxFiles {
			// Delete the oldest file.
			_ = os.Remove(dst)
		}

		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}

	// Rename current log to .1
	_ = os.Rename(l.filePath, l.rotatedPath(1))

	// Open new log file.
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to create new log file after rotation: %w", err)
	}
	l.file = f

	return nil
}

// rotatedPath returns the path for a rotated log file.
// Index 0 means the current file, 1+ means history files.
func (l *Logger) rotatedPath(index int) string {
	if index == 0 {
		return l.filePath
	}
	return fmt.Sprintf("%s.%d", l.filePath, index)
}
