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
	Domain string
	Port   int
	Alias  string
}

const (
	// RemoteSubFilePath 远程订阅 URL 列表持久化路径
	RemoteSubFilePath = "subscribe_remote/remoteSubscribeUrl"
)

// ParseRemoteSubscription 解析远程订阅输入（格式：域名:端口:别名）
func ParseRemoteSubscription(input string) (*RemoteSubscription, error) {
	parts := strings.SplitN(input, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid format, expected domain:port:alias")
	}
	domain := parts[0]
	if err := security.ValidateDomain(domain); err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}
	port := 0
	if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %s", parts[1])
	}
	alias := parts[2]
	if alias == "" {
		return nil, fmt.Errorf("alias cannot be empty")
	}
	return &RemoteSubscription{Domain: domain, Port: port, Alias: alias}, nil
}

// FetchRemote 获取远程订阅内容
func FetchRemote(sub *RemoteSubscription, format string) ([]byte, error) {
	url := fmt.Sprintf("https://%s:%d/s/%s/", sub.Domain, sub.Port, format)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote subscription from %s: %w", sub.Domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote subscription %s returned status %d", sub.Domain, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote subscription body: %w", err)
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
			if alias != "" && strings.Contains(uri, "#") {
				uri = uri + "_" + alias
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
		lines = append(lines, fmt.Sprintf("%s:%d:%s", sub.Domain, sub.Port, sub.Alias))
	}
	return security.AtomicWrite(path, []byte(strings.Join(lines, "\n")), 0600)
}
