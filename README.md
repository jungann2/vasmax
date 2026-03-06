# VasmaX

Xray-core / sing-box 十五合一管理脚本（Go 重构版）

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

## 简介

VasmaX（V2ray Agent Service Management Assistant X）是一个基于 Go 语言重构的多协议代理服务管理工具，支持 Xray-core 和 sing-box 双核心，提供 15 种协议组合的一键安装与管理。

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
- Hysteria2
- Tuic
- NaiveProxy
- AnyTLS
- Socks5

## 功能特性

- 双核心支持：Xray-core + sing-box 同时运行
- 15 种协议组合一键安装管理
- Xboard 面板对接：用户同步、流量统计、在线追踪
- 独立模式：无需面板，单机运行
- 自动 TLS 证书申请与续订（acme.sh）
- 订阅链接生成（通用 / Clash / sing-box 格式）
- 分流管理：WARP、IPv6、Socks5、DNS、SNI 反向代理
- CDN 节点管理（Cloudflare 优选 IP）
- 域名黑名单 / BT 下载管理
- Hysteria2 端口跳跃与限速
- Reality 密钥管理
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

### Xboard 托管模式

对接 Xboard 面板，支持多用户管理、流量统计、到期自动停用等功能，适合机场运营。

## 系统要求

- 操作系统：Ubuntu 16+ / Debian 8+ / CentOS 7+ / Alpine
- 架构：amd64 / arm64
- 内存：≥ 128MB
- 需要 root 权限

## 管理菜单

```
1.  安装管理        8.  额外端口管理
2.  账号管理        9.  ALPN 切换
3.  分流工具        10. 核心管理
4.  BT 下载管理     11. Xboard 对接管理
5.  域名黑名单      12. TLS 证书管理
6.  CDN 管理        13. 其他工具
7.  订阅管理
```

## 致谢

本项目参考了以下开源项目，感谢原作者的贡献：

- [v2ray-agent](https://github.com/mack-a/v2ray-agent) - 原版八合一脚本
- [anytls-go](https://github.com/anytls/anytls-go) - AnyTLS 协议实现
- [Xray-core](https://github.com/XTLS/Xray-core) - VLESS/VMess/Trojan 核心
- [sing-box](https://github.com/SagerNet/sing-box) - Hysteria2/Tuic/Naive/AnyTLS 核心

## 许可证

本项目基于 [AGPL-3.0](LICENSE) 许可证开源。
