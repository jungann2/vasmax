// Package sysinfo provides system information gathering and health check utilities.
package sysinfo

// SystemInfo contains basic system information.
type SystemInfo struct {
	OS       string   `json:"os"`
	Arch     string   `json:"arch"`
	Kernel   string   `json:"kernel"`
	Hostname string   `json:"hostname"`
	Uptime   int64    `json:"uptime"`
	CPU      float64  `json:"cpu_usage"`
	Memory   MemInfo  `json:"memory"`
	Disk     DiskInfo `json:"disk"`
}

// MemInfo contains memory usage information.
type MemInfo struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Available uint64  `json:"available"`
	Percent   float64 `json:"percent"`
}

// DiskInfo contains disk usage information.
type DiskInfo struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Percent float64 `json:"percent"`
}

// HealthStatus represents the result of a health check.
type HealthStatus struct {
	XrayRunning    bool   `json:"xray_running"`
	SingBoxRunning bool   `json:"singbox_running"`
	NginxRunning   bool   `json:"nginx_running"`
	CertValid      bool   `json:"cert_valid"`
	CertExpiry     string `json:"cert_expiry,omitempty"`
	PortsOpen      []int  `json:"ports_open"`
}
