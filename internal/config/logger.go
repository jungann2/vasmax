package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// DefaultLogFilePath is the default log file location.
const DefaultLogFilePath = "/var/log/vasmax/VasmaX.log"

// InitLogger creates and configures a logrus.Logger based on the provided
// LogConfig. It parses the log level from cfg.Level, sets a text formatter
// with timestamps, and configures output to stdout (and optionally a log file
// when cfg.FilePath is set).
//
// The returned cleanup function closes the log file (if opened). Callers
// should defer it after a successful call.
func InitLogger(cfg *LogConfig) (*logrus.Logger, func(), error) {
	logger := logrus.New()

	// Parse log level.
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}
	logger.SetLevel(level)

	// Use text formatter with timestamps.
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	cleanup := func() {}

	// Configure output writers.
	if cfg.FilePath != "" {
		// Ensure the directory exists.
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory %s: %w", dir, err)
		}

		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file %s: %w", cfg.FilePath, err)
		}

		// Write to both stdout and the log file.
		logger.SetOutput(io.MultiWriter(os.Stdout, file))
		cleanup = func() { file.Close() }
	} else {
		logger.SetOutput(os.Stdout)
	}

	return logger, cleanup, nil
}
