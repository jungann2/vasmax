package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestInitLogger_ValidLevel(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}
	for _, lvl := range levels {
		cfg := &LogConfig{Level: lvl}
		logger, cleanup, err := InitLogger(cfg)
		if err != nil {
			t.Errorf("InitLogger(%q) returned error: %v", lvl, err)
			continue
		}
		defer cleanup()

		expected, _ := logrus.ParseLevel(lvl)
		if logger.GetLevel() != expected {
			t.Errorf("expected level %v, got %v", expected, logger.GetLevel())
		}
	}
}

func TestInitLogger_InvalidLevel(t *testing.T) {
	cfg := &LogConfig{Level: "invalid-level"}
	_, _, err := InitLogger(cfg)
	if err == nil {
		t.Error("expected error for invalid log level, got nil")
	}
}

func TestInitLogger_WithFileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "test.log")

	cfg := &LogConfig{
		Level:    "info",
		FilePath: logPath,
	}

	logger, cleanup, err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger with file output failed: %v", err)
	}

	// Write a log entry.
	logger.Info("test log message")

	// Close the file handle before reading / cleanup.
	cleanup()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected log file to contain data, got empty")
	}

	// Verify the log directory was created.
	subdir := filepath.Join(dir, "subdir")
	if _, err := os.Stat(subdir); os.IsNotExist(err) {
		t.Error("expected log subdirectory to be created")
	}
}

func TestInitLogger_StdoutOnly(t *testing.T) {
	cfg := &LogConfig{
		Level: "debug",
	}

	logger, cleanup, err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger stdout-only failed: %v", err)
	}
	defer cleanup()

	if logger.GetLevel() != logrus.DebugLevel {
		t.Errorf("expected debug level, got %v", logger.GetLevel())
	}
}

func TestInitLogger_FormatterHasTimestamp(t *testing.T) {
	cfg := &LogConfig{Level: "info"}
	logger, cleanup, err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("InitLogger failed: %v", err)
	}
	defer cleanup()

	formatter, ok := logger.Formatter.(*logrus.TextFormatter)
	if !ok {
		t.Fatal("expected TextFormatter")
	}
	if !formatter.FullTimestamp {
		t.Error("expected FullTimestamp to be true")
	}
}
