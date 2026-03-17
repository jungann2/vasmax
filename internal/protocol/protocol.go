// Package protocol defines interfaces and types for proxy protocol management.
// Supports Xray-core (VLESS/VMess/Trojan) and sing-box (AnyTLS/Hysteria2/Tuic/Naive).
package protocol

// Protocol represents a proxy protocol handler.
type Protocol interface {
	Name() string
	Core() string
	Install(domain string, port int) error
	Uninstall() error
	Status() (bool, error)
	GenerateConfig() ([]byte, error)
}

// CoreType identifies the proxy core engine.
type CoreType string

const (
	CoreXray    CoreType = "xray"
	CoreSingBox CoreType = "sing-box"
	CoreDual    CoreType = "dual"
)

// ProtocolName identifies supported protocols.
type ProtocolName string

const (
	VLESS_TCP_TLS        ProtocolName = "vless_tcp_tls_vision"
	VLESS_WS_TLS         ProtocolName = "vless_ws_tls"
	VLESS_GRPC_TLS       ProtocolName = "vless_grpc_tls"
	VLESS_Reality_Vision ProtocolName = "vless_reality_vision"
	VLESS_Reality_GRPC   ProtocolName = "vless_reality_grpc"
	VLESS_Reality_XHTTP  ProtocolName = "vless_reality_xhttp"
	VMess_WS_TLS         ProtocolName = "vmess_ws_tls"
	VMess_HTTPUpgrade    ProtocolName = "vmess_httpupgrade_tls"
	Trojan_TCP_TLS       ProtocolName = "trojan_tcp_tls"
	Trojan_GRPC_TLS      ProtocolName = "trojan_grpc_tls"
	AnyTLS               ProtocolName = "anytls"
	Hysteria2            ProtocolName = "hysteria2"
	Tuic                 ProtocolName = "tuic"
	NaiveProxy           ProtocolName = "naiveproxy"
	Socks5               ProtocolName = "socks5"
)
