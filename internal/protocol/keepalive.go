package protocol

import (
	"encoding/json"
	"fmt"

	"vasmax/internal/config"
)

func keepAliveEnabled(cfg config.ConnectionConfig) bool {
	return cfg.KeepAliveMode != "off"
}

func keepAliveIdle(cfg config.ConnectionConfig) int {
	if cfg.KeepAliveIdleSeconds > 0 {
		return cfg.KeepAliveIdleSeconds
	}
	return 8
}

func keepAliveInterval(cfg config.ConnectionConfig) int {
	if cfg.KeepAliveIntervalSeconds > 0 {
		return cfg.KeepAliveIntervalSeconds
	}
	return 8
}

func webSocketHeartbeat(cfg config.ConnectionConfig) int {
	if cfg.WebSocketHeartbeatSeconds > 0 {
		return cfg.WebSocketHeartbeatSeconds
	}
	return 8
}

func marshalXrayInbound(inbound map[string]interface{}, params *InboundParams) (json.RawMessage, error) {
	if streamSettings, ok := inbound["streamSettings"].(map[string]interface{}); ok {
		applyXrayKeepAlive(streamSettings, params)
	}
	return json.Marshal(inbound)
}

func applyXrayKeepAlive(streamSettings map[string]interface{}, params *InboundParams) {
	if streamSettings == nil || params == nil || !keepAliveEnabled(params.KeepAlive) {
		return
	}
	streamSettings["sockopt"] = map[string]interface{}{
		"tcpKeepAliveIdle":     keepAliveIdle(params.KeepAlive),
		"tcpKeepAliveInterval": keepAliveInterval(params.KeepAlive),
	}

	if network, _ := streamSettings["network"].(string); network == "ws" {
		if wsSettings, ok := streamSettings["wsSettings"].(map[string]interface{}); ok {
			wsSettings["heartbeatPeriod"] = webSocketHeartbeat(params.KeepAlive)
		}
	}
}

func applySingBoxTCPKeepAlive(inbound map[string]interface{}, params *InboundParams) {
	if inbound == nil || params == nil || !keepAliveEnabled(params.KeepAlive) {
		return
	}
	inbound["tcp_keep_alive"] = fmt.Sprintf("%ds", keepAliveIdle(params.KeepAlive))
	inbound["tcp_keep_alive_interval"] = fmt.Sprintf("%ds", keepAliveInterval(params.KeepAlive))
}
