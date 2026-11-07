# Протоколы

[Документация](README.md) › Протоколы

## Обзор

| Протокол | Семейство | Передаётся через | Порт по умолчанию | Сертификат | Транспорт в панели |
|---|---|---|---|---|---|
| [VLESS поверх WebSocket](#vless-over-websocket) | Прокси | WebSocket + TLS | 443/tcp через Naïve или 2053/tcp | Let's Encrypt (через Naïve) или самоподписанный | `vless` |
| [NaïveProxy](#naiveproxy) | Прокси | HTTP/2 + TLS | 443/tcp | Let's Encrypt | `naive` |
| [SSH over TLS](#ssh-over-tls) (StunTLS) | SSH | TLS / WebSocket | 443/tcp через Naïve или 8443/tcp | Самоподписанный или Let's Encrypt через Naïve | `stuntls` |
| [SSH](#ssh-and-socks5) | SSH | TCP | 22/tcp | — | `direct-ssh` |
| [SOCKS5](#ssh-and-socks5) | Прокси | TCP без шифрования | 1080/tcp | — | `direct-socks5` |
| [DNSTT](#dns-tunnels) / NoizDNS | DNS-туннель | DNS, Curve25519 | 53/udp + 53/tcp | — | `dnstt` |
| [VayDNS](#dns-tunnels) | DNS-туннель | DNS, KCP + Curve25519 | 53/udp + 53/tcp | — | `vaydns` |
| [Slipstream](#dns-tunnels) | DNS-туннель | QUIC поверх DNS | 53/udp + 53/tcp | Самоподписанный | `slipstream` |
| [Soar](#dns-tunnels) | DNS-туннель | Многопутевой DNS, X25519 + Ed25519 | 53/udp + 53/tcp | — | `soar` |
| [VLESS + REALITY](xray-cores.md) | Ядро Xray | TCP, REALITY + XTLS Vision | любой свободный TCP-порт | Не нужен (имитирует настоящий сайт) | `core-install` |
| [VLESS + XHTTP](xray-cores.md) | Ядро Xray | HTTP-запросы + TLS | любой свободный TCP-порт | Нужен | `core-install` |
| [Hysteria 2](xray-cores.md) | Ядро Xray | QUIC | любой свободный UDP-порт | Нужен | `core-install` |
| [WireGuard](xray-cores.md) | Ядро Xray | UDP | любой свободный UDP-порт | — | `core-install` |
| [Внешний DNS-маршрут](#external-dns-route) | Свой | DNS на локальный порт | 53/udp | — | `external` |

## Выбор протокола

Полевые результаты с одного сервера из Ирана, осень 2026 года. Это отправная точка, а не гарантия: сети различаются и меняются.

| Ситуация | Что попробовать первым |
|---|---|
| HTTPS работает | VLESS через NaïveProxy на 443: быстро, стабильно, выглядит как обычный сайт |
| Открыт порт 22 | SSH: быстро |
| IP сервера заблокирован | VLESS через Cloudflare: работает, но заметно медленнее |
| TLS до известных сайтов проходит | VLESS + REALITY |
| Сети с потерями, где проходит UDP | Hysteria 2 |
| Проходит только DNS | VayDNS или DNSTT: медленно, но работают |
| SSH over TLS на своём порту сбрасывается | Пустите его через NaïveProxy на 443 |
| Только открытые сети | SOCKS5: без шифрования, фильтруемые сети его обрывают |

## Бэкенды

DNS-туннели и NaïveProxy передают расшифрованный трафик бэкенду:

| Бэкенд | Поведение |
|---|---|
| `socks` | Встроенный SOCKS5-прокси панели. Просто и быстро |
| `ssh` | Трафик передаёт OpenSSH сервера, поэтому каждому пользователю нужен вход по SSH |
| `both` | Создаёт два туннеля, `<tag>-socks` и `<tag>-ssh`; поддомен SSH-варианта получает дополнительную `s` |
| `direct` | Только Soar: сервер сам подключается к цели каждого потока |

<a id="vless-over-websocket"></a>
## VLESS поверх WebSocket

Панель сама обслуживает VLESS поверх WebSocket. Каждый пользователь получает UUID, выведенный из имени, пароля и общего секрета сервера (`/etc/rovunet/vless.secret`): удаление пользователя или смена пароля отзывает его за секунды, а выданная конфигурация никогда не содержит пароль. Потоки идут только на публичные адреса; порт 25 по умолчанию закрыт (`--block-ports`). Поддерживаются TCP и простой UDP, Mux и XUDP — нет.

Три способа подключения:

| Режим | Как | Когда |
|---|---|---|
| **Через NaïveProxy** (рекомендуется) | Caddy несёт его на 443 со своим сертификатом Let's Encrypt; остальные пути показывают сайт-приманку | По умолчанию, если установлен NaïveProxy |
| **Через Cloudflare** | Клиенты подключаются к IP Cloudflare; IP сервера скрыт | Когда IP сервера заблокирован |
| **Напрямую** | Собственный порт с самоподписанным сертификатом | Тестирование |

```bash
# Fronted by NaïveProxy
sudo rovunet tunnel add --transport vless --tag myvless --domain example.com --cdn no --listen-port 2053
sudo rovunet tunnel edit --tag myvless --front myproxy      # "-" undoes it

# Behind Cloudflare
sudo rovunet tunnel add --transport vless --tag myvless-cf --domain cdn.example.com --cdn yes --listen-port 2053
sudo rovunet tunnel edit --tag myvless-cf --cdn-address 104.16.0.1   # clean Cloudflare IP, "-" resets
```

Настройки Cloudflare для проксируемого имени:

1. DNS: `A cdn.example.com → SERVER_IP`, **Proxied** (оранжевое облако).
2. SSL/TLS › Overview: **Full**. Flexible не работает, а Full (strict) отвергает самоподписанный сертификат сервера.
3. Network: включите **WebSockets**.
4. Порт: Cloudflare проксирует HTTPS только на 443, 2053, 2083, 2087, 2096 и 8443; если NaïveProxy занимает 443, берите 2053.

Конфигурации выдаются каждому пользователю отдельно: `sudo rovunet tunnel share myvless --user alice` печатает ссылку `rovunet://` для приложения Rovunet и стандартную ссылку `vless://` для v2rayNG, Hiddify, NekoBox и подобных клиентов. Ссылкам через Naïve не нужен `allowInsecure`, который клиенты Xray перестают принимать с августа 2026 года. Фрагментация SNI в обоих типах ссылок выключена и включается в приложении.

<a id="naiveproxy"></a>
## NaïveProxy

Caddy с плагином forwardproxy. Caddy получает и продлевает сертификат Let's Encrypt (порты 80 и 443 должны быть открыты) и показывает сайт-приманку всему, что не является аутентифицированным прокси-запросом. Войти может любой пользователь панели; `users add`, `remove` и `passwd` обновляют Caddy на лету. Туннель Naïve без единого логина получает случайный и остаётся закрытым.

```bash
sudo rovunet tunnel add --transport naive --backend socks --tag myproxy \
  --domain example.com --email admin@example.com --decoy-url https://www.wikipedia.org
```

### Работа через 443

Caddy от NaïveProxy может нести VLESS и SSH over TLS на том же домене, порту и сертификате. Caddy маршрутизирует каждый запрос: секретный путь WebSocket — к VLESS или SSH over TLS, аутентифицированный прокси-запрос — к NaïveProxy, всё остальное — к сайту-приманке. Каждое изменение Caddyfile перед применением проверяется командой `caddy-naive validate`; если проверка не проходит, остаётся прежний файл.

```bash
sudo rovunet tunnel edit --tag myvless --front naive
sudo rovunet tunnel edit --tag mytls --front naive
sudo rovunet users configs        # hand out the new configurations
```

<a id="ssh-over-tls"></a>
## SSH over TLS (StunTLS)

SSH внутри TLS и WebSocket, обслуживается панелью. Автоматически распознаёт режимы WebSocket, HTTP CONNECT, «сырого» TLS и payload. На собственном порту (8443) использует самоподписанный сертификат и не требует домена; через NaïveProxy клиенты подключаются к нему как к SSH поверх WebSocket на домене Naïve, порт 443, с настоящим сертификатом. Если приложение сообщает `Connection was reset` на собственном порту, сеть блокирует этот порт — пустите туннель через Naïve.

```bash
sudo rovunet tunnel add --transport stuntls --tag mytls
sudo rovunet tunnel edit --tag mytls --front myproxy
```

<a id="ssh-and-socks5"></a>
## SSH и SOCKS5

```bash
sudo rovunet tunnel add --transport direct-ssh --tag myssh
sudo rovunet tunnel add --transport direct-socks5 --tag mysocks
```

SSH использует OpenSSH сервера на порту 22; пользователи туннеля входят по паролю без командной оболочки (только перенаправление). SOCKS5 слушает 1080 и не шифрует трафик. SOCKS5-прокси без пользователей отклоняет подключения, а не становится открытым прокси.

<a id="dns-tunnels"></a>
## DNS-туннели

| Туннель | Описание |
|---|---|
| **DNSTT** | DNS-туннель с шифрованием Curve25519. Тот же сервер обслуживает клиентов **NoizDNS**, которые добавляют обход DPI (кодирование base36/hex, удаление префикса CDN) |
| **VayDNS** | DNS-туннель на основе KCP с Curve25519 и настраиваемым форматом: тип записи, idle timeout, keep-alive, размер client ID, размер очереди |
| **Slipstream** | QUIC поверх DNS с аутентификацией по сертификату. Экспериментальный |
| **Soar** | Многопутевой DNS-туннель ([Lantern](https://github.com/getlantern/soar)) с поиском резолверов, встроен в панель. Бэкенд **direct**: без входа, сервер подключается к публичным адресам на разрешённых портах (по умолчанию `80,443`), совместим со стандартными клиентами Soar. Бэкенд **SSH**: доходит только до SSH-сервера, доступ — через пользователей SSH |

```bash
sudo rovunet tunnel add --transport dnstt  --backend socks  --tag mydnstt  --domain t.example.com
sudo rovunet tunnel add --transport vaydns --backend both   --tag myvaydns --domain v.example.com \
  --record-type txt --idle-timeout 10s --keep-alive 2s --clientid-size 2 --queue-size 512
sudo rovunet tunnel add --transport slipstream --backend ssh --tag myslip --domain s.example.com
sudo rovunet tunnel add --transport soar --backend direct --tag mysoar --domain r.example.com --allow-ports 80,443
sudo rovunet tunnel add --transport soar --backend ssh    --tag mysoar-ssh --domain rs.example.com
```

DNSTT и VayDNS принимают существующие ключи через `--private-key` (и необязательный `--public-key`); Soar принимает seed Ed25519 в base64 через `--private-key`. `sudo rovunet mtu 1200` задаёт MTU сразу всем туннелям DNSTT, NoizDNS и VayDNS (по умолчанию 1232).

<a id="external-dns-route"></a>
## Внешний DNS-маршрут

Перенаправляет DNS-запросы для домена на локальный UDP-порт — для протокола, который вы запускаете сами. Службой для него панель не управляет.

```bash
sudo rovunet tunnel add --transport external --tag my-proto --domain j.example.com --port 5301
```

## Схема доменов

| Туннель | Домен | Бэкенд |
|---|---|---|
| dnstt-socks / dnstt-ssh | `t.example.com` / `ts.example.com` | SOCKS5 / SSH |
| vaydns-socks / vaydns-ssh | `v.example.com` / `vs.example.com` | SOCKS5 / SSH |
| slipstream-socks / slipstream-ssh | `s.example.com` / `ss.example.com` | SOCKS5 / SSH |
| soar-direct / soar-ssh | `r.example.com` / `rs.example.com` | direct / SSH |
| naive-socks / naive-ssh | `example.com` (общий) | SOCKS5 / SSH |
| vless, stuntls | `example.com` через NaïveProxy | UUID пользователей / SSH |

Каждому DNS-туннелю нужен свой поддомен; DNSTT и NoizDNS делят один. Туннели NaïveProxy делят домен, потому что работают по HTTPS, а VLESS и SSH over TLS идут через него и не требуют своих записей.

## Режимы маршрутизации

| Режим | Поведение |
|---|---|
| **single** | Активен один DNS-туннель; DNS-маршрутизатор на порту 53 передаёт запросы ему |
| **multi** | Все DNS-туннели работают на локальных портах; маршрутизатор распределяет запросы по домену, всегда к наиболее точному совпадению. Включается автоматически со вторым DNS-туннелем |

Маршрутизатор отвечает на UDP и TCP порт 53. Запросы по TCP (RFC 7766, конвейерные) передаются туннелю по UDP, а ответы возвращаются в то же соединение, поэтому все DNS-туннели работают и для резолверов и клиентов, использующих TCP/53.

## Архитектура

```text
                                 Rovunet client
            ┌──────────────────────────┼─────────────────────────────┐
     DNS :53 udp+tcp             HTTPS :443 tcp              Cloudflare :2053 (optional)
            │                          │                             │
┌───────────┼──────────────────────────┼─────────────────────────────┼──────────┐
│ server    ▼                          ▼                             │          │
│  ┌───────────────────┐   ┌─────────────────────────────────┐      │          │
│  │    DNS router     │   │ Caddy (NaïveProxy) :443         │      │          │
│  │ dispatch by domain│   │ Let's Encrypt + decoy website   │      │          │
│  └──┬────┬────┬───┬──┘   └────┬──────────────┬───────────┬─┘      │          │
│     ▼    ▼    ▼   ▼           ▼              ▼           ▼        ▼          │
│  DNSTT VayDNS Slip Soar   forward proxy  SSH over TLS   VLESS over WebSocket │
│     └────┴────┴───┘           │              │           │                   │
│              ▼                ▼              ▼           │                   │
│   ┌───────────────────────────────────────────────┐      │   Xray cores      │
│   │  Backend: SOCKS5 :1080 · SSH forwarding :22  │      │   (own ports)     │
│   └───────────────────────┬───────────────────────┘      │        │          │
│                     WARP (optional)                      │        │          │
│                           └─────────────┬────────────────┴────────┘          │
│                                      Internet                                │
└───────────────────────────────────────────────────────────────────────────────┘
```

Soar с бэкендом direct, VLESS и ядра Xray сами подключаются к цели, поэтому их трафик минует слой бэкендов и WARP.

Далее: [Пользователи и клиентские конфигурации](users-and-configs.md)
