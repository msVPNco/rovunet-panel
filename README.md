<p align="center"><img src="assets/rovunet-logo.png" alt="Rovunet" width="140"></p>

<h1 align="center">Rovunet Panel</h1>

<p align="center"><b>Self-hosted, multi-protocol censorship-circumvention server</b><br>One install, every managed protocol, ready-made configurations and a subscription link for each user.</p>

<p align="center">
  <a href="https://github.com/msVPNco/rovunet-panel/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/msVPNco/rovunet-panel?sort=date&display_name=tag&label=release&color=0E7C86"></a>
  <img alt="Platform" src="https://img.shields.io/badge/platform-Linux-14304D">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-6D4AFF"></a>
  <a href="https://t.me/RovunetVPN"><img alt="Telegram" src="https://img.shields.io/badge/Telegram-RovunetVPN-2F6FEB?logo=telegram&logoColor=white"></a>
</p>

<p align="center"><b>English</b> · <a href="README.fa.md">فارسی</a> · <a href="README.ru.md">Русский</a> · <a href="README.zh-CN.md">简体中文</a></p>

<p align="center">📘 <a href="docs/guides/Rovunet-Panel-Guide-EN.pdf">Complete guide (PDF)</a> · <a href="docs/en/README.md">Documentation</a> · <a href="https://github.com/msVPNco/rovunet-android">Android app</a> · <a href="CHANGELOG.md">Changelog</a></p>

---

Rovunet Panel is an open-source tool that turns a Linux server into a multi-protocol access server. It installs and manages each protocol as a systemd service, creates users and exports their client configurations, so every user has several independent connection paths instead of depending on one protocol or one network condition.

## Highlights

- **14 protocols** across proxies, SSH, DNS tunnels and Xray cores, managed from one command-line tool with an interactive menu.
- **One login per user** for every tunnel, and per-user VLESS identities that are revoked within seconds.
- **Ready-to-import configurations**: native `rovunet://` links for the [Rovunet app](https://github.com/msVPNco/rovunet-android), standard `vless://`, `hysteria2://` and WireGuard files for other clients.
- **Port 443 fronting**: VLESS and SSH over TLS share NaïveProxy's domain, port and Let's Encrypt certificate behind a decoy website.
- **Safe operations**: checksum-verified updates with rollback, validated configurations before deployment, full-server backup and restore.
- **Diagnostics**: live dashboard, health checks, certificate expiry and per-service logs.
- **Limits and subscription links**: unlimited or limited users (days and/or GB, enforced every 5 minutes), and one `https://…/sub/<token>.json` link per user that carries all their configurations for the app's Subscriptions menu.
- **Several addresses for one server**: add the server's other IPs and the links carry each address-based configuration once per address, so one blocked IP does not cut users off.
- **Automatic REALITY target**: the camouflage site is chosen from a bank of high-traffic sites that are rarely blocked.

## Protocols

| Protocol | Carried over | Port | Best for |
|---|---|---|---|
| **VLESS over WebSocket** (fronted by NaïveProxy) | WebSocket + TLS, real certificate | 443/tcp | First choice where HTTPS works |
| **VLESS over WebSocket** (behind Cloudflare) | WebSocket through Cloudflare's CDN | 2053/tcp | When the server's IP is blocked |
| **NaïveProxy** | HTTP/2 + TLS with a decoy website | 443/tcp | Where TLS to your domain works |
| **SSH over TLS** (StunTLS) | SSH inside TLS / WebSocket | 443/tcp | An alternative on the same port |
| **SSH** | Plain SSH | 22/tcp | Where port 22 is open |
| **SOCKS5** | Plain SOCKS5, unencrypted | 1080/tcp | Open networks only |
| **DNSTT** / **NoizDNS** | Encrypted DNS tunnel | 53/udp | Restricted networks where DNS passes |
| **VayDNS** | DNS tunnel with a tunable wire format | 53/udp | Like DNSTT, with more tuning |
| **Slipstream** | QUIC over DNS | 53/udp | Experimental |
| **Soar** | Multipath DNS with resolver discovery | 53/udp | Unevenly filtered resolvers |
| **VLESS + REALITY** (Xray) | TCP, imitates a real TLS site | any TCP | Where TLS to well-known sites passes |
| **VLESS + XHTTP** (Xray) | HTTP requests over TLS | any TCP | Behind a CDN, or where WebSocket is blocked |
| **Hysteria 2** (Xray) | QUIC | any UDP | Lossy networks where UDP passes |
| **WireGuard** (Xray) | UDP | any UDP | Where WireGuard is not blocked |

Suitability depends on the server, operator and network; these are guidelines, not guarantees. See [Protocols](docs/en/protocols.md) for details.

## Quick start

On a fresh Ubuntu 20.04+ or Debian 10+ server, as root:

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

The installer asks which protocols to enable, sets them up, creates the first user and prints that user's configurations. Then:

```bash
sudo rovunet                 # interactive menu
sudo rovunet users add       # add a user
sudo rovunet users configs   # print every user's configurations
sudo rovunet diag            # check services, ports, keys and DNS
```

Requirements: root access, a domain whose DNS you control, and open ports for the protocols you enable (at least 53/udp, 80/tcp and 443/tcp). See [Getting started](docs/en/getting-started.md).

## Documentation

| | |
|---|---|
| 📘 [Complete guide (PDF)](docs/guides/Rovunet-Panel-Guide-EN.pdf) | Illustrated guide: architecture, protocol chooser, install flow, users and limits, operations. Also in [فارسی](docs/guides/Rovunet-Panel-Guide-FA.pdf) · [Русский](docs/guides/Rovunet-Panel-Guide-RU.pdf) · [简体中文](docs/guides/Rovunet-Panel-Guide-ZH.pdf) |
| [Getting started](docs/en/getting-started.md) | Requirements, DNS records, installation, first tunnel |
| [Protocols](docs/en/protocols.md) | Every protocol, ports, domain layout, routing |
| [Users and configurations](docs/en/users-and-configs.md) | Users, sharing, export formats, client apps |
| [Limits and subscriptions](docs/en/limits-and-subscriptions.md) | Traffic and time limits, subscription links, extra addresses |
| [Xray cores](docs/en/xray-cores.md) | VLESS + REALITY, VLESS + XHTTP, Hysteria 2, WireGuard |
| [Operations](docs/en/operations.md) | Updating, backup and restore, monitoring, files and services |
| [Command reference](docs/en/commands.md) | Every command and option |
| [Troubleshooting](docs/en/troubleshooting.md) | Symptoms, causes and fixes |

> **Status:** early release under active development. Test the protocols you need in your own environment before relying on them, and keep a backup (`sudo rovunet backup`). See [integration status](docs/ROVUNET-STATUS.md).

## Support

Rovunet is unpaid volunteer work. Donations are welcome: **USDT on Tron (TRC20)** to `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ` (send only USDT on the TRC20 network). Questions and news: [t.me/RovunetVPN](https://t.me/RovunetVPN).

## License

AGPL-3.0. Rovunet Panel is based on [SlipGate](https://github.com/anonvector/slipgate) by Anonvector and is not affiliated with it; see [NOTICE](NOTICE.md) for upstream and third-party attributions.
