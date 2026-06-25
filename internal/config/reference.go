package config

const referenceConfigYAML = `# VasmaX 配置参考文件
# 路径: /etc/vasmax/config.reference.yaml
# 作用: 只用于查看字段含义，不会被程序直接读取。
# 实际生效配置是同目录 config.yaml，菜单保存时会重写 config.yaml。

# 运行模式
# true  = 独立模式，本机菜单管理用户和协议
# false = 托管模式，对接 Xboard-Plus 拉取用户、上报流量
standalone: true

# VasmaX 管理/API 监听地址，通常保持默认即可
listen: 127.0.0.1:8080

# Xboard-Plus 托管模式配置
# api_host 必须带 http:// 或 https://，结尾不需要 /
# api_prefix 只填路径前缀，不填域名，不带 http:// 或 https://
# node_type 常用: vless/vmess/trojan/hysteria2/tuic/anytls/naive
api_host: https://panel.example.com
api_token: your-xboard-communication-key
api_prefix: api
node_id: 1
node_type: vless

# TLS 证书配置
# domain 只填域名，不带 http:// 或 https://
# provider 常见: acme/cloudflare/aliyun/manual/bt/1panel
# min_version/max_version 留空时默认 TLS 1.2 到 TLS 1.3
tls:
  cert_file: /etc/vasmax/tls/fullchain.crt
  key_file: /etc/vasmax/tls/private.key
  domain: node.example.com
  provider: acme
  min_version: "1.2"
  max_version: "1.3"

# 日志配置
# level: debug/info/warn/error
log:
  level: info
  file_path: /var/log/vasmax/VasmaX.log

# 审计日志配置
# max_size 单位 MB，max_files 为保留文件数量
audit:
  enabled: true
  file_path: /var/log/vasmax/audit.log
  max_size: 50
  max_files: 3

# 菜单语言，目前主要使用 zh
lang: zh

# 已安装协议列表，由菜单自动维护
# 示例: vless_ws_tls、vmess_ws_tls、trojan_tcp_tls、hysteria2、tuic、anytls、naive、socks5
protocols:
  - vless_ws_tls

# 协议安装模式，由菜单自动维护
# domain   = 绑定域名模式
# nodomain = 无域名模式
protocol_modes:
  vless_ws_tls: domain

# 协议端口覆盖，由菜单自动维护；不写则使用协议默认端口
protocol_ports:
  vless_ws_tls: 31297

# 多域名模式下每个协议对应的独立域名
# 只填域名，不带 http:// 或 https://
protocol_domains:
  vless_ws_tls: node.example.com

# 核心类型
# xray/singbox/dual，dual 表示按协议自动同时管理 Xray 和 sing-box
core_type: dual

# CDN 配置
# address 填 CDN 域名或 IP，不带 http:// 或 https://
cdn:
  enabled: false
  address: cdn.example.com

# 订阅配置
# domain 只填订阅域名，不带 http:// 或 https://；生成订阅链接时自动使用 https://
# dns_mode: auto/cn/global/privacy/custom
# auto    = 国内 DNS 直连，国外 DNS 走代理，推荐默认
# cn      = 全部使用国内 DNS
# global  = 全部使用海外 DNS
# privacy = 使用隐私优先 DNS
# custom  = 使用 dns_custom
# DoH 地址必须带 https://；普通 IP 不要带协议
# test_url 用于 Clash/sing-box 自动测速，必须是完整 https:// 地址
subscription:
  salt: ""
  domain: sub.example.com
  dns_mode: auto
  dns_domestic:
    - https://dns.alidns.com/dns-query
    - https://doh.pub/dns-query
  dns_global:
    - https://cloudflare-dns.com/dns-query
    - https://dns.google/dns-query
    - https://dns.quad9.net/dns-query
  dns_privacy:
    - https://dns.quad9.net/dns-query
    - https://cloudflare-dns.com/dns-query
    - https://dns.adguard-dns.com/dns-query
  dns_custom:
    - https://dns.example.com/dns-query
  test_url: https://www.gstatic.com/generate_204

# Hysteria2 配置
# down_mbps/up_mbps 为客户端速率提示
# hop_start/hop_end 为端口跳跃范围，0 表示不启用
hysteria2:
  port: 8443
  down_mbps: 100
  up_mbps: 50
  hop_start: 0
  hop_end: 0

# TUIC 配置
# congestion_control 常见: bbr/cubic/new_reno
tuic:
  port: 8443
  congestion_control: bbr

# Reality 配置
# dest/server_name 只填域名或 域名:端口，不带 http:// 或 https://
# port 为 Reality 监听端口，默认 443，部分云厂商可用 8443
reality:
  private_key: ""
  public_key: ""
  short_id: ""
  dest: www.apple.com:443
  server_name: www.apple.com
  port: 443

# Nginx 反向代理配置
# long_connection_timeout 用于 WebSocket/gRPC/HTTPUpgrade 长连接
# 86400s = 24 小时；1G 内存机器不建议随意拉到 48 小时以上
nginx:
  long_connection_timeout: 86400s

# 托管模式同步保护
# empty_users_apply_threshold:
#   面板 API 返回空用户列表时，连续出现多少次才真正应用。
#   默认 3，可避免面板偶发返回空列表导致所有用户断流。
#   设为 1 表示第一次空列表就应用；设为 -1 表示关闭此保护。
# min_pull_interval_seconds:
#   面板下发的 pull_interval 低于此值时，使用这里的最小值。
#   默认 30 秒，避免面板异常下发过短轮询导致 1G 机器压力升高。
# min_push_interval_seconds:
#   面板下发的 push_interval 低于此值时，使用这里的最小值。
#   默认 30 秒，减少高频上报对小内存机器和面板 API 的冲击。
sync:
  empty_users_apply_threshold: 3
  min_pull_interval_seconds: 30
  min_push_interval_seconds: 30

# 连接保活配置
# 用于缓解 NAT/CDN/运营商链路过早回收空闲连接导致的断流。
# keepalive_mode:
#   auto = 自动给 Nginx、Xray、sing-box TCP 入站启用保活
#   off  = 关闭 VasmaX 生成的保活字段，使用核心/系统默认行为
# idle_seconds:
#   TCP 连接空闲多久后开始发送 keepalive 探测。默认 8 秒，适合空闲 10 秒左右断流的急救场景。
# interval_seconds:
#   keepalive 探测间隔。默认 8 秒。
# probes:
#   Nginx 监听 socket 探测次数。默认 3。
# websocket_heartbeat_seconds:
#   Xray WebSocket Ping 心跳间隔。默认 8 秒；用于缓解 WS 空闲后被中间链路回收。
connection:
  keepalive_mode: auto
  keepalive_idle_seconds: 8
  keepalive_interval_seconds: 8
  keepalive_probes: 3
  websocket_heartbeat_seconds: 8

# 文件路径配置，通常不需要改
paths:
  xray_conf: /etc/vasmax/xray/conf/
  singbox_conf: /etc/vasmax/sing-box/conf/config/
  subscribe: /etc/vasmax/subscribe/
  cache: /etc/vasmax/cache/
  nginx_conf: /etc/nginx/conf.d/

# 实时监控开关
monitoring_enabled: true

# 额外开放端口
# protocol: tcp/udp/both
extra_ports:
  - port: 8080
    protocol: tcp
    note: example

# ALPN 全局配置
# h2_http11/h2_only/http11_only/h3_only/all
alpn:
  mode: h2_http11
`
