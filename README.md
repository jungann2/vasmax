# VasmaX

<div align="center">

⚡ Xray-core / sing-box 十五合一管理脚本（Go 重构版）

**Codex / AI 长任务连接专项优化版：针对 WebSocket/WSS 场景下打开或提问时反复出现 `Reconnecting...1/5` 到 `Reconnecting...5/5` 的问题，强化 Nginx 长连接、TCP/WS keepalive、Clash TUN/UDP 订阅兼容和托管同步稳定性，降低空闲断流与反复重连概率。**

[![GitHub](https://img.shields.io/badge/GitHub-VasmaX-181717?logo=github)](https://github.com/jungann2/vasmax)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

</div>

---

## 🔗 配套项目

| 项目 | 说明 |
|------|------|
| [🛡️ Xboard-Plus](https://github.com/jungann2/Xboard-Plus) | 基于 Laravel 11 + Octane 的高性能面板系统，用户管理、订阅分发、流量统计一站式解决 |

> 💡 **VasmaX + Xboard-Plus 搭配使用，效果极佳。** VasmaX 负责节点部署、协议配置、证书管理，Xboard-Plus 负责用户管理、订阅分发、流量统计。两者对接后可实现：用户自动同步、流量实时统计、到期自动停用、多节点集中管理。从单机自用到多用户运营，一套方案全搞定。

---

## 📖 简介

VasmaX（V2ray Agent Service Management Assistant X）是一个基于 Go 语言重构的多协议代理服务管理工具，支持 Xray-core 和 sing-box 双核心，提供 15 种协议组合的一键安装与管理。

支持独立运行和 Xboard-Plus 面板托管两种模式。

## 支持协议（十五合一）

**Xray-core：**
- VLESS+TCP+TLS+Vision
- VLESS+WS+TLS
- VLESS+gRPC+TLS
- VLESS+Reality+Vision
- VLESS+Reality+gRPC
- VLESS+Reality+XHTTP
- VMess+WS+TLS
- VMess+HTTPUpgrade+TLS
- Trojan+TCP+TLS
- Trojan+gRPC+TLS

**sing-box：**
- Hysteria2
- Tuic
- NaiveProxy
- AnyTLS
- Socks5

## 功能特性

- 双核心支持：Xray-core + sing-box 同时运行
- 15 种协议组合一键安装管理
- Xboard-Plus 面板对接：用户同步、流量统计、在线追踪、运行时配置热更新
- 独立模式：无需面板，单机管理协议、用户和订阅
- 自动 TLS 证书申请与续订（acme.sh），支持证书状态检查、提供商切换和面板证书路径检测
- TLS 版本管理：支持设置最低/最高 TLS 版本（1.0 ~ 1.3）
- 订阅链接生成：通用 / Clash / sing-box 三类订阅入口，支持静态重新生成
- 订阅管理：订阅域名、远程订阅、客户端 DNS/测速配置
- 分流工具：WARP 分流、BT 下载管理、域名黑名单、路由规则查看
- CDN 节点管理（Cloudflare 优选 IP）
- 协议参数管理：Reality 伪装目标与密钥、Hysteria2 端口跳跃/上下行、TUIC 拥塞控制
- 额外端口管理：TCP/UDP/双协议端口开放、关闭与防火墙状态查看
- ALPN 协议切换：h2+http/1.1 / h2 only / http/1.1 only / h3 only / 全开模式
- 系统工具：Nginx 伪装站、健康检查、BBR+FQ、服务端 DNS、深度健康诊断
- 配置自动备份与回滚
- 多语言支持（中文 / English）
- 系统健康检查与资源监控

## 快速安装

### 第一步：更新系统并安装必要依赖

root 用户执行：
```bash
apt update -y && apt install -y curl socat wget
```

非 root 用户执行：
```bash
sudo apt update -y && sudo apt install -y curl socat wget
```

### 第二步：运行一键安装脚本

```bash
wget -P /root -N --no-check-certificate "https://raw.githubusercontent.com/jungann2/vasmax/main/install.sh" && chmod 700 /root/install.sh && /root/install.sh
```

### 使用

安装后，在命令行输入以下命令即可打开管理菜单：

```bash
vasmax
```

## 运行模式

### 独立模式

无需面板，直接在服务器上管理协议和用户，适合个人使用。

### Xboard-Plus 托管模式

对接 Xboard-Plus 面板，支持多用户管理、流量统计、到期自动停用等功能，适合机场运营。

## 系统要求

- 推荐系统：Debian 12 (Bookworm)，Ubuntu 22.04+ / CentOS 7+ / Alpine 也可使用
- 兼容系统：Ubuntu 16+ / Debian 8+ / CentOS 7+ / Alpine
- 架构：amd64 / arm64
- 内存：≥ 128MB
- 需要 root 权限

> Debian 推荐理由：VasmaX 的 BBR 加速管理直接操作内核参数（`/proc/sys`、`sysctl`），Debian 12 内核 6.1 LTS 原生支持 BBR，且系统干净轻量，无多余组件。Ubuntu 基于 Debian 同样兼容，CentOS/RHEL 需注意内核版本差异。

## 管理菜单

```text
1.  更新 VasmaX                    9.  ALPN 切换
2.  卸载 VasmaX                    10. 核心管理（启动/停止/重启 Xray 和 sing-box）
3.  安装管理（安装/卸载协议）       11. Xboard-Plus 对接管理
4.  账号管理                       12. TLS 证书管理
5.  分流工具                       13. 系统工具（BBR/DNS/Nginx 伪装站/健康诊断）
6.  CDN 管理                       14. 实时监控
7.  订阅管理                       15. VasmaX 服务管理（启动/停止/重启/状态/日志）
8.  额外端口管理                   16. 协议参数管理（Reality/Hysteria2/TUIC 专项参数）
0.  退出
```

> 💡 当前菜单已精简：原 BT 下载管理、域名黑名单独立入口已合并到「5 分流工具」子菜单；BBR、DNS、伪装站和诊断集中到「13 系统工具」。老用户请按新编号进入。

### 安装管理（选项 3）

- 安装 / 卸载 Xray-core 与 sing-box 协议组合
- 支持独立模式和 Xboard-Plus 托管模式
- 安装完成后生成单节点链接、二维码和订阅配置
- 协议配置失败会触发回滚，避免服务装一半不可用

### 分流工具（选项 5）

- WARP 分流管理：安装、配置、测试连接、卸载
- BT 下载管理：阻断或允许 BT 下载
- 域名黑名单管理：维护需要阻断的域名规则
- 查看当前路由规则

### CDN 管理（选项 6）

- Cloudflare 优选 IP 管理
- 启用 / 禁用 CDN 相关配置
- 配合订阅重新生成，保证客户端拿到最新入口

### 订阅管理（选项 7）

- 查看订阅链接：通用（v2ray/clash）、Clash、sing-box
- 重新生成静态订阅文件
- 设置订阅域名
- 远程订阅管理
- 订阅 DNS / 测速配置

订阅示例：

```text
https://你的订阅域名/s/<用户路径>/default
https://你的订阅域名/s/<用户路径>/clash
https://你的订阅域名/s/<用户路径>/singbox
```

> 新增 / 删除用户、安装 / 卸载协议、修改域名 / CDN / Reality / 订阅 DNS 后，建议执行「重新生成订阅」。

### 额外端口管理（选项 8）

- 开放新端口：TCP / UDP / TCP+UDP
- 关闭已开放端口
- 查看防火墙状态
- 自动写入配置并联动防火墙规则

### ALPN 切换（选项 9）

- h2 + http/1.1（默认，兼容性最好）
- 仅 h2（HTTP/2 专用）
- 仅 http/1.1（旧客户端兼容）
- 仅 h3（QUIC 类协议专用，如 Hysteria2 / TUIC）
- h2 + http/1.1 + h3（全开，客户端自动协商）

> h3 只对 QUIC 类协议有效；直连 TCP TLS / AnyTLS 会自动过滤不合适的 h3，WS/gRPC/HTTPUpgrade 由 Nginx 终结 TLS。

### 核心管理（选项 10）

- 更新 / 回滚 / 卸载 Xray-core
- 更新 / 回滚 / 卸载 sing-box
- 更新 GeoData
- 启动、重启或停止当前配置需要的核心
- 查看核心运行状态与版本

### Xboard-Plus 对接管理（选项 11）

- 启用 / 禁用 Xboard-Plus 托管模式
- 测试面板连接
- 修改面板地址、通信密钥、API 路径前缀、节点 ID、节点类型
- 支持托管用户同步、流量上报、在线状态上报与运行时配置更新

### TLS 证书管理（选项 12）

- 查看证书状态
- 申请证书（acme.sh）
- 手动续期证书
- 切换证书提供商
- 检测面板证书路径
- TLS 版本设置（最低 / 最高版本 1.0 ~ 1.3）

### 系统工具（选项 13）

- Nginx 伪装站管理：部署假网页，降低代理特征暴露
- 健康检查：快速检查核心、端口、证书、订阅和基础运行状态
- BBR 加速管理：启用 / 重载推荐 BBR + FQ、应用网络优化或恢复 cubic
- 服务端 DNS 配置：设置 Xray / sing-box 出站解析 DNS
- 深度健康诊断：检查配置、核心、网络、证书、DNS、Xboard 连通性等

### 实时监控（选项 14）

- 查看系统资源与服务运行状态
- 辅助观察核心运行、网络和健康状态

### VasmaX 服务管理（选项 15）

- 启动 VasmaX 主服务
- 停止 VasmaX 主服务
- 重启 VasmaX 主服务
- 查看 systemd 状态
- 查看最近 120 行日志

### 协议参数管理（选项 16）

- Reality：修改伪装目标 / SNI，查看或重新生成 Reality 密钥
- Hysteria2：端口跳跃、上下行速率配置
- TUIC：拥塞控制算法配置

## BBR 加速管理

进入路径：主菜单 → 13. 系统工具 → 3. BBR 加速管理

菜单顶部会显示当前内核版本、推荐组合状态、拥塞控制算法、队列调度、默认网卡 qdisc 和可用算法。

| 编号 | 功能 | 说明 |
|------|------|------|
| 1 | 启用 / 重载 BBR + FQ（推荐） | 写入并立即应用推荐拥塞控制和队列调度，重启后持续生效 |
| 2 | 重新应用默认网卡队列调度 FQ | 当 sysctl 已启用但网卡 qdisc 未完整确认时使用 |
| 3 | 应用网络优化 | 调整 conntrack、keepalive、连接数等系统参数 |
| 4 | 关闭 BBR 并恢复默认 cubic | 删除 VasmaX BBR / 优化配置，恢复默认 cubic |

> 当前 BBR 菜单走“推荐组合 + 状态诊断 + 一键恢复”路线，不再展示旧版 32 项内核安装表。需要更换系统内核时，建议先确认发行版、内核版本和 VPS 环境再操作。
## 致谢

本项目参考了以下开源项目，感谢原作者的贡献：

- [v2ray-agent](https://github.com/mack-a/v2ray-agent) - 原版八合一脚本
- [anytls-go](https://github.com/anytls/anytls-go) - AnyTLS 协议实现
- [Xray-core](https://github.com/XTLS/Xray-core) - VLESS/VMess/Trojan 核心
- [sing-box](https://github.com/SagerNet/sing-box) - Hysteria2/Tuic/Naive/AnyTLS 核心

## 许可证

本项目基于 [AGPL-3.0](LICENSE) 许可证开源。

---

# English

<div align="center">

⚡ 15-in-1 Xray-core / sing-box management script (Go rewrite)

</div>

---

## 🔗 Companion Project

| Project | Description |
|---------|-------------|
| [🛡️ Xboard-Plus](https://github.com/jungann2/Xboard-Plus) | High-performance panel system built on Laravel 11 + Octane, all-in-one user management, subscription distribution, and traffic statistics |

> 💡 **VasmaX + Xboard-Plus work best together.** VasmaX handles node deployment, protocol configuration, and certificate management. Xboard-Plus handles user management, subscription distribution, and traffic statistics. Together they enable: automatic user sync, real-time traffic stats, auto-disable on expiry, and centralized multi-node management. One solution from personal use to multi-user operations.

---

## 📖 Introduction

VasmaX (V2ray Agent Service Management Assistant X) is a multi-protocol proxy service management tool rewritten in Go, supporting Xray-core and sing-box dual-core with 15 protocol combinations for one-click installation and management.

Supports standalone mode and Xboard-Plus panel managed mode.

## Supported Protocols (15-in-1)

**Xray-core:**
- VLESS+TCP+TLS+Vision
- VLESS+WS+TLS
- VLESS+gRPC+TLS
- VLESS+Reality+Vision
- VLESS+Reality+gRPC
- VLESS+Reality+XHTTP
- VMess+WS+TLS
- VMess+HTTPUpgrade+TLS
- Trojan+TCP+TLS
- Trojan+gRPC+TLS

**sing-box:**
- Hysteria2
- Tuic
- NaiveProxy
- AnyTLS
- Socks5

## Quick Install

### Step 1: Update system and install dependencies

As root:
```bash
apt update -y && apt install -y curl socat wget
```

Non-root:
```bash
sudo apt update -y && sudo apt install -y curl socat wget
```

### Step 2: Run the install script

```bash
wget -P /root -N --no-check-certificate "https://raw.githubusercontent.com/jungann2/vasmax/main/install.sh" && chmod 700 /root/install.sh && /root/install.sh
```

### Usage

After installation, type the following command to open the management menu:

```bash
vasmax
```

## Operating Modes

### Standalone Mode

Manage protocols and users directly on the server without a panel. Ideal for personal use.

### Xboard-Plus Managed Mode

Connect to Xboard-Plus panel for multi-user management, traffic statistics, auto-disable on expiry, and more. Ideal for service operations.

## System Requirements

- Recommended: Debian 12 (Bookworm), Ubuntu 22.04+
- Compatible: Ubuntu 16+ / Debian 8+ / CentOS 7+ / Alpine
- Architecture: amd64 / arm64
- Memory: ≥ 128MB
- Root access required

## License

This project is licensed under [AGPL-3.0](LICENSE).
