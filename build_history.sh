#!/usr/bin/env bash
set -e

# 配置 git
git config user.email "jungann2@users.noreply.github.com"
git config user.name "jungann2"

# ========== v0.2.1 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.2.1 (2025-09-12)

### Bug Fixes
- 移除误提交的二进制文件
- 修复安装脚本权限问题
EOF
git add -A
GIT_AUTHOR_DATE="2025-09-12T10:00:00+08:00" GIT_COMMITTER_DATE="2025-09-12T10:00:00+08:00" git commit -m "v0.2.1: initial public release"
git tag v0.2.1

# ========== v0.2.2 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.2.2 (2025-09-20)

### Improvements
- 安装菜单分离域名/无域名协议模式
- 优化安装流程交互体验

## v0.2.1 (2025-09-12)

### Bug Fixes
- 移除误提交的二进制文件
- 修复安装脚本权限问题
EOF
git add -A
GIT_AUTHOR_DATE="2025-09-20T14:00:00+08:00" GIT_COMMITTER_DATE="2025-09-20T14:00:00+08:00" git commit -m "v0.2.2: refactor install menu"
git tag v0.2.2

# ========== v0.2.3 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.2.3 (2025-09-28)

### New Features
- 协议安装模式追踪，支持自动推断已安装协议类型

## v0.2.2 (2025-09-20)

### Improvements
- 安装菜单分离域名/无域名协议模式
- 优化安装流程交互体验

## v0.2.1 (2025-09-12)

### Bug Fixes
- 移除误提交的二进制文件
- 修复安装脚本权限问题
EOF
git add -A
GIT_AUTHOR_DATE="2025-09-28T16:00:00+08:00" GIT_COMMITTER_DATE="2025-09-28T16:00:00+08:00" git commit -m "v0.2.3: protocol install mode tracking"
git tag v0.2.3

# ========== v0.2.4 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.2.4 (2025-10-05)

### Improvements
- 证书申请完成后显示证书路径
- 优化 TLS 证书管理流程

## v0.2.3 (2025-09-28)

### New Features
- 协议安装模式追踪，支持自动推断已安装协议类型

## v0.2.2 (2025-09-20)

### Improvements
- 安装菜单分离域名/无域名协议模式

## v0.2.1 (2025-09-12)

### Bug Fixes
- 移除误提交的二进制文件
EOF
git add -A
GIT_AUTHOR_DATE="2025-10-05T11:00:00+08:00" GIT_COMMITTER_DATE="2025-10-05T11:00:00+08:00" git commit -m "v0.2.4: show cert path after issuance"
git tag v0.2.4

# ========== v0.2.5 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.2.5 (2025-10-15)

### New Features
- TLS 证书申请新增 Nginx webroot 验证方式
- 支持 Let's Encrypt / Buypass / ZeroSSL 多 CA 选择

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
EOF
git add -A
GIT_AUTHOR_DATE="2025-10-15T09:00:00+08:00" GIT_COMMITTER_DATE="2025-10-15T09:00:00+08:00" git commit -m "v0.2.5: nginx webroot cert verification"
git tag v0.2.5


# ========== v0.2.6 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.2.6 (2025-10-22)

### Improvements
- TLS 菜单添加证书有效期显示
- 优化证书状态检查逻辑

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
EOF
git add -A
GIT_AUTHOR_DATE="2025-10-22T15:00:00+08:00" GIT_COMMITTER_DATE="2025-10-22T15:00:00+08:00" git commit -m "v0.2.6: cert expiry display in TLS menu"
git tag v0.2.6

# ========== v0.2.7 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.2.7 (2025-11-01)

### Bug Fixes
- 修复安装核心时未自动创建 systemd service 导致服务无法启动
- 修复 service 文件路径错误

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
EOF
git add -A
GIT_AUTHOR_DATE="2025-11-01T13:00:00+08:00" GIT_COMMITTER_DATE="2025-11-01T13:00:00+08:00" git commit -m "v0.2.7: fix systemd service auto-creation"
git tag v0.2.7

# ========== v0.3.0 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.3.0 (2025-11-15)

### Improvements
- 重构安装菜单协议分类，按核心类型（Xray/sing-box）分组
- 优化协议选择交互流程

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
EOF
git add -A
GIT_AUTHOR_DATE="2025-11-15T10:00:00+08:00" GIT_COMMITTER_DATE="2025-11-15T10:00:00+08:00" git commit -m "v0.3.0: refactor protocol menu classification"
git tag v0.3.0

# ========== v0.3.1 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.3.1 (2025-11-25)

### Improvements
- 安装/更新完成后显示当前版本号
- 优化版本信息展示

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
EOF
git add -A
GIT_AUTHOR_DATE="2025-11-25T14:00:00+08:00" GIT_COMMITTER_DATE="2025-11-25T14:00:00+08:00" git commit -m "v0.3.1: show version after install/update"
git tag v0.3.1

# ========== v0.3.2 ==========
cat > CHANGELOG.md << 'EOF'
# Changelog

## v0.3.2 (2025-12-05)

### Bug Fixes
- 修复旧版本已安装协议无法自动推断安装模式
- 修复升级后配置迁移兼容性问题

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
EOF
git add -A
GIT_AUTHOR_DATE="2025-12-05T11:00:00+08:00" GIT_COMMITTER_DATE="2025-12-05T11:00:00+08:00" git commit -m "v0.3.2: fix legacy protocol mode detection"
git tag v0.3.2

# ========== v1.0.0 ==========
cat > CHANGELOG.md << 'EOF'
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
EOF
git add -A
GIT_AUTHOR_DATE="2026-01-10T10:00:00+08:00" GIT_COMMITTER_DATE="2026-01-10T10:00:00+08:00" git commit -m "v1.0.0: Alpine support, BBR management, full uninstall"
git tag v1.0.0


# ========== v2.1.0 ==========
cat > CHANGELOG.md << 'EOF'
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
EOF
git add -A
GIT_AUTHOR_DATE="2026-03-18T10:00:00+08:00" GIT_COMMITTER_DATE="2026-03-18T10:00:00+08:00" git commit -m "v2.1.0: multi-domain support, Reality port fix, certificate improvements"
git tag v2.1.0

echo "=== All commits and tags created ==="
git log --oneline --all --decorate
echo ""
git tag -l
