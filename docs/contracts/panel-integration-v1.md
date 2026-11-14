# Panel–client integration contract v1

This is the handoff contract for the Android, Windows, macOS and integration
agents. Client versions in acceptance reports are dated test specimens, not
requirements or a promise about later builds. Each final client must run the
same contract checks against its own final artifact before joint publication.
This document describes the strict subscription wire contract, which remains
version1 in Panel2. Historical capability snapshots retain their dated counts.

## Subscription and identity

- Keep strict JSON v1: root `version`, optional `name`, `items`; each item has
  only `id` and `link`. Numeric version is `1`; IDs are unique, nonblank, at
  most 200 characters; 1–1000 items; nesting at most 16. Do not insert version
  negotiation, device identifiers or capabilities into this envelope.
- Keep saved HTTPS URLs/tokens. `.json` and `.windows.json` remain valid;
  their status aliases are `.status.json` and `.windows.status.json`.
- Merge by saved source and item ID. Names are display text. Existing tags
  can differ between servers (`xh`/`xhttp`, `hy2`/`hysteria2`); never rename an
  ID to fit a new UI or infer its protocol from its display name.
- Apply a valid refresh atomically; preserve local edits and review removals.
  Failed fetch, malformed/protected data or invalid credentials retain the
  previous collection. A well-formed but unsupported protocol may be skipped
  with a safe reason; it must not reject unrelated supported items.
- Validate native envelopes/protections before deciding an item is merely
  unsupported. Never downgrade unknown/protected data into an unlocked link.
- Status/account/period IDs and traffic lower bounds retain their existing
  meaning. Unknown accounting intervals are unavailable/partial, not zero.

## Required capabilities for the full fixture

The fully provisioned, single-address, active unlimited Panel2 account has **30
configurations**, not 30 protocols. Time-limited accounts exclude shared-key
Soar Direct (29). Finite-volume accounts, with or without expiry, receive six
shared-budget native/bridge paths; excluded transports receive no credential.
Expired/disabled cases follow server policy. Additional
addresses can increase the count. A client must report its supported subset
honestly instead of padding to 30 or labeling untested connections healthy.
Main protocols precede auxiliary SSH paths; all ten DNS paths are last. This
presentation policy preserves item IDs and is not a WAN speed measurement.

| Path | Required interpretation |
| --- | --- |
| 24 native items | Existing native v33/88-field parser and transport adapters, including the two additional VLESS variants; original fields/IDs retained except approved display labels and intentional Hy2 mode migration |
| AmneziaWG | Standard `wireguard://` URI with AWG options; preserve every key, peer/address/DNS/MTU/keepalive and v3 HeaderProtectionKey/ContentPaddingAddition option. Detect AWG; do not strip its options or silently run ordinary WireGuard |
| Shadowsocks 2022 | Standard `ss://` URI; validate the 2022 method, key length and complete server:user PSK chain without truncation |
| Hysteria 2 | Preserve TLS validation, auth/SNI/ALPN and `obfs=salamander`/`obfs-password`; map to the engine's effective Salamander configuration |
| Trojan TLS | Trusted TLS/SNI and exact per-user authentication; no insecure fallback |
| VLESS Encryption | The pinned ML-KEM/X25519 encryption dialect; never silently substitute `encryption=none` |
| TLS Vision | VLESS TCP/TLS, trusted SNI, `flow=xtls-rprx-vision`; preserve the effective inner URI options |
| XHTTP REALITY | VLESS XHTTP with REALITY, public key, short ID, SNI, path and mode; preserve every effective option |
| AnyTLS | Standard `anytls://` URI with exact per-user password and trusted TLS SNI |
| TUIC | Standard `tuic://uuid:password@host:port`, trusted TLS SNI, h3 ALPN, BBR congestion control and native UDP relay |
| Mieru TCP/UDP | Distinct `mierus://` profiles; username/password, host, profile label, explicit port and `protocol=TCP`/`UDP` must survive import. The scheme name does not imply TLS |

AWG/SS2022 subscription URI encodings preserve the old native inner profile's
credentials, endpoint and effective options. The four additional AnyTLS/TUIC/
Mieru items also use standard URIs; the other24 retain the native envelope.
Native CLI exports remain available. Collection identity in
subscription imports comes from the saved source/item IDs and root name.

## Server lifecycle

Normal install/update reconciles missing managed cores without replacing
existing users/keys/IDs/URLs. DNS, valid TLS, TUN and free ports are real
prerequisites; an incomplete setup must be reported as pending.

New Hy2 defaults to Salamander. An old account-managed core without an explicit
choice migrates once. `hysteriaObfsConfigured=true` records an operator choice;
an explicit off survives future updates. Per-user auth, endpoint and saved URL
stay fixed. A client refreshes the saved subscription to receive the changed
Hy2 options. Only the selected active Hy2 core needs a migration restart.

Removal/expiry denial of a new Xray session is **not** proof that established
streams ended. Native finite relays close the exhausted account while retaining
the healthy account's held session. Compatibility SS2022/Trojan/VLESS Encryption
revocation restarts that shared core, so other users of it may briefly reconnect.
Protocols lacking durable per-user finite accounting remain excluded from
traffic-limited accounts. Ordinary WireGuard and AWG remain separate; no silent substitution.
An exact simultaneous-device limit is not implemented or activated.

Display binary units consistently:25GB means25GiB,25,600MiB or26,843,545,600bytes.
Use bytes as the source of truth; do not divide by10^9 and label that result as
the configured GB. Unknown or incomplete consumption is never invented zero.
Private status2 is a separate opt-in body described in
[the status contract](subscription-status-v2.md); strict feed/status1 is retained.
Slipstream's authenticated `.trust.json` companion contains public material only.
Clients must validate and persist certificate pins, retain the last valid pin on
failed refresh, and expose explicit changed-pin recovery. Never disable TLS
verification or fetch a private certificate key to make a profile connect.

## Joint acceptance and publication

The integration agent must collect exact final panel/client source commits,
artifact hashes, OS/architecture and actual runtime selections. Repeat import,
same-URL refresh (24→30 and30→30), unsupported/protected/malformed cases,
credential/endpoint equivalence, real payload, renewal/revoke, restart/reboot
and applicable accounting tests on those artifacts. Parser acceptance alone
does not prove a working engine, phone/Mac networking, TUN or battery behavior.

Changes requiring a new client parser/adapter are prepared in all affected
apps first, then activated in the panel after joint acceptance. Introduce any
future capability negotiation separately with a reviewed migration; do not
guess support from a version string or change today's strict v1 feed.

The current functional panel release can publish its ready, backward-compatible
behavior and supported client subsets. Future app-dependent changes wait for
app foundations and joint acceptance; the current release does not certify
all future client builds. Signing keys,
customer links, raw credential profiles and recovery secrets are never handoff
attachments. Public identity and the exact test results/remaining limits are.
