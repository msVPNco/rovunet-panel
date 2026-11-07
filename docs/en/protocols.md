# Protocols

[Documentation](README.md) › Protocols

## Overview

| Protocol | Family | Carried over | Default port | Certificate | Panel transport |
|---|---|---|---|---|---|
| [VLESS over WebSocket](#vless-over-websocket) | Proxy | WebSocket + TLS | 443/tcp fronted, or 2053/tcp | Let's Encrypt (fronted) or self-signed | `vless` |
| [NaïveProxy](#naiveproxy) | Proxy | HTTP/2 + TLS | 443/tcp | Let's Encrypt | `naive` |
| [SSH over TLS](#ssh-over-tls) (StunTLS) | SSH | TLS / WebSocket | 443/tcp fronted, or 8443/tcp | Self-signed, or Let's Encrypt when fronted | `stuntls` |
| [SSH](#ssh-and-socks5) | SSH | TCP | 22/tcp | — | `direct-ssh` |
| [SOCKS5](#ssh-and-socks5) | Proxy | TCP, unencrypted | 1080/tcp | — | `direct-socks5` |
| [DNSTT](#dns-tunnels) / NoizDNS | DNS tunnel | DNS, Curve25519 | 53/udp + 53/tcp | — | `dnstt` |
| [VayDNS](#dns-tunnels) | DNS tunnel | DNS, KCP + Curve25519 | 53/udp + 53/tcp | — | `vaydns` |
| [Slipstream](#dns-tunnels) | DNS tunnel | QUIC over DNS | 53/udp + 53/tcp | Self-signed | `slipstream` |
| [Soar](#dns-tunnels) | DNS tunnel | Multipath DNS, X25519 + Ed25519 | 53/udp + 53/tcp | — | `soar` |
| [VLESS + REALITY](xray-cores.md) | Xray core | TCP, REALITY + XTLS Vision | any free TCP port | None (imitates a real site) | `core-install` |
| [VLESS + XHTTP](xray-cores.md) | Xray core | HTTP requests + TLS | any free TCP port | Required | `core-install` |
| [Hysteria 2](xray-cores.md) | Xray core | QUIC | any free UDP port | Required | `core-install` |
| [WireGuard](xray-cores.md) | Xray core | UDP | any free UDP port | — | `core-install` |
| [External DNS route](#external-dns-route) | Custom | DNS forwarded to a local port | 53/udp | — | `external` |

## Choosing a protocol

Field results from one server in Iran, late 2026. They are a starting point, not a guarantee; networks differ and change.

| Situation | Try first |
|---|---|
| HTTPS works | VLESS fronted by NaïveProxy on 443: fast, stable, looks like an ordinary website |
| Port 22 is open | SSH: fast |
| The server's IP is blocked | VLESS behind Cloudflare: works, noticeably slower |
| TLS to well-known sites passes | VLESS + REALITY |
| Lossy networks where UDP passes | Hysteria 2 |
| Only DNS gets through | VayDNS or DNSTT: slow, but they work |
| SSH over TLS on its own port is reset | Front it with NaïveProxy on 443 |
| Open networks only | SOCKS5: it is unencrypted and filtered networks cut it |

## Backends

DNS tunnels and NaïveProxy hand decrypted traffic to a backend:

| Backend | Behaviour |
|---|---|
| `socks` | The panel's built-in SOCKS5 proxy. Simple and fast |
| `ssh` | The server's OpenSSH forwards traffic, so every user needs an SSH login |
| `both` | Creates two tunnels, `<tag>-socks` and `<tag>-ssh`; the SSH variant's subdomain gets an extra `s` |
| `direct` | Soar only: the server dials each stream's destination itself |

## VLESS over WebSocket

The panel serves VLESS over WebSocket itself. Each user gets a UUID derived from their username, password and a server-wide secret (`/etc/rovunet/vless.secret`): removing a user or changing the password revokes it within seconds, and a shared configuration never carries the password. Streams reach public addresses only; port 25 is blocked by default (`--block-ports`). TCP and plain UDP are supported, Mux and XUDP are not.

There are three ways to reach it:

| Mode | How | When |
|---|---|---|
| **Fronted by NaïveProxy** (recommended) | Caddy carries it on 443 with its Let's Encrypt certificate; other paths show the decoy site | Default when NaïveProxy is installed |
| **Behind Cloudflare** | Clients reach a Cloudflare IP; the server's IP stays hidden | When the server's IP is blocked |
| **Direct** | Its own port with a self-signed certificate | Testing |

```bash
# Fronted by NaïveProxy
sudo rovunet tunnel add --transport vless --tag myvless --domain example.com --cdn no --listen-port 2053
sudo rovunet tunnel edit --tag myvless --front myproxy      # "-" undoes it

# Behind Cloudflare
sudo rovunet tunnel add --transport vless --tag myvless-cf --domain cdn.example.com --cdn yes --listen-port 2053
sudo rovunet tunnel edit --tag myvless-cf --cdn-address 104.16.0.1   # clean Cloudflare IP, "-" resets
```

Cloudflare settings for the proxied hostname:

1. DNS: `A cdn.example.com → SERVER_IP`, **Proxied** (orange cloud).
2. SSL/TLS › Overview: **Full**. Flexible does not work and Full (strict) rejects the self-signed origin certificate.
3. Network: **WebSockets** on.
4. Port: Cloudflare proxies HTTPS only on 443, 2053, 2083, 2087, 2096 and 8443; with NaïveProxy on 443, use 2053.

Configurations are per user: `sudo rovunet tunnel share myvless --user alice` prints a `rovunet://` link for the Rovunet app and a standard `vless://` link for v2rayNG, Hiddify, NekoBox and similar clients. Fronted links need no `allowInsecure`, which Xray clients stop accepting from August 2026. SNI fragmentation is off in both link types and can be turned on in the app.

<a id="naiveproxy"></a>
## NaïveProxy

Caddy with the forwardproxy plugin. Caddy obtains and renews a Let's Encrypt certificate (ports 80 and 443 must be open) and shows a decoy website to anything that is not an authenticated proxy request. Every panel user can log in; `users add`, `remove` and `passwd` update Caddy live. A Naïve tunnel without any login gets a random one and stays closed.

```bash
sudo rovunet tunnel add --transport naive --backend socks --tag myproxy \
  --domain example.com --email admin@example.com --decoy-url https://www.wikipedia.org
```

### Fronting on 443

NaïveProxy's Caddy can carry VLESS and SSH over TLS on the same domain, port and certificate. Caddy routes each request: a secret WebSocket path goes to VLESS or SSH over TLS, an authenticated proxy request to NaïveProxy, everything else to the decoy site. Every Caddyfile change is validated with `caddy-naive validate` before it is applied; if validation fails, the previous file stays.

```bash
sudo rovunet tunnel edit --tag myvless --front naive
sudo rovunet tunnel edit --tag mytls --front naive
sudo rovunet users configs        # hand out the new configurations
```

<a id="ssh-over-tls"></a>
## SSH over TLS (StunTLS)

SSH inside TLS and WebSocket, served by the panel. It detects WebSocket, HTTP CONNECT, raw TLS and payload modes. On its own port (8443) it uses a self-signed certificate and needs no domain; fronted by NaïveProxy, clients reach it as SSH over WebSocket on the Naïve domain, port 443, with a real certificate. If the app reports `Connection was reset` on its own port, the network blocks that port: front it.

```bash
sudo rovunet tunnel add --transport stuntls --tag mytls
sudo rovunet tunnel edit --tag mytls --front myproxy
```

## SSH and SOCKS5

```bash
sudo rovunet tunnel add --transport direct-ssh --tag myssh
sudo rovunet tunnel add --transport direct-socks5 --tag mysocks
```

SSH uses the server's OpenSSH on port 22; tunnel users get password logins with no shell (forwarding only). SOCKS5 listens on 1080 and is unencrypted. A SOCKS5 proxy with no users refuses connections rather than becoming an open proxy.

## DNS tunnels

| Tunnel | Description |
|---|---|
| **DNSTT** | Curve25519-encrypted DNS tunnel. The same server also serves **NoizDNS** clients, which add DPI evasion (base36/hex encoding, CDN prefix stripping) |
| **VayDNS** | KCP-based DNS tunnel with Curve25519 and a tunable wire format: record type, idle timeout, keep-alive, client ID size, queue size |
| **Slipstream** | QUIC over DNS with certificate authentication. Experimental |
| **Soar** | Multipath DNS tunnel ([Lantern](https://github.com/getlantern/soar)) with resolver discovery, built into the panel. **Direct** backend: no login, the server dials public destinations on the allowed ports (default `80,443`), compatible with stock Soar clients. **SSH** backend: reaches only the SSH server, so access goes through SSH users |

```bash
sudo rovunet tunnel add --transport dnstt  --backend socks  --tag mydnstt  --domain t.example.com
sudo rovunet tunnel add --transport vaydns --backend both   --tag myvaydns --domain v.example.com \
  --record-type txt --idle-timeout 10s --keep-alive 2s --clientid-size 2 --queue-size 512
sudo rovunet tunnel add --transport slipstream --backend ssh --tag myslip --domain s.example.com
sudo rovunet tunnel add --transport soar --backend direct --tag mysoar --domain r.example.com --allow-ports 80,443
sudo rovunet tunnel add --transport soar --backend ssh    --tag mysoar-ssh --domain rs.example.com
```

DNSTT and VayDNS accept existing keys with `--private-key` (and an optional `--public-key`); Soar accepts a base64 Ed25519 seed with `--private-key`. `sudo rovunet mtu 1200` sets the MTU of every DNSTT, NoizDNS and VayDNS tunnel at once (1232 is the default).

## External DNS route

Forwards DNS queries for a domain to a local UDP port, for a protocol you run yourself. The panel manages no service for it.

```bash
sudo rovunet tunnel add --transport external --tag my-proto --domain j.example.com --port 5301
```

## Domain layout

| Tunnel | Domain | Backend |
|---|---|---|
| dnstt-socks / dnstt-ssh | `t.example.com` / `ts.example.com` | SOCKS5 / SSH |
| vaydns-socks / vaydns-ssh | `v.example.com` / `vs.example.com` | SOCKS5 / SSH |
| slipstream-socks / slipstream-ssh | `s.example.com` / `ss.example.com` | SOCKS5 / SSH |
| soar-direct / soar-ssh | `r.example.com` / `rs.example.com` | Direct / SSH |
| naive-socks / naive-ssh | `example.com` (shared) | SOCKS5 / SSH |
| vless, stuntls | `example.com`, through NaïveProxy | per-user UUIDs / SSH |

Each DNS tunnel needs its own subdomain; DNSTT and NoizDNS share one. NaïveProxy tunnels share a domain because they use HTTPS, and VLESS and SSH over TLS ride on it, so they need no record of their own.

## Routing modes

| Mode | Behaviour |
|---|---|
| **Single** | One DNS tunnel is active; the DNS router on port 53 forwards to it |
| **Multi** | Every DNS tunnel runs on a local port; the router dispatches queries by domain, always to the most specific matching domain. Enabled automatically with a second DNS tunnel |

The router answers on UDP and TCP port 53. TCP queries (RFC 7766, pipelined) are forwarded to the tunnel over UDP and answered on the same connection, so every DNS tunnel also works for resolvers and clients that use TCP/53.

## Architecture

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

Soar with the direct backend, VLESS and the Xray cores dial each destination themselves, so their traffic skips the backend layer and WARP.

Next: [Users and client configurations](users-and-configs.md)
