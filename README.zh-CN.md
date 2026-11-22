# Rovunet Panel 2.1.0

[English](README.md) · [فارسی](README.fa.md) · [Русский](README.ru.md) · **简体中文**

[v2.1.0 正式版](https://github.com/msVPNco/rovunet-panel/releases/tag/v2.1.0) · [版本说明与验证范围](docs/releases/v2.1.0.md) · [文档目录](docs/zh-CN/README.md)

2.1.0 基于公开的 2.0.2。两个默认协议已替换：WireGuard 改为 Shadowsocks 2022 AES-128，TUIC 改为经典 Shadowsocks AEAD ChaCha20-Poly1305；用户菜单新增 Free config。原有 AmneziaWG 和 Shadowsocks 2022 AES-256 保留。

## 安装：Ubuntu 24.04、systemd、root

下载固定版本的安装脚本，先核对 SHA-256 再运行。`&&` 确保下载或校验失败时停止安装。安装器继续使用原有生产信任根验证发布文件的签名。

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v2.1.0/install.sh -o rovunet-install.sh &&
python3 -c 'import hashlib,pathlib,sys; sys.exit(0 if hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="4ecf359cd96cfb8583ad89bdae5bfe4e14bfe1ee4dc12cedd603b77ec7519555" else "Installer checksum mismatch")' &&
sudo env ROVUNET_RELEASE_TAG=v2.1.0 bash rovunet-install.sh
```

## 协议与端口

下表列出全新安装的首选端口。升级时，替代协议沿用旧核心已保存的端口，包括自定义端口；其他核心及其端口保留。补装缺少的核心时按固定顺序选择备用端口，不随机分配。

| 配置组 | 默认端口 | 适用账户 |
|---|---|---|
| VLESS TLS Vision、REALITY 1/2、XHTTP REALITY、XHTTP TLS | 36444、9443、2087、37444、2096 TCP | 无限制、仅限时、限流量 |
| VLESS WS TLS | 443 TCP，经 NaiveProxy 前端 | 无限制、仅限时、限流量 |
| Hysteria 2（Salamander） | 8443 UDP | 无限制、仅限时 |
| Mieru TCP / UDP | 34444 TCP / 35444 UDP | 无限制、仅限时 |
| AnyTLS | 32444 TCP | 无限制、仅限时 |
| **Shadowsocks 2022 AES-128** — `2022-blake3-aes-128-gcm` | **51820 TCP + UDP** | 无限制、仅限时 |
| **Shadowsocks AEAD ChaCha20** — `chacha20-ietf-poly1305` | **33444 TCP + UDP** | 无限制、仅限时 |
| Shadowsocks 2022 AES-256 | 28444 TCP + UDP | 无限制、仅限时 |
| VLESS Encryption / Trojan TLS | 31444 / 30444 TCP | 无限制、仅限时 |
| AmneziaWG | 51840 UDP | 无限制、仅限时 |
| NaiveProxy SOCKS5 / SSH；StunTLS SSH；直连 SSH | 443 TCP；直连 SSH 为 22 TCP | 无限制、仅限时 |
| Soar Direct / SSH | 53 UDP | Direct：仅无限制；SSH：无限制、仅限时 |
| DNSTT / NoizDNS SOCKS5 / SSH | 53 UDP | 无限制、仅限时 |
| VayDNS SOCKS5 / SSH；Slipstream SOCKS5 / SSH | 53 UDP | 无限制、仅限时 |

新增 ChaCha20 使用经典 AEAD，具体方法为 `chacha20-ietf-poly1305`，与 Shadowsocks 2022 的 ChaCha20 方法不同。两个新增方法都为每位用户提供独立凭据和标准 `ss://` 链接，无需 TLS 证书。

完整的受支持配置包含 15 个核心及基础服务：无限制账户有 30 个配置，仅限时账户有 29 个，限流量账户有 6 个可计量配置。10 个 DNS 配置排在最后，顺序不代表实测速率。请正确设置域名、DNS 委派及云防火墙端口。

## Free config：50 GiB / 30 天

运行 `sudo rovunet`，选择 **3 — Users → 2 — Free config (50 GB / 30 days)**。面板会新建带标记的账户，共享 **50 GiB** 流量额度，**30 天**到期。普通账户不会被转换；中断后重试保留已创建的身份和剩余额度。

**Users → 3 — Export Free config for review** 可重试私有导出，文件位于 `/etc/rovunet/donations/<username>.json`。导出要求账户绑定的计量记录正常且额度未耗尽。三个 Shadowsocks 方法，包括新增 AES-128 和 ChaCha20，都不向限流量账户开放。

这是面板功能与访问凭据的私有导出。应用中的 Free 目录和标签页尚未发布，本次面板更新不会自动启用它们；客户端功能需要单独发布和激活。

## 更新与证书

使用面板正常的 Update 菜单，成功后在应用中刷新原有订阅。账户、密码、期限、已记录流量、订阅 URL 及无关配置保持不变。两个被替换的协议会获得新的配置标识和凭据；替换前会创建私有核心备份及恢复日志。未知流量不会被当作零。

保留普通 TLS 验证、证书恢复，以及两个 Slipstream 订阅别名共用的 `/sub/TOKEN.trust.json`。受管理的引擎继续固定使用已认证的 v2.0.2 签名资产。私有 2.0.3 的独立签名 trust 格式仍需另行验证兼容性。

## 已完成的验证

[发布源码 CI](https://github.com/msVPNco/rovunet-panel/actions/runs/45071747305)、[使用精确引擎资产的准备流程](https://github.com/msVPNco/rovunet-panel/actions/runs/45072135345)和[最终签名验收与发布流程](https://github.com/msVPNco/rovunet-panel/actions/runs/45076586381)均已成功。

最终签名验收覆盖安装器 `--no-run`、**官方 2.0.2 → 2.1.0** 的真实 Update 菜单，以及 SQLite 流量记录和 TLS/trust 文件的保留。执行环境为隔离的 Linux amd64 命名空间，systemd 使用模拟器。ARM64 的签名字节已验证，但未运行；私有 2.0.3 升级未执行验收。该流程不等于完整的首次交互配置、Caddy/Slipstream 实际代理流量或所有应用、运营商网络的验证。详见[测试范围](docs/releases/v2.1.0-test-scope.txt)和[整体安全验收边界](docs/release-gates.json)。

## 常用命令

```bash
sudo rovunet                         # 交互菜单
sudo rovunet users add               # 普通账户、期限和流量限额
sudo rovunet subscriptions-refresh   # 保留 URL，重新生成订阅内容
sudo rovunet certificates --mode check
sudo rovunet certificates --mode recover --tag slipstream-socks
sudo rovunet diag
sudo rovunet backup --file /root/rovunet-backup.age --recipient age1YOUR_PUBLIC_RECIPIENT
```

[完整英文参考](README.md)说明了加密备份身份、证书恢复及更新日志。[Panel2 契约](docs/contracts/panel-v2-addendum.md)：严格的 feed1、单独启用的 status2，不提供设备会话 API。

面板采用 [AGPL-3.0](LICENSE)，各组件遵循各自许可证。项目基于 SlipGate，与其无隶属关系；见 [NOTICE](NOTICE.md)。
