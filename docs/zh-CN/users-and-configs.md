# 用户与客户端配置

[文档](README.md) › 用户与客户端配置

## 用户模型

用户是**全局**的：同一个用户名和密码可用于所有隧道。协议属于隧道，而不属于用户。每个用户可以同时在所有隧道上认证。

> **切勿把 `root` 或任何带 shell 的账户当作 VPN 登录分发。** 面板用户仅用于转发；泄露一个只会失去隧道访问，泄露 `root` 则会失去整台服务器。为每个人（包括你自己）单独创建面板用户。

## 管理用户

```bash
sudo rovunet users add                              # one user
sudo rovunet users bulk_add --count=50 --prefix=user  # user001…user050, random passwords, up to 500
sudo rovunet users list                             # users and their configurations
sudo rovunet users remove                           # remove a user
```

给每个人单独的用户，这样手机丢失时可以只撤销那一个。

## 获取配置

| 命令 | 输出 |
|---|---|
| `sudo rovunet users configs` | 所有用户在所有隧道上的全部配置 |
| `sudo rovunet tunnel share <tag>` | 单个隧道的配置；会询问嵌入哪个用户的凭据 |
| `sudo rovunet tunnel share <tag> --user alice` | VLESS：alice 的 `rovunet://` 和 `vless://` 链接 |
| `sudo rovunet export-clients --user alice --format rovunet` | 便于脚本使用的 alice 链接列表；`--annotated` 为每条标注其隧道 |

对 DNSTT 隧道，分享时会询问使用 **DNSTT** 还是 **NoizDNS** 配置。两者连接同一服务器；NoizDNS 在客户端开启 DPI 规避（更难识别，但更慢）。

配置包含用户凭据。请通过私密渠道发送，切勿发到公开群组。

## 导出格式

| 格式 | 协议头 | 适用于 |
|---|---|---|
| 原生 | `rovunet://` | Rovunet 应用。携带完整配置、用户所属集合（分组），自 v33 起无损携带凭据，包括 `\|` 等字符 |
| 标准 VLESS | `vless://` | 任意 VLESS 客户端（v2rayNG、Hiddify、NekoBox 等），在每个 VLESS 配置旁输出 |
| Xray 内核 | `rovunet://`（类型 `xray`），或 `vless://`、`hysteria2://`、WireGuard `.conf` | 每个内核上每个用户一份；参见 [Xray 内核](xray-cores.md) |
| 旧版 | `slipnet://` | 旧版应用；仍可导入 |

### 应用中的集合

应用按服务器和用户对导入的配置分组，因此一个用户的所有路径构成一个集合。direct 后端的 Soar 无需登录，但会为每个用户单独输出，从而加入各自的集合。Xray 内核配置也按用户输出到同一集合中。

## 服务器地址

独立端口上的 SSH over TLS、SSH 和 SOCKS5 配置包含服务器 IP。云服务器（Oracle、AWS、GCP、Azure）的网卡上只有私有 IP，因此面板会寻找公网 IP：如果服务器自身 IP 是公网 IP 就用它，否则使用 `ns.<域名>` 或 NaïveProxy 域名指向的 IP。都找不到时会询问一次并保存。

```bash
sudo rovunet config address 203.0.113.7   # an IP or hostname, without a port
sudo rovunet config address auto          # detect again
```

DNS 隧道、Soar、NaïveProxy、VLESS 和前置的 SSH over TLS 配置通过域名连接，不受其影响。

## 客户端应用

- **[Rovunet for Android](https://github.com/msVPNco/rovunet-android)**（推荐）：导入 `rovunet://` 链接，将每个用户的路径归入一个集合，无需预先 ping 即可连接所选路径，并提供独立的真实隧道 HTTPS 测试。
- **标准客户端**，如 v2rayNG、Hiddify 或 NekoBox：使用 `vless://` 链接和 Xray 内核文件。

下一页：[Xray 内核](xray-cores.md)
