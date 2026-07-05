package protocol

import (
	"encoding/json"
	"sync"

	"vasmax/internal/api"
	"vasmax/internal/config"
)

// Protocol 协议接口，所有协议实现必须满足此接口
type Protocol interface {
	// Name 协议标识名（如 "vless_ws_tls"）
	Name() string
	// CoreType 所属核心类型 "xray" 或 "singbox"
	CoreType() string
	// GenerateInbound 生成入站配置 JSON
	GenerateInbound(params *InboundParams) (json.RawMessage, error)
	// GenerateUserEntry 生成单个用户的配置条目
	GenerateUserEntry(user *api.User) (json.RawMessage, error)
	// GenerateURI 生成协议 URI 链接
	GenerateURI(user *api.User, serverInfo *ServerInfo) string
	// GenerateClashProxy 生成 ClashMeta 代理条目
	GenerateClashProxy(user *api.User, serverInfo *ServerInfo) map[string]interface{}
	// GenerateSingBoxOutbound 生成 sing-box 出站条目
	GenerateSingBoxOutbound(user *api.User, serverInfo *ServerInfo) map[string]interface{}
	// DefaultPort 默认监听端口
	DefaultPort() int
	// TransportType 传输类型（ws/grpc/httpupgrade/tcp/quic）
	TransportType() string
	// IsCDNCompatible 是否支持 CDN 中转
	IsCDNCompatible() bool
}

// InboundParams 入站配置参数
type InboundParams struct {
	Port          int
	Domain        string
	CertFile      string
	KeyFile       string
	Path          string // WS/HTTPUpgrade 路径
	ServiceName   string // gRPC serviceName
	Users         []*api.User
	Tag           string // 入站 tag
	Reality       *config.RealityConfig
	Hysteria2     *config.Hysteria2Config
	Tuic          *config.TuicConfig
	PaddingScheme []string // AnyTLS padding scheme
	TLSMinVersion string   // "1.0"/"1.1"/"1.2"/"1.3"，空值默认 "1.2"
	TLSMaxVersion string   // "1.2"/"1.3"，空值默认 "1.3"
	ALPN          []string // TLS ALPN list；空值使用协议默认
	KeepAlive     config.ConnectionConfig
}

// ServerInfo 服务器连接信息（用于生成订阅链接）
type ServerInfo struct {
	Name        string
	Host        string
	CDNHost     string // CDN 地址（如配置）
	Port        int
	Domain      string // TLS SNI 域名
	Path        string
	ServiceName string
	Reality     *config.RealityConfig
	Tuic        *config.TuicConfig
	ALPN        []string
}

// Registry 协议注册表
type Registry struct {
	mu        sync.RWMutex
	protocols map[string]Protocol
}

// NewRegistry 创建空注册表
func NewRegistry() *Registry {
	return &Registry{
		protocols: make(map[string]Protocol),
	}
}

// Register 注册协议
func (r *Registry) Register(p Protocol) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.protocols[p.Name()] = p
}

// Get 获取协议
func (r *Registry) Get(name string) (Protocol, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.protocols[name]
	return p, ok
}

// ListByCore 按核心类型列出协议
func (r *Registry) ListByCore(coreType string) []Protocol {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Protocol
	for _, p := range r.protocols {
		if p.CoreType() == coreType {
			result = append(result, p)
		}
	}
	return result
}

// ListAll 列出所有已注册协议
func (r *Registry) ListAll() []Protocol {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Protocol, 0, len(r.protocols))
	for _, p := range r.protocols {
		result = append(result, p)
	}
	return result
}

// domainOrder 绑定域名安装的固定排序（推荐 → 稳定 → 一般 → 不安全）
var domainOrder = []string{
	"vless_reality_vision",
	"anytls",
	"vless_tcp_tls_vision",
	"vless_ws_tls",
	"hysteria2",
	"tuic",
	"vless_grpc_tls",
	"vmess_ws_tls",
	"vmess_httpupgrade_tls",
	"trojan_tcp_tls",
	"trojan_grpc_tls",
	"naive",
	"vless_reality_grpc",
	"vless_reality_xhttp",
	"socks5",
}

// noDomainOrder 无域名安装的固定排序
var noDomainOrder = []string{
	"vless_reality_vision",
	"anytls",
	"hysteria2",
	"tuic",
	"vless_reality_grpc",
	"vless_reality_xhttp",
	"socks5",
}

// ListAllOrdered 按固定顺序列出所有协议（绑定域名安装用）
func (r *Registry) ListAllOrdered() []Protocol {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Protocol, 0, len(r.protocols))
	for _, name := range domainOrder {
		if p, ok := r.protocols[name]; ok {
			result = append(result, p)
		}
	}
	return result
}

// ListNoDomainOrdered 按固定顺序列出无域名协议
func (r *Registry) ListNoDomainOrdered() []Protocol {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Protocol, 0, len(noDomainOrder))
	for _, name := range noDomainOrder {
		if p, ok := r.protocols[name]; ok {
			result = append(result, p)
		}
	}
	return result
}

// DefaultRegistry 默认注册表，包含所有 15 种协议
func DefaultRegistry() *Registry {
	r := NewRegistry()
	// Xray VLESS 变体
	r.Register(&VlessTCPTLSVision{})
	r.Register(&VlessWSTLS{})
	r.Register(&VlessGRPCTLS{})
	r.Register(&VlessRealityVision{})
	r.Register(&VlessRealityGRPC{})
	r.Register(&VlessRealityXHTTP{})
	// Xray VMess 变体
	r.Register(&VMessWSTLS{})
	r.Register(&VMessHTTPUpgradeTLS{})
	// Xray Trojan 变体
	r.Register(&TrojanTCPTLS{})
	r.Register(&TrojanGRPCTLS{})
	// SingBox 协议
	r.Register(&Hysteria2{})
	r.Register(&Tuic{})
	r.Register(&AnyTLS{})
	r.Register(&Naive{})
	r.Register(&Socks5{})
	return r
}

// GenerateClashProxiesForUser 为单个用户生成所有协议的 ClashMeta 代理条目
func GenerateClashProxiesForUser(protocols []Protocol, user *api.User, info *ServerInfo) []map[string]interface{} {
	var proxies []map[string]interface{}
	for _, p := range protocols {
		proxy := p.GenerateClashProxy(user, info)
		if proxy != nil {
			proxies = append(proxies, proxy)
		}
	}
	return proxies
}

// GenerateSingBoxOutboundsForUser 为单个用户生成所有协议的 sing-box 出站条目
func GenerateSingBoxOutboundsForUser(protocols []Protocol, user *api.User, info *ServerInfo) []map[string]interface{} {
	var outbounds []map[string]interface{}
	for _, p := range protocols {
		ob := p.GenerateSingBoxOutbound(user, info)
		if ob != nil {
			outbounds = append(outbounds, ob)
		}
	}
	return outbounds
}
