# Changelog

## v1.0.0 (2026-01-10)

### Bug Fixes
- 修复 Alpine OpenRC 兼容性问题
- 修复菜单卸载选项未清理 systemd service
- 修复 CentOS/RHEL 依赖安装失败

### New Features
- 正式支持 Alpine Linux（OpenRC）
- 新增完整卸载功能（含 Xray/sing-box/BBR 配置清理）
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
