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

## 账户状态文件

每个订阅旁边都有一个状态文件：同一链接，把 `.json` 换成 `.status.json`。

```
https://your-domain/sub/<token>.status.json
```

它告诉应用该账户还能使用什么，以便应用显示剩余流量和天数，并说明配置为何停止工作。它不授予任何权限：限制由服务器自行执行，文件中没有密码或配置。订阅文件本身没有变化。

```json
{
  "version": 1,
  "server_id": "<32 hex>",
  "account_id": "<32 hex>",
  "period_id": "<32 hex>",
  "account_status": "active",
  "expires_at": "2026-12-01T00:00:00Z",
  "limit_bytes": 10737418240,
  "used_bytes": 1234567,
  "metering_status": "complete",
  "sampled_at": "2026-11-08T10:00:00Z",
  "generated_at": "2026-11-08T10:00:03Z",
  "revision": 7
}
```

| 键 | 含义 |
|---|---|
| `server_id` | 本面板。随机生成一次，与服务器的 IP 或名称无关。 |
| `account_id` | 本账户。`users renew` 和 `users passwd` 后保持不变；删除后重新添加的用户会得到新的。 |
| `period_id` | 当前周期。`users renew` 开始新周期，应用不会把两个周期的流量相加。 |
| `account_status` | `active`、`expired` 或 `limit_reached`。 |
| `expires_at` | 到期时间（UTC），无到期则为 `null`。 |
| `limit_bytes` / `used_bytes` | 本周期的流量上限和已用流量，双向合计，1 GB = 1024³ 字节。没有流量上限的用户两者均为 `null`（其流量并非在所有协议上都被统计）。 |
| `metering_status` | `complete`、`partial`（上次读取时有核心未响应，或某个计数文件无法读取，数值可能偏低）、`unavailable`（从未读取）或 `not_metered`（无流量上限）。 |
| `sampled_at` | 最近一次所有核心都响应的 Xray 计数器读取时间。读取失败不会更新它，因此旧数值不会显得是新的。无流量上限或首次完整读取之前为 `null`。VLESS 计数器最多比 `generated_at` 早 15 秒。 |
| `generated_at` | 文件写入时间。与 HTTP `Date` 响应头比较，可避免手机时钟错误的影响。 |
| `revision` | 除时间外任何内容变化时加一。它属于该 URL：执行 `users passwd` 后链接有新令牌，新文件重新从 1 开始（`account_id` 和 `period_id` 不变），因此改用新链接的应用应重新开始比较。 |

- 每次用户变更以及每次限额检查（只要有用户设了限制，每 5 分钟一次）都会重写该文件，并以 `Cache-Control: no-store` 提供。
- 更新后或在新服务器上，`metering_status` 在首次限额检查（约 5 分钟后，计时器会加最多 20 秒的随机延迟）之前为 `unavailable`。
- 被停用的用户保留状态文件（`expired` 或 `limit_reached`），但订阅文件会被删除。彻底删除的用户两者都没有。**404 不代表已到期**：旧版面板或已删除的账户也会返回 404，因此应用应显示“账户信息不可用”。
- 与订阅链接一样，状态链接也包含令牌：任何拿到它的人都能看到该账户的用量。

## 同一服务器的多个地址

如果服务器的某个 IP 被封锁，同一服务器的其他 IP 可能仍可用。用逗号分隔的列表添加它们；第一个是主地址（`auto` 表示自动检测）：

```bash
sudo rovunet config address auto,198.51.100.9
```

之后订阅链接会为每个额外地址各提供一份依赖地址的配置，应用可以切换到另一个地址。每个额外地址必须确实指向本服务器（网卡上的 IP，或指向它的主机名），并开放相同端口。这只是增加地址，不是增加地理位置。`sudo rovunet config address auto` 会清除额外地址。
