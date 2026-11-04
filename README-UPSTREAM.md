# SlipGate

Unified tunnel manager for Linux servers. Manages DNS tunnels (DNSTT, NoizDNS, Slipstream, VayDNS, Soar), HTTPS proxies (NaiveProxy, StunTLS) and VLESS over WebSocket, with systemd services, multi-tunnel DNS routing, and user management. Designed for use with the [SlipNet](https://github.com/anonvector/SlipNet) Android VPN app.

> **This fork** ([msVPNco/rovunet-panel](https://github.com/msVPNco/rovunet-panel)) adds a built-in **VLESS over WebSocket** transport and lets VLESS and StunTLS ride on **NaiveProxy's Caddy on port 443** with its real Let's Encrypt certificate, so they look like an ordinary HTTPS website, with no Cloudflare hop and no self-signed certificate. It also fixes multi-user NaiveProxy, StunTLS keepalives, direct SOCKS5 and Soar configs (see [What this fork changes](#what-this-fork-changes)). It installs and self-updates from this fork's releases; transport binaries (dnstt-server, caddy-naive, …) still come from upstream [anonvector/slipgate](https://github.com/anonvector/slipgate). Install with:
>
> ```bash
> curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
> ```
>
> A Persian guide is in [docs/VLESS_FA.md](docs/VLESS_FA.md); release notes are in [CHANGELOG.md](CHANGELOG.md).

## What this fork changes

| Area | Change |
|------|--------|
| **VLESS** | New built-in VLESS over WebSocket server: per-user UUIDs derived from SlipGate users (revoked within seconds when a user is removed), nginx-like decoy, public destinations only. Three ways to reach it: **through NaiveProxy's Caddy on 443** (recommended), behind Cloudflare, or straight to its own port |
| **Fronting on 443** | `tunnel edit --front <naive-tag>` puts VLESS and/or StunTLS behind Caddy: real certificate and SNI, standard port, every other path shows the decoy site. Install does it automatically when NaiveProxy is selected. Caddyfile changes are validated with `caddy-naive validate` before they are applied |
| **NaiveProxy** | Every SlipGate user can log in (only the first user could before); users add/remove/passwd update Caddy live |
| **StunTLS** | WebSocket pings are answered to the client (they used to corrupt the SSH stream) |
| **SOCKS5** | The direct SOCKS5 tunnel listens publicly again after user changes |
| **Soar** | Direct (no-login) Soar configs appear in `users configs` |
| **Install** | One command sets up every selected protocol and prints all configs at the end. Running it again on a configured server keeps existing tunnels (no clashing copies) and still prints every config. `/run/sshd` is created on systems with socket-activated SSH; firewall rules are inserted ahead of REJECT rules |
| **VLESS links** | SNI fragmentation stays off when SlipNet imports either the `slipnet://` config or the `vless://` link (it slowed connections to time-outs behind Cloudflare); it can be turned on in the app |

### Which protocol where

Field results from Iran (one server, late 2026), as a starting point:

- **VLESS fronted by NaiveProxy (443)**: fast and stable; looks like a normal HTTPS site. First choice.
- **Direct SSH (22)**: fast where port 22 is open.
- **VayDNS / DNSTT**: slow but work when only DNS gets through.
- **VLESS behind Cloudflare**: works, but much slower than direct; keep it for when the server's IP is blocked.
- **StunTLS**: its own port (8443) on a bare IP was dropped; front it with NaiveProxy.
- **Direct SOCKS5**: plaintext, so filtered networks cut it. A fallback for open networks only.
- **NaiveProxy**: the server side works (tested with curl through it), but SlipNet's Naive client dropped its streams in our tests; try another Naive client (e.g. NekoBox) if SlipNet shows no traffic.

[![oosmetrics](https://api.oosmetrics.com/api/v1/badge/achievement/12767996-2dd6-42ed-89a6-5f786c4662b4.svg)](https://oosmetrics.com/repo/anonvector/slipgate)

## Features

- **Multi-transport**: DNSTT/NoizDNS (DNS tunnels with Curve25519 encryption), Slipstream (QUIC-based DNS), VayDNS (KCP-based DNS with Curve25519), Soar (multipath DNS, built in), NaiveProxy (HTTPS with Caddy), StunTLS (SSH over TLS + WebSocket), VLESS (over WebSocket, built in)
- **Fronting on 443**: VLESS and StunTLS can share NaiveProxy's Caddy, port 443 and Let's Encrypt certificate, behind its decoy site
- **Dual backend**: Built-in SOCKS5 proxy or SSH forwarding (custom SSH port supported)
- **DNS routing**: Single-tunnel or multi-tunnel mode with domain-based dispatch
- **External routing**: Forward DNS queries for a domain to a custom port for user-managed protocols
- **WARP integration**: Optional Cloudflare WARP outbound routing (see [dnstun-ezpz](https://github.com/aleskxyz/dnstun-ezpz) for an alternative approach)
- **User management**: Multi-user SSH + SOCKS credentials (all users authenticate simultaneously), each also giving the user their own VLESS UUID, with bulk creation of up to 500 users per call
- **Live dashboard**: Real-time TUI with CPU, RAM, traffic sparklines, per-protocol connection stats, and tunnel status
- **Backup & restore**: Move a whole server (config, keys, certificates, users) as one file or as text to copy-paste
- **Diagnostics**: Built-in health checks for services, ports, keys, DNS resolution, and boot persistence
- **Interactive TUI + CLI**: Menu-driven setup or scriptable subcommands
- **Systemd integration**: Service creation, lifecycle, and logs
- **Auto-TLS**: Let's Encrypt via Caddy for NaiveProxy tunnels
- **Self-update**: Version checking and binary replacement from GitHub releases
- **Client sharing**: Generates `slipnet://` URIs for one-tap app import, plus standard `vless://` links for VLESS tunnels

## Requirements

- **OS**: Linux (Ubuntu 20.04+, Debian 10+, or similar)
- **Domain**: DNS records pointed at your server (required for DNS tunnels, NaiveProxy and VLESS; see [Domain Layout](#domain-layout))
- **Cloudflare** (only for VLESS behind Cloudflare): the domain's DNS hosted at Cloudflare, so the VLESS hostname can be proxied (orange cloud). Not needed when VLESS is fronted by NaiveProxy
- **Ports**: 53/udp + 53/tcp (DNS tunnels), 80/tcp + 443/tcp (NaiveProxy and everything it fronts), 8443/tcp (StunTLS on its own port), a Cloudflare-proxied port such as 2053 (VLESS behind Cloudflare)

## Quick Start

**One-liner install:**

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

**Or build from source:**

```bash
git clone https://github.com/msVPNco/rovunet-panel.git
cd slipgate
make build
sudo ./slipgate install
```

**Offline install (SCP to server):**

Download slipgate from this fork's [latest release](https://github.com/msVPNco/rovunet-panel/releases) and the transport binaries you need from [upstream's](https://github.com/anonvector/slipgate/releases):

```bash
# On your local machine — download binaries
mkdir slipgate-bundle && cd slipgate-bundle
curl -LO https://github.com/msVPNco/rovunet-panel/releases/latest/download/slipgate-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/dnstt-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/slipstream-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/vaydns-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/caddy-naive-linux-amd64

# SCP to server
scp * user@server:/tmp/slipgate/

# On the server
chmod +x /tmp/slipgate/*
sudo cp /tmp/slipgate/slipgate-linux-amd64 /usr/local/bin/slipgate
sudo slipgate install --bin-dir /tmp/slipgate
```

**Switch a server running upstream SlipGate to this fork** (config, tunnels and users are kept; use `arm64` on ARM servers):

```bash
sudo curl -fsSL -o /usr/local/bin/slipgate https://github.com/msVPNco/rovunet-panel/releases/latest/download/slipgate-linux-amd64
sudo chmod +x /usr/local/bin/slipgate
sudo slipgate post-update      # refreshes transport binaries and regenerates services
```

From then on `slipgate update` updates from this fork.

Then launch the interactive menu:

```bash
sudo slipgate
```

## CLI Usage

```
slipgate                        # Interactive TUI menu
slipgate install                # Install dependencies and configure server
slipgate uninstall              # Remove all services, configs, and binaries
slipgate update                 # Self-update and restart all services
slipgate restart                # Restart all services (DNS router, tunnels, SOCKS)
slipgate backup [file]          # Back up config, keys and users (file, or text to copy-paste)
slipgate restore [file]         # Restore a backup on this server ("-" reads stdin)
slipgate users                  # Manage users (SSH, SOCKS and VLESS) and view configs
slipgate users add              # Add a single user
slipgate users bulk_add         # Add multiple users in one batch (random creds, up to 500)
slipgate users remove           # Remove a user
slipgate users list             # List users and their per-tunnel configs
slipgate users configs          # Show every config (all tunnels and users, incl. no-login Soar and vless:// links)
slipgate stats                  # Live dashboard (CPU, RAM, traffic, connections, tunnels)
slipgate diag                   # Run diagnostics (services, ports, keys, DNS, boot status)
slipgate mtu [value]            # Set MTU for all DNSTT/NoizDNS/VayDNS tunnels at once

# Tunnel management
slipgate tunnel add             # Add tunnel(s) — supports multi-select and "both" backend
slipgate tunnel edit [tag]      # Edit tunnel settings (tag, MTU, keys, VLESS path / CDN IP, --front)
slipgate tunnel remove [tag]    # Remove a tunnel
slipgate tunnel remove --all    # Remove all tunnels at once
slipgate tunnel start [tag]     # Start a tunnel
slipgate tunnel stop [tag]      # Stop a tunnel
slipgate tunnel status          # Show all tunnel statuses
slipgate tunnel status [tag]    # Show tunnel details (keys, MTU, port)
slipgate tunnel share [tag]     # Generate slipnet:// URI for clients (+ vless:// link per user for VLESS)
slipgate tunnel logs [tag]      # View tunnel logs

# DNS routing
slipgate router status          # Show DNS routing config
slipgate router mode            # Switch between single/multi mode
slipgate router switch          # Change active tunnel (single mode)

# Configuration
slipgate config export          # Print config.json (no keys; use backup to move a server)
slipgate config import [path]   # Replace config.json (no keys)
slipgate config address [ip]    # Public IP or hostname for StunTLS/SSH/SOCKS5 configs ("auto" to detect)

# Internal (used by systemd services)
slipgate dnsrouter serve        # Start DNS router
slipgate socks serve            # Start built-in SOCKS5 proxy
slipgate stuntls serve          # Start StunTLS proxy
slipgate vless serve            # Start VLESS over WebSocket server
```

### Non-Interactive Examples

All commands support flags for scripting and automation. If any required flag is omitted, slipgate falls back to an interactive prompt.

```bash
# DNSTT tunnel
sudo slipgate tunnel add \
  --transport dnstt \
  --backend socks \
  --tag mydnstt \
  --domain t.example.com

# DNSTT tunnel with custom Curve25519 keys
sudo slipgate tunnel add \
  --transport dnstt \
  --backend socks \
  --tag mytunnel \
  --domain t.example.com \
  --private-key <64-char-hex> \
  --public-key <64-char-hex>   # optional, validated if provided

# DNSTT with both backends (creates mydnstt-socks + mydnstt-ssh)
sudo slipgate tunnel add \
  --transport dnstt \
  --backend both \
  --tag mydnstt \
  --domain t.example.com

# VayDNS tunnel (KCP + Curve25519)
sudo slipgate tunnel add \
  --transport vaydns \
  --backend socks \
  --tag myvaydns \
  --domain v.example.com

# VayDNS with all tuning parameters
sudo slipgate tunnel add \
  --transport vaydns \
  --backend both \
  --tag myvaydns \
  --domain v.example.com \
  --record-type txt \
  --idle-timeout 10s \
  --keep-alive 2s \
  --clientid-size 2 \
  --queue-size 512

# Soar tunnel (built into slipgate, no extra binary). Direct backend: no
# login, the server reaches public sites itself (works with stock soar clients)
# Optional: --private-key <base64 Ed25519 seed> to import an existing key
sudo slipgate tunnel add \
  --transport soar \
  --backend direct \
  --tag mysoar \
  --domain r.example.com \
  --allow-ports 80,443

# Soar with the SSH backend: the server reaches only the SSH server, so each
# person needs their own SSH user (SlipNet; share with 'slipgate tunnel share')
sudo slipgate tunnel add \
  --transport soar \
  --backend ssh \
  --tag mysoar-ssh \
  --domain rs.example.com

# Slipstream tunnel
sudo slipgate tunnel add \
  --transport slipstream \
  --backend ssh \
  --tag myslip \
  --domain s.example.com

# NaiveProxy tunnel
sudo slipgate tunnel add \
  --transport naive \
  --backend socks \
  --tag myproxy \
  --domain example.com \
  --email admin@example.com \
  --decoy-url https://www.wikipedia.org

# StunTLS tunnel (SSH over TLS + WebSocket), then carry it on NaiveProxy's 443
sudo slipgate tunnel add \
  --transport stuntls \
  --tag mytls
sudo slipgate tunnel edit --tag mytls --front myproxy

# VLESS over WebSocket on NaiveProxy's 443 (recommended). Each user gets their
# own UUID; share per user with 'slipgate tunnel share myvless --user alice'.
# Optional: --block-ports "25,465" (default 25, "none" for no limit)
sudo slipgate tunnel add \
  --transport vless \
  --tag myvless \
  --domain example.com \
  --cdn no \
  --listen-port 2053
sudo slipgate tunnel edit --tag myvless --front myproxy

# Optional: VLESS behind Cloudflare instead (orange-cloud hostname, SSL mode Full),
# for when the server's IP is blocked. Also: --tls no, --cdn-address <clean IP>
sudo slipgate tunnel add \
  --transport vless \
  --tag myvless-cf \
  --domain cdn.example.com \
  --cdn yes \
  --listen-port 2053

# External DNS routing (forward queries to a custom port)
sudo slipgate tunnel add \
  --transport external \
  --tag my-proto \
  --domain j.example.com \
  --port 5301
# Queries for j.example.com route to 127.0.0.1:5301

# Direct SSH / SOCKS5 transports
sudo slipgate tunnel add --transport direct-ssh --tag myssh
sudo slipgate tunnel add --transport direct-socks5 --tag mysocks

# Rename a tunnel
sudo slipgate tunnel edit --tag mydnstt --new-tag my-tunnel

# Change MTU on a DNSTT tunnel
sudo slipgate tunnel edit --tag mydnstt --mtu 1232

# Put a clean Cloudflare IP into future VLESS configs ("-" goes back to the domain)
sudo slipgate tunnel edit --tag myvless --cdn-address 104.16.0.1

# Serve VLESS through NaiveProxy's Caddy: port 443, its Let's Encrypt
# certificate, the decoy site for probes, no Cloudflare ("-" undoes it)
sudo slipgate tunnel edit --tag myvless --front naive

# Carry StunTLS (SSH over WebSocket) the same way, for networks that drop its own port
sudo slipgate tunnel edit --tag stuntls --front naive

# Set MTU for all DNSTT/NoizDNS/VayDNS tunnels at once (rewrites and restarts each service)
sudo slipgate mtu 1200

# Tune VayDNS parameters
sudo slipgate tunnel edit --tag myvaydns \
  --mtu 1232 \
  --record-type txt \
  --idle-timeout 10s \
  --keep-alive 2s \
  --clientid-size 2 \
  --queue-size 512

# View tunnel details (keys, MTU, port, status)
sudo slipgate tunnel status --tag mydnstt

# Share tunnel config as slipnet:// URI
sudo slipgate tunnel share mydnstt

# VLESS configs are per user: slipnet:// and vless:// for alice
sudo slipgate tunnel share myvless --user alice

# Move to a new server: same keys and passwords, so client configs keep working.
# Afterwards point the DNS records (ns.<domain>, NaiveProxy and VLESS domains) at the new IP.
ssh root@old-server slipgate backup - | ssh root@new-server slipgate restore - --yes

# Bulk-add SSH/SOCKS users (random passwords, up to 500 per call)
sudo slipgate users bulk_add --count=50 --prefix=user
# Creates user001..user050 with random passwords. A single SOCKS reload
# and WARP rule sync runs for the whole batch.
```

## Architecture

```
                                  ┌──────────────────┐
                                  │  SlipNet Client  │
                                  └────────┬─────────┘
          ┌────────────────────────────────┼──────────────────────────────┐
          │                                │                              │
   DNS :53/udp+tcp                   HTTPS :443/tcp             optional: Cloudflare
          │                     one domain, real certificate     (orange cloud) :2053
          │                                │                              │
┌─────────┼────────────────────────────────┼──────────────────────────────┼─────┐
│ SERVER  v                                v                              │     │
│ ┌────────────────────────┐ ┌──────────────────────────────────────┐     │     │
│ │       DNS Router       │ │      Caddy (NaiveProxy) on :443      │     │     │
│ │ domain-based dispatch  │ │  Let's Encrypt cert + decoy website  │     │     │
│ │ single / multi mode    │ │  routes by secret WebSocket path     │     │     │
│ └──┬─────┬─────┬─────┬───┘ └────┬─────────────┬────────────────┬──┘     │     │
│    v     v     v     v          v             v                v        v     │
│ ┌─────┐┌─────┐┌─────┐┌─────┐ ┌─────────┐ ┌──────────┐ ┌────────────────────┐  │
│ │DNSTT││Slip-││VayD-││Soar │ │ forward │ │ StunTLS  │ │       VLESS        │  │
│ │+Noiz││strea││ NS  ││     │ │  proxy  │ │ SSH over │ │   per-user UUIDs   │  │
│ │     ││  m  ││     ││     │ │ (Naive) │ │WebSocket │ │  nginx-like decoy  │  │
│ └──┬──┘└──┬──┘└──┬──┘└──┬──┘ └────┬────┘ └────┬─────┘ └─────────┬──────────┘  │
│    └──────┴──┬───┴──────┘         │           │                 │             │
│              v                    v           v                 │             │
│ ┌────────────────────────────────────────────────────────┐      │             │
│ │                     Backend Layer                      │      │             │
│ │   SOCKS5 proxy :1080          SSH forwarding :22       │      │             │
│ └───────────────────────────┬────────────────────────────┘      │             │
│                             v                                   │             │
│                   ┌───────────────────┐                         │             │
│                   │  WARP (optional)  │                         │             │
│                   └─────────┬─────────┘                         │             │
│                             └───────────────────┬───────────────┘             │
│                                                 v                             │
│                                             Internet                          │
└───────────────────────────────────────────────────────────────────────────────┘

  systemd: slipgate-dnsrouter, slipgate-socks5, slipgate-{tag}
  Also: direct SSH (:22), direct SOCKS5 (:1080), StunTLS on its own port (:8443)
```

Soar with the direct backend and VLESS dial each stream's destination themselves, so their traffic skips the backend layer and WARP.

Everything HTTPS arrives on one domain and port 443. Caddy (NaiveProxy) holds the Let's Encrypt certificate and looks at each request: a VLESS or StunTLS secret WebSocket path goes to that server (VLESS on localhost, StunTLS over TLS), an authenticated proxy request goes to NaiveProxy's `forward_proxy`, and anything else gets the decoy website. Install sets this up whenever NaiveProxy is selected; `slipgate tunnel edit --tag <tunnel> --front <naive-tag|->` switches an existing tunnel. The Cloudflare path on the right is optional, for when the server's IP is blocked.

### Transport Types

| Transport | Protocol | Port | Description |
|-----------|----------|------|-------------|
| **DNSTT/NoizDNS** | DNS | 53/udp | Curve25519 encrypted DNS tunnel. A single server serves both DNSTT and NoizDNS clients. NoizDNS adds DPI evasion with base36/hex encoding and CDN prefix stripping |
| **Slipstream** | QUIC DNS | 53/udp | QUIC-based tunnel with certificate authentication |
| **VayDNS** | KCP DNS | 53/udp | KCP-based DNS tunnel with Curve25519 encryption. Supports configurable idle timeout, keepalive, queue size, and multiple DNS record types |
| **Soar** | DNS | 53/udp | [Soar](https://github.com/getlantern/soar) multipath DNS tunnel (X25519 + Ed25519-signed handshake), served by slipgate itself. Soar has no per-user login, so it has two backends. **Direct**: like soar-server, it dials the destination each stream names, limited to public addresses and the allowed ports (default `80,443`, or `any`); anyone who finds the zone can use it, and it works with stock soar clients (zone + base64 public key from `tunnel status`). **SSH**: it reaches only the SSH server, so access goes through SSH users, added and removed one by one. Both export `slipnet://` configs for a SlipNet version with Soar support |
| **NaiveProxy** | HTTPS | 443/tcp | Caddy with forwardproxy plugin. Auto-TLS via Let's Encrypt. Probe-resistant with decoy site |
| **StunTLS** | TLS/WSS | 8443/tcp, or 443 fronted | SSH over TLS + WebSocket proxy. Auto-detects WebSocket, HTTP CONNECT, raw TLS, and payload (DPI bypass) modes. Self-signed TLS cert, no domain required. Fronted by NaiveProxy (`--front`), clients reach it as SSH over WebSocket on the Naive domain, port 443, real certificate |
| **VLESS** | WS/WSS | 443 fronted; or 2053, 2083, 2087, 2096, 8443/tcp | VLESS over WebSocket, served by slipgate itself, with SlipNet's VLESS profiles and any VLESS client (v2rayNG, Hiddify, NekoBox…). **Fronted by NaiveProxy** (recommended): Caddy carries it on 443 with its Let's Encrypt certificate, so links need no `allowInsecure` (which Xray clients drop from 2026-08). **Behind Cloudflare**: clients reach a Cloudflare IP and the server's IP stays hidden; set SSL/TLS mode to **Full**. **Direct**: its own port with a self-signed certificate. Each user's UUID is derived from their credentials and a server-wide secret: removing a user or changing the password revokes it within seconds, and a shared config never carries the password. Streams reach public addresses only, port 25 blocked by default; TCP and plain UDP (no mux/XUDP). Other paths answer like a stock nginx. Outbound traffic doesn't go through WARP |
| **External** | DNS | 53/udp | Routes DNS queries for a domain to a user-specified UDP port. No managed service — use for custom/private protocol testing |

### Domain Layout

Each DNS tunnel instance requires its own subdomain. When using both SOCKS and SSH backends, the install auto-generates subdomains by appending `s` to the SSH variant. NaiveProxy is an HTTPS service on an ordinary hostname, and VLESS and StunTLS ride on it (port 443, same certificate):

| Tunnel | Domain | Backend |
|--------|--------|---------|
| dnstt-socks | `t.example.com` | SOCKS5 |
| dnstt-ssh | `ts.example.com` | SSH |
| slipstream-socks | `s.example.com` | SOCKS5 |
| slipstream-ssh | `ss.example.com` | SSH |
| vaydns-socks | `v.example.com` | SOCKS5 |
| vaydns-ssh | `vs.example.com` | SSH |
| soar-direct | `r.example.com` | Direct (dials client destinations) |
| soar-ssh | `rs.example.com` | SSH |
| naive-socks | `example.com` | SOCKS5 (shared domain) |
| naive-ssh | `example.com` | SSH (shared domain) |
| vless | `example.com` (through NaiveProxy) | Direct (per-user UUIDs) |
| stuntls | `example.com` (through NaiveProxy) | SSH |

NaiveProxy tunnels share a domain since they use HTTPS (port 443), not DNS. DNSTT and NoizDNS also share a domain — the same server handles both client types. VLESS and StunTLS use the NaiveProxy domain as well, so they need no record of their own.

**Required DNS records** (for the example above):

```
A   ns.example.com       → <server IP>      DNS only (grey cloud)
NS  t.example.com        → ns.example.com
NS  ts.example.com       → ns.example.com
NS  s.example.com        → ns.example.com
NS  ss.example.com       → ns.example.com
NS  v.example.com        → ns.example.com
NS  vs.example.com       → ns.example.com
NS  r.example.com        → ns.example.com
NS  rs.example.com       → ns.example.com
A   example.com          → <server IP>      DNS only (grey cloud)
```

Every record stays **DNS only** (grey cloud): Caddy has to be reached directly to get its Let's Encrypt certificate.

**Optional — VLESS behind Cloudflare.** Only if the server's IP gets blocked: a VLESS tunnel created with `--cdn yes` on its own port (e.g. 2053) needs its own hostname, the one record proxied by Cloudflare, with **SSL/TLS → Overview** set to **Full** and **Network → WebSockets** on:

```
A   cdn.example.com      → <server IP>      Proxied (orange cloud)
```

Without NaiveProxy, StunTLS is reached at the server's IP on its own port (8443) and VLESS at its own domain and port.

### Routing Modes

- **Single mode**: One active tunnel runs; DNS router on port 53 forwards to it
- **Multi mode**: All tunnels run on local ports; DNS router on port 53 dispatches queries by domain. Auto-enabled when multiple DNS tunnels are created.

The DNS router answers on both UDP and TCP port 53. Queries arriving over TCP (RFC 7766, pipelined) are forwarded to the tunnel over UDP and the answers returned on the same TCP connection, so every DNS tunnel works for resolvers and clients that use TCP/53 (e.g. Soar clients on networks that block UDP/53), with no change to the tunnel servers.

## Client Configuration

After creating a tunnel, generate a shareable config:

```bash
sudo slipgate tunnel share mytunnel
```

This outputs a `slipnet://` URI that can be scanned or imported into the SlipNet Android app. For DNSTT tunnels, you'll be asked to choose between a DNSTT or NoizDNS client profile — both connect to the same server, but NoizDNS profiles enable DPI evasion on the client side.

`slipgate install` offers VLESS too and then asks only for its domain: it picks a free port, derives the WebSocket path from a server-wide secret (`/etc/slipgate/vless.secret`, kept when a tunnel is removed, so removing and re-adding a tunnel gives the same links) and prints every user's links at the end. When NaiveProxy is selected as well, VLESS and StunTLS are fronted by it from the start.

For VLESS tunnels it asks which user's config to show (or pass `--user <name>`) and prints both a `slipnet://` URI and a standard `vless://` link for other clients. Behind Cloudflare you can replace the CDN address with a clean Cloudflare IP in SlipNet at any time. A step-by-step guide in Persian is in [docs/VLESS_FA.md](docs/VLESS_FA.md).

To move an existing VLESS or StunTLS tunnel onto NaiveProxy's 443 (or back with `-`), then hand out the new configs:

```bash
sudo slipgate tunnel edit --tag vless --front naive
sudo slipgate tunnel edit --tag stuntls --front naive
sudo slipgate users configs
```

To print every config at once, for all tunnels and users (also in the menu under **Users → Show all configs**):

```bash
sudo slipgate users configs
```

### Server address (Oracle, AWS, GCP, Azure)

StunTLS, direct SSH and direct SOCKS5 configs carry the server's IP, since clients connect to it directly. Cloud servers only see a private IP (e.g. `10.0.0.5`) on their network interface, so slipgate looks for the public one: the server's own IP when it is public, otherwise the IP that `ns.<your domain>` or a NaiveProxy domain points at. If it finds none, it asks once and saves the answer. Set or change it at any time (also in the menu under **Tunnels → Server address**):

```bash
sudo slipgate config address 203.0.113.7   # an IP or hostname, without a port
sudo slipgate config address auto          # detect it again
```

Configs for DNS tunnels, Soar, NaiveProxy, VLESS and fronted StunTLS connect by domain and don't depend on it. VLESS configs never carry the server's IP.

### User Model

Users are **global**, not scoped to specific tunnels or transports. `slipgate users add` only asks for a username and password — the protocol is a property of the tunnel, chosen at `tunnel add` time. Every user can authenticate against every tunnel using the same credentials, and `slipgate users list` prints one config block per (user × tunnel) pair. `slipgate users configs` prints them all at once, together with the configs that need no login (Soar with the direct backend), which belong to no user. The client picks which tunnel to use by importing the matching `slipnet://` URI. On VLESS tunnels each user's UUID is derived from their username and password (with a per-tunnel secret), so removing a user or changing their password also revokes their VLESS access, within seconds and without a restart.

## File Locations

| Path | Description |
|------|-------------|
| `/etc/slipgate/config.json` | Main configuration |
| `/etc/slipgate/tunnels/` | Per-tunnel keys, certs, and configs (VLESS: `vless.secret`, which turns users' credentials into their UUIDs) |
| `/usr/local/bin/slipgate` | SlipGate binary (includes the built-in SOCKS5, Soar, StunTLS and VLESS servers) |
| `/usr/local/bin/dnstt-server` | DNSTT transport binary |
| `/usr/local/bin/slipstream-server` | Slipstream transport binary |
| `/usr/local/bin/vaydns-server` | VayDNS transport binary |
| `/usr/local/bin/caddy-naive` | Caddy with NaiveProxy plugin |

## Building

```bash
make build              # Build for current platform
make build-linux        # Cross-compile for linux/amd64 and linux/arm64
make test               # Run tests
make release            # Build release binaries
```

**Releases**: set `VERSION` in the Makefile and merge to `main`; the Release workflow then publishes `v<VERSION>` with `slipgate-linux-amd64`, `slipgate-linux-arm64` and `SHA256SUMS` (it skips versions already released). Pushing a `v*` tag works too.

## Credits

SlipGate builds on these projects:

- [dnstt](https://www.bamsoftware.com/software/dnstt/) by David Fifield - the DNS tunnel behind the DNSTT and NoizDNS transports, via the [tladesignz/dnstt](https://github.com/tladesignz/dnstt) fork (public domain / CC0)
- [Slipstream](https://github.com/EndPositive/slipstream) by [EndPositive](https://github.com/EndPositive) and its Rust rewrite [slipstream-rust](https://github.com/Mygod/slipstream-rust) by [Mygod](https://github.com/Mygod) - the QUIC-over-DNS tunnel behind the Slipstream transport (Apache-2.0)
- [VayDNS](https://github.com/net2share/vaydns) by [net2share](https://github.com/net2share) - the KCP-based DNS tunnel behind the VayDNS transport (CC0)
- [Soar](https://github.com/getlantern/soar) by [Lantern](https://github.com/getlantern) - the multi-resolver DNS tunnel behind the Soar transport, built into slipgate (Apache-2.0)
- [dnstm](https://github.com/net2share/dnstm) by [net2share](https://github.com/net2share) - the DNS tunnel manager SlipGate was originally built on
- [VLESS](https://github.com/XTLS/Xray-core) by [Project X](https://github.com/XTLS) - the protocol the VLESS transport speaks, implemented independently in slipgate
- WARP integration inspired by [dnstun-ezpz](https://github.com/aleskxyz/dnstun-ezpz) by [aleskxyz](https://github.com/aleskxyz)

## License

AGPL-3.0
