package api

import "encoding/json"

// NodeConfig 从 xboard API 获取的节点配置
type NodeConfig struct {
	ServerPort    int               `json:"server_port"`
	ServerName    string            `json:"server_name"`
	PaddingScheme []string          `json:"padding_scheme"`
	Routes        []json.RawMessage `json:"routes"`
	BaseConfig    struct {
		PushInterval int `json:"push_interval"`
		PullInterval int `json:"pull_interval"`
	} `json:"base_config"`
}

// User 用户信息
// SpeedLimit/DeviceLimit 可以为 JSON null，使用 *int 指针类型处理
type User struct {
	ID          int    `json:"id"`
	UUID        string `json:"uuid"`
	SpeedLimit  *int   `json:"speed_limit"`  // Mbps, nil 或 0=不限
	DeviceLimit *int   `json:"device_limit"` // nil 或 0=不限
}

// NodeStatus 节点负载状态
type NodeStatus struct {
	CPU        float64       `json:"cpu"`
	Mem        ResourceUsage `json:"mem"`
	Swap       ResourceUsage `json:"swap"`
	Disk       ResourceUsage `json:"disk"`
	Network    NetworkUsage  `json:"network,omitempty"`
	DiskIO     DiskIOUsage   `json:"disk_io,omitempty"`
	Hostname   string        `json:"hostname,omitempty"`
	Uptime     int64         `json:"uptime,omitempty"`
	CPUModel   string        `json:"cpu_model,omitempty"`
	IPv4       string        `json:"ipv4,omitempty"`
	IPv6       string        `json:"ipv6,omitempty"`
	Goroutines int           `json:"goroutines,omitempty"`
}

// ResourceUsage 资源使用量
type ResourceUsage struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
}

// NetworkUsage 网络流量使用量
type NetworkUsage struct {
	Sent int64 `json:"sent"`
	Recv int64 `json:"recv"`
}

// DiskIOUsage 磁盘IO使用量
type DiskIOUsage struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}
