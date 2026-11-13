# Getting started

[Documentation](README.md) › Getting started

## 1. Requirements

| Item | Requirement |
|---|---|
| Operating system | Linux with systemd; Ubuntu 24.04 LTS tested. Other distributions require runtime/ABI compatibility checks |
| Access | Root (or sudo) over SSH |
| Domain | A domain whose DNS records you control. Needed for DNS tunnels, NaïveProxy, VLESS and the TLS-based Xray cores |
| Hardware | 1 vCPU, 1 GB RAM and 20 GB disk are enough for a small group; bandwidth matters more than CPU |
| Ports | Depend on the protocols you enable; see the table below |

| Port | Used by |
|---|---|
| 53/udp and 53/tcp | DNS tunnels (DNSTT, NoizDNS, VayDNS, Slipstream, Soar) |
| 80/tcp and 443/tcp | NaïveProxy and everything fronted by it (VLESS, SSH over TLS) |
| 22/tcp | SSH |
| 1080/tcp | SOCKS5 (open networks only) |
| 2053/tcp (or 2083, 2087, 2096, 8443) | VLESS behind Cloudflare |
| Any free TCP or UDP port | Xray cores |

## 2. Choosing a server

- **Inbound UDP/53 must be allowed** for DNS tunnels. Some providers block or throttle it; check your provider's firewall and network policy before buying.
- Choose a region close to your users that their networks can still reach.
- Prefer providers without port restrictions and with generous bandwidth.

## 3. DNS records

Create the records for the protocols you plan to use **before** installing. Every record is **DNS only** (grey cloud in Cloudflare): Caddy must be reached directly to obtain its Let's Encrypt certificate, and DNS tunnels need raw DNS at the server.

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

Minimal examples:

- **NaïveProxy only:** `A example.com → SERVER_IP`.
- **DNSTT only:** `A ns.example.com → SERVER_IP` and `NS t.example.com → ns.example.com`.

An `NS` record delegates a subdomain to your server: queries for `t.example.com` go to `ns.example.com`, which is your server, and the tunnel answers them on port 53. Changes take from minutes to a few hours to propagate. Check them with:

```bash
dig +short A ns.example.com
dig +short NS t.example.com
```

VLESS behind Cloudflare is the one exception that uses a **proxied** record; see [Protocols › VLESS over WebSocket](protocols.md#vless-over-websocket).

## 4. Installation

### Online (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v1.21.2/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="a8aaa38d979049fa5e42c9f8f444e519d28625d1d67872ddaeff2d0dfc6b3498", "Installer checksum mismatch"'
sudo bash rovunet-install.sh
```

The installer asks which protocols to enable, downloads the binaries they need, generates keys, creates the systemd services and firewall rules, creates the first user and prints all of that user's configurations.

Running the installer again on a configured server keeps the existing tunnels, adds only new protocols and prints every configuration again. To add a second tunnel of a type you already have, use `sudo rovunet tunnel add`.

When NaïveProxy and VLESS are selected together, VLESS (and SSH over TLS, if selected) are fronted by NaïveProxy on port 443 from the start.

### Offline (server cannot reach GitHub)

Download the panel from this project's [latest release](https://github.com/msVPNco/rovunet-panel/releases) and the transport binaries from [upstream](https://github.com/anonvector/slipgate/releases), then copy them to the server:

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

Use `arm64` instead of `amd64` on ARM servers.

### From source

```bash
git clone https://github.com/msVPNco/rovunet-panel.git
cd rovunet-panel
make build
sudo ./rovunet install
```

### Moving from SlipGate or an older Rovunet Panel

A server installed under the panel's earlier name (`slipgate`, `/etc/slipgate`) is migrated in place; see [Operations › Updating](operations.md#updating).

## 5. Interactive menu and command line

- `sudo rovunet` with no arguments opens the interactive menu, best for first setup.
- Every menu action is also a command with flags, for scripting; see the [command reference](commands.md). A required flag that is missing is asked for interactively.

## 6. First tunnel and first user

```bash
sudo rovunet tunnel add --transport dnstt --backend socks --tag mytunnel --domain t.example.com
sudo rovunet users add
sudo rovunet users configs
sudo rovunet tunnel status
```

`tunnel add` generates the tunnel's keys, creates the service `rovunet-mytunnel` (started now and at boot) and registers the domain with the DNS router. `users configs` prints every configuration for every user. If a tunnel is not `active`, run `sudo rovunet diag`.

Next: [Protocols](protocols.md)
