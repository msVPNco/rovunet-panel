# Начало работы

[Документация](README.md) › Начало работы

## 1. Требования

| Пункт | Требование |
|---|---|
| Операционная система | Linux с systemd: Ubuntu 20.04+, Debian 10+ или аналог |
| Доступ | root (или sudo) по SSH |
| Домен | Домен, DNS-записями которого вы управляете. Нужен для DNS-туннелей, NaïveProxy, VLESS и ядер Xray с TLS |
| Оборудование | 1 vCPU, 1 ГБ RAM и 20 ГБ диска достаточно для небольшой группы; пропускная способность важнее процессора |
| Порты | Зависят от включаемых протоколов; см. таблицу ниже |

| Порт | Используется |
|---|---|
| 53/udp и 53/tcp | DNS-туннели (DNSTT, NoizDNS, VayDNS, Slipstream, Soar) |
| 80/tcp и 443/tcp | NaïveProxy и всё, что идёт через него (VLESS, SSH over TLS) |
| 22/tcp | SSH |
| 1080/tcp | SOCKS5 (только открытые сети) |
| 2053/tcp (или 2083, 2087, 2096, 8443) | VLESS через Cloudflare |
| Любой свободный порт TCP или UDP | Ядра Xray |

## 2. Выбор сервера

- **Входящий UDP/53 должен быть разрешён** для DNS-туннелей. Некоторые провайдеры блокируют или ограничивают его; проверьте политику сети провайдера до покупки.
- Выбирайте регион рядом с пользователями, до которого их сети всё ещё доходят.
- Предпочтительны провайдеры без ограничений портов и с достаточной пропускной способностью.

## 3. DNS-записи

Создайте записи для нужных протоколов **до** установки. Все записи — **DNS only** (серое облако в Cloudflare): Caddy должен быть доступен напрямую, чтобы получить сертификат Let's Encrypt, а DNS-туннелям нужен «сырой» DNS на сервере.

```text
A   ns.example.com   → SERVER_IP          DNS tunnels: the name server
NS  t.example.com    → ns.example.com     DNSTT / NoizDNS, SOCKS5 backend
NS  ts.example.com   → ns.example.com     DNSTT / NoizDNS, SSH backend
NS  v.example.com    → ns.example.com     VayDNS, SOCKS5 backend
NS  vs.example.com   → ns.example.com     VayDNS, SSH backend
NS  s.example.com    → ns.example.com     Slipstream, SOCKS5 backend
NS  ss.example.com   → ns.example.com     Slipstream, SSH backend
NS  r.example.com    → ns.example.com     Soar, direct backend
NS  rs.example.com   → ns.example.com     Soar, SSH backend
A   example.com      → SERVER_IP          NaïveProxy, and VLESS / SSH over TLS fronted by it
```

Минимальные варианты:

- **Только NaïveProxy:** `A example.com → SERVER_IP`.
- **Только DNSTT:** `A ns.example.com → SERVER_IP` и `NS t.example.com → ns.example.com`.

Запись `NS` делегирует поддомен вашему серверу: запросы к `t.example.com` уходят на `ns.example.com`, то есть на ваш сервер, и туннель отвечает на них на порту 53. Изменения распространяются от нескольких минут до нескольких часов. Проверка:

```bash
dig +short A ns.example.com
dig +short NS t.example.com
```

Единственное исключение — VLESS через Cloudflare, ему нужна **проксируемая** запись; см. [Протоколы › VLESS поверх WebSocket](protocols.md#vless-over-websocket).

## 4. Установка

### Онлайн (рекомендуется)

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

Установщик спрашивает, какие протоколы включить, загружает нужные бинарные файлы, создаёт ключи, службы systemd и правила брандмауэра, создаёт первого пользователя и печатает все его конфигурации.

Повторный запуск установщика на настроенном сервере сохраняет существующие туннели, добавляет только новые протоколы и снова печатает все конфигурации. Чтобы добавить второй туннель уже имеющегося типа, используйте `sudo rovunet tunnel add`.

Если NaïveProxy и VLESS выбраны вместе, VLESS (и SSH over TLS, если он выбран) сразу работают через NaïveProxy на порту 443.

### Офлайн (сервер не видит GitHub)

Скачайте панель из [последнего выпуска](https://github.com/msVPNco/rovunet-panel/releases) этого проекта, а транспортные бинарные файлы — из [исходного проекта](https://github.com/anonvector/slipgate/releases), и скопируйте их на сервер:

```bash
# On your computer
mkdir rovunet-bundle && cd rovunet-bundle
curl -LO https://github.com/msVPNco/rovunet-panel/releases/latest/download/rovunet-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/dnstt-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/slipstream-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/vaydns-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/caddy-naive-linux-amd64
scp * root@SERVER_IP:/tmp/rovunet/

# On the server
chmod +x /tmp/rovunet/*
sudo cp /tmp/rovunet/rovunet-linux-amd64 /usr/local/bin/rovunet
sudo rovunet install --bin-dir /tmp/rovunet
```

На ARM-серверах используйте `arm64` вместо `amd64`.

### Из исходного кода

```bash
git clone https://github.com/msVPNco/rovunet-panel.git
cd rovunet-panel
make build
sudo ./rovunet install
```

### Переход с SlipGate или старой версии панели

Сервер, установленный под прежним именем панели (`slipgate`, `/etc/slipgate`), переносится на месте; см. [Эксплуатация › Обновление](operations.md#updating).

## 5. Интерактивное меню и командная строка

- `sudo rovunet` без аргументов открывает интерактивное меню — удобнее всего для первой настройки.
- Каждое действие меню — также команда с параметрами для скриптов; см. [справочник команд](commands.md). Недостающий обязательный параметр запрашивается интерактивно.

## 6. Первый туннель и первый пользователь

```bash
sudo rovunet tunnel add --transport dnstt --backend socks --tag mytunnel --domain t.example.com
sudo rovunet users add
sudo rovunet users configs
sudo rovunet tunnel status
```

`tunnel add` создаёт ключи туннеля и службу `rovunet-mytunnel` (запускается сейчас и при загрузке) и регистрирует домен в DNS-маршрутизаторе. `users configs` печатает все конфигурации всех пользователей. Если туннель не `active`, выполните `sudo rovunet diag`.

Далее: [Протоколы](protocols.md)
