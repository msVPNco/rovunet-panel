<p align="center"><img src="assets/rovunet-logo.png" alt="Rovunet" width="140"></p>

<h1 align="center">Rovunet Panel</h1>

<p align="center"><b>自托管的多协议抗封锁服务器</b><br>一次安装，管理全部协议，为每位用户提供现成配置和订阅链接。</p>

<p align="center">
  <a href="https://github.com/msVPNco/rovunet-panel/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/msVPNco/rovunet-panel?sort=date&display_name=tag&label=release&color=0E7C86"></a>
  <img alt="Platform" src="https://img.shields.io/badge/platform-Linux-14304D">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-6D4AFF"></a>
  <a href="https://t.me/RovunetVPN"><img alt="Telegram" src="https://img.shields.io/badge/Telegram-RovunetVPN-2F6FEB?logo=telegram&logoColor=white"></a>
</p>

<p align="center"><a href="README.md">English</a> · <a href="README.fa.md">فارسی</a> · <a href="README.ru.md">Русский</a> · <b>简体中文</b></p>

<p align="center">📘 <a href="docs/guides/Rovunet-Panel-Guide-ZH.pdf">完整指南（PDF）</a> · <a href="docs/zh-CN/README.md">文档</a> · <a href="https://github.com/msVPNco/rovunet-app">Android 应用</a> · <a href="CHANGELOG.md">更新日志</a></p>

---

Rovunet Panel 是一个开源工具，可将 Linux 服务器变成多协议接入服务器。它把每个协议作为 systemd 服务安装和管理，创建用户并导出客户端配置，使每个用户拥有多条相互独立的连接路径，而不必依赖单一协议或某种网络状况。

## 特性

- **统一管理代理、SSH、DNS 隧道和协议内核**，包括 AmneziaWG、Shadowsocks 2022、Trojan TLS 和 VLESS Encryption，提供交互式命令行菜单。
- **在支持认证的路径上管理独立用户凭据**，四种新增协议支持定向用户更新。直连 Soar 使用共享密钥。
- **可直接导入的配置**：供 [Rovunet 应用](https://github.com/msVPNco/rovunet-app) 使用的原生 `rovunet://` 链接，以及供其他客户端使用的标准 `vless://`、`hysteria2://` 链接和 WireGuard 文件。
- **443 端口前置**：VLESS 和 SSH over TLS 与 NaïveProxy 共用域名、端口和 Let's Encrypt 证书，隐藏在伪装网站之后。
- **安全运维**：带校验和与回滚的更新、部署前验证配置、整机备份与恢复。
- **诊断**：实时仪表盘、健康检查、证书到期提示和各服务日志。
- **流量限额与订阅链接**：不限量或限量用户（天数和/或 GB，每 5 分钟检查），每个用户一条订阅链接，包含其全部配置，可导入应用的订阅菜单。
- **同一服务器的多个地址**：添加服务器的其他 IP，链接会为每个地址各提供一份依赖地址的配置，单个 IP 被封不会让用户断线。
- **REALITY 目标自动选择**：伪装站点从一批高流量、很少被封锁的网站中选取。全新安装还会在另一个端口上添加第二个 REALITY 内核，伪装成另一家公司的网站，让用户有两条入口。
- **固定订阅地址与清晰名称**：常规更新保留原有令牌和配置 ID。Android 使用 `<token>.json`，Windows 使用 `<token>.windows.json`，分别提供客户端支持的协议。更新已保存的订阅即可获得新的显示名称。

## 协议

| 协议 | 承载方式 | 端口 | 适用场景 |
|---|---|---|---|
| **VLESS over WebSocket**（NaïveProxy 前置） | WebSocket + TLS，真实证书 | 443/tcp | HTTPS 可用时的首选 |
| **VLESS over WebSocket**（Cloudflare 后） | 经 Cloudflare CDN 的 WebSocket | 2053/tcp | 服务器 IP 被封锁时 |
| **NaïveProxy** | 带伪装网站的 HTTP/2 + TLS | 443/tcp | 到你的域名的 TLS 可用时 |
| **SSH over TLS**（StunTLS） | TLS / WebSocket 内的 SSH | 443/tcp | 同一端口上的替代方案 |
| **SSH** | 普通 SSH | 22/tcp | 22 端口开放时 |
| **SOCKS5** | 普通 SOCKS5，不加密 | 1080/tcp | 仅限开放网络 |
| **DNSTT** / **NoizDNS** | 加密 DNS 隧道 | 53/udp | DNS 能通过的受限网络 |
| **VayDNS** | 线路格式可调的 DNS 隧道 | 53/udp | 类似 DNSTT，可调项更多 |
| **Slipstream** | DNS 上的 QUIC | 53/udp | 实验性 |
| **Soar** | 带解析器发现的多路径 DNS | 53/udp | 解析器被不均匀过滤时 |
| **VLESS + REALITY**（Xray） | TCP，模仿真实 TLS 网站 | 任意 TCP | 到知名网站的 TLS 能通过时 |
| **VLESS + XHTTP**（Xray） | 基于 TLS 的 HTTP 请求 | 任意 TCP | 在 CDN 后，或 WebSocket 被封锁时 |
| **Hysteria 2**（Xray） | QUIC | 任意 UDP | UDP 可通过的高丢包网络 |
| **WireGuard**（Xray） | UDP | 任意 UDP | WireGuard 未被封锁时 |
| **AmneziaWG** | 用户空间 UDP 隧道 | 任意 UDP | WireGuard 的可选替代方案 |
| **Shadowsocks 2022**（Xray） | TCP 和 UDP | 任意 TCP/UDP | 可选兼容协议；固定版本 Xray 已标记弃用 |
| **Trojan TLS**（Xray） | TCP + 受信任证书的 TLS | 任意 TCP | 可选 TLS 兼容；固定版本 Xray 已弃用无 Flow 的 Trojan |
| **VLESS Encryption**（Xray） | TCP + ML-KEM/X25519 加密 | 任意 TCP | 支持固定加密格式的客户端 |

协议是否合适取决于服务器、运营商和网络；以上为建议而非保证。详见 [协议](docs/zh-CN/protocols.md)。

**v1.21.0：**已测试的完整配置提供 **24 个 Android 配置和 22 个 Windows 配置**，并非 24 种独立协议。数量取决于已安装服务及账户资格。已发布的 Windows 客户端不接收 AmneziaWG 和 SS2022。四种新增协议均不提供给流量受限账户；三种新增 Xray 协议在到期或删除后拒绝新会话，但现有流可能继续。尚未实现精确的同时在线设备限制。参见[托管协议与兼容性](docs/MANAGED-PROTOCOLS.md)。

## 快速开始

在全新的 Ubuntu 20.04+ 或 Debian 10+ 服务器上以 root 运行：

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

安装程序会询问要启用哪些协议，完成配置，创建第一个用户并输出其配置。之后：

```bash
sudo rovunet                 # interactive menu
sudo rovunet users add       # add a user
sudo rovunet users configs   # print every user's configurations
sudo rovunet diag            # check services, ports, keys and DNS
```

要求：root 权限、一个由你控制 DNS 的域名，以及为所启用协议开放的端口（至少 53/udp、80/tcp 和 443/tcp）。参见 [快速开始](docs/zh-CN/getting-started.md)。

## 文档

| | |
|---|---|
| 📘 [完整指南（PDF）](docs/guides/Rovunet-Panel-Guide-ZH.pdf) | 图解指南：架构、协议选择、安装流程、用户与限额、运维。另有 [English](docs/guides/Rovunet-Panel-Guide-EN.pdf) · [فارسی](docs/guides/Rovunet-Panel-Guide-FA.pdf) · [Русский](docs/guides/Rovunet-Panel-Guide-RU.pdf) |
| [快速开始](docs/zh-CN/getting-started.md) | 要求、DNS 记录、安装、第一个隧道 |
| [协议](docs/zh-CN/protocols.md) | 所有协议、端口、域名布局、路由 |
| [用户与配置](docs/zh-CN/users-and-configs.md) | 用户、分享、导出格式、客户端应用 |
| [限额与订阅](docs/zh-CN/limits-and-subscriptions.md) | 流量与时间限制、订阅链接、额外地址 |
| [Xray 内核](docs/zh-CN/xray-cores.md) | VLESS + REALITY、VLESS + XHTTP、Hysteria 2、WireGuard |
| [运维](docs/zh-CN/operations.md) | 更新、备份与恢复、监控、文件与服务 |
| [命令参考](docs/zh-CN/commands.md) | 所有命令和选项 |
| [故障排查](docs/zh-CN/troubleshooting.md) | 现象、原因与解决方法 |

> **状态：** 早期版本，持续开发中。依赖之前请在自己的环境中测试所需协议，并保留备份（`sudo rovunet backup`）。参见 [集成状态](docs/ROVUNET-STATUS.md)。

## 支持

Rovunet 是无偿的志愿工作，欢迎捐助：向 `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ` 发送 **Tron（TRC20）网络上的 USDT**（只发送 TRC20 网络上的 USDT）。问题与动态：[t.me/RovunetVPN](https://t.me/RovunetVPN)。

## 许可证

AGPL-3.0。Rovunet Panel 基于 Anonvector 的 [SlipGate](https://github.com/anonvector/slipgate)，与其无隶属关系；上游及第三方组件的署名见 [NOTICE](NOTICE.md)。
