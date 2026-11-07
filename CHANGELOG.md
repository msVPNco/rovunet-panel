# Changelog

Changes in Rovunet Panel. Each release's notes come from its section here.

## v1.15.2

- **A traffic-limited user's subscription is refreshed every 5 minutes.** Their feed includes a REALITY or XHTTP config only while that core's statistics answer. Right after an update the cores are still restarting, so the feed written at that moment could miss them and stay that way until the next user change. The limits timer now rewrites the feeds on each run (only files whose content changed are touched), so the missing configs appear within 5 minutes. This was seen on a renewed user whose configs did not grow after the update.

## v1.15.1

- **`rovunet diag` no longer lists the limits timer as an orphaned service** (v1.14.0 added `rovunet-user-expiry` and `diag` did not know it).
- **`diag` checks each REALITY and XHTTP core's traffic statistics** and says whether they answer, since traffic limits on those cores depend on it. A core whose statistics do not answer is a warning, not a failure: limited users are simply not given that core.

## v1.15.0

- **Traffic limits now cover REALITY and XHTTP too.** Those cores expose Xray's statistics on a loopback-only port (a statistics API, a counter per user named by the panel username, and a routing rule; WireGuard and Hysteria 2 have none and stay unmetered). The limits timer reads each core's counters every 5 minutes and adds them to the user's total together with VLESS and SOCKS5. A traffic-limited user is now given the VLESS, REALITY and XHTTP configs, and a core is given only if its statistics answer (checked when the configs are printed); otherwise the user keeps just VLESS.
- **`rovunet update` restarts each REALITY and XHTTP core once** to apply the new configuration. Xray clears its counters when a core restarts, so the panel reads them just before it restarts a core for a user change, and a restart outside the panel loses up to 5 minutes of counting.
- This relies on the `xray api statsquery` command and the classic statistics configuration. They were written from the documented form and a CI test of the generated config, not run against your Xray yet: if a limited user is not given REALITY/XHTTP configs, or `users list` shows no growing usage for it, the statistics are not answering and the output of `xray api statsquery -s 127.0.0.1:10085 -pattern "user>>>"` is what is needed.

## v1.14.0

### Traffic limits, and stopped users can be renewed

- **The GB limit is now enforced, for VLESS (WebSocket) traffic.** The panel's own VLESS and SOCKS5 servers count each user's bytes (both directions, saved every 15 seconds in `/var/lib/rovunet/usage`). A timer checks every 5 minutes (it was hourly), so a little more than the limit can pass. 1 GB is 1024³ bytes.
- **A user with a traffic limit is given only the VLESS config**, in `users configs`, the install summary and the subscription feed, and no shared Soar config. Traffic over SSH, NaïveProxy, the DNS tunnels and the Xray cores is not counted, and the user is not handed their credentials, so the limit is real rather than an easy-to-bypass number. The panel says so when you set one, and warns if there is no VLESS tunnel yet. Counting the Xray cores is not done.
- **Expired or used-up users are stopped, not lost.** They are removed from every service exactly as before (open sessions end), but a record keeps the password and the subscription link. `users --action renew` brings the user back with new limits counted from now and **the same password and link**, so nothing has to be re-sent. `users list` shows stopped users and each limited user's usage; `users remove <name>` on a stopped user forgets the record.
- Only the VLESS and SOCKS5 servers count: VLESS needs `rovunet update` (the servers restart once to start counting).

## v1.13.1

- **The subscription link now appears everywhere configs are shown.** v1.13.0 printed it only for a single user's configs (`users` > list > a user, and after adding a user). The "Show all configs" action and the install summary, which print `[soar] <user>, no login needed` and the rest, left it out. They now print each user's link, and a user created before links existed gets theirs on the spot. No new user is needed.

## v1.13.0

### Subscription link for every user

- **Each user gets one link that delivers all of their configs to the app**, whether the user is unlimited or limited. `users configs` prints it under the configs: `https://<naive-domain>/sub/<token>.json`. Paste it into the app's Subscriptions menu and the app imports (and later refreshes) every config in one step.
- **It is served on port 443 by the NaïveProxy Caddy**, with the domain's real certificate, as plain static files: no new service, no new port, no application behind it. The app requires HTTPS, so port 22 (SSH) or any plain-HTTP port cannot carry it. Without a NaïveProxy tunnel there is no link.
- **The file holds exactly what `users configs` prints** (rovunet:// configs), in the one JSON shape the app accepts, so the two cannot disagree. It is rewritten whenever users or tunnels change and on `rovunet update`.
- **The link is an unguessable 128-bit name, like a share link:** anyone who has it can read that user's configs, so treat it like the password. Changing the user's password makes a new link and retires the old one; removing or expiring the user deletes the file. The directory can be entered but not listed, so a local SSH account cannot enumerate the files.
- A NaïveProxy Caddy restarts once on update to start serving `/sub/`.

## v1.12.1

- **REALITY picks its disguise at random.** When no `--target` is given, `core-install` and the installer choose a random site from a built-in list of 16 (Microsoft, Amazon and its image servers, Apple, Cloudflare, Google download and font hosts, NVIDIA, Samsung, Intel, Oracle, Mozilla). Each candidate is first checked from your server for TLS 1.3 and HTTP/2, which REALITY needs, and only working ones are drawn. The server name follows the chosen site. If none can be reached the installer falls back to `www.microsoft.com`; `core-install` stops and asks for `--target`. Cores installed earlier keep their target.
- The list is a judgement about sites that are rarely blocked, not a guarantee. It is `RealityTargets` in `internal/coreplan/targets.go`.

## v1.12.0

### Users can expire

- **Adding a user asks "Unlimited or Limited?"** Unlimited is the default. For Limited you are asked for days and then for GB, and a blank answer to either means unlimited. For scripts, `rovunet users --action add --username bob --days 30 --gb 50` skips the questions; `bulk_add` takes the same two options.
- **An expired user is removed automatically.** An hourly timer (`rovunet-user-expiry.timer`, set up when the first user with an expiry is added, or by `rovunet update`) runs `rovunet expire-users`, which removes the user exactly like `users remove`, so open sessions end too. The user is deleted, not suspended.
- **`users list` shows each user's limits.** Changing a password keeps the user's expiry and limit.
- **The GB limit is recorded but not enforced yet.** The panel says so when you set one. Enforcement, and a subscription link, are the next releases.

## v1.11.3

- **`rovunet diag` names the process on every listening port**, including VLESS (2053), the DNS router (53) and SOCKS5 (1080), which v1.11.2 had left out. Names are no longer cut at 15 characters (`slipstream-serv` is now `slipstream-server`).
- **A TLS-SSH tunnel carried by NaïveProxy now says so** (`none of its own (carried by <tag>)`) instead of showing no port line.

## v1.11.2

- **`rovunet diag` checks the NaïveProxy and TLS-SSH ports.** Until now a Caddy that was running but held no port still showed green. A TLS-SSH tunnel carried by NaïveProxy has no port of its own and is skipped.
- **Listening ports name the process that holds them**, e.g. `listening (xray)`, so a port taken by the wrong program stands out. This is a local check only: it does not show whether the port is reachable from the internet.

## v1.11.1

- **`rovunet diag` now checks the Xray cores.** For each installed core (REALITY, XHTTP, Hysteria 2, WireGuard) it reports whether the service is active and enabled at boot, whether the port is listening, how many days its TLS certificate has left, and whether the renewal timer is on. A certificate with under 7 days left is a failure.
- **Fixed false "Orphaned Services" warnings:** `diag` listed every Xray core and the certificate check as orphaned, because it only knew the tunnels.

## v1.11.0

- **Plain SOCKS5 is no longer in the default output.** The direct SOCKS5 server is unencrypted and was blocked on the mobile network we measured, so `users configs`, the install summary and `export-clients` leave it out and say how many they hid. List it with `ROVUNET_ALL_CONFIGS=1` (menus) or `export-clients --all`. Nothing is removed: the server keeps running and existing tunnels are unchanged. NaïveProxy's SOCKS variant is not affected.

## v1.10.2

- **NaïveProxy now gives both configs everywhere.** One NaïveProxy tunnel serves SOCKS and SSH clients, and the install summary printed both, but `users configs` and `export-clients` printed only the SOCKS one. They now print `<name>-socks` and `<name>-ssh`.

## v1.10.1

- **Removing a user now ends that user's open connections.** Until now only new logins were refused: a VLESS (WebSocket) or SOCKS5 connection that was already open kept working after `users remove`, or after a password change, until the app disconnected. Both servers now close the sessions of a removed or re-keyed user at once and leave other users' sessions alone. SSH, NaïveProxy and the Xray cores already dropped them.

## v1.10.0

### Fastest first, and every protocol the app supports from one install

- **Configurations are listed from the fastest to the slowest:** VLESS over WebSocket, XHTTP, REALITY, NaïveProxy, SSH over TLS, SSH, then Hysteria 2, WireGuard and SOCKS5, and the DNS tunnels last. `users configs`, the install summary and `export-clients` follow it, with Xray cores slotted in among the tunnels.
- **The installer sets up VLESS + XHTTP and Hysteria 2** with the certificate Caddy obtained for the NaïveProxy domain, next to REALITY and WireGuard. REALITY and XHTTP are marked as recommended; Hysteria 2, WireGuard and SOCKS5 are marked as blocked on some networks, which a test on a filtered mobile network confirmed.
- **A renewed TLS certificate is served without any action.** A timer runs `rovunet core-certs` twice a day and restarts XHTTP and Hysteria 2 cores whose certificate file changed. `rovunet update` sets the timer up for cores installed earlier.
- Docs in four languages: a "Choosing" section with the measured result, and the order of the configurations.

## v1.9.3

- The WireGuard file printed by `users configs` no longer has indented lines. The official WireGuard app does not recognise an indented `[Peer]` section and rejected the file with "Missing attribute in Peer's PublicKey".

## v1.9.2

- `users configs` and the install summary also print, under each Xray core profile, the plain link or the WireGuard file for other apps, so a core can be tested with the official WireGuard app or any VLESS client.

## v1.9.1

### Core installs no longer accept pasted or placeholder addresses

- `core-install` turns a hostname pasted as a Markdown link, such as `[www.example.com](https://www.example.com)` (chat apps and browsers paste these), back into the plain name, and cores already stored that way are repaired the next time the panel updates them.
- It rejects a host, server name or REALITY target that is not an IP address or a domain name, such as the placeholder `SERVER_IP`, and names the offending value. Before, such a core installed without complaint and its clients pointed at an address that does not exist.

## v1.9.0

### Xray cores for every user

- **One install delivers every protocol.** The installer offers VLESS + REALITY and WireGuard next to the other protocols and sets them up after the first user, on the first free default port.
- **Cores serve the panel's users.** Every user gets a configuration on every Xray core. `users add`, `remove` and `passwd` keep the cores in step: Xray validates each change, a rejected change keeps the previous configuration, and a changed password revokes the old configuration. Adding a user never changes an existing user's configuration.
- **One group per user in the app.** `users configs`, the install summary and `export-clients` print each user's core profiles as native `rovunet://` profiles in the user's collection, next to their tunnels. `export-clients --format standard` prints the plain `vless://` and `hysteria2://` links.
- `core-install` serves the panel's users by default; `--devices` keeps the per-device mode. Cores installed by v1.8 keep working as they are.
- Backups carry the cores' new state, so restored cores keep their keys and every user's configuration.

## v1.8.2

### Fixes

- **Services failing after updating from v1.7.0.** The old updater starts the new version from the old command's path, so the SOCKS5 proxy, Soar, SSH over TLS and VLESS services were regenerated to run `/usr/local/bin/slipgate`, which the migration had removed, and failed with `203/EXEC`. Services now always run the installed `rovunet`, and the next update repairs any unit still pointing at the old command.
- `tunnel status` showed every non-running state as `unknown`, which hid those failures. It now shows systemd's own state (`activating`, `failed`, `inactive`), and direct SSH and SOCKS5 tunnels show the state of sshd and the shared SOCKS5 proxy.

## v1.8.1

### Fixes

- **Soar configs join each user's group in the app.** The install summary, `tunnel add` and `tunnel share` printed the login-free Soar config once without a user, so the app put it in a group of its own. It is now printed per user, like `users configs`.
- **Updating from v1.7.0 is safer.** If a step after the migration fails, the update no longer makes the old updater rebuild old-named services on paths that were moved; the config directory is moved before users and groups are renamed, and a failed move restarts the stopped services.
- The migration makes `config.json` owner-only (0600) right away; v1.7.0 wrote it readable by all local users.

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
