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

<p align="center">📘 <a href="docs/guides/Rovunet-Panel-Guide-EN.pdf">Complete guide (PDF)</a> · <a href="docs/en/README.md">Documentation</a> · <a href="https://github.com/msVPNco/rovunet-app">Android app</a> · <a href="CHANGELOG.md">Changelog</a></p>

---

**v1.21.2 — install, update, refresh the same subscription.** A fully eligible unlimited account gets24 subscription profiles on Android and current Windows. The update completes missing cores and migrates old Hy2 to Salamander while keeping account auth and subscription URLs. Then press **Update** on the saved subscription in the app; no replacement link is needed. Older Windows Beta keeps its supported entries; AWG needs the current client. [Release/compatibility notes](docs/releases/v1.21.2.md) · [Full qualification still open](docs/release-gates.json).


Rovunet Panel is an open-source tool that turns a Linux server into a multi-protocol access server. It installs and manages each protocol as a systemd service, creates users and exports their client configurations, so every user has several independent connection paths instead of depending on one protocol or one network condition.

## Highlights

- **Managed proxies, SSH, DNS tunnels and protocol cores**, including AmneziaWG, Shadowsocks 2022, Trojan TLS and VLESS Encryption, from one command-line tool with an interactive menu.
- **Account-managed credentials** on authenticated paths, with targeted updates for the four added protocols. Direct Soar uses a shared key.
- **Ready-to-import configurations**: native `rovunet://` links for the [Rovunet app](https://github.com/msVPNco/rovunet-app), standard `vless://`, `hysteria2://` and WireGuard files for other clients.
- **Port 443 fronting**: VLESS and SSH over TLS share NaïveProxy's domain, port and Let's Encrypt certificate behind a decoy website.
- **Safe operations**: checksum-verified updates with rollback, validated configurations before deployment, full-server backup and restore.
- **Diagnostics**: live dashboard, health checks, certificate expiry and per-service logs.
- **Limits and subscription links**: unlimited or limited users (days and/or GB, enforced every 5 minutes), and one `https://…/sub/<token>.json` link per user that carries all their configurations for the app's Subscriptions menu.
- **Several addresses for one server**: add the server's other IPs and the links carry each address-based configuration once per address, so one blocked IP does not cut users off.
- **Automatic REALITY target**: the camouflage site is chosen from a bank of high-traffic sites that are rarely blocked. A new install adds a second REALITY core on another port with a site of another company, so a user has two ways in.
- **Stable subscription URLs and readable names**: normal updates preserve existing tokens and profile IDs. Android uses `<token>.json`; Windows uses `<token>.windows.json` with its own supported protocol set. Refresh the saved subscription to receive the approved display names.

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
| **AmneziaWG** | Userspace UDP tunnel | any UDP | An optional alternative to WireGuard |
| **Shadowsocks 2022** (Xray) | TCP and UDP | any TCP/UDP | Optional compatibility; deprecated by the pinned Xray |
| **Trojan TLS** (Xray) | TCP + trusted TLS certificate | any TCP | Optional TLS compatibility; the pinned Xray deprecates Trojan without Flow |
| **VLESS Encryption** (Xray) | TCP + ML-KEM/X25519 encryption | any TCP | Clients supporting the pinned encryption format |

Suitability depends on the server, operator and network; these are guidelines, not guarantees. See [Protocols](docs/en/protocols.md) for details.

**v1.21.2:** the tested full setup exports **24 Android configurations and 24 Windows configurations**, not 24 independent protocols. Counts depend on installed services and account eligibility. Current Windows receives both; public Beta0.1.1 skips unsupported AWG while retaining supported entries. All four added protocols are excluded from traffic-limited accounts; the three new Xray protocols deny new sessions after expiry/removal, but established streams may continue. An exact simultaneous-device cap is not implemented. See [managed protocols and compatibility](docs/MANAGED-PROTOCOLS.md).

**Automatic complete setup (v1.21.2):** normal installation and Quick Wizard select the full tunnel set and both backends. Update adds missing protocol cores automatically, preserving existing users, keys, core names and subscription URLs. A configured 20-profile server grows to 24 Android / 24 current Windows profiles without four manual installations. Incomplete base setups ask for missing DNS/domain settings during an interactive update. A valid NaiveProxy certificate, usable TUN device and free candidate ports are required; failures report the actual counts and a pending setup instead of claiming completion. `sudo rovunet complete-setup` resumes setup; `sudo rovunet install --custom` keeps selective installation available. Traffic-limited accounts retain their restricted protocol set.

## Quick start

On a fresh Ubuntu 24.04 LTS (tested) or systemd-based Debian (runtime compatibility required) server, as root:

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v1.21.2/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="a8aaa38d979049fa5e42c9f8f444e519d28625d1d67872ddaeff2d0dfc6b3498", "Installer checksum mismatch"'
sudo bash rovunet-install.sh
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
