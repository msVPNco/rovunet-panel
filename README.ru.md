# Rovunet Panel 2

[English — complete reference](README.md) · [فارسی](README.fa.md) · [Release notes](docs/releases/v2.0.0.md)

Полная установка создаёт 15 ядер и базовые службы. Безлимитный аккаунт получает 30 профилей, аккаунт только со сроком — 29, с лимитом трафика — 6. Основные протоколы идут первыми, десять DNS-профилей — последними. Это порядок отображения, а не измерение скорости. 25 GB = 25 600 MB = 26 843 545 600 байт. Обновление сохраняет адрес подписки и записанный расход; обновите существующую подписку в приложении. Неизвестный расход не заменяется нулём. Проверьте домены, делегирование DNS и облачный межсетевой экран. Компоненты имеют отдельные лицензии. Тесты панели не гарантируют работу каждого приложения и провайдера.

## Install / Ubuntu 24.04, systemd, root

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v2.0.0/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="897108ce22f619bad23aecc6792283e4af01df0172b917b28e03082675fe76a6", "Installer checksum mismatch"'
sudo env ROVUNET_RELEASE_TAG=v2.0.0 bash rovunet-install.sh
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
