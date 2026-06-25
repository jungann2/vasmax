# Changelog

## v2.2.4 (2026-06-25)

### Improvements
- 专项优化 Codex / AI 长任务 WebSocket/WSS 连接稳定性，降低空闲断流与反复重连概率
- 优化安装协议组合的端口提示、端口复用检测和 Reality 参数显示
- 增强 Clash / sing-box 订阅兼容性，改善 TUN、UDP、自签证书和 AI 平台规则体验
- 优化 BBR 菜单说明和危险操作保护，减少误操作风险
- 完善配置参考说明，补充长连接、同步保护和保活相关字段

### Bug Fixes
- 修复部分协议订阅字段不完整导致客户端导入后超时或不可用的问题
- 修复 Hysteria2 / TUIC 专项参数与订阅输出不一致的问题
- 修复 Reality / 多协议组合安装时端口显示和真实监听不一致的问题

## v2.2.3 (2026-06-25)

### Improvements
- Xray 服务端 DNS 默认跟随系统解析，不再固定公共 DNS
- 订阅 DNS 支持 auto/cn/global/privacy/custom 模式
- 优化 Clash 与 sing-box 客户端订阅的 DNS 默认策略
- 订阅自动测速 URL 支持配置
- 新增完整配置参考文件，便于查看字段说明
- Nginx 长连接超时支持配置，默认保持 24 小时
- 增强托管同步保护，降低面板临时异常导致节点断流的概率
- 优化 Xboard 下发配置的启动应用与同步间隔保护
- 新增 TCP/WS 连接保活配置，缓解空闲连接被过早回收导致的断流

## v2.2.2 (2026-06-25)

### Improvements
- 优化 WebSocket、HTTPUpgrade、gRPC 长连接稳定性
- 将 Nginx 反代空闲超时调整为 24 小时
- 关闭流式连接的代理缓冲，提升长任务体验

## v2.2.1 (2026-06-25)

### Improvements
- 优化托管模式同步稳定性，减少无变化配置导致的重复重载
- 统一订阅链接与实际服务配置，提升节点连接可靠性
- 优化安装、更新、卸载流程，增强失败回滚和清理安全性
- 调整管理菜单，新增服务管理和协议专项管理入口
- 优化核心服务启动、停止、重启体验

## v2.2.0 (2026-04-07)

### Bug Fixes (Critical)
- 修复订阅链接端口全部硬编码为 443，导致 Hysteria2/TUIC 等非 443 协议订阅链接错误
- 修复订阅链接 WS 路径/gRPC serviceName 硬编码为 `/vasmax`，客户端无法连接
- 修复无域名模式（Reality）订阅链接地址为空
- 修复 `showLinks()` 使用空 salt 生成 hash，与磁盘文件目录不匹配导致订阅 404
- 修复 Xray Stats 缺少用户级流量统计配置（`statsUserUplink/Downlink`），导致流量监控和托管模式流量上报全部为零
- 修复整个项目因三个旧版残留文件（`protocol.go`/`api.go`/`sysinfo.go`）重复声明导致无法编译
- 修复回滚快照不记录运行中的服务，回滚后核心不会重启
- 修复 `statEntry.Value` 类型错误（定义为 string 但 Xray 返回 int），监控菜单流量数据解析失败
- 修复远程订阅 URL 拼接错误（缺少用户 hash），远程订阅功能完全不可用
- 修复核心安装解压时 `defer close` 在循环内，文件关闭错误被静默丢弃
- 修复端口管理菜单编号冲突，已有端口列表与操作选项编号重叠导致无法正常操作
- 修复 VMess URI 生成忽略 `json.Marshal` 错误
- 修复节点类型验证缺少 `hysteria2`/`naive`，托管模式配置校验误报

### New Features
- 账号管理新增「编辑用户」：支持设置速率限制（Mbps）和设备数限制（独立模式）
- 远程订阅格式改为完整 URL（`https://域名/s/hash/default:别名`），实际可用
- 订阅管理新增操作说明，提示何时需要重新生成订阅
- 其他工具菜单「伪装站管理」添加说明，区分与 Reality 伪装域名的区别
- CDN 管理从「其他工具」移除重复入口，统一在主菜单 8



### Bug Fixes
- 修复 Reality XHTTP/gRPC 无域名模式缺少 path 和 serviceName 导致连接失败
- 修复启动时旧版 Reality 端口配置未自动迁移导致冲突
- 修复 Reality 协议端口冲突导致 Xray 崩溃（各协议使用独立默认端口）
- 修复端口冲突导致 xray/singbox 无法启动
- 修复 Nginx reload 失败时无 fallback 处理
- 修复 Nginx 版本检测与自动升级逻辑
- 修复 sing-box 配置合并问题
- 修复 sing-box 旧版 DNS 配置不兼容 1.12+ 版本
- 修复 acme.sh 使用 example.com 邮箱被拒绝
- 修复证书申请失败后无法重新选择验证方式

### New Features
- 多域名支持：每个协议可绑定不同域名，证书按域名去重检测/申请
- 域名模式安装支持自定义端口
- 智能证书检测 + acme.sh exit code 2 处理
- 证书验证方式选择前预检 80 端口状态
- AnyTLS 加推荐标签并调整为有域名安装首选

## v1.0.0 (2026-01-10)

### Bug Fixes
- 修复 Alpine OpenRC 兼容性问题
- 修复菜单卸载选项未清理 systemd service

### New Features
- 正式支持 Alpine Linux（OpenRC）
- 新增完整卸载功能
- 新增 BBR 加速管理（32 项功能）

## v0.3.2 (2025-12-05)

### Bug Fixes
- 修复旧版本已安装协议无法自动推断安装模式

## v0.3.1 (2025-11-25)

### Improvements
- 安装/更新完成后显示当前版本号

## v0.3.0 (2025-11-15)

### Improvements
- 重构安装菜单协议分类

## v0.2.7 (2025-11-01)

### Bug Fixes
- 修复安装核心时未自动创建 systemd service

## v0.2.6 (2025-10-22)

### Improvements
- TLS 菜单添加证书有效期显示

## v0.2.5 (2025-10-15)

### New Features
- TLS 证书申请新增 Nginx webroot 验证方式

## v0.2.4 (2025-10-05)

### Improvements
- 证书申请完成后显示证书路径

## v0.2.3 (2025-09-28)

### New Features
- 协议安装模式追踪

## v0.2.2 (2025-09-20)

### Improvements
- 安装菜单分离域名/无域名协议模式

## v0.2.1 (2025-09-12)

### Bug Fixes
- 移除误提交的二进制文件
