# Command reference

[Documentation](README.md) › Command reference

Run every command as root (`sudo rovunet …`). A required option that is not given is asked for interactively; `rovunet` with no arguments opens the menu.

## System

| Command | Description |
|---|---|
| `rovunet` | Interactive menu |
| `rovunet install [--bin-dir DIR]` | Install dependencies and set up the selected protocols |
| `rovunet update` | Update to the latest release and regenerate services |
| `rovunet restart` | Restart the DNS router, SOCKS5 proxy and every tunnel |
| `rovunet stats` | Live dashboard |
| `rovunet diag` | Diagnostics |
| `rovunet mtu VALUE` | Set the MTU of every DNSTT, NoizDNS and VayDNS tunnel |
| `rovunet backup [FILE \| -]` | Back up configuration, keys, users and Xray cores |
| `rovunet restore [FILE \| -] [--yes]` | Restore a backup; `-` reads standard input |
| `rovunet uninstall` | Remove everything the panel installed |
| `rovunet --version` | Print the version |

## Users

| Command | Description |
|---|---|
| `rovunet users` | User menu |
| `rovunet users add` | Add a user |
| `rovunet users bulk_add --count N --prefix P` | Add up to 500 users with random passwords |
| `rovunet users remove` | Remove a user |
| `rovunet users list` | Users and their configurations |
| `rovunet users configs` | Every configuration of every user |
| `rovunet export-clients --user U --format rovunet [--annotated]` | One user's links for scripts |

## Tunnels

| Command | Description |
|---|---|
| `rovunet tunnel add` | Add a tunnel (options below) |
| `rovunet tunnel edit --tag T` | Change a tunnel (options below) |
| `rovunet tunnel remove T` / `--all` | Remove one tunnel or all |
| `rovunet tunnel start T` / `stop T` | Start or stop a tunnel |
| `rovunet tunnel status [T]` | All tunnels and cores, or one tunnel's details |
| `rovunet tunnel share T [--user U]` | Print a tunnel's configuration |
| `rovunet tunnel logs T` | A tunnel's service log |

### `tunnel add` options

| Option | Applies to | Meaning |
|---|---|---|
| `--transport` | all | `dnstt`, `vaydns`, `slipstream`, `soar`, `naive`, `stuntls`, `vless`, `direct-ssh`, `direct-socks5`, `external` |
| `--backend` | DNS tunnels, NaïveProxy | `socks`, `ssh`, `both`; Soar: `direct` or `ssh` |
| `--tag` | all | Tunnel name |
| `--domain` | all with a domain | Tunnel domain or subdomain |
| `--private-key`, `--public-key` | DNSTT, VayDNS, Soar | Import existing keys |
| `--record-type`, `--idle-timeout`, `--keep-alive`, `--clientid-size`, `--queue-size` | VayDNS | Wire-format tuning |
| `--allow-ports` | Soar direct | Allowed destination ports, e.g. `80,443` or `any` |
| `--email`, `--decoy-url` | NaïveProxy | Let's Encrypt e-mail, decoy website |
| `--cdn`, `--tls`, `--listen-port`, `--cdn-address`, `--block-ports` | VLESS | Cloudflare mode, TLS, listen port, clean CDN IP, blocked destination ports |
| `--port` | External | Local UDP port to forward to |

### `tunnel edit` options

| Option | Meaning |
|---|---|
| `--new-tag` | Rename the tunnel |
| `--mtu` | DNS tunnel MTU |
| `--front NAIVE_TAG \| -` | Carry VLESS or SSH over TLS on NaïveProxy's 443, or undo it |
| `--cdn-address IP \| -` | Clean Cloudflare IP for future VLESS configurations |
| VayDNS tuning options | As for `tunnel add` |

## Xray cores

| Command | Description |
|---|---|
| `rovunet core-install --name N --protocol P --host H [options]` | Install a core serving every user; see [Xray cores](xray-cores.md#installing-a-core) |
| `rovunet core-list` | Installed cores |
| `rovunet core-remove N` | Remove a core |
| `rovunet core-plan --protocol P --host H --output DIR [options]` | Write a core's files only |

## DNS routing

| Command | Description |
|---|---|
| `rovunet router status` | Routing mode and routes |
| `rovunet router mode` | Switch between single and multi mode |
| `rovunet router switch` | Change the active tunnel in single mode |

## Configuration

| Command | Description |
|---|---|
| `rovunet config export` | Print `config.json` without keys |
| `rovunet config import PATH` | Replace `config.json` (no keys) |
| `rovunet config address IP \| auto` | Public address for SSH, SOCKS5 and SSH over TLS configurations |

## Internal

Used by the systemd services: `rovunet dnsrouter serve`, `rovunet socks serve`, `rovunet stuntls serve`, `rovunet vless serve`, `rovunet post-update`.

Next: [Troubleshooting](troubleshooting.md)
