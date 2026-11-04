<p align="center">
  <img src="assets/rovunet-logo.png" alt="Rovunet" width="200">
</p>

# Rovunet Panel

Self-hosted, multi-protocol censorship-circumvention server panel: DNS tunnels
(DNSTT, NoizDNS, VayDNS, Slipstream, Soar), NaiveProxy, VLESS over WebSocket,
SSH over TLS (StunTLS), SOCKS5 and Xray (REALITY, XHTTP, Hysteria2, WireGuard).
One install, ready-to-use client configs for the
[Rovunet Android app](https://github.com/msVPNco/rovunet-android) and for
standard clients.

> **Status: alpha.** Not yet published as a release. Test on a disposable
> server first.

Telegram group: [t.me/RovunetVPN](https://t.me/RovunetVPN)

## Highlights

- Native client exports use `rovunet://` v32 links that carry a server/user
  collection ID, so the Rovunet app groups each user's paths together.
  Standard `vless://` links and the legacy `slipnet://` format remain available.
- VLESS and StunTLS can share NaiveProxy's Caddy on port 443 behind a secret
  path, using its real Let's Encrypt certificate.
- `core-plan` generates reviewable Xray server configs and per-device client
  profiles; it never installs services or opens ports by itself.

The command is `slipgate`, kept for compatibility with existing servers:

```
slipgate export-clients --user USER --format rovunet
```

## Documentation

- [Guide](docs/GUIDE.md): install, protocols, domain layout and CLI
- [User guide](docs/USER_GUIDE.md) (English and Persian)
- [Persian VLESS guide](docs/VLESS_FA.md)
- [Export notes](ROVUNET-EXPORT.txt) and [Xray plan notes](VEYRA-MANAGER.md)
- [Changelog](CHANGELOG.md)

## Support

Rovunet is unpaid volunteer work. If it helps you, donations are welcome:

- **USDT (Tron, TRC20):** `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ`

Send only USDT on the Tron (TRC20) network to this address.

## License

AGPL-3.0. Rovunet Panel is based on SlipGate by Anonvector and is not
affiliated with it; see [NOTICE](NOTICE.md).
