<p align="center">
  <img src="assets/rovunet-logo.png" alt="Rovunet" width="180">
</p>

<h1 align="center">Rovunet Panel</h1>

<p align="center">
  Self-hosted, multi-protocol censorship-circumvention server.<br>
  One install, every protocol, ready-to-use configs for each user.
</p>

<p align="center">
  <b>English</b> · <a href="README.fa.md">فارسی</a> ·
  <a href="https://t.me/RovunetVPN">Telegram</a> ·
  <a href="https://github.com/msVPNco/rovunet-android">Android app</a>
</p>

---

Rovunet Panel sets up and manages the server side of the
[Rovunet](https://github.com/msVPNco/rovunet-android) Android app on any Linux
VPS: DNS tunnels for the hardest networks, HTTPS proxies that look like an
ordinary website, VLESS, SSH over TLS and SOCKS5. It creates users, keeps every
service running under systemd and prints each user's configs at the end of the
install.

> **Status: early release.** Run it on a server you can rebuild, and keep a
> backup (`slipgate backup`).

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

These notes come from field tests in Iran in late 2026; networks change, so the
Rovunet app tests each path and picks one that works. Xray protocols (REALITY,
XHTTP, Hysteria2, WireGuard) can already be generated with `slipgate core-plan`
for manual deployment; managed support is in progress.

## Client apps

- **[Rovunet for Android](https://github.com/msVPNco/rovunet-android)**
  (recommended): imports the `rovunet://` links, groups each user's paths into
  one collection and tests them for you.
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
