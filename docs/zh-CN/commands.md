# 命令参考

[文档](README.md) › 命令参考

所有命令都以 root 运行（`sudo rovunet …`）。未提供的必填选项会以交互方式询问；不带参数的 `rovunet` 打开菜单。

## 系统

| 命令 | 说明 |
|---|---|
| `rovunet` | 交互式菜单 |
| `rovunet install [--bin-dir DIR]` | 安装依赖并配置所选协议 |
| `rovunet update` | 更新到最新版本并重新生成服务 |
| `rovunet restart` | 重启 DNS 路由器、SOCKS5 代理和所有隧道 |
| `rovunet stats` | 实时仪表盘 |
| `rovunet diag` | 诊断 |
| `rovunet mtu VALUE` | 设置所有 DNSTT、NoizDNS 和 VayDNS 隧道的 MTU |
| `rovunet backup [FILE \| -]` | 备份配置、密钥、用户和 Xray 内核 |
| `rovunet restore [FILE \| -] [--yes]` | 恢复备份；`-` 从标准输入读取 |
| `rovunet uninstall` | 移除面板安装的一切 |
| `rovunet --version` | 显示版本 |

## 用户

| 命令 | 说明 |
|---|---|
| `rovunet users` | 用户菜单 |
| `rovunet users add` | 添加用户（询问不限量或限量，再问天数和 GB） |
| `rovunet users renew` | 恢复已停用（到期或超额）的用户，已用流量清零 |
| `rovunet expire-users` | 停用到期或超额的用户并刷新订阅链接（由定时器每 5 分钟运行） |
| `rovunet users bulk_add --count N --prefix P` | 以随机密码添加最多 500 个用户 |
| `rovunet users remove` | 删除用户 |
| `rovunet users list` | 用户及其配置 |
| `rovunet users configs` | 所有用户的全部配置 |
| `rovunet export-clients --user U --format rovunet [--annotated]` | 供脚本使用的单个用户链接 |

## 隧道

| 命令 | 说明 |
|---|---|
| `rovunet tunnel add` | 添加隧道（选项见下） |
| `rovunet tunnel edit --tag T` | 修改隧道（选项见下） |
| `rovunet tunnel remove T` / `--all` | 删除一个或全部隧道 |
| `rovunet tunnel start T` / `stop T` | 启动或停止隧道 |
| `rovunet tunnel status [T]` | 所有隧道和内核，或单个隧道详情 |
| `rovunet tunnel share T [--user U]` | 输出隧道配置 |
| `rovunet tunnel logs T` | 隧道的服务日志 |

### `tunnel add` 选项

| 选项 | 适用 | 含义 |
|---|---|---|
| `--transport` | 全部 | `dnstt`、`vaydns`、`slipstream`、`soar`、`naive`、`stuntls`、`vless`、`direct-ssh`、`direct-socks5`、`external` |
| `--backend` | DNS 隧道、NaïveProxy | `socks`、`ssh`、`both`；Soar：`direct` 或 `ssh` |
| `--tag` | 全部 | 隧道名称 |
| `--domain` | 需要域名的类型 | 隧道域名或子域名 |
| `--private-key`、`--public-key` | DNSTT、VayDNS、Soar | 导入现有密钥 |
| `--record-type`、`--idle-timeout`、`--keep-alive`、`--clientid-size`、`--queue-size` | VayDNS | 线路格式调优 |
| `--allow-ports` | Soar direct | 允许的目标端口，如 `80,443` 或 `any` |
| `--email`、`--decoy-url` | NaïveProxy | Let's Encrypt 邮箱、伪装网站 |
| `--cdn`、`--tls`、`--listen-port`、`--cdn-address`、`--block-ports` | VLESS | Cloudflare 模式、TLS、监听端口、干净 CDN IP、封锁的目标端口 |
| `--port` | External | 转发到的本地 UDP 端口 |

### `tunnel edit` 选项

| 选项 | 含义 |
|---|---|
| `--new-tag` | 重命名隧道 |
| `--mtu` | DNS 隧道 MTU |
| `--front NAIVE_TAG \| -` | 让 VLESS 或 SSH over TLS 经 NaïveProxy 的 443 端口承载，或撤销 |
| `--cdn-address IP \| -` | 用于后续 VLESS 配置的干净 Cloudflare IP |
| VayDNS 调优选项 | 同 `tunnel add` |

## Xray 内核

| 命令 | 说明 |
|---|---|
| `rovunet core-install --name N --protocol P --host H [options]` | 安装服务所有用户的内核；参见 [Xray 内核](xray-cores.md#installing-a-core) |
| `rovunet core-list` | 已安装的内核 |
| `rovunet core-remove N` | 删除内核 |
| `rovunet core-plan --protocol P --host H --output DIR [options]` | 仅写出内核文件 |

## DNS 路由

| 命令 | 说明 |
|---|---|
| `rovunet router status` | 路由模式和路由 |
| `rovunet router mode` | 在 single 和 multi 模式间切换 |
| `rovunet router switch` | single 模式下更换活动隧道 |

## 配置

| 命令 | 说明 |
|---|---|
| `rovunet config export` | 输出不含密钥的 `config.json` |
| `rovunet config import PATH` | 替换 `config.json`（不含密钥） |
| `rovunet config address IP \| auto` | SSH、SOCKS5 和 SSH over TLS 配置使用的公网地址；逗号后可添加本服务器的其他地址，如 `auto,198.51.100.9` |

## 内部命令

供 systemd 服务使用：`rovunet dnsrouter serve`、`rovunet socks serve`、`rovunet stuntls serve`、`rovunet vless serve`、`rovunet post-update`。

下一页：[故障排查](troubleshooting.md)
