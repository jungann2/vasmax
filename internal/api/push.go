package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// PushTraffic 上报流量数据
// data 格式: map[user_id][2]int64 -> {"user_id": [upload, download]}
// 上报原始字节数，Xboard 自动乘以节点 rate 倍率
func (c *Client) PushTraffic(data map[int][2]int64) error {
	if len(data) == 0 {
		return nil
	}

	// JSON key 必须为字符串
	payload := make(map[string][2]int64, len(data))
	for uid, traffic := range data {
		payload[strconv.Itoa(uid)] = traffic
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化流量数据失败: %w", err)
	}

	resp, err := doWithRetry(func() (*http.Response, error) {
		return c.doRequest(http.MethodPost, "push", body)
	}, c.logger)
	if err != nil {
		return fmt.Errorf("上报流量失败: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上报流量失败, 状态码: %d", resp.StatusCode)
	}

	c.logger.WithField("users", len(data)).Info("流量数据已上报")
	return nil
}

// PushAlive 上报在线用户
// data 格式: map[user_id][]string -> {"user_id": ["ip1_nodeId", "ip2_nodeId"]}
// IP 必须附加 "_{node_id}" 后缀
func (c *Client) PushAlive(data map[int][]string) error {
	if len(data) == 0 {
		return nil
	}

	payload := make(map[string][]string, len(data))
	for uid, ips := range data {
		payload[strconv.Itoa(uid)] = ips
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化在线数据失败: %w", err)
	}

	resp, err := doWithRetry(func() (*http.Response, error) {
		return c.doRequest(http.MethodPost, "alive", body)
	}, c.logger)
	if err != nil {
		return fmt.Errorf("上报在线数据失败: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上报在线数据失败, 状态码: %d", resp.StatusCode)
	}

	c.logger.WithField("users", len(data)).Info("在线数据已上报")
	return nil
}

// FetchAliveList 获取全局在线设备数量
// 返回格式: {"alive": {"user_id": count}} -> map[int]int
// 仅包含 device_limit > 0 的用户
func (c *Client) FetchAliveList() (map[int]int, error) {
	resp, err := doWithRetry(func() (*http.Response, error) {
		return c.doRequest(http.MethodGet, "alivelist", nil)
	}, c.logger)
	if err != nil {
		return nil, fmt.Errorf("获取在线设备数失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取在线设备数失败: HTTP %d, body: %s", resp.StatusCode, truncateBody(body))
	}

	var result struct {
		Alive map[string]int `json:"alive"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析在线设备数失败: %w", err)
	}

	aliveMap := make(map[int]int, len(result.Alive))
	for uidStr, count := range result.Alive {
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			continue
		}
		aliveMap[uid] = count
	}

	return aliveMap, nil
}

// PushStatus 上报节点负载状态
// 包含 cpu/mem/swap/disk
func (c *Client) PushStatus(status *NodeStatus) error {
	body, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("序列化状态数据失败: %w", err)
	}

	resp, err := doWithRetry(func() (*http.Response, error) {
		return c.doRequest(http.MethodPost, "status", body)
	}, c.logger)
	if err != nil {
		return fmt.Errorf("上报节点状态失败: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上报节点状态失败, 状态码: %d", resp.StatusCode)
	}

	return nil
}
