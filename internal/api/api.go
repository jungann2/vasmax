// Package api provides Xboard panel API client for node synchronization,
// traffic reporting, and user management.
package api

// Client represents an Xboard API client.
type Client struct {
	Host   string
	APIKey string
	NodeID int
}

// UserInfo represents a synchronized user from Xboard panel.
type UserInfo struct {
	ID          int    `json:"id"`
	UUID        string `json:"uuid"`
	SpeedLimit  int    `json:"speed_limit"`
	DeviceLimit int    `json:"device_limit"`
	Expired     bool   `json:"expired"`
}

// TrafficReport represents traffic data to be reported to Xboard.
type TrafficReport struct {
	UserID   int   `json:"user_id"`
	Upload   int64 `json:"u"`
	Download int64 `json:"d"`
}

// NodeConfig represents node configuration from Xboard panel.
type NodeConfig struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Settings string `json:"settings"`
}
