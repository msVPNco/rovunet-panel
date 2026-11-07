# Xray 内核

[文档](README.md) › Xray 内核

Xray 内核是由面板在隧道旁安装和管理的独立 Xray 服务器，提供内置服务器所没有的四种协议。每个面板用户在每个内核上都有一份配置；`users configs` 会把它与该用户的隧道一起输出，Rovunet 应用也会把它放入同一集合。

| 协议 | 传输 | 证书 | 最适合 |
|---|---|---|---|
| **VLESS + REALITY**（XTLS Vision） | TCP | 无需：握手模仿真实网站（`--target`） | 到知名网站的 TLS 能通过的网络 |
| **VLESS + XHTTP** | TCP，基于 TLS 的 HTTP 请求 | 需要，且对 `--sni` 有效 | 在 CDN 后，或 WebSocket 被封锁时 |
| **Hysteria 2** | UDP（QUIC） | 需要，且对 `--sni` 有效 | UDP 可通过的高丢包或不稳定网络 |
| **WireGuard** | UDP | 无需 | 不封锁 WireGuard 的网络 |

<a id="installing-a-core"></a>
## 安装内核

安装程序（`sudo rovunet install`）会在其他协议旁提供 **VLESS + REALITY** 和 **WireGuard**，并在创建第一个用户后，在第一个空闲的默认端口上配置它们（REALITY：9443、2096 或 8880/tcp；WireGuard：51820–51822/udp）。任何内核也都可以稍后添加：

```bash
# VLESS + REALITY
sudo rovunet core-install --name r1 --protocol reality --host SERVER_IP \
  --target www.microsoft.com:443 --sni www.microsoft.com --port 9443

# VLESS + XHTTP
sudo rovunet core-install --name x1 --protocol xhttp --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 2096

# Hysteria 2
sudo rovunet core-install --name h1 --protocol hysteria2 --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 8443

# WireGuard
sudo rovunet core-install --name w1 --protocol wireguard --host SERVER_IP --port 51820
```

| 选项 | 含义 |
|---|---|
| `--name` | 内核名称：1–31 个字符，`a-z`、`0-9` 和 `-` |
| `--protocol` | `reality`、`xhttp`、`hysteria2` 或 `wireguard` |
| `--host` | 客户端连接的公网主机名或 IP |
| `--sni` | 证书名称，或 REALITY 呈现的服务器名（默认取 `--host`） |
| `--target` | 仅 REALITY：要模仿的真实 TLS 网站，格式为 `host:port` |
| `--port` | 监听端口（默认 8443），必须空闲 |
| `--devices` | 按设备而非按用户的模式：生成这么多份独立凭据（1–1000），仅写成文件 |
| `--cert`、`--key` | XHTTP 和 Hysteria 2：证书和私钥路径 |

## 安装过程

1. 按正确协议检查端口是否空闲（REALITY 和 XHTTP 为 TCP，Hysteria 2 和 WireGuard 为 UDP）。
2. 安装随本发布版本提供的固定版本 Xray，并按发布的 SHA256SUMS 校验。已有的 `/usr/local/bin/xray` 会保留。
3. 写入服务器配置，并在上线前由 Xray 自行测试（`xray run -test`）。
4. 创建并启动服务 `rovunet-core-<名称>`，并在防火墙中开放端口。
5. 为每个用户在 `/etc/rovunet/core/<名称>/clients/` 写入一个客户端文件。

任一步骤失败，所创建的内容都会被清除。

## 管理内核

```bash
sudo rovunet core-list            # name, protocol, port
sudo rovunet tunnel status        # cores appear with their service state
sudo rovunet core-remove r1       # stop and delete
```

`core-plan` 接受相同选项以及 `--output <新目录>`，只写出文件，用于手动部署。

## 用户与配置

每个用户的凭据由内核自身的密钥、用户名和密码派生：

- `users add` 会在所有内核上为新用户生成配置；已有用户的配置保持不变。
- `users remove` 和 `passwd` 会在几秒内撤销旧配置。Xray 会先验证每次变更；若被拒绝，则保留原有配置。
- `users configs` 和 `export-clients --format rovunet` 会把每个用户的内核配置作为原生 `rovunet://` 配置输出到该用户的集合中。`export-clients --format standard` 会为其他客户端输出普通的 `vless://` 和 `hysteria2://` 链接。
- 每个用户的普通配置也保存在 `/etc/rovunet/core/<名称>/clients/<用户>.txt`（WireGuard 为 `.conf`）。切勿在两台设备上同时使用同一份 WireGuard 配置。

## 备份

内核包含在 `rovunet backup` 中，并由 `restore` 重新启动。通过 `--cert`/`--key` 提供的证书位于面板之外，必须在新服务器的相同路径存在，否则该内核会被报告为未启动。

## 限制

- XHTTP 和 Hysteria 2 在启动时读取证书。证书续期后，请重启内核（`systemctl restart rovunet-core-<名称>`）。
- v1.9.0 之前（按设备）安装的内核会照常工作；要让它们服务面板用户，请重新安装。

下一页：[运维](operations.md)
