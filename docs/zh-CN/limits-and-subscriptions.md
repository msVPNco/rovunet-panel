# 流量限额与订阅链接

[文档](README.md) › 流量限额与订阅链接

## 不限量与限量用户

添加用户（`sudo rovunet users add`）时，面板会询问**不限量还是限量**，默认不限量。对限量用户，面板会询问**天数**和流量（**GB**）；两项默认都是不限，所以可以只限时间、只限流量，或两者都限。`users bulk_add` 也可一次为多个用户设置相同限制。

```bash
sudo rovunet users list     # 显示每个用户的到期时间、额度和已用流量，以及已停用的用户
sudo rovunet users renew    # 恢复已停用的用户：登录信息和订阅链接不变，已用流量清零
```

- 1 GB 为 1024³ 字节，上传和下载都计入。
- systemd 定时器（`rovunet-user-expiry.timer`）每 5 分钟检查一次。已到期或用完额度的用户会被**停用**：访问权限被移除，但登录信息和订阅链接会保留，以便 `users renew` 恢复。
- 修改用户密码会保留其限制，并生成新的订阅链接。

### 哪些流量会被计数

流量限额只能对面板能够统计的流量生效，因此限量用户只会获得以下配置：

| 会计数（提供给限量用户） | 不计数（只提供给不限量用户） |
|---|---|
| 基于 WebSocket 的 VLESS | SSH、TLS 上的 SSH、NaïveProxy |
| VLESS + REALITY 和 VLESS + XHTTP（前提是该内核的统计接口有响应，可用 `sudo rovunet diag` 检查） | DNS 隧道、Soar、Hysteria 2、WireGuard |

面板在服务器端也执行这一规则，而不只是在输出的配置里。限流用户**没有 SSH 账号**，也不能登录 NaïveProxy、SOCKS5、Hysteria 2 和 WireGuard，因此他之前保存的配置（例如不限流时拿到的）会失效，`users renew` 之后也一样。他的用户记录、限额和订阅链接保持不变。如果某个 Hysteria 2 或 WireGuard 内核的所有用户都被限流，该内核会停止，直到出现不限流的用户。

有些访问无法绑定到某个用户，不受任何配额约束：无需登录的 Soar、背后没有 SSH 账号的 DNS 隧道，以及用旧的 `--devices` 选项创建的内核。

## 订阅链接

每个用户（无论是否限量）都有**一条链接**，包含其所有配置：

```
https://your-domain/sub/<token>.json
```

将它粘贴到应用的 **Subscriptions（订阅）** 菜单即可。该文件是静态 JSON 列表，由 443 端口上的 NaïveProxy 站点提供，因此只需要一个 NaïveProxy 隧道。链接会在创建用户后以及执行 `sudo rovunet users configs` 时打印出来。

- 链接会自动保持最新：用户或隧道变更后、更新后，以及对限量用户每 5 分钟一次。
- 令牌是随机的，链接无法被猜到。请像对待配置本身一样保密。
- 限量用户被停用期间，链接不再提供新配置；执行 `users renew` 后同一链接恢复可用。

## 同一服务器的多个地址

如果服务器的某个 IP 被封锁，同一服务器的其他 IP 可能仍可用。用逗号分隔的列表添加它们；第一个是主地址（`auto` 表示自动检测）：

```bash
sudo rovunet config address auto,198.51.100.9
```

之后订阅链接会为每个额外地址各提供一份依赖地址的配置，应用可以切换到另一个地址。每个额外地址必须确实指向本服务器（网卡上的 IP，或指向它的主机名），并开放相同端口。这只是增加地址，不是增加地理位置。`sudo rovunet config address auto` 会清除额外地址。
