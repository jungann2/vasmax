package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// Client xboard API 客户端
// API 基础路径: {baseURL}/api/v1/server/UniProxy/
// 所有请求携带查询参数: ?token={token}&node_id={nodeID}&node_type={nodeType}
type Client struct {
	httpClient *http.Client // 30 秒超时，启用 TLS 验证
	baseURL    string
	token      string
	nodeID     int
	nodeType   string
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

// doRequest 通用请求方法
// 自动拼接 URL: {baseURL}/api/v1/server/UniProxy/{path}?token=&node_id=&node_type=
// POST 请求自动设置 Content-Type: application/json，使用 json.Marshal 序列化 raw body
// HTTPS 通信启用 TLS 证书验证，不设置 InsecureSkipVerify
func (c *Client) doRequest(method, path string, body []byte) (*http.Response, error) {
	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/%s", c.baseURL, path)

	params := url.Values{}
	params.Set("token", c.token)
	params.Set("node_id", strconv.Itoa(c.nodeID))
	params.Set("node_type", c.nodeType)
	fullURL += "?" + params.Encode()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}
