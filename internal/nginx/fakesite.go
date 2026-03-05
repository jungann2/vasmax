package nginx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// PresetFakeSites provides a list of preset static blog template URLs.
var PresetFakeSites = []string{
	"https://github.com/AmazingRise/hugo-theme-flavor/archive/refs/heads/master.zip",
	"https://github.com/suspended/suspended.github.io/archive/refs/heads/master.zip",
}

// DeployFakeSite downloads a template from templateURL and deploys it to the
// Nginx HTML root directory. If the download fails, the current site is preserved.
func (m *Manager) DeployFakeSite(templateURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Download to temp file.
	tmpFile, err := os.CreateTemp("", "fakesite-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, templateURL, nil)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download template: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	n, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to save template: %w", err)
	}
	tmpFile.Close()

	if n == 0 {
		return fmt.Errorf("downloaded file is empty")
	}

	// Deploy: extract to HTML dir.
	htmlDir := DefaultHTMLDir
	backupDir := htmlDir + ".bak"

	// Backup current site.
	if _, err := os.Stat(htmlDir); err == nil {
		_ = os.RemoveAll(backupDir)
		if err := os.Rename(htmlDir, backupDir); err != nil {
			return fmt.Errorf("failed to backup current site: %w", err)
		}
	}

	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		// Restore backup on failure.
		_ = os.Rename(backupDir, htmlDir)
		return fmt.Errorf("failed to create html dir: %w", err)
	}

	// Extract zip using unzip command with argument array.
	extractDir, err := os.MkdirTemp("", "fakesite-extract-*")
	if err != nil {
		_ = os.RemoveAll(htmlDir)
		_ = os.Rename(backupDir, htmlDir)
		return fmt.Errorf("failed to create extract dir: %w", err)
	}
	defer os.RemoveAll(extractDir)

	cmd := exec.Command("unzip", "-o", "-q", tmpPath, "-d", extractDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(htmlDir)
		_ = os.Rename(backupDir, htmlDir)
		return fmt.Errorf("failed to extract template: %s: %w", string(output), err)
	}

	// Find the extracted directory (usually has one subdirectory).
	if err := copyExtractedFiles(extractDir, htmlDir); err != nil {
		_ = os.RemoveAll(htmlDir)
		_ = os.Rename(backupDir, htmlDir)
		return fmt.Errorf("failed to copy extracted files: %w", err)
	}

	// Clean up backup on success.
	_ = os.RemoveAll(backupDir)

	m.logger.Infof("fake site deployed from %s", templateURL)
	return nil
}

// copyExtractedFiles copies files from the extracted directory to the target.
// If the extracted dir contains a single subdirectory, its contents are used.
func copyExtractedFiles(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	// If single subdirectory, use its contents.
	effectiveSrc := srcDir
	if len(entries) == 1 && entries[0].IsDir() {
		effectiveSrc = filepath.Join(srcDir, entries[0].Name())
	}

	// Use cp -a for recursive copy.
	cmd := exec.Command("cp", "-a", effectiveSrc+"/.", dstDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp failed: %s: %w", string(output), err)
	}
	return nil
}
