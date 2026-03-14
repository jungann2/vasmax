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
- TLS 版本管理：支持设置最低/最高 TLS 版本（1.0 ~ 1.3）
- 订阅链接生成（通用 / Clash / sing-box 格式）
- 订阅管理：Salt 配置、域名设置、链接预览
- 分流管理：WARP、IPv6、Socks5、DNS、SNI 反向代理
- CDN 节点管理（Cloudflare 优选 IP）
- 域名黑名单 / BT 下载管理
- Hysteria2 端口跳跃与限速
- Reality 密钥管理
- 额外端口管理：TCP/UDP/双协议端口开放与防火墙联动
- ALPN 协议切换：h2+http/1.1 / h2 only / http/1.1 only / h3 only
- BBR 加速管理：32 项功能覆盖内核安装、加速启用、系统优化、内核管理
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

- 推荐系统：Debian 12 (Bookworm)，Ubuntu 22.04+ / CentOS 7+ / Alpine 也可使用
- 兼容系统：Ubuntu 16+ / Debian 8+ / CentOS 7+ / Alpine
- 架构：amd64 / arm64
- 内存：≥ 128MB
- 需要 root 权限

> Debian 推荐理由：VasmaX 的 BBR 加速管理直接操作内核参数（`/proc/sys`、`sysctl`），Debian 12 内核 6.1 LTS 原生支持 BBR，且系统干净轻量，无多余组件。Ubuntu 基于 Debian 同样兼容，CentOS/RHEL 需注意内核版本差异。

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

### 订阅管理（选项 7）

- Salt 配置管理
- 订阅域名设置
- 订阅链接预览（通用 / Clash / sing-box 格式）

### 额外端口管理（选项 8）

- 添加/删除额外端口（TCP/UDP/双协议）
- 查看已开放端口列表
- 自动联动防火墙规则

### ALPN 切换（选项 9）

- h2 + http/1.1（默认）
- h2 only
- http/1.1 only
- h3 only

### TLS 证书管理（选项 12）

- 证书申请与续订（acme.sh）
- 证书状态查看
- 手动指定证书路径
- 证书自动续订管理
- 强制续订
- TLS 版本设置（最低/最高版本 1.0 ~ 1.3）

### 其他工具（选项 13）

- CDN 管理（启用/禁用/预设列表）
- 伪装站管理（预设模板/自定义 URL）
- 健康检查
- BBR 加速管理（32 项功能，详见下方）
- 卸载 VasmaX

## BBR 加速管理

进入路径：主菜单 → 13. 其他工具 → 4. BBR 加速管理

菜单顶部显示当前内核版本、拥塞控制算法、队列调度和可用算法。

共 32 项功能，分为四大类：

### 内核安装类（需要重启）

| 编号 | 功能 | 说明 |
|------|------|------|
| 1 | 安装 BBR 原版内核 | Debian/Ubuntu/CentOS/RHEL |
| 2 | 安装 BBRplus 版内核 | 仅 Debian/Ubuntu |
| 3 | 安装 Lotserver（锐速）内核 | 全平台 |
| 4 | 安装 BBRplus 新版内核 | 仅 Debian/Ubuntu |
| 5 | 安装 Zen 官方版内核 | 仅 Debian/Ubuntu |
| 6 | 安装官方 cloud 内核 | Debian/Ubuntu/CentOS |
| 7 | 安装官方稳定内核 | 全平台 |
| 8 | 安装官方最新内核 | 全平台 |
| 9 | 安装 XANMOD-main 内核 | 仅 Debian/Ubuntu |
| 10 | 安装 XANMOD-LTS 内核 | 仅 Debian/Ubuntu |
| 11 | 安装 XANMOD-EDGE 内核 | 仅 Debian/Ubuntu |
| 12 | 安装 XANMOD-RT 内核 | 仅 Debian/Ubuntu |

### 加速启用类（无需重启）

| 编号 | 功能 | 拥塞控制 | 队列调度 |
|------|------|----------|----------|
| 13 | BBR + FQ（推荐） | bbr | fq |
| 14 | BBR + FQ_PIE | bbr | fq_pie |
| 15 | BBR + CAKE | bbr | cake |
| 16 | BBR2 + FQ | bbr2 | fq |
| 17 | BBR2 + FQ_PIE | bbr2 | fq_pie |
| 18 | BBR2 + CAKE | bbr2 | cake |
| 19 | BBRplus + FQ | bbrplus | fq |
| 20 | Lotserver（锐速）加速 | — | — |
| 21 | 编译安装 brutal 模块 | brutal | — |

### 系统配置类

| 编号 | 功能 |
|------|------|
| 22 | 开启 ECN |
| 23 | 关闭 ECN |
| 24 | 系统配置优化（旧方案） |
| 25 | 系统配置优化（新方案，含更多 sysctl 调优） |
| 26 | 禁用 IPv6 |
| 27 | 开启 IPv6 |
| 28 | 手动提交合并内核参数 |
| 29 | 手动编辑内核参数 |

### 内核管理类

| 编号 | 功能 |
|------|------|
| 30 | 查看已安装内核列表 |
| 31 | 删除/保留指定内核 |
| 32 | 卸载全部加速配置 |

## 致谢

本项目参考了以下开源项目，感谢原作者的贡献：

- [v2ray-agent](https://github.com/mack-a/v2ray-agent) - 原版八合一脚本
- [anytls-go](https://github.com/anytls/anytls-go) - AnyTLS 协议实现
- [Xray-core](https://github.com/XTLS/Xray-core) - VLESS/VMess/Trojan 核心
- [sing-box](https://github.com/SagerNet/sing-box) - Hysteria2/Tuic/Naive/AnyTLS 核心

## 许可证

本项目基于 [AGPL-3.0](LICENSE) 许可证开源。
