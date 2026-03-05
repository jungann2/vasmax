package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// buildURL 构建完整的 API URL（含查询参数）
func (c *Client) buildURL(path string) string {
	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/%s", c.baseURL, path)

	params := url.Values{}
	params.Set("token", c.token)
	params.Set("node_id", strconv.Itoa(c.nodeID))
	params.Set("node_type", c.nodeType)
	fullURL += "?" + params.Encode()

	return fullURL
}

// FetchConfig 获取节点配置（支持 ETag 缓存）
// 返回 nil, nil 表示 304 未修改
// 提取 server_port、push_interval、pull_interval、routes、padding_scheme 等参数
func (c *Client) FetchConfig() (*NodeConfig, error) {
	req, err := http.NewRequest(http.MethodGet, c.buildURL("config"), nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 原样发送 ETag（含双引号）
	if c.configETag != "" {
		req.Header.Set("If-None-Match", c.configETag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取节点配置失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		c.logger.Debug("节点配置未变化 (304)")
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取节点配置失败: HTTP %d, body: %s", resp.StatusCode, truncateBody(body))
	}

	var cfg NodeConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("解析节点配置失败: %w", err)
	}

	// 原样存储 ETag（含双引号，如 "abc123"）
	if etag := resp.Header.Get("ETag"); etag != "" {
		c.configETag = etag
	}

	c.logger.WithField("server_port", cfg.ServerPort).Info("节点配置已加载")
	return &cfg, nil
}

// FetchUsers 获取用户列表（支持 ETag 缓存）
// 返回 nil, nil 表示 304 未修改
// ETag 含双引号原样存储和发送
func (c *Client) FetchUsers() ([]User, error) {
	req, err := http.NewRequest(http.MethodGet, c.buildURL("user"), nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 原样发送 ETag（含双引号）
	if c.userETag != "" {
		req.Header.Set("If-None-Match", c.userETag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		c.logger.Debug("用户列表未变化 (304)")
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取用户列表失败: HTTP %d, body: %s", resp.StatusCode, truncateBody(body))
	}

	var result struct {
		Users []User `json:"users"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析用户列表失败: %w", err)
	}

	// 原样存储 ETag（含双引号，如 "abc123"）
	if etag := resp.Header.Get("ETag"); etag != "" {
		c.userETag = etag
	}

	c.logger.WithField("count", len(result.Users)).Info("用户列表已更新")
	return result.Users, nil
}

// TestConnection 测试 API 连接（调用 config 接口）
func (c *Client) TestConnection() error {
	_, err := c.FetchConfig()
	return err
}

// truncateBody 截断响应体用于日志记录
func truncateBody(body []byte) string {
	const maxLen = 200
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}
