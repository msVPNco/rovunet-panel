# Operations

[Documentation](README.md) › Operations

## Updating

```bash
sudo rovunet update
```

The update downloads the latest release, verifies it against the release's SHA256SUMS, replaces the binary atomically (rolling back if the install fails), refreshes the transport binaries and regenerates the services. Tunnels, users and keys are untouched, and existing client links keep working.

### Migrating from the earlier name

A server installed under the panel's earlier name (`slipgate` command, `/etc/slipgate`) updates once with the old command and is migrated automatically:

```bash
sudo slipgate backup
sudo slipgate update
sudo rovunet tunnel status
```

The migration stops the old services, moves `/etc/slipgate` to `/etc/rovunet`, rewrites stored paths without touching any password, renames the system users and groups with the same UIDs and GIDs, recreates the services as `rovunet-*`, updates the sshd configuration only after `sshd -t` accepts it, makes `config.json` owner-only and installs the command as `/usr/local/bin/rovunet`. A server running upstream SlipGate can switch the same way:

```bash
curl -fsSL -o /tmp/rovunet https://github.com/msVPNco/rovunet-panel/releases/latest/download/rovunet-linux-amd64
sudo install -m 755 /tmp/rovunet /usr/local/bin/rovunet
sudo rovunet post-update
```

## Backup and restore

A backup holds everything needed to run the setup elsewhere: the configuration, every tunnel's private keys and certificates, all users and their passwords, and the Xray cores. Restored on another server, the same tunnels come back with the same keys, so clients keep working without new links. It also holds each user's traffic used so far, so restoring does not give limited users their quota again.

```bash
sudo rovunet backup                      # asks: save to a file, or print text to copy
sudo rovunet backup /root/rovunet-backup.age
sudo rovunet backup-check /root/rovunet-backup.age
sudo rovunet restore /root/rovunet-backup.age
# Automated transfer: see ../security/encrypted-backups.md
```

Backups are authenticated age-encrypted text by default. Keep the recovery passphrase/identity separately. The old ROVUNET BACKUP format remains readable; creating plaintext now requires an explicit --plaintext choice. Automated migration needs a public --recipient at the source and an owner-only --identity-file at the destination; the old unkeyed SSH pipe is no longer sufficient. See [encrypted backup guidance](../security/encrypted-backups.md).

Restoring on a new server:

1. Install the panel; when the wizard asks for transports, press Ctrl-C, since restore sets them up.
2. Run `sudo rovunet restore` and paste the text, or pass the file.
3. Point the DNS records at the new IP; restore lists the ones to change (`ns.<domain>` and NaïveProxy domains).

Restore installs the transport binaries, writes the keys and configuration, creates the users, opens the firewall ports, sets WARP up again if it was on, starts every service and restarts the Xray cores. Anything already on the server is replaced, after being saved to `/root/rovunet-pre-restore-<date>.txt`.

`rovunet config export` prints only `config.json`, without keys: use `backup` to move a server.

## Monitoring

| Command | Shows |
|---|---|
| `sudo rovunet stats` | Live dashboard: CPU, RAM, traffic, connections per protocol, tunnel status |
| `sudo rovunet tunnel status` | Every tunnel and Xray core with its service state |
| `sudo rovunet tunnel status <tag>` | One tunnel's details: keys, port, MTU, certificate names, issuer type and expiry |
| `sudo rovunet diag` | Health checks: services, ports, keys, DNS resolution, boot persistence |
| `sudo rovunet tunnel logs <tag>` | A tunnel's service log |

Certificate details flag expired, soon-to-expire (under 14 days) and unreadable certificates.

## WARP outbound (optional)

Cloudflare WARP can carry the server's outbound traffic, for when the server's IP is refused by sites users need, or as an extra hop. Enable it in the interactive menu. It adds latency and a dependency on Cloudflare. Soar with the direct backend, VLESS and the Xray cores do not use WARP.

## Firewall

The panel opens the ports its tunnels and cores use with UFW, firewalld or iptables, whichever is active. With iptables it records the ports it opened in `/etc/iptables/rovunet-ports` and inserts only those at boot, leaving the rules of other tools (Docker, fail2ban) alone.

## Files and services

| Path | Contents |
|---|---|
| `/etc/rovunet/config.json` | Main configuration: tunnels, users, routing (owner-only) |
| `/etc/rovunet/tunnels/<tag>/` | A tunnel's keys, certificates and configuration |
| `/etc/rovunet/vless.secret` | Server-wide secret that derives VLESS UUIDs and paths |
| `/etc/rovunet/core/<name>/` | An Xray core: `server.json`, `meta`, per-device `clients/` |
| `/usr/local/bin/rovunet` | The panel, including the built-in SOCKS5, Soar, SSH over TLS and VLESS servers |
| `/usr/local/bin/dnstt-server`, `slipstream-server`, `vaydns-server`, `caddy-naive` | Transport binaries |
| `/usr/local/bin/xray` | Pinned Xray core for `core-install` |
| `/etc/iptables/rovunet-ports` | Ports the panel opened in iptables |

| systemd unit | Role |
|---|---|
| `rovunet-dnsrouter` | DNS router on port 53 |
| `rovunet-socks5` | Built-in SOCKS5 proxy |
| `rovunet-<tag>` | One per tunnel |
| `rovunet-core-<name>` | One per Xray core |
| `rovunet-iptables` | Restores the panel's firewall ports at boot |

```bash
journalctl -u rovunet-mytunnel -n 200 --no-pager
```

## Uninstalling

```bash
sudo rovunet uninstall
```

Removes every service, configuration, binary, system user and SSH tunnel user. DNS records are left as they are; delete them at your DNS provider if needed.

Next: [Command reference](commands.md)
