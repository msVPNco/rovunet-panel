<p align="center">
  <img src="assets/rovunet-logo.png" alt="Rovunet" width="180">
</p>

<h1 align="center">Rovunet Panel</h1>

<p align="center">
  自托管的多协议抗审查服务器。<br>
  一次安装即可部署受支持的托管协议，并为每个用户生成客户端配置。
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.fa.md">فارسی</a> · <b>简体中文</b> · <a href="README.ru.md">Русский</a> ·
  <a href="https://t.me/RovunetVPN">Telegram</a> ·
  <a href="https://github.com/msVPNco/rovunet-android">Android 应用</a>
</p>

---

**Rovunet Panel 是一个开源、自托管的工具，用于在兼容的 Linux 服务器上部署和管理多协议互联网访问。** 它为每个用户提供多条连接路径，使访问不依赖于单一协议或某种固定的网络状况。

面板把托管协议的安装与配置、用户创建、客户端配置导出和 systemd 服务管理整合为带交互式菜单的命令行流程。可用路径包括 VLESS over WebSocket、NaiveProxy、直接 SSH、SSH over TLS、SOCKS5，DNSTT、NoizDNS、VayDNS、Slipstream 和 Soar 等 DNS 隧道系列，以及 Xray 内核（VLESS REALITY、VLESS XHTTP、Hysteria2 和 WireGuard）。诊断和备份工具用于日常维护和故障排查。

面板与 [Rovunet Android 应用](https://github.com/msVPNco/rovunet-android) 配合使用：原生导出格式可以随相关连接路径一起携带集合元数据。应用通过实际隧道测试连通性，并在进入 DNS 回退之前征求同意。其他兼容客户端可以使用标准 VLESS 链接。

**支持范围：** Xray 内核由 `rovunet core-install` 安装和管理（`core-plan` 仍只生成用于手动部署的文件）。连接质量和可用性取决于网络、服务器配置和所选协议。已可用功能与剩余工作的区别见 [集成状态与开发优先级](docs/ROVUNET-STATUS.md)。

> **项目状态：早期版本，持续开发中。** 正式使用前，请在自己的环境中测试所需协议，并用 `rovunet backup` 备份配置。

## 快速安装

在全新的 Ubuntu 20.04+ 或 Debian 10+ 服务器上以 root 运行：

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

安装程序会询问要启用哪些协议，完成配置，创建第一个用户并输出该用户的全部配置。之后：

```bash
sudo rovunet                # 交互式菜单
sudo rovunet users add      # 添加用户
sudo rovunet users configs  # 再次输出所有用户的配置
sudo rovunet diag           # 检查服务、端口、密钥和 DNS
```

从 SlipGate 迁移？请参阅 [切换服务器](docs/GUIDE.md#quick-start)；
配置、隧道和用户都会保留。

## 协议

| 协议 | 传输方式 | 端口 | 适用场景 |
|---|---|---|---|
| **NaiveProxy 前置的 VLESS** | Caddy 后的 WebSocket，使用真实证书 | 443/tcp | 首选：快速，看起来像普通 HTTPS 网站 |
| **NaiveProxy** | 带伪装网站的 HTTP/2 代理 | 443/tcp | 到你的域名的 TLS 可用时 |
| **SSH over TLS（StunTLS）** | TLS/WebSocket 内的 SSH，前置于 443 | 443/tcp | 同一端口上 VLESS 的替代方案 |
| **直接 SSH** | 普通 SSH | 22/tcp | 22 端口开放时速度快 |
| **Cloudflare 后的 VLESS** | 经 Cloudflare CDN 的 WebSocket | 2053/tcp | 服务器 IP 被封锁时；较慢 |
| **DNSTT / NoizDNS** | 加密 DNS 隧道 | 53/udp | 用于受限网络；性能取决于解析器和路径 |
| **VayDNS** | 线路格式可调的 DNS 隧道 | 53/udp | 类似 DNSTT，可调选项更多 |
| **Slipstream** | DNS 上的 QUIC | 53/udp | 实验性 |
| **Soar** | 带解析器发现的多路径 DNS | 53/udp | DNS 解析器被不均匀过滤时 |
| **SOCKS5** | 普通 SOCKS5 | 1080/tcp | 仅限开放网络（不加密） |
| **VLESS REALITY**（Xray 内核） | 看起来像真实网站的 TLS，无需证书 | 任意空闲 TCP 端口 | 到知名网站的 TLS 可以通过时 |
| **VLESS XHTTP**（Xray 内核） | 基于 HTTP 请求的 VLESS，真实证书 | 任意空闲 TCP 端口 | 在 CDN 后，或 WebSocket 被封锁时 |
| **Hysteria2**（Xray 内核） | 自带拥塞控制的 QUIC，真实证书 | 任意空闲 UDP 端口 | UDP 可通过的高丢包或不稳定网络 |
| **WireGuard**（Xray 内核） | 标准 WireGuard，每台设备一个文件 | 任意空闲 UDP 端口 | WireGuard 未被封锁时 |

协议是否合适取决于服务器、运营商和当前网络。以上是使用建议，不代表可用性保证或对比测试结果。Rovunet 应用会测试连接路径。

Xray 内核作为独立服务与隧道并行运行：

```bash
sudo rovunet core-install --name r1 --protocol reality --host SERVER_IP \
  --target www.microsoft.com:443 --sni www.microsoft.com --port 8443 --devices 5
sudo rovunet core-list
sudo rovunet core-remove r1
```

安装前先检查端口是否空闲；Xray 按发布的校验和进行验证，并在上线前测试配置；
随后开放 TCP 或 UDP 端口。每台设备在 `/etc/rovunet/core/<名称>/clients/` 下获得
自己的文件。XHTTP 和 Hysteria2 需要用 `--cert` 和 `--key` 提供对 `--sni` 有效的证书。

## 客户端应用

[Rovunet Android 发布页](https://github.com/msVPNco/rovunet-android/releases) 包含已发布的安装包和源码资源。安装前请阅读应用的 [当前状态](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md) 以及签名/迁移说明；私有候选版本不是公开版本的原位更新。另见 [集成状态](docs/ROVUNET-STATUS.md)。

- **[Rovunet for Android](https://github.com/msVPNco/rovunet-android)**
  （推荐）：导入 `rovunet://` 链接，将每个用户的路径归入一个集合，
  无需预先 ping 即可连接所选路径，并提供独立的真实隧道 HTTPS 测试。
  [下载最新版本](https://github.com/msVPNco/rovunet-android/releases)。
- **标准客户端**，如 v2rayNG、Hiddify 或 NekoBox：使用面板在每个 VLESS
  配置旁输出的 `vless://` 链接。

```bash
sudo rovunet export-clients --user USER --format rovunet
```

## 要求

- 具有 root 权限的 Linux 服务器（Ubuntu 20.04+、Debian 10+ 或类似系统）
- 一个由你控制 DNS 并指向该服务器的域名（参见
  [域名布局](docs/GUIDE.md#domain-layout)）
- 为所启用协议开放的端口：至少 53/udp、80/tcp 和 443/tcp

## 文档

- [指南](docs/GUIDE.md)：安装、协议、域名布局和完整命令行
- [用户指南](docs/USER_GUIDE.md)（英文和波斯文）
- [波斯文 VLESS 指南](docs/VLESS_FA.md)
- [导出说明](ROVUNET-EXPORT.txt) 和 [Xray 说明](VEYRA-MANAGER.md)
- [更新日志](CHANGELOG.md)

## 支持

Rovunet 是无偿的志愿工作。如果它对你有帮助，欢迎捐助：

- **USDT（Tron，TRC20）：** `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ`

只向此地址发送 Tron（TRC20）网络上的 USDT。

问题与动态：[t.me/RovunetVPN](https://t.me/RovunetVPN)

## 许可证

AGPL-3.0。Rovunet Panel 基于 Anonvector 的
[SlipGate](https://github.com/anonvector/slipgate)，与其没有隶属关系；参见
[NOTICE](NOTICE.md)。命令为 `rovunet`；以旧名称安装的服务器在更新时会自动迁移。
