package protocol

import (
	"strings"

	"vasmax/internal/config"
)

// NeedsNginxProxy reports whether a protocol is served behind Nginx.
// Reality protocols terminate their own TLS and must stay direct.
func NeedsNginxProxy(p Protocol) bool {
	transport := p.TransportType()
	if strings.Contains(p.Name(), "reality") {
		return false
	}
	return transport == "ws" || transport == "grpc" || transport == "httpupgrade"
}

// EffectiveInboundPort returns the local listen port for the protocol.
func EffectiveInboundPort(p Protocol, cfg *config.Config) int {
	if cfg != nil && cfg.ProtocolPorts != nil {
		if port, ok := cfg.ProtocolPorts[p.Name()]; ok && port > 0 {
			return port
		}
	}
	return p.DefaultPort()
}

// ExternalPort returns the client-facing port.
func ExternalPort(p Protocol, cfg *config.Config) int {
	if NeedsNginxProxy(p) {
		return 443
	}
	return EffectiveInboundPort(p, cfg)
}

// DefaultWSPath returns the stable WS/HTTPUpgrade/XHTTP path for a protocol.
func DefaultWSPath(p Protocol) string {
	switch p.Name() {
	case "vless_ws_tls":
		return "/vlessws"
	case "vmess_ws_tls":
		return "/vmessws"
	case "vmess_httpupgrade_tls":
		return "/vmesshup"
	case "vless_reality_xhttp":
		return "/realityxhttp"
	default:
		return "/" + strings.ReplaceAll(p.Name(), "_", "")
	}
}

// DefaultGRPCServiceName returns the stable gRPC service name for a protocol.
func DefaultGRPCServiceName(p Protocol) string {
	switch p.Name() {
	case "vless_grpc_tls":
		return "vlessgrpc"
	case "trojan_grpc_tls":
		return "trojangrpc"
	case "vless_reality_grpc":
		return "realitygrpc"
	default:
		return strings.ReplaceAll(p.Name(), "_", "")
	}
}
