# Xray 内核

[文档](README.md) › Xray 内核

Xray 内核是由面板在隧道旁安装和管理的独立 Xray 服务器，提供内置服务器所没有的四种协议。

| 协议 | 传输 | 证书 | 最适合 |
|---|---|---|---|
| **VLESS + REALITY**（XTLS Vision） | TCP | 无需：握手模仿真实网站（`--target`） | 到知名网站的 TLS 能通过的网络 |
| **VLESS + XHTTP** | TCP，基于 TLS 的 HTTP 请求 | 需要，且对 `--sni` 有效 | 在 CDN 后，或 WebSocket 被封锁时 |
| **Hysteria 2** | UDP（QUIC） | 需要，且对 `--sni` 有效 | UDP 可通过的高丢包或不稳定网络 |
| **WireGuard** | UDP | 无需 | 不封锁 WireGuard 的网络 |

<a id="installing-a-core"></a>
## 安装内核

```bash
# VLESS + REALITY
sudo rovunet core-install --name r1 --protocol reality --host SERVER_IP \
  --target www.microsoft.com:443 --sni www.microsoft.com --port 8443 --devices 5

# VLESS + XHTTP
sudo rovunet core-install --name x1 --protocol xhttp --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 2096 --devices 5

# Hysteria 2
sudo rovunet core-install --name h1 --protocol hysteria2 --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 8443 --devices 5

# WireGuard
sudo rovunet core-install --name w1 --protocol wireguard --host SERVER_IP --port 51820 --devices 5
```

| 选项 | 含义 |
|---|---|
| `--name` | 内核名称：1–31 个字符，`a-z`、`0-9` 和 `-` |
| `--protocol` | `reality`、`xhttp`、`hysteria2` 或 `wireguard` |
| `--host` | 客户端连接的公网主机名或 IP |
| `--sni` | 证书名称，或 REALITY 呈现的服务器名（默认取 `--host`） |
| `--target` | 仅 REALITY：要模仿的真实 TLS 网站，格式为 `host:port` |
| `--port` | 监听端口（默认 8443），必须空闲 |
| `--devices` | 独立客户端凭据数量，1–1000 |
| `--cert`、`--key` | XHTTP 和 Hysteria 2：证书和私钥路径 |

## 安装过程

1. 按正确协议检查端口是否空闲（REALITY 和 XHTTP 为 TCP，Hysteria 2 和 WireGuard 为 UDP）。
2. 安装随本发布版本提供的固定版本 Xray，并按发布的 SHA256SUMS 校验。已有的 `/usr/local/bin/xray` 会保留。
3. 写入服务器配置，并在上线前由 Xray 自行测试（`xray run -test`）。
4. 创建并启动服务 `rovunet-core-<名称>`，并在防火墙中开放端口。
5. 为每台设备在 `/etc/rovunet/core/<名称>/clients/` 写入一个客户端文件。

任一步骤失败，所创建的内容都会被清除。

## 管理内核

```bash
sudo rovunet core-list            # name, protocol, port
sudo rovunet tunnel status        # cores appear with their service state
sudo rovunet core-remove r1       # stop and delete
```

`core-plan` 接受相同选项以及 `--output <新目录>`，只写出文件，用于手动部署。

## 客户端文件

每台设备获得独立凭据：一条 `vless://` 或 `hysteria2://` 链接，或一个 WireGuard `.conf` 文件。它们可导入 Rovunet 应用以及基于 Xray 和 WireGuard 的标准客户端。切勿在两台设备上同时使用同一个 WireGuard 文件。

## 备份

内核包含在 `rovunet backup` 中，并由 `restore` 重新启动。通过 `--cert`/`--key` 提供的证书位于面板之外，必须在新服务器的相同路径存在，否则该内核会被报告为未启动。

## 限制

- 内核配置作为独立配置导入，而不是加入用户的集合。
- 内核独立于面板用户：删除用户不会撤销内核凭据；请删除或重新安装内核。

下一页：[运维](operations.md)
