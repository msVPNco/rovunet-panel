# Rovunet Panel 2.1.0 文档

[English](../en/README.md) · [فارسی](../fa/README.md) · [Русский](../ru/README.md) · **简体中文**

Rovunet Panel 将一台 Linux 服务器变成多协议接入服务器。它把每个协议作为 systemd 服务安装和管理，创建用户，并为每个用户输出可直接导入的客户端配置。

## 当前正式版：2.1.0

[正式版下载](https://github.com/msVPNco/rovunet-panel/releases/tag/v2.1.0) · [固定 v2.1.0 安装与 SHA-256 校验](../../README.zh-CN.md) · [正式版本说明与边界](../releases/v2.1.0.md)

本版基于公开的 2.0.2，全新安装使用以下默认值：

| 协议 | 端口 | 变化 |
|---|---|---|
| Shadowsocks 2022 AES-128 — `2022-blake3-aes-128-gcm` | 51820 TCP + UDP | 替换默认 WireGuard |
| 经典 Shadowsocks AEAD ChaCha20 — `chacha20-ietf-poly1305` | 33444 TCP + UDP | 替换默认 TUIC |
| Shadowsocks 2022 AES-256 | 28444 TCP + UDP | 保留 |
| AmneziaWG | 51840 UDP | 保留 |

升级时，替代协议沿用旧核心已保存的端口；其他核心、账户、已记录流量和订阅 URL 保留。请在应用中刷新原有订阅；被替换的两个协议使用新的凭据。受管理的引擎继续固定使用已认证的 v2.0.2 签名资产。

菜单 **3 — Users → 2 — Free config (50 GB / 30 days)** 新建共享 **50 GiB / 30 天**额度的独立账户。**Users → 3 — Export Free config for review** 可重试私有导出，路径为 `/etc/rovunet/donations/<username>.json`。导出要求计量正常且额度未耗尽；三个 Shadowsocks 方法均不向限流量账户开放。应用中的 Free 目录和标签页尚未发布，导出文件仍是私有访问凭据。

[源码 CI](https://github.com/msVPNco/rovunet-panel/actions/runs/45071747305)、[准备流程](https://github.com/msVPNco/rovunet-panel/actions/runs/45072135345)和[最终验收与发布](https://github.com/msVPNco/rovunet-panel/actions/runs/45076586381)均已通过。最终签名验收在 Linux amd64 隔离命名空间中运行，systemd 使用模拟器，覆盖安装器 `--no-run` 和官方 **2.0.2 → 2.1.0** 的 Update 菜单。ARM64 字节经过验证但未执行；私有 2.0.3 未执行该升级验收。此项验收不覆盖完整首次交互配置或 Caddy/Slipstream 实际代理流量。详见[测试范围](../releases/v2.1.0-test-scope.txt)。

安装、新默认值及 Free config 请以上述链接为准。下面的专题指南也保留了此前支持的协议说明；目录中出现某个协议，并不表示它仍是 2.1.0 的默认项。

## 目录

| # | 页面 | 内容 |
|---|---|---|
| 1 | [快速开始](getting-started.md) | 要求、选择服务器、DNS 记录、安装、第一个隧道 |
| 2 | [协议](protocols.md) | 所有协议、传输方式、端口、域名布局、路由 |
| 3 | [用户与客户端配置](users-and-configs.md) | 用户模型、分享配置、导出格式、客户端应用 |
| 4 | [Xray 内核](xray-cores.md) | REALITY、XHTTP、Hysteria 2 和现有核心设置；2.1.0 的变化见上文 |
| 5 | [运维](operations.md) | 更新、备份与恢复、监控、WARP、文件与服务、卸载 |
| 6 | [命令参考](commands.md) | 所有命令及其选项 |
| 7 | [故障排查](troubleshooting.md) | 现象、原因与解决方法 |
| 8 | [流量限额与订阅链接](limits-and-subscriptions.md) | 不限量与限量用户、订阅链接、额外地址 |

## 约定

- 命令以 root 身份在服务器上运行，因此带有 `sudo`。
- `example.com` 表示你的域名，`SERVER_IP` 表示服务器的公网 IP。
- 协议名称沿用上游项目：VLESS、REALITY 和 XHTTP（Project X）、Shadowsocks、Hysteria 2、AmneziaWG、NaïveProxy、SSH 和 SOCKS5。DNSTT、NoizDNS、VayDNS、Slipstream 和 Soar 是 DNS 隧道。
- 在面板中，**隧道**是一个带标签（名称）的已配置协议实例，**内核**是由 `core-install` 管理的协议服务实例。

## 项目

- 源码：[github.com/msVPNco/rovunet-panel](https://github.com/msVPNco/rovunet-panel) · [更新日志](../../CHANGELOG.md) · [集成状态](../ROVUNET-STATUS.md)
- Android 应用：[github.com/msVPNco/rovunet-app](https://github.com/msVPNco/rovunet-app)
- 社区：[t.me/RovunetVPN](https://t.me/RovunetVPN)
- 基于 Anonvector 的 [SlipGate](https://github.com/anonvector/slipgate)，与其无隶属关系；参见 [NOTICE](../../NOTICE.md)。许可证：AGPL-3.0。
