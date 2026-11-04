<p align="center">
  <img src="assets/rovunet-logo.png" alt="Rovunet" width="180">
</p>

<h1 align="center">Rovunet Panel</h1>

<p align="center">
  Self-hosted, multi-protocol censorship-circumvention server.<br>
  One install for supported managed protocols, with client configs for each user.
</p>

<p align="center">
  <b>English</b> · <a href="README.fa.md">فارسی</a> ·
  <a href="https://t.me/RovunetVPN">Telegram</a> ·
  <a href="https://github.com/msVPNco/rovunet-android">Android app</a>
</p>

---

**Rovunet Panel is an open-source, self-hosted tool for deploying and managing multi-protocol internet access on compatible Linux servers.** It is designed to provide several connection paths for each user, so access does not depend on a single protocol or a fixed set of network conditions.

The panel brings installation and configuration of managed protocols, user creation, client configuration export and systemd service management into a command-line workflow with an interactive menu. Available paths include VLESS over WebSocket, NaiveProxy, direct SSH, SSH over TLS, SOCKS5 and DNS tunnel families such as DNSTT, NoizDNS, VayDNS, Slipstream and Soar. Diagnostics and backup tools support ongoing server maintenance and troubleshooting.

Designed to work with the [Rovunet Android app](https://github.com/msVPNco/rovunet-android), the panel's native export can carry collection metadata alongside related connection paths. The app tests connectivity through the actual tunnel and asks before proceeding to DNS fallback. Standard VLESS links are also available for other compatible clients.

**Scope of support:** `core-plan` can generate configurations for additional paths for manual deployment; this does not mean that all such paths are installed and managed automatically. Connection quality and availability depend on the network, server configuration and selected protocol. See [integration status and development priorities](docs/ROVUNET-STATUS.md) for the distinction between available functionality and remaining work.

> **Project status: early release under active development.** Test the required protocols in your own environment before operational use, and back up configuration with `slipgate backup`.

## Quick install

On a fresh Ubuntu 20.04+ or Debian 10+ server, as root:

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

The installer asks which protocols to enable, sets them up, creates the first
user and prints all of that user's configs. Then:

```bash
sudo slipgate                 # interactive menu
sudo slipgate users add       # add a user
sudo slipgate users configs   # print every user's configs again
sudo slipgate diag            # check services, ports, keys and DNS
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
| **DNSTT / NoizDNS** | Encrypted DNS tunnel | 53/udp | When only DNS gets through; slow but resilient |
| **VayDNS** | DNS tunnel with tunable wire format | 53/udp | Like DNSTT, with more tuning options |
| **Slipstream** | QUIC over DNS | 53/udp | Experimental |
| **Soar** | Multipath DNS with resolver discovery | 53/udp | When DNS resolvers are filtered unevenly |
| **SOCKS5** | Plain SOCKS5 | 1080/tcp | Open networks only (not encrypted) |

Protocol suitability depends on the server, operator and current network. These are usage guidelines, not guaranteed availability or comparative benchmark results. The Rovunet app tests connection paths. Xray protocols (REALITY,
XHTTP, Hysteria2, WireGuard) can already be generated with `slipgate core-plan`
for manual deployment; managed support is in progress.

## Client apps

[Download Rovunet Android 0.3.0-alpha06 Full](https://github.com/msVPNco/rovunet-android/releases/tag/v0.3.0-alpha06) — debug-signed. APK, Android source and Persian guide are attached. See [integration status](docs/ROVUNET-STATUS.md).

- **[Rovunet for Android](https://github.com/msVPNco/rovunet-android)**
  (recommended): imports the `rovunet://` links, groups each user's paths into
  one collection and tests them for you.
  [Download the latest release](https://github.com/msVPNco/rovunet-android/releases).
- **Standard clients** such as v2rayNG, Hiddify or NekoBox: use the `vless://`
  links the panel prints next to each VLESS config.

```bash
sudo slipgate export-clients --user USER --format rovunet
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
affiliated with it; see [NOTICE](NOTICE.md). For compatibility the command is
still `slipgate`.
