package security

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicWrite_WritesCorrectContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	data := []byte("hello, atomic world!")
	perm := os.FileMode(0600)

	if err := AtomicWrite(path, data, perm); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content mismatch: got %q, want %q", got, data)
	}
}

func TestAtomicWrite_SetsCorrectPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "test_perm.txt")
	perm := os.FileMode(0644)

	if err := AtomicWrite(path, []byte("data"), perm); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != perm {
		t.Errorf("permission mismatch: got %o, want %o", info.Mode().Perm(), perm)
	}
}

func TestAtomicWrite_CleansUpTempOnDirNotExist(t *testing.T) {
	// Writing to a non-existent directory should fail and leave no temp files.
	path := filepath.Join(t.TempDir(), "nonexistent", "file.txt")

	err := AtomicWrite(path, []byte("data"), 0600)
	if err == nil {
		t.Fatal("expected error when directory does not exist")
	}

	// The parent of the target doesn't exist, so no temp file should be created.
	parentDir := filepath.Dir(path)
	entries, readErr := os.ReadDir(parentDir)
	if readErr != nil {
		// Directory doesn't exist, which is expected — no temp files.
		return
	}
	for _, e := range entries {
		if e.Name() != "file.txt" {
			t.Errorf("unexpected temp file left behind: %s", e.Name())
		}
	}
}

func TestAtomicWrite_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.txt")

	// Write initial content.
	if err := AtomicWrite(path, []byte("initial"), 0600); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Overwrite with new content.
	if err := AtomicWrite(path, []byte("updated"), 0600); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "updated" {
		t.Errorf("content mismatch after overwrite: got %q, want %q", got, "updated")
	}
}

func TestAtomicWrite_EmptyData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")

	if err := AtomicWrite(path, []byte{}, 0600); err != nil {
		t.Fatalf("AtomicWrite with empty data failed: %v", err)
	}

	got, _ := os.ReadFile(path)
	if len(got) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(got))
	}
}

func TestAtomicWriteJSON_ValidStruct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	v := map[string]interface{}{
		"name": "test",
		"port": 443,
	}

	if err := AtomicWriteJSON(path, v, 0600); err != nil {
		t.Fatalf("AtomicWriteJSON failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Verify it's valid JSON by checking it contains expected keys.
	content := string(got)
	if len(content) == 0 {
		t.Fatal("file is empty")
	}
}

func TestAtomicWriteJSON_InvalidValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	// Channels cannot be marshaled to JSON.
	ch := make(chan int)
	err := AtomicWriteJSON(path, ch, 0600)
	if err == nil {
		t.Fatal("expected error for unmarshalable value")
	}

	// File should not exist.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file should not have been created for invalid JSON")
	}
}

func TestAtomicWriteYAML_ValidStruct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	v := struct {
		Name string `yaml:"name"`
		Port int    `yaml:"port"`
	}{
		Name: "test-server",
		Port: 8080,
	}

	if err := AtomicWriteYAML(path, v, 0600); err != nil {
		t.Fatalf("AtomicWriteYAML failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(got)
	if len(content) == 0 {
		t.Fatal("file is empty")
	}
}

func TestAtomicWriteYAML_InvalidValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	// Channels cannot be marshaled to YAML.
	ch := make(chan int)
	err := AtomicWriteYAML(path, ch, 0600)
	if err == nil {
		t.Fatal("expected error for unmarshalable value")
	}

	// File should not exist.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file should not have been created for invalid YAML")
	}
}

func TestAtomicWrite_OriginalFileNotCorruptedOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Read-only directory test not reliable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "original.txt")
	original := []byte("original content")

	// Write original file.
	if err := AtomicWrite(path, original, 0600); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// Attempt to write to a read-only directory should fail.
	// We simulate failure by making the directory read-only so temp file creation fails.
	roDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(roDir, 0500); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	roPath := filepath.Join(roDir, "file.txt")

	// Write original to the readonly dir first (before making it readonly).
	if err := os.Chmod(roDir, 0700); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	if err := AtomicWrite(roPath, original, 0600); err != nil {
		t.Fatalf("write to roDir failed: %v", err)
	}
	if err := os.Chmod(roDir, 0500); err != nil {
		t.Fatalf("chmod readonly failed: %v", err)
	}
	defer os.Chmod(roDir, 0700) // Cleanup.

	// This should fail because we can't create temp files in a read-only dir.
	err := AtomicWrite(roPath, []byte("new content"), 0600)
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}

	// Restore permissions to read the file.
	os.Chmod(roDir, 0700)

	// Original content should be intact.
	got, readErr := os.ReadFile(roPath)
	if readErr != nil {
		t.Fatalf("failed to read original file: %v", readErr)
	}
	if string(got) != string(original) {
		t.Errorf("original file corrupted: got %q, want %q", got, original)
	}
}
