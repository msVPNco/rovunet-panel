# Rovunet Panel

Self-hosted Linux access server with account management, signed updates, encrypted backups and multi-protocol subscriptions.

**Current release: [v2.1.0](https://github.com/msVPNco/rovunet-panel/releases/tag/v2.1.0)** — built from the official v2.0.2 source, with saved-port Shadowsocks replacements and a Free config menu.

[فارسی](README.fa.md) · [Русский](README.ru.md) · [简体中文](README.zh-CN.md) · [Documentation](docs/en/README.md) · [Release notes](docs/releases/v2.1.0.md) · [Changelog](CHANGELOG.md) · [License](LICENSE)

## What's new in 2.1.0

- The normal setup replaces **WireGuard with Shadowsocks 2022 AES-128** and **TUIC with Shadowsocks ChaCha20-Poly1305**. Existing installations reuse each replaced core's saved port and endpoint.
- **Users → Free config (50 GB / 30 days)** creates a dedicated account with a shared traffic budget and expiry. A separate menu action exports its eligible profiles for review.
- Other saved ports, account identities, recorded usage and subscription URLs are preserved. AmneziaWG and the existing Shadowsocks 2022 AES-256 core remain available.
- The official 2.0.2 Slipstream public-certificate companion and TLS validation are preserved. The two new Shadowsocks methods do not require a TLS certificate.

## Update an existing server

Run this on the Linux server:

```bash
sudo rovunet
```

Choose **8) Update**. The normal updater authenticates v2.1.0, keeps a private recovery snapshot and migrates the selected default cores. After it reports success, leave the old menu and check the installed version:

```bash
sudo rovunet --version
sudo rovunet core-list
```

The version should begin with `rovunet v2.1.0`. Refresh the **same subscription already saved in the app** to receive the replacement profiles. Version 2.1.0 is newer than 2.0.3, so selecting it is not a downgrade. The signed acceptance run exercised official 2.0.2 → 2.1.0; the private 2.0.3 preview has not undergone that end-to-end upgrade test.

## Complete installation

A normal installation provisions **15 managed cores** and the base HTTPS, SSH and DNS services. An account receives **30 profiles when unlimited**, **29 when time limited**, and **6 when traffic limited**, with or without a time limit. These are configuration paths, not 30 separate protocols. Missing certificates, occupied ports or failed engines are reported; a partial setup is not reported as complete.

Use a fresh Ubuntu 24.04 LTS server with root access and systemd. AMD64 and ARM64 binaries are supplied. The v2.1.0 acceptance run executes Linux AMD64 in isolated namespaces; ARM64 binaries are built and authenticated but are not executed in that run. Other distributions need runtime qualification. Point the HTTPS hostname to the server and delegate DNS tunnel subdomains to its authoritative DNS listener. Cloud proxying must be disabled for UDP/raw protocol endpoints. Permit the required cloud-firewall ports.

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v2.1.0/install.sh -o rovunet-install.sh &&
python3 -c 'import hashlib,pathlib,sys; sys.exit(0 if hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="4ecf359cd96cfb8583ad89bdae5bfe4e14bfe1ee4dc12cedd603b77ec7519555" else "Installer checksum mismatch")' &&
sudo env ROVUNET_RELEASE_TAG=v2.1.0 bash rovunet-install.sh
```

The bootstrap verifies the production Ed25519 manifest and every downloaded executable. Keep the installer trust root independently; a checksum on this same page is not independent authentication. Follow the setup prompts for the public address, domains and first account. `rovunet install --custom` selects a subset; `rovunet complete-setup` resumes missing components.

## Profile order and protocols

The main protocol group appears first, auxiliary SSH paths follow, and **all ten DNS paths come last**. This fixed order is a display policy, not a speed-test result. Names contain protocol names; profile IDs and credentials remain separate from labels.

| Profile group | Default ports | Account policy |
|---|---|---|
| VLESS TLS Vision, REALITY 1/2, XHTTP REALITY, XHTTP TLS | 36444, 9443, 2087, 37444, 2096 TCP | Unlimited, timed, finite |
| VLESS WS TLS | 443 TCP, fronted by NaiveProxy | Unlimited, timed, finite |
| Hysteria 2 (Salamander) | 8443 UDP | Unlimited, timed |
| Shadowsocks 2022 AES-128 | 51820 TCP + UDP | Unlimited, timed |
| Shadowsocks ChaCha20-Poly1305 (classic AEAD) | 33444 TCP + UDP | Unlimited, timed |
| Mieru TCP / UDP | 34444 TCP / 35444 UDP | Unlimited, timed |
| AnyTLS | 32444 TCP | Unlimited, timed |
| Shadowsocks 2022 AES-256, VLESS Encryption, Trojan TLS | 28444 TCP+UDP, 31444 TCP, 30444 TCP | Unlimited, timed; compatibility protocols |
| AmneziaWG | 51840 UDP | Unlimited, timed |
| NaiveProxy SOCKS5 / SSH; StunTLS SSH; direct SSH | 443 TCP; direct SSH 22 TCP | Unlimited, timed |
| Soar Direct / SSH | 53 UDP | Direct: unlimited only; SSH: unlimited, timed |
| DNSTT / NoizDNS SOCKS5 / SSH | 53 UDP | Unlimited, timed |
| VayDNS SOCKS5 / SSH; Slipstream SOCKS5 / SSH | 53 UDP | Unlimited, timed |

The table shows fresh-install defaults. An upgrade keeps saved ports, including custom ports; missing cores use the fixed fallback choices from 2.0.2 when a preferred port is occupied. WireGuard and TUIC remain available through explicit manual core commands, while normal setup uses the replacements above.

DNS tunnel backends bind locally behind the router. Port 1080 is loopback-only unless direct SOCKS5 is explicitly enabled; a public SOCKS listener always requires authentication. Plain SSH/SOCKS provide their normal transport security characteristics; choose a suitable encrypted outer transport for the network.

Shadowsocks and Trojan without Flow are deprecated by the pinned Xray and retained as optional compatibility routes. A shared compatibility core is restarted on access revocation to close held streams; other users on that core can briefly reconnect. Finite accounts are excluded from unproven per-user accounting paths. Direct Soar uses a shared key and is excluded from time- and traffic-limited accounts.

## Free config in the panel

Open **3) Users → 2) Free config (50 GB / 30 days)**. The account has one shared **50 GiB** quota and a **30-day** expiry. Eligible profiles use the existing server-side traffic accounting policy; the three Shadowsocks methods remain excluded from traffic-limited accounts.

**3) Users → 3) Export Free config for review** exports or retries the private candidate at `/etc/rovunet/donations/<username>.json`. This file contains access credentials. The panel export does not publish donated access or activate the Free feature in released Android or Windows apps; that client feature requires its own release and activation.

## Accounts, usage and existing links

New accounts use a shared durable SQLite budget. Finite native/bridge paths reserve traffic before forwarding it. Time expiry also prevents future access and retires existing access through the relevant engine policy. Historical incomplete counters remain explicitly incomplete; missing usage is never invented as zero or presented as an exact remaining balance.

Traffic units use base 1024: **25 GB = 25,600 MB = 26,843,545,600 bytes**. Applications must not display 26.84 decimal GB for a 25 GB binary quota. Counts depend on installed components and account eligibility; adding extra public IP addresses can multiply address-specific profiles.

Normal updates preserve account identity, saved subscription URLs, account passwords, periods and recorded usage. The two replaced protocols receive new profile IDs and connection credentials; unrelated profiles retain theirs. After server update, refresh the **existing subscription in the app**. Android uses `.json`; Windows uses `.windows.json`. Do not use a Windows feed as a workaround for an unsupported Android protocol. An exact simultaneous-device limit is not enabled.

## Update, certificates and recovery

Menu **8 / Update** authenticates the release, refreshes reviewed runtimes, validates affected configurations and resumes interrupted changes from durable journals. Keep a private encrypted backup before maintenance. Do not replace an accounting database with an older copy while live writers continue running. Compatibility with every historical private preview is not claimed; see the tested upgrade baseline below.

```bash
sudo rovunet                         # interactive menu
sudo rovunet users --action add      # account creation, time and traffic limits
sudo rovunet subscriptions-refresh   # regenerate saved URLs
sudo rovunet certificates --mode check
sudo rovunet certificates --mode recover --tag slipstream-socks
sudo rovunet diag
sudo rovunet backup --file /root/rovunet-backup.age --recipient age1YOUR_PUBLIC_RECIPIENT
```

Generate a backup identity first with `rovunet backup-keygen --help`; retain its private identity off the server and store only the public recipient in an automation command. `rovunet backup-check --help` validates encrypted backups; `rovunet restore --help` describes restoration. SQLite backups use format 2: restore reconciles observations upward into the existing same-identity authority and preserves current links, periods and credentials. It refuses a missing database or changed configuration. This online checkpoint alone is not fresh-server disaster recovery; that requires the separately authenticated sealed `recovery-checkpoint` workflow. Backup files, credentials, subscription URLs and diagnostic logs are private.

Public HTTPS certificates renew through Caddy. `certificates --mode check` reports certificate health; recovery selects the TLS front or Slipstream tag and preserves failure evidence. Slipstream distributes only its public certificate through the authenticated `.trust.json` companion. Apps must validate and persist pins, keep a previous valid pin on download failure, and provide explicit recovery for a changed pin. Private certificate keys are never sent to apps.

## Security and integration scope

Runtime source revisions, dependency locks, patches, notices and vulnerability-scan receipts accompany the release. A clean scan is bounded by its database snapshot and does not prove the absence of all vulnerabilities. Native TLS/QUIC security backports have separate sanitizer and regression evidence. Runtime licenses are component-specific: the panel is AGPL-3.0; Mieru and sing-box are GPL components; Naive/Caddy's dependency combination includes GPL components. Read the distributed notices and corresponding sources.

The [v2.1.0 preparation run](https://github.com/msVPNco/rovunet-panel/actions/runs/45072135345) passed reviewed-Xray configuration, TCP/UDP payload and lifecycle tests for all three Shadowsocks methods, plus setup, migration and recovery checks. The [final signed acceptance and publication run](https://github.com/msVPNco/rovunet-panel/actions/runs/45076586381) passed all four gates: signed installer staging, the actual old panel's Update menu, account identity/usage preservation, and TLS/trust preservation.

That final run uses Linux AMD64 namespaces, simulated systemd and real HTTPS with a private transport CA. Its baseline is official v2.0.2; fresh installation is tested with `--no-run`. ARM64 execution, the private 2.0.3 upgrade and live client/ISP connectivity are outside that run. The TLS front and Slipstream are configured but administratively inactive in the upgrade fixture, so preserving their identities is not a live traffic test. No WAN speed ranking or universal Android/macOS/Windows compatibility is claimed. The [client contract](docs/contracts/panel-integration-v1.md) defines required capabilities rather than freezing app version numbers. [Broader production qualification gates](docs/release-gates.json) remain separately tracked; server functionality does not certify app behavior, independent/off-site key custody or every ISP.

[Panel2 status, certificate and session contract](docs/contracts/panel-v2-addendum.md) — strict feed1; opt-in status2; no device-session API.
