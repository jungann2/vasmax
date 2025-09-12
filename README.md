# VasmaX

AnyTLS / VLESS / VMess / Reality / gRPC / XHTTP 多协议代理节点管理工具

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Release](https://img.shields.io/github/v/release/jungann2/vasmax)](https://github.com/jungann2/vasmax/releases)

## 简介

VasmaX（V2ray Agent Service Management Assistant X）是基于 Go 语言构建的多协议代理服务管理工具，支持 Xray-core 和 sing-box 双核心，提供 15 种协议组合的一键安装与管理。

支持独立运行和 Xboard 面板托管两种模式。

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
- AnyTLS（推荐）
- Hysteria2
- Tuic
- NaiveProxy
- Socks5

## 功能特性

- 🚀 双核心支持：Xray-core + sing-box 同时运行
- 🔐 15 种协议组合一键安装管理
- 🌐 多域名支持：每个协议可绑定不同域名
- 📡 Xboard 面板对接：用户同步、流量统计、在线追踪
- 🔒 自动 TLS 证书申请与续订（acme.sh）
- 📊 系统健康检查与资源监控
- ⚡ BBR 加速管理：32 项功能覆盖内核安装、加速启用、系统优化
- 🔄 配置自动备份与回滚
- 🌍 多语言支持（中文 / English）

## 快速安装

### 第一步：更新系统并安装依赖

```bash
apt update -y && apt install -y curl socat wget
```

### 第二步：一键安装

```bash
wget -P /root -N --no-check-certificate "https://raw.githubusercontent.com/jungann2/vasmax/main/install.sh" && chmod 700 /root/install.sh && /root/install.sh
```

### 使用

安装后输入以下命令打开管理菜单：

```bash
vasmax
```

## 管理菜单

```
1.  安装管理        8.  额外端口管理
2.  账号管理        9.  ALPN 切换
3.  分流工具        10. 核心管理
4.  BT 下载管理     11. Xboard 对接管理
5.  域名黑名单      12. TLS 证书管理
6.  CDN 管理        13. 其他工具
7.  订阅管理        0.  退出
```

## 运行模式

### 独立模式
无需面板，直接在服务器上管理协议和用户，适合个人使用。

### Xboard 托管模式
对接 Xboard 面板，支持多用户管理、流量统计、到期自动停用等功能，适合机场运营。

## 系统要求

- 推荐系统：Debian 12 / Ubuntu 22.04+
- 兼容系统：Ubuntu 16+ / Debian 8+ / CentOS 7+ / Alpine
- 架构：amd64 / arm64
- 内存：≥ 128MB
- 需要 root 权限

## 致谢

- [v2ray-agent](https://github.com/mack-a/v2ray-agent) - 原版八合一脚本
- [anytls-go](https://github.com/anytls/anytls-go) - AnyTLS 协议实现
- [Xray-core](https://github.com/XTLS/Xray-core) - VLESS/VMess/Trojan 核心
- [sing-box](https://github.com/SagerNet/sing-box) - Hysteria2/Tuic/Naive/AnyTLS 核心

## 许可证

本项目基于 [AGPL-3.0](LICENSE) 许可证开源。
