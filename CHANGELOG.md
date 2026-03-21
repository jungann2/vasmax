# Changelog

## v2.1.0 (2026-03-18)

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
