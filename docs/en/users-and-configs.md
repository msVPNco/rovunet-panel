# Users and client configurations

[Documentation](README.md) › Users and client configurations

## User model

Users are **global**: one username and password work on every tunnel. The protocol belongs to the tunnel, not the user. Every user can authenticate on every tunnel at the same time.

> **Never hand out `root` or any account with a shell as a VPN login.** Panel users are forwarding-only accounts; a leaked one loses tunnel access, a leaked `root` loses the server. Create a separate panel user for everyone, yourself included.

## Managing users

```bash
sudo rovunet users add                              # one user
sudo rovunet users bulk_add --count=50 --prefix=user  # user001…user050, random passwords, up to 500
sudo rovunet users list                             # users and their configurations
sudo rovunet users remove                           # remove a user
```

Give each person their own user, so a lost phone can be revoked alone.

## Getting configurations

| Command | Output |
|---|---|
| `sudo rovunet users configs` | Every configuration of every user on every tunnel |
| `sudo rovunet tunnel share <tag>` | One tunnel's configuration; asks which user's credentials to embed |
| `sudo rovunet tunnel share <tag> --user alice` | VLESS: alice's `rovunet://` and `vless://` links |
| `sudo rovunet export-clients --user alice --format rovunet` | Script-friendly list of alice's links; `--annotated` labels each with its tunnel |

For DNSTT tunnels, sharing asks for a **DNSTT** or **NoizDNS** profile. Both use the same server; NoizDNS turns on DPI evasion in the client (harder to fingerprint, slower).

A configuration contains the user's credentials. Send it over a private channel, never in a public group.

## Export formats

| Format | Scheme | Used by |
|---|---|---|
| Native | `rovunet://` | The Rovunet app. Carries the full profile, the user's collection (group) and, since v33, credentials losslessly, including characters such as `\|` |
| Standard VLESS | `vless://` | Any VLESS client (v2rayNG, Hiddify, NekoBox…), printed next to every VLESS configuration |
| Xray core | `rovunet://` (type `xray`), or `vless://`, `hysteria2://`, WireGuard `.conf` | One per user on every core; see [Xray cores](xray-cores.md) |
| Legacy | `slipnet://` | Older app versions; still accepted |

### Collections in the app

The app groups imported configurations by server and user, so all of one user's paths form one collection. Soar with the direct backend needs no login but is printed once per user, so it joins each user's collection. Xray core profiles are printed per user in the same collection.

## Server address

SSH over TLS on its own port, SSH and SOCKS5 configurations carry the server's IP. On cloud servers (Oracle, AWS, GCP, Azure) the network interface only has a private IP, so the panel looks for the public one: the server's own IP if public, otherwise the IP of `ns.<domain>` or a NaïveProxy domain. If none is found it asks once and saves the answer.

```bash
sudo rovunet config address 203.0.113.7   # an IP or hostname, without a port
sudo rovunet config address auto          # detect again
```

Configurations for DNS tunnels, Soar, NaïveProxy, VLESS and fronted SSH over TLS connect by domain and do not depend on it.

## Client apps

- **[Rovunet for Android](https://github.com/msVPNco/rovunet-android)** (recommended): imports `rovunet://` links, groups each user's paths into one collection, connects a chosen path without a prerequisite ping and offers separate HTTPS tests through the real tunnel.
- **Standard clients** such as v2rayNG, Hiddify or NekoBox: use the `vless://` links and the Xray core files.

Next: [Xray cores](xray-cores.md)
