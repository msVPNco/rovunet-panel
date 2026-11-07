<p align="center">
  <img src="assets/rovunet-logo.png" alt="Rovunet" width="180">
</p>

<h1 align="center">Rovunet Panel</h1>

<p align="center">
  Self-hosted, multi-protocol censorship-circumvention server.<br>
  One install for supported managed protocols, with client configs for each user.
</p>

<p align="center">
  <b>English</b> · <a href="README.fa.md">فارسی</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ru.md">Русский</a> ·
  <a href="https://t.me/RovunetVPN">Telegram</a> ·
  <a href="https://github.com/msVPNco/rovunet-android">Android app</a>
</p>

---

**Rovunet Panel is an open-source, self-hosted tool for deploying and managing multi-protocol internet access on compatible Linux servers.** It is designed to provide several connection paths for each user, so access does not depend on a single protocol or a fixed set of network conditions.

The panel brings installation and configuration of managed protocols, user creation, client configuration export and systemd service management into a command-line workflow with an interactive menu. Available paths include VLESS over WebSocket, NaiveProxy, direct SSH, SSH over TLS, SOCKS5, DNS tunnel families such as DNSTT, NoizDNS, VayDNS, Slipstream and Soar, and Xray cores (VLESS REALITY, VLESS XHTTP, Hysteria2 and WireGuard). Diagnostics and backup tools support ongoing server maintenance and troubleshooting.

Designed to work with the [Rovunet Android app](https://github.com/msVPNco/rovunet-android), the panel's native export can carry collection metadata alongside related connection paths. The app tests connectivity through the actual tunnel and asks before proceeding to DNS fallback. Standard VLESS links are also available for other compatible clients.

**Scope of support:** Xray cores are installed and managed with `rovunet core-install` (`core-plan` still only writes files for manual deployment). Connection quality and availability depend on the network, server configuration and selected protocol. See [integration status and development priorities](docs/ROVUNET-STATUS.md) for the distinction between available functionality and remaining work.

> **Project status: early release under active development.** Test the required protocols in your own environment before operational use, and back up configuration with `rovunet backup`.

## Quick install

On a fresh Ubuntu 20.04+ or Debian 10+ server, as root:

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

The installer asks which protocols to enable, sets them up, creates the first
user and prints all of that user's configs. Then:

```bash
sudo rovunet                # interactive menu
sudo rovunet users add      # add a user
sudo rovunet users configs  # print every user's configs again
sudo rovunet diag           # check services, ports, keys and DNS
```

Coming from SlipGate? See [switching a server](docs/GUIDE.md#quick-start);
your config, tunnels and users are kept.

## Protocols

| Protocol | How it travels | Port | When to use it |
|---|---|---|---|
| **VLESS fronted by NaiveProxy** | WebSocket behind Caddy and a real certificate | 443/tcp | First choice: fast, looks like a normal HTTPS site |
| **NaiveProxy** | HTTP/2 proxy with a decoy website | 443/tcp | When TLS to your domain works |
| **SSH over TLS (StunTLS)** | SSH inside TLS/WebSocket, fronted on 443 | 443/tcp | Alternative to VLESS on the same port |
| **Direct SSH** | Plain SSH | 22/tcp | Fast where port 22 is open |
| **VLESS behind Cloudflare** | WebSocket through Cloudflare's CDN | 2053/tcp | When the server's IP is blocked; slower |
| **DNSTT / NoizDNS** | Encrypted DNS tunnel | 53/udp | For restricted networks; performance depends on the resolver and path |
| **VayDNS** | DNS tunnel with tunable wire format | 53/udp | Like DNSTT, with more tuning options |
| **Slipstream** | QUIC over DNS | 53/udp | Experimental |
| **Soar** | Multipath DNS with resolver discovery | 53/udp | When DNS resolvers are filtered unevenly |
| **SOCKS5** | Plain SOCKS5 | 1080/tcp | Open networks only (not encrypted) |
| **VLESS REALITY** (Xray core) | TLS that looks like a real site, no certificate needed | any free TCP port | When TLS to well-known sites passes |
| **VLESS XHTTP** (Xray core) | VLESS over HTTP requests, real certificate | any free TCP port | Behind a CDN or where WebSocket is blocked |
| **Hysteria2** (Xray core) | QUIC with its own congestion control, real certificate | any free UDP port | Lossy or unstable networks where UDP passes |
| **WireGuard** (Xray core) | Standard WireGuard, one file per device | any free UDP port | Where WireGuard is not blocked |

Protocol suitability depends on the server, operator and current network. These are usage guidelines, not guaranteed availability or comparative benchmark results. The Rovunet app tests connection paths.

Xray cores run as their own services beside the tunnels:

```bash
sudo rovunet core-install --name r1 --protocol reality --host SERVER_IP \
  --target www.microsoft.com:443 --sni www.microsoft.com --port 8443 --devices 5
sudo rovunet core-list
sudo rovunet core-remove r1
```

The port is checked first, Xray is verified against the release checksums and
tests the config before it goes live, and the TCP or UDP port is opened. Each
device gets its own file under `/etc/rovunet/core/<name>/clients/`. XHTTP and
Hysteria2 need `--cert` and `--key` for a certificate valid for `--sni`.

## Client apps

[Rovunet Android releases](https://github.com/msVPNco/rovunet-android/releases) contain published installers and source assets. Read the app's [current status](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md) and signing/migration notes before installation; private candidates are not public in-place updates. See [integration status](docs/ROVUNET-STATUS.md).

- **[Rovunet for Android](https://github.com/msVPNco/rovunet-android)**
  (recommended): imports the `rovunet://` links, groups each user's paths into
  one collection, connects a selected path without a prerequisite ping, and offers separate real-tunnel HTTPS tests.
  [Download the latest release](https://github.com/msVPNco/rovunet-android/releases).
- **Standard clients** such as v2rayNG, Hiddify or NekoBox: use the `vless://`
  links the panel prints next to each VLESS config.

```bash
sudo rovunet export-clients --user USER --format rovunet
```

## Requirements

- Linux server (Ubuntu 20.04+, Debian 10+ or similar) with root access
- A domain whose DNS you control, pointed at the server (see
  [domain layout](docs/GUIDE.md#domain-layout))
- Open ports for the protocols you enable: 53/udp, 80/tcp and 443/tcp at least

## Documentation

- [Guide](docs/GUIDE.md): install, protocols, domain layout and the full CLI
- [User guide](docs/USER_GUIDE.md) (English and Persian)
- [Persian VLESS guide](docs/VLESS_FA.md)
- [Export notes](ROVUNET-EXPORT.txt) and [Xray plan notes](VEYRA-MANAGER.md)
- [Changelog](CHANGELOG.md)

## Support

Rovunet is unpaid volunteer work. If it helps you, donations are welcome:

- **USDT (Tron, TRC20):** `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ`

Send only USDT on the Tron (TRC20) network to this address.

Questions and news: [t.me/RovunetVPN](https://t.me/RovunetVPN)

## License

AGPL-3.0. Rovunet Panel is based on
[SlipGate](https://github.com/anonvector/slipgate) by Anonvector and is not
affiliated with it; see [NOTICE](NOTICE.md). The command is `rovunet`; a
server installed under the old name is migrated automatically on update.
