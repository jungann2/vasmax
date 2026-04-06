package subscription

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vasmax/internal/security"
)

// RemoteSubscription 远程订阅配置
type RemoteSubscription struct {
	URL   string // 完整订阅 URL（default 格式，Base64 URI 列表）
	Alias string // 节点别名后缀
}

const (
	// RemoteSubFilePath 远程订阅 URL 列表持久化路径
	RemoteSubFilePath = "subscribe_remote/remoteSubscribeUrl"
)

// ParseRemoteSubscription 解析远程订阅输入（格式：URL:别名）
// URL 必须以 http:// 或 https:// 开头，别名不能为空
// 示例: https://hk.example.com/s/abc123/default:香港
func ParseRemoteSubscription(input string) (*RemoteSubscription, error) {
	// 从末尾找最后一个冒号作为 URL 和别名的分隔符
	// 因为 URL 本身含冒号（https://），所以从右往左找
	lastColon := strings.LastIndex(input, ":")
	if lastColon < 0 {
		return nil, fmt.Errorf("格式错误，应为 URL:别名，例如 https://example.com/s/hash/default:香港")
	}
	rawURL := strings.TrimSpace(input[:lastColon])
	alias := strings.TrimSpace(input[lastColon+1:])

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("URL 必须以 http:// 或 https:// 开头")
	}
	if alias == "" {
		return nil, fmt.Errorf("别名不能为空")
	}
	return &RemoteSubscription{URL: rawURL, Alias: alias}, nil
}

// FetchRemote 获取远程订阅内容（直接请求完整 URL）
func FetchRemote(sub *RemoteSubscription) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(sub.URL)
	if err != nil {
		return nil, fmt.Errorf("请求远程订阅失败 %s: %w", sub.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("远程订阅返回状态码 %d: %s", resp.StatusCode, sub.URL)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取远程订阅内容失败: %w", err)
	}
	return data, nil
}

// MergeRemote 合并远程订阅到本地订阅（Base64 格式）
// 远程节点追加到本地节点之后，email 标签追加服务器别名后缀
func MergeRemote(local []byte, remotes [][]byte, aliases []string) ([]byte, error) {
	// 解码本地 Base64 订阅
	localDecoded, err := base64.StdEncoding.DecodeString(string(local))
	if err != nil {
		return nil, fmt.Errorf("failed to decode local subscription: %w", err)
	}
	localURIs := strings.Split(strings.TrimSpace(string(localDecoded)), "\n")

	// 解码并追加远程订阅
	for i, remote := range remotes {
		remoteDecoded, err := base64.StdEncoding.DecodeString(string(remote))
		if err != nil {
			continue // 跳过解码失败的远程订阅
		}
		alias := ""
		if i < len(aliases) {
			alias = aliases[i]
		}
		remoteURIs := strings.Split(strings.TrimSpace(string(remoteDecoded)), "\n")
		for _, uri := range remoteURIs {
			if uri == "" {
				continue
			}
			// 追加别名后缀到 fragment
			if alias != "" {
				if strings.Contains(uri, "#") {
					uri = uri + "_" + alias
				} else {
					uri = uri + "#" + alias
				}
			}
			localURIs = append(localURIs, uri)
		}
	}

	merged := strings.Join(localURIs, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(merged))), nil
}

// LoadRemoteSubscriptions 从文件加载远程订阅列表
func LoadRemoteSubscriptions(baseDir string) ([]RemoteSubscription, error) {
	path := filepath.Join(baseDir, RemoteSubFilePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var subs []RemoteSubscription
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sub, err := ParseRemoteSubscription(line)
		if err != nil {
			continue
		}
		subs = append(subs, *sub)
	}
	return subs, nil
}

// SaveRemoteSubscriptions 保存远程订阅列表到文件
func SaveRemoteSubscriptions(baseDir string, subs []RemoteSubscription) error {
	path := filepath.Join(baseDir, RemoteSubFilePath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var lines []string
	for _, sub := range subs {
		lines = append(lines, fmt.Sprintf("%s:%s", sub.URL, sub.Alias))
	}
	return security.AtomicWrite(path, []byte(strings.Join(lines, "\n")), 0600)
}
