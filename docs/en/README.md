# Rovunet Panel documentation

**English** · [فارسی](../fa/README.md) · [Русский](../ru/README.md) · [简体中文](../zh-CN/README.md)

Rovunet Panel turns a Linux server into a multi-protocol access server. It installs and manages protocol services, creates users, and exports client configurations eligible for each account.

## Current release: 2.1.0

Use the [current installation and update guide](../../README.md), [v2.1.0 release notes](../releases/v2.1.0.md) and [signed release](https://github.com/msVPNco/rovunet-panel/releases/tag/v2.1.0).

- **Update:** run `sudo rovunet` on the server, choose **8) Update**, then refresh the existing subscription in the app.
- **Default replacements:** WireGuard → Shadowsocks 2022 AES-128; TUIC → classic AEAD Shadowsocks ChaCha20-Poly1305. Fresh-install ports are **51820** and **33444 TCP + UDP**; upgrades retain saved ports. AmneziaWG and AES-256 Shadowsocks 2022 are retained.
- **Free config:** **3) Users → 2) Free config (50 GB / 30 days)**. **3) Export Free config for review** writes a private candidate for eligible, accounted profiles. This does not activate the Free feature in released apps.
- **Acceptance:** [the final signed run passed](https://github.com/msVPNco/rovunet-panel/actions/runs/45076586381) on Linux AMD64 namespaces with simulated systemd, using official v2.0.2 as its upgrade baseline. ARM64 execution, the private 2.0.3 upgrade and live client connectivity are outside that run.

The guides below also describe explicit manual and legacy core commands; the release notes define the current default setup.

## Contents

| # | Page | What it covers |
|---|---|---|
| 1 | [Getting started](getting-started.md) | Requirements, choosing a server, DNS records, installation, first tunnel |
| 2 | [Protocols](protocols.md) | Every supported protocol, how it travels, ports, domain layout, routing |
| 3 | [Users and client configurations](users-and-configs.md) | User model, sharing configurations, export formats, client apps |
| 4 | [Xray cores](xray-cores.md) | Manual core installation; VLESS + REALITY, VLESS + XHTTP and legacy examples |
| 5 | [Operations](operations.md) | Updating, backup and restore, monitoring, WARP, files and services, uninstalling |
| 6 | [Command reference](commands.md) | Every command and its options |
| 7 | [Troubleshooting](troubleshooting.md) | Symptoms, causes and fixes |
| 8 | [Limits and subscription links](limits-and-subscriptions.md) | Unlimited or limited users, subscription links, extra addresses |

## Conventions

- Commands run on the server as root, so they are shown with `sudo`.
- `example.com` stands for your domain and `SERVER_IP` for your server's public IP address.
- Protocol names follow their upstream projects: VLESS, REALITY and XHTTP (Project X), Hysteria 2, Shadowsocks, AmneziaWG, NaïveProxy, SSH and SOCKS5; legacy WireGuard and TUIC can be selected manually. DNSTT, NoizDNS, VayDNS, Slipstream and Soar are DNS tunnels.
- In the panel, a **tunnel** is one configured protocol instance with a tag (its name), and a **core** is a managed protocol service installed with `core-install`. Its runtime depends on the protocol.

## Project

- Source: [github.com/msVPNco/rovunet-panel](https://github.com/msVPNco/rovunet-panel) · [Changelog](../../CHANGELOG.md) · [Integration status](../ROVUNET-STATUS.md)
- Android app: [github.com/msVPNco/rovunet-app](https://github.com/msVPNco/rovunet-app)
- Community: [t.me/RovunetVPN](https://t.me/RovunetVPN)
- Based on [SlipGate](https://github.com/anonvector/slipgate) by Anonvector, not affiliated. See [NOTICE](../../NOTICE.md). License: AGPL-3.0.
