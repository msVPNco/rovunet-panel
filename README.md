# Rovunet Panel

Self-hosted, multi-protocol censorship-circumvention server panel: DNS tunnels
(DNSTT, NoizDNS, VayDNS, Slipstream, Soar), NaiveProxy, VLESS over WebSocket,
SSH over TLS (StunTLS), SOCKS5 and Xray (REALITY, XHTTP, Hysteria2, WireGuard).
One install, ready-to-use client configs for the Rovunet Android app and for
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

The command is still `slipgate` for compatibility with existing servers:

```
slipgate export-clients --user USER --format rovunet
```

See [export notes](ROVUNET-EXPORT.txt), [Xray plan notes](VEYRA-MANAGER.md)
and the [inherited documentation](README-UPSTREAM.md).

## Attribution and license

Rovunet Panel is based on [SlipGate](https://github.com/anonvector/slipgate) by
Anonvector and is **not affiliated** with that project. It is distributed under
the GNU Affero General Public License v3.0; original copyright and license
notices are retained in [LICENSE](LICENSE). The full history of upstream and
modified commits is kept in this repository.
