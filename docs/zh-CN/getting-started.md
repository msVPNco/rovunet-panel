# 快速开始

[文档](README.md) › 快速开始

## 1. 要求

| 项目 | 要求 |
|---|---|
| 操作系统 | 带 systemd 的 Linux：Ubuntu 20.04+、Debian 10+ 或类似系统 |
| 权限 | 通过 SSH 的 root（或 sudo） |
| 域名 | 你能控制 DNS 记录的域名。DNS 隧道、NaïveProxy、VLESS 以及基于 TLS 的 Xray 内核需要 |
| 硬件 | 1 vCPU、1 GB 内存和 20 GB 磁盘足以满足小团体；带宽比 CPU 更重要 |
| 端口 | 取决于启用的协议；见下表 |

| 端口 | 用途 |
|---|---|
| 53/udp 和 53/tcp | DNS 隧道（DNSTT、NoizDNS、VayDNS、Slipstream、Soar） |
| 80/tcp 和 443/tcp | NaïveProxy 及其前置的一切（VLESS、SSH over TLS） |
| 22/tcp | SSH |
| 1080/tcp | SOCKS5（仅限开放网络） |
| 2053/tcp（或 2083、2087、2096、8443） | Cloudflare 后的 VLESS |
| 任意空闲 TCP 或 UDP 端口 | Xray 内核 |

## 2. 选择服务器

- DNS 隧道需要**允许入站 UDP/53**。部分服务商会封锁或限速，购买前请确认其防火墙和网络策略。
- 选择离用户近、且用户网络仍能访问的地区。
- 优先选择没有端口限制、带宽充足的服务商。

## 3. DNS 记录

在安装**之前**为计划使用的协议创建记录。所有记录都设为 **DNS only**（Cloudflare 中的灰色云朵）：Caddy 必须能被直接访问才能获取 Let's Encrypt 证书，DNS 隧道需要服务器上的原始 DNS。

```text
A   ns.example.com   → SERVER_IP          DNS tunnels: the name server
NS  t.example.com    → ns.example.com     DNSTT / NoizDNS, SOCKS5 backend
NS  ts.example.com   → ns.example.com     DNSTT / NoizDNS, SSH backend
NS  v.example.com    → ns.example.com     VayDNS, SOCKS5 backend
NS  vs.example.com   → ns.example.com     VayDNS, SSH backend
NS  s.example.com    → ns.example.com     Slipstream, SOCKS5 backend
NS  ss.example.com   → ns.example.com     Slipstream, SSH backend
NS  r.example.com    → ns.example.com     Soar, direct backend
NS  rs.example.com   → ns.example.com     Soar, SSH backend
A   example.com      → SERVER_IP          NaïveProxy, and VLESS / SSH over TLS fronted by it
```

最简示例：

- **仅 NaïveProxy：** `A example.com → SERVER_IP`。
- **仅 DNSTT：** `A ns.example.com → SERVER_IP` 和 `NS t.example.com → ns.example.com`。

`NS` 记录把子域名委派给你的服务器：对 `t.example.com` 的查询会发到 `ns.example.com`，也就是你的服务器，由隧道在 53 端口应答。变更生效需要几分钟到几小时。检查方法：

```bash
dig +short A ns.example.com
dig +short NS t.example.com
```

唯一的例外是 Cloudflare 后的 VLESS，它需要一条**代理（Proxied）**记录；参见 [协议 › VLESS over WebSocket](protocols.md#vless-over-websocket)。

## 4. 安装

### 在线安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v1.21.2/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="a8aaa38d979049fa5e42c9f8f444e519d28625d1d67872ddaeff2d0dfc6b3498", "Installer checksum mismatch"'
sudo bash rovunet-install.sh
```

安装程序会询问要启用哪些协议，下载所需二进制文件，生成密钥，创建 systemd 服务和防火墙规则，创建第一个用户并输出该用户的全部配置。

在已配置的服务器上再次运行安装程序会保留现有隧道，只添加新协议，并再次输出所有配置。要为已有类型添加第二个隧道，请使用 `sudo rovunet tunnel add`。

同时选择 NaïveProxy 和 VLESS 时，VLESS（以及选中的 SSH over TLS）从一开始就由 NaïveProxy 前置在 443 端口。

### 离线安装（服务器无法访问 GitHub）

从本项目的 [最新发布](https://github.com/msVPNco/rovunet-panel/releases) 下载面板，从 [上游](https://github.com/anonvector/slipgate/releases) 下载传输二进制文件，然后复制到服务器：

```bash
# On your computer
mkdir rovunet-bundle && cd rovunet-bundle
curl -LO https://github.com/msVPNco/rovunet-panel/releases/latest/download/rovunet-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/dnstt-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/slipstream-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/vaydns-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/caddy-naive-linux-amd64
scp * root@SERVER_IP:/tmp/rovunet/

# On the server
chmod +x /tmp/rovunet/*
sudo cp /tmp/rovunet/rovunet-linux-amd64 /usr/local/bin/rovunet
sudo rovunet install --bin-dir /tmp/rovunet
```

ARM 服务器请用 `arm64` 代替 `amd64`。

### 从源码构建

```bash
git clone https://github.com/msVPNco/rovunet-panel.git
cd rovunet-panel
make build
sudo ./rovunet install
```

### 从 SlipGate 或旧版面板迁移

以面板旧名称（`slipgate`、`/etc/slipgate`）安装的服务器会原地迁移；参见 [运维 › 更新](operations.md#updating)。

## 5. 交互式菜单与命令行

- 不带参数运行 `sudo rovunet` 会打开交互式菜单，最适合首次配置。
- 每个菜单操作都对应一个带选项的命令，便于脚本化；参见 [命令参考](commands.md)。缺少的必填选项会以交互方式询问。

## 6. 第一个隧道和第一个用户

```bash
sudo rovunet tunnel add --transport dnstt --backend socks --tag mytunnel --domain t.example.com
sudo rovunet users add
sudo rovunet users configs
sudo rovunet tunnel status
```

`tunnel add` 生成隧道密钥，创建服务 `rovunet-mytunnel`（立即启动并开机自启），并在 DNS 路由器中注册域名。`users configs` 输出所有用户的全部配置。如果某个隧道不是 `active`，请运行 `sudo rovunet diag`。

下一页：[协议](protocols.md)
