# 协议

[文档](README.md) › 协议

## 概览

| 协议 | 类别 | 承载方式 | 默认端口 | 证书 | 面板传输类型 |
|---|---|---|---|---|---|
| [VLESS over WebSocket](#vless-over-websocket) | 代理 | WebSocket + TLS | 443/tcp（Naïve 前置）或 2053/tcp | Let's Encrypt（前置时）或自签名 | `vless` |
| [NaïveProxy](#naiveproxy) | 代理 | HTTP/2 + TLS | 443/tcp | Let's Encrypt | `naive` |
| [SSH over TLS](#ssh-over-tls)（StunTLS） | SSH | TLS / WebSocket | 443/tcp（Naïve 前置）或 8443/tcp | 自签名，前置时为 Let's Encrypt | `stuntls` |
| [SSH](#ssh-and-socks5) | SSH | TCP | 22/tcp | — | `direct-ssh` |
| [SOCKS5](#ssh-and-socks5) | 代理 | TCP，不加密 | 1080/tcp | — | `direct-socks5` |
| [DNSTT](#dns-tunnels) / NoizDNS | DNS 隧道 | DNS，Curve25519 | 53/udp + 53/tcp | — | `dnstt` |
| [VayDNS](#dns-tunnels) | DNS 隧道 | DNS，KCP + Curve25519 | 53/udp + 53/tcp | — | `vaydns` |
| [Slipstream](#dns-tunnels) | DNS 隧道 | DNS 上的 QUIC | 53/udp + 53/tcp | 自签名 | `slipstream` |
| [Soar](#dns-tunnels) | DNS 隧道 | 多路径 DNS，X25519 + Ed25519 | 53/udp + 53/tcp | — | `soar` |
| [VLESS + REALITY](xray-cores.md) | Xray 内核 | TCP，REALITY + XTLS Vision | 任意空闲 TCP 端口 | 无需（模仿真实网站） | `core-install` |
| [VLESS + XHTTP](xray-cores.md) | Xray 内核 | HTTP 请求 + TLS | 任意空闲 TCP 端口 | 需要 | `core-install` |
| [Hysteria 2](xray-cores.md) | Xray 内核 | QUIC | 任意空闲 UDP 端口 | 需要 | `core-install` |
| [WireGuard](xray-cores.md) | Xray 内核 | UDP | 任意空闲 UDP 端口 | — | `core-install` |
| [外部 DNS 路由](#external-dns-route) | 自定义 | DNS 转发到本地端口 | 53/udp | — | `external` |

## 如何选择协议

以下是 2026 年秋季在伊朗一台服务器上的实测结果，仅供起步参考，不作保证；网络各不相同且不断变化。

| 情况 | 优先尝试 |
|---|---|
| HTTPS 可用 | NaïveProxy 前置在 443 的 VLESS：快速、稳定，看起来像普通网站 |
| 22 端口开放 | SSH：快速 |
| 服务器 IP 被封锁 | Cloudflare 后的 VLESS：可用，但明显更慢 |
| 到知名网站的 TLS 能通过 | VLESS + REALITY |
| 高丢包但 UDP 可通过的网络 | Hysteria 2 |
| 只有 DNS 能通过 | VayDNS 或 DNSTT：慢，但能用 |
| 独立端口上的 SSH over TLS 被重置 | 改由 NaïveProxy 在 443 前置 |
| 仅限开放网络 | SOCKS5：不加密，过滤网络会切断它 |

## 后端

DNS 隧道和 NaïveProxy 把解密后的流量交给后端：

| 后端 | 行为 |
|---|---|
| `socks` | 面板内置的 SOCKS5 代理，简单快速 |
| `ssh` | 由服务器的 OpenSSH 转发，因此每个用户都需要 SSH 登录 |
| `both` | 创建两个隧道 `<tag>-socks` 和 `<tag>-ssh`；SSH 版本的子域名多一个 `s` |
| `direct` | 仅限 Soar：服务器自行连接每个流的目标 |

<a id="vless-over-websocket"></a>
## VLESS over WebSocket

面板自身提供 VLESS over WebSocket 服务。每个用户的 UUID 由用户名、密码和服务器级密钥（`/etc/rovunet/vless.secret`）派生：删除用户或修改密码会在几秒内使其失效，分享出去的配置从不包含密码。流只能访问公网地址；默认封锁 25 端口（`--block-ports`）。支持 TCP 和普通 UDP，不支持 Mux 和 XUDP。

三种接入方式：

| 模式 | 方式 | 适用 |
|---|---|---|
| **NaïveProxy 前置**（推荐） | Caddy 用其 Let's Encrypt 证书在 443 承载；其他路径显示伪装网站 | 安装了 NaïveProxy 时的默认方式 |
| **Cloudflare 后** | 客户端连接 Cloudflare IP，服务器 IP 保持隐藏 | 服务器 IP 被封锁时 |
| **直连** | 独立端口，自签名证书 | 测试 |

```bash
# Fronted by NaïveProxy
sudo rovunet tunnel add --transport vless --tag myvless --domain example.com --cdn no --listen-port 2053
sudo rovunet tunnel edit --tag myvless --front myproxy      # "-" undoes it

# Behind Cloudflare
sudo rovunet tunnel add --transport vless --tag myvless-cf --domain cdn.example.com --cdn yes --listen-port 2053
sudo rovunet tunnel edit --tag myvless-cf --cdn-address 104.16.0.1   # clean Cloudflare IP, "-" resets
```

代理主机名的 Cloudflare 设置：

1. DNS：`A cdn.example.com → SERVER_IP`，**Proxied**（橙色云朵）。
2. SSL/TLS › Overview：**Full**。Flexible 不可用，Full (strict) 会拒绝服务器的自签名证书。
3. Network：开启 **WebSockets**。
4. 端口：Cloudflare 仅在 443、2053、2083、2087、2096 和 8443 代理 HTTPS；NaïveProxy 占用 443 时使用 2053。

配置按用户分发：`sudo rovunet tunnel share myvless --user alice` 会输出供 Rovunet 应用使用的 `rovunet://` 链接，以及供 v2rayNG、Hiddify、NekoBox 等客户端使用的标准 `vless://` 链接。前置链接不需要 `allowInsecure`，而 Xray 客户端从 2026 年 8 月起不再接受该选项。两种链接默认关闭 SNI 分片，可在应用中开启。

<a id="naiveproxy"></a>
## NaïveProxy

带 forwardproxy 插件的 Caddy。Caddy 自动获取并续期 Let's Encrypt 证书（需开放 80 和 443 端口），并对所有非认证代理请求显示伪装网站。所有面板用户都能登录；`users add`、`remove` 和 `passwd` 会实时更新 Caddy。没有任何登录的 Naïve 隧道会获得一个随机登录并保持关闭。

```bash
sudo rovunet tunnel add --transport naive --backend socks --tag myproxy \
  --domain example.com --email admin@example.com --decoy-url https://www.wikipedia.org
```

### 在 443 端口前置

NaïveProxy 的 Caddy 可以在同一域名、端口和证书上承载 VLESS 和 SSH over TLS。Caddy 逐个路由请求：秘密 WebSocket 路径转给 VLESS 或 SSH over TLS，认证代理请求转给 NaïveProxy，其余显示伪装网站。每次修改 Caddyfile 前都会用 `caddy-naive validate` 校验；校验失败则保留原文件。

```bash
sudo rovunet tunnel edit --tag myvless --front naive
sudo rovunet tunnel edit --tag mytls --front naive
sudo rovunet users configs        # hand out the new configurations
```

<a id="ssh-over-tls"></a>
## SSH over TLS（StunTLS）

由面板提供的 TLS 和 WebSocket 内 SSH。自动识别 WebSocket、HTTP CONNECT、原始 TLS 和 payload 模式。在独立端口（8443）上使用自签名证书，无需域名；由 NaïveProxy 前置时，客户端以 SSH over WebSocket 方式通过 Naïve 域名的 443 端口连接，使用真实证书。如果应用在独立端口上报告 `Connection was reset`，说明网络封锁了该端口：改为前置。

```bash
sudo rovunet tunnel add --transport stuntls --tag mytls
sudo rovunet tunnel edit --tag mytls --front myproxy
```

<a id="ssh-and-socks5"></a>
## SSH 与 SOCKS5

```bash
sudo rovunet tunnel add --transport direct-ssh --tag myssh
sudo rovunet tunnel add --transport direct-socks5 --tag mysocks
```

SSH 使用服务器在 22 端口的 OpenSSH；隧道用户以密码登录且没有 shell（仅转发）。SOCKS5 监听 1080，不加密。没有用户的 SOCKS5 代理会拒绝连接，而不会变成开放代理。

<a id="dns-tunnels"></a>
## DNS 隧道

| 隧道 | 说明 |
|---|---|
| **DNSTT** | Curve25519 加密的 DNS 隧道。同一服务器也服务 **NoizDNS** 客户端，后者增加 DPI 规避（base36/hex 编码、去除 CDN 前缀） |
| **VayDNS** | 基于 KCP、使用 Curve25519 的 DNS 隧道，线路格式可调：记录类型、空闲超时、keep-alive、客户端 ID 大小、队列大小 |
| **Slipstream** | DNS 上的 QUIC，使用证书认证。实验性 |
| **Soar** | 带解析器发现的多路径 DNS 隧道（[Lantern](https://github.com/getlantern/soar)），内置于面板。**direct** 后端：无需登录，服务器在允许的端口（默认 `80,443`）上连接公网目标，兼容标准 Soar 客户端。**SSH** 后端：只能到达 SSH 服务器，因此通过 SSH 用户控制访问 |

```bash
sudo rovunet tunnel add --transport dnstt  --backend socks  --tag mydnstt  --domain t.example.com
sudo rovunet tunnel add --transport vaydns --backend both   --tag myvaydns --domain v.example.com \
  --record-type txt --idle-timeout 10s --keep-alive 2s --clientid-size 2 --queue-size 512
sudo rovunet tunnel add --transport slipstream --backend ssh --tag myslip --domain s.example.com
sudo rovunet tunnel add --transport soar --backend direct --tag mysoar --domain r.example.com --allow-ports 80,443
sudo rovunet tunnel add --transport soar --backend ssh    --tag mysoar-ssh --domain rs.example.com
```

DNSTT 和 VayDNS 可通过 `--private-key`（以及可选的 `--public-key`）导入现有密钥；Soar 可通过 `--private-key` 导入 base64 格式的 Ed25519 种子。`sudo rovunet mtu 1200` 一次性设置所有 DNSTT、NoizDNS 和 VayDNS 隧道的 MTU（默认 1232）。

<a id="external-dns-route"></a>
## 外部 DNS 路由

把某个域名的 DNS 查询转发到本地 UDP 端口，用于你自己运行的协议。面板不为其管理服务。

```bash
sudo rovunet tunnel add --transport external --tag my-proto --domain j.example.com --port 5301
```

## 域名布局

| 隧道 | 域名 | 后端 |
|---|---|---|
| dnstt-socks / dnstt-ssh | `t.example.com` / `ts.example.com` | SOCKS5 / SSH |
| vaydns-socks / vaydns-ssh | `v.example.com` / `vs.example.com` | SOCKS5 / SSH |
| slipstream-socks / slipstream-ssh | `s.example.com` / `ss.example.com` | SOCKS5 / SSH |
| soar-direct / soar-ssh | `r.example.com` / `rs.example.com` | direct / SSH |
| naive-socks / naive-ssh | `example.com`（共享） | SOCKS5 / SSH |
| vless、stuntls | `example.com`，经 NaïveProxy | 每用户 UUID / SSH |

每个 DNS 隧道需要自己的子域名；DNSTT 和 NoizDNS 共用一个。NaïveProxy 隧道使用 HTTPS，因此共用域名；VLESS 和 SSH over TLS 通过它承载，不需要单独记录。

## 路由模式

| 模式 | 行为 |
|---|---|
| **single** | 只有一个 DNS 隧道活动；53 端口的 DNS 路由器转发给它 |
| **multi** | 所有 DNS 隧道运行在本地端口；路由器按域名分发查询，总是选择最具体的匹配域名。添加第二个 DNS 隧道时自动启用 |

路由器同时在 UDP 和 TCP 的 53 端口应答。TCP 查询（RFC 7766，流水线）经 UDP 转发给隧道，并在同一连接上返回应答，因此所有 DNS 隧道也适用于使用 TCP/53 的解析器和客户端。

## 架构

```text
                                 Rovunet client
            ┌──────────────────────────┼─────────────────────────────┐
     DNS :53 udp+tcp             HTTPS :443 tcp              Cloudflare :2053 (optional)
            │                          │                             │
┌───────────┼──────────────────────────┼─────────────────────────────┼──────────┐
│ server    ▼                          ▼                             │          │
│  ┌───────────────────┐   ┌─────────────────────────────────┐      │          │
│  │    DNS router     │   │ Caddy (NaïveProxy) :443         │      │          │
│  │ dispatch by domain│   │ Let's Encrypt + decoy website   │      │          │
│  └──┬────┬────┬───┬──┘   └────┬──────────────┬───────────┬─┘      │          │
│     ▼    ▼    ▼   ▼           ▼              ▼           ▼        ▼          │
│  DNSTT VayDNS Slip Soar   forward proxy  SSH over TLS   VLESS over WebSocket │
│     └────┴────┴───┘           │              │           │                   │
│              ▼                ▼              ▼           │                   │
│   ┌───────────────────────────────────────────────┐      │   Xray cores      │
│   │  Backend: SOCKS5 :1080 · SSH forwarding :22  │      │   (own ports)     │
│   └───────────────────────┬───────────────────────┘      │        │          │
│                     WARP (optional)                      │        │          │
│                           └─────────────┬────────────────┴────────┘          │
│                                      Internet                                │
└───────────────────────────────────────────────────────────────────────────────┘
```

direct 后端的 Soar、VLESS 和 Xray 内核自行连接目标，因此其流量绕过后端层和 WARP。

下一页：[用户与客户端配置](users-and-configs.md)
