package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// DownloadTask 下载任务
type DownloadTask struct {
	URL       string // 下载地址
	DestPath  string // 目标路径
	SHA256URL string // SHA256 校验文件 URL（可选）
	Name      string // 显示名称
}

// DownloadAll 并发下载多个文件，最大并发数 4
func DownloadAll(ctx context.Context, tasks []DownloadTask, progress func(name string, pct int)) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)

	for _, task := range tasks {
		t := task
		g.Go(func() error {
			err := DownloadOne(ctx, t)
			if err == nil && progress != nil {
				progress(t.Name, 100)
			}
			return err
		})
	}

	return g.Wait()
}

// DownloadOne 单文件下载 + SHA256 校验
func DownloadOne(ctx context.Context, task DownloadTask) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(task.DestPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建临时文件（同目录，确保 rename 原子性）
	tmpFile, err := os.CreateTemp(filepath.Dir(task.DestPath), ".download-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath) // 失败时清理
	}()

	// 下载文件
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.URL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 边下载边计算 SHA256
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	if _, err := io.Copy(writer, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	tmpFile.Close()

	// SHA256 校验
	if task.SHA256URL != "" {
		expectedHash, err := fetchExpectedHash(ctx, task.SHA256URL, filepath.Base(task.DestPath))
		if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("获取 SHA256 校验值失败: %w", err)
		}
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualHash, expectedHash) {
			os.Remove(tmpPath)
			return fmt.Errorf("SHA256 校验失败: 期望 %s, 实际 %s", expectedHash, actualHash)
		}
	}

	// 原子移动到目标路径
	if err := os.Rename(tmpPath, task.DestPath); err != nil {
		return fmt.Errorf("移动文件失败: %w", err)
	}

	return nil
}

// fetchExpectedHash 从 SHA256 校验文件获取期望的哈希值
func fetchExpectedHash(ctx context.Context, sha256URL, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sha256URL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析 SHA256SUMS 格式: "hash  filename" 或纯 hash
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 && strings.Contains(parts[1], filename) {
			return parts[0], nil
		}
		if len(parts) == 1 && len(parts[0]) == 64 {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("未找到 %s 的校验值", filename)
}
