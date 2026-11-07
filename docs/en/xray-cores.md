# Xray cores

[Documentation](README.md) › Xray cores

An Xray core is a separate Xray server installed and managed by the panel, beside the tunnels. It provides four protocols the built-in servers do not.

| Protocol | Transport | Certificate | Best for |
|---|---|---|---|
| **VLESS + REALITY** (XTLS Vision) | TCP | None: the handshake imitates a real site (`--target`) | Networks where TLS to well-known sites passes |
| **VLESS + XHTTP** | TCP, HTTP requests over TLS | Required, valid for `--sni` | Behind a CDN, or where WebSocket is blocked |
| **Hysteria 2** | UDP (QUIC) | Required, valid for `--sni` | Lossy or unstable networks where UDP passes |
| **WireGuard** | UDP | None | Networks that do not block WireGuard |

## Installing a core

```bash
# VLESS + REALITY
sudo rovunet core-install --name r1 --protocol reality --host SERVER_IP \
  --target www.microsoft.com:443 --sni www.microsoft.com --port 8443 --devices 5

# VLESS + XHTTP
sudo rovunet core-install --name x1 --protocol xhttp --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 2096 --devices 5

# Hysteria 2
sudo rovunet core-install --name h1 --protocol hysteria2 --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 8443 --devices 5

# WireGuard
sudo rovunet core-install --name w1 --protocol wireguard --host SERVER_IP --port 51820 --devices 5
```

| Option | Meaning |
|---|---|
| `--name` | Core name: 1–31 characters, `a-z`, `0-9` and `-` |
| `--protocol` | `reality`, `xhttp`, `hysteria2` or `wireguard` |
| `--host` | Public hostname or IP clients connect to |
| `--sni` | Certificate name, or the server name REALITY presents (defaults to `--host`) |
| `--target` | REALITY only: the real TLS site to imitate, as `host:port` |
| `--port` | Listen port (default 8443); must be free |
| `--devices` | Number of distinct client credentials, 1–1000 |
| `--cert`, `--key` | XHTTP and Hysteria 2: certificate and private key paths |

## What installation does

1. Checks that the port is free on the right protocol (TCP for REALITY and XHTTP, UDP for Hysteria 2 and WireGuard).
2. Installs the pinned Xray build published with this release, verified against the release's SHA256SUMS. An existing `/usr/local/bin/xray` is kept.
3. Writes the server configuration and has Xray test it (`xray run -test`) before anything goes live.
4. Creates and starts the service `rovunet-core-<name>` and opens the port in the firewall.
5. Writes one client file per device to `/etc/rovunet/core/<name>/clients/`.

If any step fails, everything created is removed again.

## Managing cores

```bash
sudo rovunet core-list            # name, protocol, port
sudo rovunet tunnel status        # cores appear with their service state
sudo rovunet core-remove r1       # stop and delete
```

`core-plan` takes the same options plus `--output <new directory>` and only writes the files, for manual deployment.

## Client files

Each device gets distinct credentials: a `vless://` or `hysteria2://` link, or a WireGuard `.conf` file. They import into the Rovunet app and into standard Xray-based and WireGuard clients. Never use one WireGuard file on two devices at the same time.

## Backups

Cores are included in `rovunet backup` and started again by `restore`. Certificates given with `--cert`/`--key` live outside the panel and must exist at the same paths on the new server, or that core is reported as not started.

## Limitations

- Core profiles are imported as separate profiles, not into the user's collection.
- Cores are independent of panel users: removing a user does not revoke a core credential; remove or reinstall the core.

Next: [Operations](operations.md)
