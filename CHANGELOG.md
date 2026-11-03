# Changelog

Changes in this fork. Each release's notes come from its section here.

## v1.6.8-vless.9

### Fixes

- **Running `slipgate install` again** on a configured server keeps the tunnels that are there instead of adding clashing copies (e.g. a second StunTLS on the same port), and the summary prints the configs of **every** tunnel, not only the new ones.
- No more "slipgate-socks5.service is not active, cannot reload" during install: a stopped SOCKS5 service is restarted instead of reloaded.

## v1.6.8-vless.8

### Highlights

- **VLESS and StunTLS on port 443 through NaiveProxy.** `slipgate tunnel edit --tag <tunnel> --front naive` serves them behind NaiveProxy's Caddy with its real Let's Encrypt certificate and decoy site: no Cloudflare hop, no self-signed certificate, and links without `allowInsecure` (which Xray clients drop from 2026-08). Install does this automatically when NaiveProxy is selected. Caddyfile changes are checked with `caddy-naive validate` before they are applied.
- **NaiveProxy accepts every SlipGate user**, not only the first one; user changes update Caddy live.
- **StunTLS**: WebSocket pings are answered to the client instead of corrupting the SSH stream.
- **Direct SOCKS5** listens publicly again after user changes.
- **Soar**: direct (no-login) configs appear in `slipgate users configs`.
- **VLESS links** keep SlipNet's SNI fragmentation off on import, like the `slipnet://` configs.
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
