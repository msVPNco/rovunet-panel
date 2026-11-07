# Changelog

Changes in Rovunet Panel. Each release's notes come from its section here.

## v1.8.0

The panel is now **`rovunet`** everywhere. Updating from v1.7.0 migrates the server automatically.

### Migration

- Command, binary, systemd services, system users and groups, `/etc/rovunet`, sshd_config blocks, firewall and resolver files all use the Rovunet name.
- The first run of the new version moves an existing server over in place: services are stopped, users and groups are renamed with the same UIDs/GIDs, the config directory is moved, stored paths are rewritten without touching any password, services come back under their new names, sshd_config is changed only after `sshd -t` accepts it, and the command becomes `/usr/local/bin/rovunet`.
- Old backups still restore. Issued client configs keep working: the VLESS key derivation and the `slipnet://` format are unchanged.

### New

- **Xray cores:** `rovunet core-install`, `core-remove` and `core-list` install REALITY, XHTTP, Hysteria2 or WireGuard as managed services: the port is checked first, the pinned Xray build is verified against this release's SHA256SUMS, Xray tests the config before it is deployed, and the TCP or UDP port is opened. Cores appear in `tunnel status` and are included in backups.
- `tunnel status` shows each certificate's names, issuer type and expiry, and flags expired, soon-to-expire and unreadable ones.
- `export-clients --annotated` labels each link with its tunnel.

### Security

- An external SOCKS5 proxy with no users now refuses connections and drops live sessions, instead of becoming an open proxy.
- An oversized EDNS0 size in a DNS query can no longer crash the DNS router.
- `config.json` is written owner-only (0600), including files saved by older versions.
- `rovunet update` verifies the download against SHA256SUMS, installs atomically and rolls back if the install fails.

### Fixes

- Firewall rules survive reboot without replaying a full iptables dump: only the panel's own ports are inserted, so Docker, fail2ban and other tools' rules are left alone.
- Editing a DNSTT, Slipstream, VayDNS or Soar tunnel restarts its service instead of leaving the old settings running.
- Overlapping tunnel domains always route to the most specific one.
- Exported configs keep passwords that contain `|` intact (v33).
- A new StunTLS tunnel's port is opened in the firewall, and a TCP port already in use is refused before the tunnel is saved.
- Renaming a Slipstream tunnel keeps its certificate paths valid.
- Removing the default tunnel clears the default route.
- Freeing a UDP port no longer looks at TCP listeners.
- Uninstall removes SSH tunnel users and their group.

## v1.7.0

First release as **Rovunet Panel**.

### Security

- **NaiveProxy no longer accepts the built-in `slipgate:slipgate` login.** A Naive tunnel created without its own login used to fall back to that well-known login in Caddy, so anyone could use the proxy. Now only the tunnel's own login (if set) and panel users can log in; with neither, Caddy gets a random login and stays closed. A Naive config without any login is refused with a hint to run `slipgate users add`.

### Changes

- Caddy serves HTTP/1.1 and HTTP/2 only and no longer opens UDP 443 for HTTP/3, so Hysteria2 or another UDP service can use that port. Naive clients use HTTP/2 and are unaffected.
- Rebranded as **Rovunet Panel** (based on SlipGate, not affiliated): logo, README, guides under `docs/`, NOTICE.md. The `slipgate` command, service names and `/etc/slipgate` are unchanged.
- `rovunet://` v32 exports, `export-clients` and `core-plan` (from the Rovunet 0.3.0 alpha work).
- A release is published only when `VERSION` in the Makefile changes on main (or by hand, or from a `v*` tag); other pushes to main never release.
- New README in English and Persian, Rovunet banner in the installer and the menu.
- Install and `slipgate update` now use this repository's releases (`msVPNco/rovunet-panel`).

## v1.6.8-vless.9

### Fixes

- **Running `slipgate install` again** on a configured server keeps the tunnels that are there instead of adding clashing copies (e.g. a second StunTLS on the same port), and the summary prints the configs of **every** tunnel, not only the new ones.
- No more "slipgate-socks5.service is not active, cannot reload" during install: a stopped SOCKS5 service is restarted instead of reloaded.

## v1.6.8-vless.8

### Highlights

- **VLESS and StunTLS on port 443 through NaiveProxy.** `slipgate tunnel edit --tag <tunnel> --front naive` serves them behind NaiveProxy's Caddy with its real Let's Encrypt certificate and decoy site: no Cloudflare hop, no self-signed certificate, and links without `allowInsecure` (which Xray clients drop from 2026-08). Install does this automatically when NaiveProxy is selected. Caddyfile changes are checked with `caddy-naive validate` before they are applied.
- **NaiveProxy accepts every Rovunet Panel user**, not only the first one; user changes update Caddy live.
- **StunTLS**: WebSocket pings are answered to the client instead of corrupting the SSH stream.
- **Direct SOCKS5** listens publicly again after user changes.
- **Soar**: direct (no-login) configs appear in `slipgate users configs`.
- **VLESS links** keep Rovunet's SNI fragmentation off on import, like the `slipnet://` configs.
- Docs: rewritten README fork section, a "which protocol where" guide from field tests, and the full Persian guide in `docs/VLESS_FA.md`.

### Upgrade

```bash
sudo slipgate update
sudo slipgate tunnel edit --tag vless --front naive     # optional, recommended
sudo slipgate tunnel edit --tag stuntls --front naive   # if StunTLS's own port is blocked
sudo slipgate users configs
```

Includes v1.6.8-vless.5 to vless.7.

## v1.6.8-vless.4 and earlier

- Built-in VLESS over WebSocket (behind Cloudflare or direct), per-user UUIDs, stable links from a server-wide secret, VLESS in the install wizard.
- Fork release and self-update infrastructure; firewall rules inserted ahead of REJECT; `/run/sshd` created before `sshd -t`.
