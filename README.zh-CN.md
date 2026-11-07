# Rovunet Panel

[English](README.md) · [فارسی](README.fa.md) · 简体中文 · [Русский](README.ru.md)

**在兼容 Linux 服务器上部署和管理多协议访问的开源自托管工具。** 面板整合服务安装、用户创建、客户端配置导出、systemd 管理、诊断和备份。多个路径减少对单一协议的依赖，不保证在每个网络中可用。

管理的路径包括 VLESS WebSocket、NaiveProxy、直接 SSH、SSH over TLS/STunTLS、SOCKS5，以及 DNSTT、NoizDNS、VayDNS、Slipstream、Soar 等 DNS 家族。Xray 内核（VLESS REALITY、VLESS XHTTP、Hysteria2、WireGuard）由 `rovunet core-install` 安装和管理：先检查端口，Xray 按发布校验和验证，并在上线前测试配置，再开放所需的 TCP 或 UDP 端口。每台设备在 `/etc/rovunet/core/<名称>/clients/` 获得自己的文件；XHTTP 和 Hysteria2 需要 `--cert` 与 `--key`。[集成状态](docs/ROVUNET-STATUS.md)。

## 安装

在新的 Ubuntu 20.04+ 或 Debian 10+ 服务器上以 root 权限运行：

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

需要自有服务器、控制 DNS 的域名和所选服务所需端口。安装器询问协议，配置服务，创建首个用户并输出配置。以旧面板名称安装的服务器在更新（`slipgate update`）时自动迁移；请先备份，不要直接在不可重建服务器上试装。

```bash
sudo rovunet
sudo rovunet users add
sudo rovunet users configs
sudo rovunet diag
sudo rovunet backup
sudo rovunet export-clients --user USER --format rovunet
sudo rovunet core-install --name r1 --protocol reality --host SERVER_IP \
  --target www.microsoft.com:443 --sni www.microsoft.com --port 8443 --devices 5
sudo rovunet core-list
```

## 客户端与诊断

[Rovunet Android](https://github.com/msVPNco/rovunet-android) 导入 rovunet://，保留集合元数据，直接连接选择的路径，并提供独立真实隧道 HTTPS 测试；测试进入 DNS 回退前需要同意。其他兼容客户端使用标准 vless:// 链接。[公开发布](https://github.com/msVPNco/rovunet-android/releases) · [应用状态与签名](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md)。私有包不是公开版本的原位更新，勿将未签名文件作为安装包。

SOCKS CONNECT 成功不证明数据通过；分别检查认证、REP、DNS 和有效载荷。基础 Soar 可用不代表 SSH 地址/端口获准访问，需检查服务器实际策略。不要把 DNS 错误直接归因于审查。保留 Caddy/REALITY TCP 和 HTTP/3/Hysteria2 UDP 的端口计划。

## 文档、社区与支持

[完整指南](docs/GUIDE.md) · [用户指南](docs/USER_GUIDE.md) · [变更记录](CHANGELOG.md) · [Telegram](https://t.me/RovunetVPN)。生产前测试所需协议并运行 rovunet backup；多用户、资源和恢复需在实际环境验证。

自愿帮助开发：**USDT，仅限 Tron/TRC20**，`TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ`。使用不需要捐款。

## 许可证

AGPL-3.0，见 [NOTICE](NOTICE.md)。基于 Anonvector 的 [SlipGate](https://github.com/anonvector/slipgate)，独立维护且无隶属关系。命令为 `rovunet`；以旧名称安装的服务器在更新时自动迁移。
