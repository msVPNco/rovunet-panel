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

## Several addresses for one server

If one IP of your server is blocked, other IPs of the same server may still work. Add them with a comma-separated list; the first is the main address (`auto` detects it):

```bash
sudo rovunet config address auto,198.51.100.9
```

Subscription links then carry each address-based configuration once per extra address, so the app can fall back to another address. Each extra address must really reach this server (an IP on its network interface, or a hostname pointing at it) with the same ports open. This adds addresses, not locations. `sudo rovunet config address auto` removes the extras.
