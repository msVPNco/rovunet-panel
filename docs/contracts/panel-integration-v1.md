# Panel–client integration contract v1

This is the handoff contract for the Android, Windows, macOS and integration
agents. Client versions in acceptance reports are dated test specimens, not
requirements or a promise about later builds. Each final client must run the
same contract checks against its own final artifact before joint publication.

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

The fully provisioned, single-address, active unlimited account has **24
configurations**, not 24 protocols. Time-limited accounts exclude shared-key
Soar Direct (23); quota/expired/disabled cases follow server policy. Additional
addresses can increase the count. A client must report its supported subset
honestly instead of padding to 24 or labeling untested connections healthy.

| Path | Required interpretation |
| --- | --- |
| 22 native items | Existing native v33/88-field parser and the existing transport adapters; original fields/IDs retained except approved display labels and intentional Hy2 mode migration |
| AmneziaWG | Standard `wireguard://` URI with AWG options; preserve every key, peer/address/DNS/MTU/keepalive and v3 HeaderProtectionKey/ContentPaddingAddition option. Detect AWG; do not strip its options or silently run ordinary WireGuard |
| Shadowsocks 2022 | Standard `ss://` URI; validate the 2022 method, key length and complete server:user PSK chain without truncation |
| Hysteria 2 | Preserve TLS validation, auth/SNI/ALPN and `obfs=salamander`/`obfs-password`; map to the engine's effective Salamander configuration |
| Trojan TLS | Trusted TLS/SNI and exact per-user authentication; no insecure fallback |
| VLESS Encryption | The pinned ML-KEM/X25519 encryption dialect; never silently substitute `encryption=none` |

AWG/SS2022 subscription URI encodings preserve the old native inner profile's
credentials, endpoint and effective options; the other 22 items retain their
native envelope. Native CLI exports remain available. Collection identity in
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
streams ended. The four added protocols remain excluded from traffic-limited
accounts. Ordinary WireGuard and AWG remain separate; no silent substitution.
An exact simultaneous-device limit is not implemented or activated.

## Joint acceptance and publication

The integration agent must collect exact final panel/client source commits,
artifact hashes, OS/architecture and actual runtime selections. Repeat import,
same-URL refresh (20→24 and 24→24), unsupported/protected/malformed cases,
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
