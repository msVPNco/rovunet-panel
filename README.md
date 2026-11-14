# Rovunet Panel 2

Self-hosted Linux access server with account management, signed updates, encrypted backups and multi-protocol subscriptions.

[فارسی](README.fa.md) · [Русский](README.ru.md) · [简体中文](README.zh-CN.md) · [Releases](https://github.com/msVPNco/rovunet-panel/releases) · [Release notes](docs/releases/v2.0.0.md) · [License](LICENSE)

## Complete installation

A normal installation provisions **15 managed cores** and the base HTTPS, SSH and DNS services. An account receives **30 profiles when unlimited**, **29 when time limited**, and **6 when traffic limited**, with or without a time limit. These are configuration paths, not 30 separate protocols. Missing certificates, occupied ports or failed engines are reported; a partial setup is not reported as complete.

Use a fresh Ubuntu 24.04 LTS server with root access and systemd. AMD64 and ARM64 binaries are supplied. ARM64 emulation and native AMD64 evidence are distinguished in the acceptance report. Other distributions need runtime qualification. Point the HTTPS hostname to the server and delegate DNS tunnel subdomains to its authoritative DNS listener. Cloud proxying must be disabled for UDP/raw protocol endpoints. Permit the required cloud-firewall ports.

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v2.0.0/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="897108ce22f619bad23aecc6792283e4af01df0172b917b28e03082675fe76a6", "Installer checksum mismatch"'
sudo env ROVUNET_RELEASE_TAG=v2.0.0 bash rovunet-install.sh
```


The bootstrap verifies the production Ed25519 manifest and every downloaded executable. Keep the installer trust root independently; a checksum on this same page is not independent authentication. Follow the setup prompts for the public address, domains and first account. `rovunet install --custom` selects a subset; `rovunet complete-setup` resumes missing components.

## Profile order and protocols

The main protocol group appears first, auxiliary SSH paths follow, and **all ten DNS paths come last**. This fixed order is a display policy, not a speed-test result. Names contain protocol names; profile IDs and credentials remain separate from labels.


| Profile group | Default ports | Account policy |
|---|---|---|
| VLESS TLS Vision, REALITY 1/2, XHTTP REALITY, XHTTP TLS | 36444, 9443, 2087, 37444, 2096 TCP | Unlimited, timed, finite |
| VLESS WS TLS | 443 TCP, fronted by NaiveProxy | Unlimited, timed, finite |
| Hysteria 2 (Salamander), TUIC | 8443, 33444 UDP | Unlimited, timed |
| Mieru TCP / UDP | 34444 TCP / 35444 UDP | Unlimited, timed |
| AnyTLS | 32444 TCP | Unlimited, timed |
| Shadowsocks 2022, VLESS Encryption, Trojan TLS | 28444 TCP+UDP, 31444 TCP, 30444 TCP | Unlimited, timed; compatibility protocols |
| AmneziaWG, WireGuard | 51840, 51820 UDP | Unlimited, timed |
| NaiveProxy SOCKS5 / SSH; StunTLS SSH; direct SSH | 443 TCP; direct SSH 22 TCP | Unlimited, timed |
| Soar Direct / SSH | 53 UDP | Direct: unlimited only; SSH: unlimited, timed |
| DNSTT / NoizDNS SOCKS5 / SSH | 53 UDP | Unlimited, timed |
| VayDNS SOCKS5 / SSH; Slipstream SOCKS5 / SSH | 53 UDP | Unlimited, timed |


DNS tunnel backends bind locally behind the router. Port 1080 is loopback-only unless direct SOCKS5 is explicitly enabled; a public SOCKS listener always requires authentication. Plain SSH/SOCKS provide their normal transport security characteristics; choose a suitable encrypted outer transport for the network.

Shadowsocks and Trojan without Flow are deprecated by the pinned Xray and retained as optional compatibility routes. A shared compatibility core is restarted on access revocation to close held streams; other users on that core can briefly reconnect. Finite accounts are excluded from unproven per-user accounting paths. Direct Soar uses a shared key and is excluded from time- and traffic-limited accounts.

## Accounts, usage and existing links

New accounts use a shared durable SQLite budget. Finite native/bridge paths reserve traffic before forwarding it. Time expiry also prevents future access and retires existing access through the relevant engine policy. Historical incomplete counters remain explicitly incomplete; missing usage is never invented as zero or presented as an exact remaining balance.

Traffic units use base 1024: **25 GB = 25,600 MB = 26,843,545,600 bytes**. Applications must not display 26.84 decimal GB for a 25 GB binary quota. Counts depend on installed components and account eligibility; adding extra public IP addresses can multiply address-specific profiles.

Normal updates preserve account identity, saved subscription URLs, credentials, periods and recorded usage. After server update, refresh the **existing subscription in the app**. Android uses `.json`; Windows uses `.windows.json`. Do not use a Windows feed as a workaround for an unsupported Android protocol. An exact simultaneous-device limit is not enabled.

## Update, certificates and recovery

Menu **8 / Update** authenticates the release, refreshes reviewed runtimes, validates affected configurations and resumes interrupted changes from durable journals. Keep a private encrypted backup before maintenance. Do not replace an accounting database with an older copy while live writers continue running. Compatibility with every historical private preview is not claimed; use a fresh full installation for version 2 qualification.

```bash
sudo rovunet                         # interactive menu
sudo rovunet users add               # account creation, time and traffic limits
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

WireGuard and TUIC pass controlled runtime payload tests; this does not guarantee reachability through a provider or filtered mobile network. Native WireGuard remains WireGuard; AmneziaWG is a separate option. No WAN speed ranking or universal Android/macOS/Windows compatibility is claimed. The [client contract](docs/contracts/panel-integration-v1.md) defines required capabilities rather than freezing app version numbers. [Broader production qualification gates](docs/release-gates.json) remain separately tracked; server functionality does not certify app behavior, independent/off-site key custody or every ISP.
