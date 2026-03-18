package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Client xboard API 客户端
// API 基础路径: {baseURL}/api/v1/server/UniProxy/ （默认）
// 或自定义前缀: {baseURL}/{customPrefix}/v1/server/UniProxy/
// 所有请求携带查询参数: ?token={token}&node_id={nodeID}&node_type={nodeType}
type Client struct {
	httpClient *http.Client // 30 秒超时，启用 TLS 验证
	baseURL    string
	token      string
	nodeID     int
	nodeType   string
	apiPrefix  string // 自定义 API 路径前缀，为空则使用默认 "api"
	userETag   string // 用户列表 ETag（含双引号，原样存储和发送）
	configETag string // 节点配置 ETag
	logger     *logrus.Logger
}

// NewClient 创建 API 客户端
func NewClient(baseURL, token string, nodeID int, nodeType string, logger *logrus.Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		token:      token,
		nodeID:     nodeID,
		nodeType:   nodeType,
		logger:     logger,
	}
}

// SetAPIPrefix 设置自定义 API 路径前缀
func (c *Client) SetAPIPrefix(prefix string) {
	c.apiPrefix = strings.Trim(prefix, "/")
}

// doRequest 通用请求方法
// 自动拼接 URL: {baseURL}/{prefix}/v1/server/UniProxy/{path}?token=&node_id=&node_type=
// prefix 默认为 "api"，可通过 SetAPIPrefix 自定义
func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	prefix := "api"
	if c.apiPrefix != "" {
		prefix = c.apiPrefix
	}
	fullURL := fmt.Sprintf("%s/%s/v1/server/UniProxy/%s", c.baseURL, prefix, path)

	params := url.Values{}
	params.Set("token", c.token)
	params.Set("node_id", strconv.Itoa(c.nodeID))
	params.Set("node_type", c.nodeType)
	fullURL += "?" + params.Encode()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}
