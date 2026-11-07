# Troubleshooting

[Documentation](README.md) › Troubleshooting

Start with `sudo rovunet diag`, then the service log of the affected tunnel (`sudo rovunet tunnel logs <tag>`).

## Server

| Symptom | Likely cause | Fix |
|---|---|---|
| Tunnel is not `active` | Port, key or configuration problem | `sudo rovunet diag`, then `sudo rovunet tunnel logs <tag>` |
| Port 53 already in use | systemd-resolved's stub listener | The panel disables it; if it returns: `sudo systemctl disable --now systemd-resolved` and set `nameserver 1.1.1.1` in `/etc/resolv.conf` |
| `dig` against the server times out | The provider blocks inbound UDP/53 | Change provider, or use protocols on 443 |
| Let's Encrypt fails (NaïveProxy) | A record not propagated, or port 80 closed | Wait, open port 80, then `sudo rovunet restart` |
| Only one of several DNS tunnels works | Single routing mode | `sudo rovunet router status`; switch with `router mode` |
| A TCP port is refused at `tunnel add` | Another program listens on it | Choose another port or stop that program |
| Certificate flagged in `tunnel status` | Expired, expiring or unreadable | Renew or replace it, then restart the tunnel |
| After an update, a service shows the old name | Migration interrupted | Run `sudo rovunet post-update` again; it is safe to repeat |

## Client

| Symptom | Likely cause | Fix |
|---|---|---|
| Connects but no traffic | Network path or client issue | Test from the server: `curl -x socks5h://USER:PASS@SERVER_IP:1080 https://api.ipify.org`. If it works there, the server is fine |
| `Connection was reset` | Filtering resets the connection on that port | Check with `tcpdump -ni any tcp port 8443` whether packets arrive; if not, front the tunnel with NaïveProxy |
| User cannot log in | Wrong credentials or backend | `sudo rovunet users list`; check the tunnel's backend |
| VLESS connects but no site opens | User removed or password changed | Hand out a fresh configuration |
| "WebSocket upgrade failed" | Path in the link differs from the server | Hand out a fresh configuration |
| Cloudflare 521 / 522 | Cloudflare cannot reach the server | Open the port in the provider's firewall |
| Cloudflare 525 / 526 | Wrong SSL mode | Set SSL/TLS to **Full** |
| v2rayNG errors on VLESS | Mux or XUDP enabled | Turn Mux off; only plain UDP is supported |
| Slow DNS tunnel | MTU too large for the path | `sudo rovunet mtu 1200` (1100 for very poor links) |
| A key had to change | Old configurations use the old key | `sudo rovunet tunnel edit --tag <tag>`, then hand out new configurations |

## Logs

```bash
journalctl -u rovunet-<tag> -n 200 --no-pager
journalctl -u rovunet-dnsrouter -n 200 --no-pager
journalctl -u rovunet-socks5 -n 200 --no-pager
journalctl -u rovunet-core-<name> -n 200 --no-pager
```

Questions: [t.me/RovunetVPN](https://t.me/RovunetVPN)
