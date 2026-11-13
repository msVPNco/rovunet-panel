# 运维

[文档](README.md) › 运维

<a id="updating"></a>
## 更新

```bash
sudo rovunet update
```

v1.21.2 在替换前使用面板内置的生产公钥验证发布清单签名和文件摘要，并保留私有恢复副本，明确报告未完成的设置。正常更新补齐缺失的内核，并将旧 Hy2 一次性迁移到 Salamander，同时保留账户凭据和订阅地址。随后在应用中对原有订阅点击 **Update**，无需删除订阅或发送新链接。手动选择的普通 Hy2 模式和已停止的服务保持不变。旧面板首次升级仍使用其原有验证方法，之后使用新的签名更新机制。[发布范围和兼容性](../releases/v1.21.2.md)。

### 从旧名称迁移

以面板旧名称（`slipgate` 命令、`/etc/slipgate`）安装的服务器用旧命令更新一次，即会自动迁移：

```bash
sudo slipgate backup
sudo slipgate update
sudo rovunet tunnel status
```

迁移会停止旧服务，把 `/etc/slipgate` 移到 `/etc/rovunet`，在不触碰任何密码的前提下改写已存储的路径，以相同的 UID 和 GID 重命名系统用户和组，以 `rovunet-*` 重建服务，仅在 `sshd -t` 通过后修改 sshd 配置，把 `config.json` 设为仅所有者可读，并将命令安装为 `/usr/local/bin/rovunet`。运行上游 SlipGate 的服务器也可同样切换：

```bash
# First use the authenticated installer from Getting started.
sudo rovunet post-update
```

## 备份与恢复

备份包含在另一台服务器上运行同样配置所需的一切：配置、所有隧道的私钥和证书、所有用户及其密码，以及 Xray 内核。在另一台服务器上恢复后，同样的隧道以同样的密钥恢复，客户端无需新链接即可继续使用。

```bash
sudo rovunet backup                      # asks: save to a file, or print text to copy
sudo rovunet backup /root/rovunet-backup.age
sudo rovunet backup-check /root/rovunet-backup.age
sudo rovunet restore /root/rovunet-backup.age
# Automated transfer: see ../security/encrypted-backups.md
```

备份默认采用具有完整性验证的 age 加密文本。请分开保存恢复口令或私钥。仍可读取旧 ROVUNET BACKUP 格式；明文导出需要明确选择 --plaintext。自动迁移时，源端需要公开的 --recipient，目标端需要私有的 --identity-file。[加密备份说明](../security/encrypted-backups.md)。

在新服务器上恢复：

1. 安装面板；向导询问传输类型时按 Ctrl-C，恢复过程会自行配置。
2. 运行 `sudo rovunet restore` 并粘贴文本，或指定文件。
3. 将 DNS 记录指向新 IP；恢复会列出需要修改的记录（`ns.<域名>` 和 NaïveProxy 域名）。

恢复会安装传输二进制文件，写入密钥和配置，创建用户，开放防火墙端口，如果之前启用了 WARP 则重新配置，启动所有服务并重新启动 Xray 内核。服务器上已有的内容会被替换，替换前先保存到 `/root/rovunet-pre-restore-<日期>.txt`。

`rovunet config export` 只输出不含密钥的 `config.json`：迁移服务器请使用 `backup`。

## 监控

| 命令 | 显示 |
|---|---|
| `sudo rovunet stats` | 实时仪表盘：CPU、内存、流量、按协议的连接数、隧道状态 |
| `sudo rovunet tunnel status` | 所有隧道和 Xray 内核及其服务状态 |
| `sudo rovunet tunnel status <tag>` | 单个隧道详情：密钥、端口、MTU、证书名称、签发类型和到期时间 |
| `sudo rovunet diag` | 健康检查：服务、端口、密钥、DNS 解析、开机持久化 |
| `sudo rovunet tunnel logs <tag>` | 隧道的服务日志 |

证书信息会标记已过期、即将过期（不足 14 天）和无法读取的证书。

## WARP 出站（可选）

Cloudflare WARP 可以承载服务器的出站流量，适用于用户所需网站拒绝服务器 IP 的情况，或作为额外一跳。在交互式菜单中启用。它会增加延迟并依赖 Cloudflare。direct 后端的 Soar、VLESS 和 Xray 内核不经过 WARP。

## 防火墙

面板通过当前启用的 UFW、firewalld 或 iptables 开放隧道和内核所用端口。使用 iptables 时，面板把自己开放的端口记录在 `/etc/iptables/rovunet-ports`，开机时只插入这些端口，不改动其他工具（Docker、fail2ban）的规则。

## 文件与服务

| 路径 | 内容 |
|---|---|
| `/etc/rovunet/config.json` | 主配置：隧道、用户、路由（仅所有者可读） |
| `/etc/rovunet/tunnels/<tag>/` | 隧道的密钥、证书和配置 |
| `/etc/rovunet/vless.secret` | 派生 VLESS UUID 和路径的服务器级密钥 |
| `/etc/rovunet/core/<名称>/` | Xray 内核：`server.json`、`meta`、各设备的 `clients/` |
| `/usr/local/bin/rovunet` | 面板，内置 SOCKS5、Soar、SSH over TLS 和 VLESS 服务器 |
| `/usr/local/bin/dnstt-server`、`slipstream-server`、`vaydns-server`、`caddy-naive` | 传输二进制文件 |
| `/usr/local/bin/xray` | 供 `core-install` 使用的固定版本 Xray |
| `/etc/iptables/rovunet-ports` | 面板在 iptables 中开放的端口 |

| systemd 单元 | 作用 |
|---|---|
| `rovunet-dnsrouter` | 53 端口的 DNS 路由器 |
| `rovunet-socks5` | 内置 SOCKS5 代理 |
| `rovunet-<tag>` | 每个隧道一个 |
| `rovunet-core-<名称>` | 每个 Xray 内核一个 |
| `rovunet-iptables` | 开机时恢复面板的防火墙端口 |

```bash
journalctl -u rovunet-mytunnel -n 200 --no-pager
```

## 卸载

```bash
sudo rovunet uninstall
```

删除所有服务、配置、二进制文件、系统用户和 SSH 隧道用户。DNS 记录保持不变；如有需要，请在 DNS 服务商处删除。

下一页：[命令参考](commands.md)
