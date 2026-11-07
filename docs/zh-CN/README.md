# Rovunet Panel 文档

[English](../en/README.md) · [فارسی](../fa/README.md) · [Русский](../ru/README.md) · **简体中文**

Rovunet Panel 将一台 Linux 服务器变成多协议接入服务器。它把每个协议作为 systemd 服务安装和管理，创建用户，并为每个用户输出可直接导入的客户端配置。

## 目录

| # | 页面 | 内容 |
|---|---|---|
| 1 | [快速开始](getting-started.md) | 要求、选择服务器、DNS 记录、安装、第一个隧道 |
| 2 | [协议](protocols.md) | 所有协议、传输方式、端口、域名布局、路由 |
| 3 | [用户与客户端配置](users-and-configs.md) | 用户模型、分享配置、导出格式、客户端应用 |
| 4 | [Xray 内核](xray-cores.md) | VLESS + REALITY、VLESS + XHTTP、Hysteria 2 和 WireGuard |
| 5 | [运维](operations.md) | 更新、备份与恢复、监控、WARP、文件与服务、卸载 |
| 6 | [命令参考](commands.md) | 所有命令及其选项 |
| 7 | [故障排查](troubleshooting.md) | 现象、原因与解决方法 |
| 8 | [流量限额与订阅链接](limits-and-subscriptions.md) | 不限量与限量用户、订阅链接、额外地址 |

## 约定

- 命令以 root 身份在服务器上运行，因此带有 `sudo`。
- `example.com` 表示你的域名，`SERVER_IP` 表示服务器的公网 IP。
- 协议名称沿用上游项目：VLESS、REALITY 和 XHTTP（Project X）、Hysteria 2、WireGuard、NaïveProxy、SSH 和 SOCKS5。DNSTT、NoizDNS、VayDNS、Slipstream 和 Soar 是 DNS 隧道。
- 在面板中，**隧道**是一个带标签（名称）的已配置协议实例，**内核**是一个用 `core-install` 安装的 Xray 服务器。

## 项目

- 源码：[github.com/msVPNco/rovunet-panel](https://github.com/msVPNco/rovunet-panel) · [更新日志](../../CHANGELOG.md) · [集成状态](../ROVUNET-STATUS.md)
- Android 应用：[github.com/msVPNco/rovunet-android](https://github.com/msVPNco/rovunet-android)
- 社区：[t.me/RovunetVPN](https://t.me/RovunetVPN)
- 基于 Anonvector 的 [SlipGate](https://github.com/anonvector/slipgate)，与其无隶属关系；参见 [NOTICE](../../NOTICE.md)。许可证：AGPL-3.0。
