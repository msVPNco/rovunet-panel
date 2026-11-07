# Rovunet Panel documentation

**English** · [فارسی](../fa/README.md) · [Русский](../ru/README.md) · [简体中文](../zh-CN/README.md)

Rovunet Panel turns a Linux server into a multi-protocol access server. It installs and manages each protocol as a systemd service, creates users, and prints ready-to-import client configurations for every user.

## Contents

| # | Page | What it covers |
|---|---|---|
| 1 | [Getting started](getting-started.md) | Requirements, choosing a server, DNS records, installation, first tunnel |
| 2 | [Protocols](protocols.md) | Every supported protocol, how it travels, ports, domain layout, routing |
| 3 | [Users and client configurations](users-and-configs.md) | User model, sharing configurations, export formats, client apps |
| 4 | [Xray cores](xray-cores.md) | VLESS + REALITY, VLESS + XHTTP, Hysteria 2 and WireGuard |
| 5 | [Operations](operations.md) | Updating, backup and restore, monitoring, WARP, files and services, uninstalling |
| 6 | [Command reference](commands.md) | Every command and its options |
| 7 | [Troubleshooting](troubleshooting.md) | Symptoms, causes and fixes |

## Conventions

- Commands run on the server as root, so they are shown with `sudo`.
- `example.com` stands for your domain and `SERVER_IP` for your server's public IP address.
- Protocol names follow their upstream projects: VLESS, REALITY and XHTTP (Project X), Hysteria 2, WireGuard, NaïveProxy, SSH and SOCKS5. DNSTT, NoizDNS, VayDNS, Slipstream and Soar are DNS tunnels.
- In the panel, a **tunnel** is one configured protocol instance with a tag (its name), and a **core** is one Xray server installed with `core-install`.

## Project

- Source: [github.com/msVPNco/rovunet-panel](https://github.com/msVPNco/rovunet-panel) · [Changelog](../../CHANGELOG.md) · [Integration status](../ROVUNET-STATUS.md)
- Android app: [github.com/msVPNco/rovunet-android](https://github.com/msVPNco/rovunet-android)
- Community: [t.me/RovunetVPN](https://t.me/RovunetVPN)
- Based on [SlipGate](https://github.com/anonvector/slipgate) by Anonvector, not affiliated. See [NOTICE](../../NOTICE.md). License: AGPL-3.0.
