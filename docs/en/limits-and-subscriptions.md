# Limits and subscription links

[Documentation](README.md) › Limits and subscription links

## Unlimited or limited users

When you add a user (`sudo rovunet users add`) the panel asks **Unlimited or Limited**. Unlimited is the default. For a limited user it asks for the number of **days** and the traffic in **GB**; each defaults to unlimited, so you can limit only time, only traffic, or both. `users bulk_add` accepts the same limits for many users at once.

```bash
sudo rovunet users list     # shows each user's expiry, quota and usage, and stopped users
sudo rovunet users renew    # bring back a stopped user: same login, same subscription link, usage reset to zero
```

- 1 GB is 1024³ bytes; upload and download both count.
- A systemd timer (`rovunet-user-expiry.timer`) checks every 5 minutes. A user who has expired or used the whole quota is **stopped**: access is removed, but the login and subscription link are kept so `users renew` can restore them.
- Changing a user's password keeps their limits and gives them a new subscription link.

### What is counted

Traffic limits can only be enforced on traffic the panel can measure. A limited user therefore receives only these configurations:

| Counted (given to limited users) | Not counted (given to unlimited users only) |
|---|---|
| VLESS over WebSocket | SSH, SSH over TLS, NaïveProxy |
| VLESS + REALITY and VLESS + XHTTP, while that core's statistics answer (`sudo rovunet diag` checks this) | DNS tunnels, Soar, Hysteria 2, WireGuard |

The panel also enforces this on the server, not only in the configs it prints. A limited user has **no SSH account** and no login on NaïveProxy, SOCKS5, Hysteria 2 or WireGuard, so a configuration they saved earlier (for example while unlimited) stops working, including after `users renew`. Their record, limit and subscription link stay. If every user of a Hysteria 2 or WireGuard core is limited, that core is stopped until an unlimited user exists.

Some access cannot be tied to one user and is outside any quota: Soar without a login, DNS tunnels without an SSH account behind them, and cores made with the older `--devices` option.

## Subscription links

Every user, limited or not, gets **one link** that carries all of their configurations:

```
https://your-domain/sub/<token>.json
```

Paste it into the app's **Subscriptions** menu. The file is a static JSON list served by the NaïveProxy site on port 443, so it needs a NaïveProxy tunnel and nothing else. The link is printed after a user is created and by `sudo rovunet users configs`.

- The link is kept up to date on its own: after any change to users or tunnels, after an update, and every 5 minutes for limited users.
- The token is random, so the link is not guessable. Treat it as a secret like the configurations themselves.
- When a limited user is stopped, the link stops delivering new configurations; after `users renew` the same link works again.

## Account status file

Next to each subscription there is a status file: the same link with `.json` replaced by `.status.json`.

```
https://your-domain/sub/<token>.status.json
```

It tells the app what the account may still use, so the app can show remaining traffic and days and say why configs stopped working. It grants nothing: the server enforces the limits on its own, and the file holds no password or config. The subscription file itself is unchanged.

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

| Key | Meaning |
|---|---|
| `server_id` | This panel. Random, made once, independent of the server's IP or name. |
| `account_id` | This account. Kept by `users renew` and `users passwd`; a user removed and added again gets a new one. |
| `period_id` | The current period. `users renew` starts a new one, so the app never adds two periods together. |
| `account_status` | `active`, `expired` or `limit_reached`. |
| `expires_at` | Expiry time (UTC), or `null` for none. |
| `limit_bytes` / `used_bytes` | Traffic limit and traffic used in this period, both directions, 1 GB = 1024³ bytes. Both `null` for a user without a traffic limit (their traffic is not counted on every protocol). |
| `metering_status` | `complete`, `partial` (a core did not answer the last read, or a counter file could not be read; the count may be low), `unavailable` (never read) or `not_metered` (no traffic limit). |
| `sampled_at` | The last read of the Xray counters in which every core answered. A failed read does not move it, so an old count never looks fresh. `null` without a traffic limit or before the first complete read. The VLESS counter is at most 15 seconds older than `generated_at`. |
| `generated_at` | When the file was written. Compare it with the HTTP `Date` header to allow for a wrong phone clock. |
| `revision` | Goes up by one whenever anything but the times changes. It belongs to this URL: after `users passwd` the link has a new token and the new file starts again at 1 (same `account_id` and `period_id`), so an app that follows a new link compares from scratch. |

- The file is rewritten on every user change and on every limits check (every 5 minutes while some user has a limit). It is served with `Cache-Control: no-store`.
- A stopped user keeps the status file (with `expired` or `limit_reached`) while the subscription file is removed. A user removed for good has neither. **404 does not mean expired**: it is also what an older panel or a removed account returns, so the app should show "account information unavailable".
- Like the subscription link, the status link carries the token: anyone who has it can see the account's usage.

## Several addresses for one server

If one IP of your server is blocked, other IPs of the same server may still work. Add them with a comma-separated list; the first is the main address (`auto` detects it):

```bash
sudo rovunet config address auto,198.51.100.9
```

Subscription links then carry each address-based configuration once per extra address, so the app can fall back to another address. Each extra address must really reach this server (an IP on its network interface, or a hostname pointing at it) with the same ports open. This adds addresses, not locations. `sudo rovunet config address auto` removes the extras.
