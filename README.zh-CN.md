# Rovunet Panel 2

[English — complete reference](README.md) · [فارسی](README.fa.md) · [Release notes](docs/releases/v2.0.1.md)

完整安装配置 15 个核心及基础服务。无限制账户获得 30 个配置；仅限制时间的账户获得 29 个；限制流量的账户获得 6 个可计量路径。主协议组在前，全部 10 个 DNS 路径在最后；顺序并非实测速率排名。25 GB = 25,600 MB = 26,843,545,600 字节。正常更新保留订阅地址及已记录流量，在应用中刷新原订阅即可。未知流量不会按零处理。需要正确的域名、DNS 委派及云防火墙端口。组件遵循各自许可证；面板测试不保证每种应用或网络的兼容性。

## Install / Ubuntu 24.04, systemd, root

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v2.0.1/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="897108ce22f619bad23aecc6792283e4af01df0172b917b28e03082675fe76a6", "Installer checksum mismatch"'
sudo env ROVUNET_RELEASE_TAG=v2.0.1 bash rovunet-install.sh
```

## Profiles

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

## Commands

```bash
sudo rovunet                         # interactive menu
sudo rovunet users add               # account creation, time and traffic limits
sudo rovunet subscriptions-refresh   # regenerate saved URLs
sudo rovunet certificates --mode check
sudo rovunet certificates --mode recover --tag slipstream-socks
sudo rovunet diag
sudo rovunet backup --file /root/rovunet-backup.age --recipient age1YOUR_PUBLIC_RECIPIENT
```

See the English reference for encrypted backup identities, certificate recovery, authenticated Slipstream certificate pins, update journals, compatibility limitations and security qualification scope.

[Panel2 status, certificate and session contract](docs/contracts/panel-v2-addendum.md) — strict feed1; opt-in status2; no device-session API.
