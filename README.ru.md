<p align="center">
  <img src="assets/rovunet-logo.png" alt="Rovunet" width="180">
</p>

<h1 align="center">Rovunet Panel</h1>

<p align="center">
  Самостоятельно размещаемый многопротокольный сервер для обхода цензуры.<br>
  Одна установка, все управляемые протоколы, готовые клиентские конфигурации для каждого пользователя.
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.fa.md">فارسی</a> · <b>Русский</b> · <a href="README.zh-CN.md">简体中文</a>
  <br>
  <a href="docs/ru/README.md">Документация</a> · <a href="https://github.com/msVPNco/rovunet-android">Приложение для Android</a> · <a href="CHANGELOG.md">Изменения</a> · <a href="https://t.me/RovunetVPN">Telegram</a>
</p>

---

Rovunet Panel — открытый инструмент, превращающий Linux-сервер в многопротокольный сервер доступа. Он устанавливает каждый протокол как службу systemd и управляет им, создаёт пользователей и экспортирует их клиентские конфигурации, чтобы у каждого пользователя было несколько независимых путей подключения вместо зависимости от одного протокола или одного состояния сети.

## Возможности

- **14 протоколов**: прокси, SSH, DNS-туннели и ядра Xray — в одном инструменте командной строки с интерактивным меню.
- **Один логин на пользователя** для всех туннелей и личные идентификаторы VLESS, которые отзываются за секунды.
- **Готовые к импорту конфигурации**: собственные ссылки `rovunet://` для [приложения Rovunet](https://github.com/msVPNco/rovunet-android), стандартные `vless://`, `hysteria2://` и файлы WireGuard для других клиентов.
- **Работа через порт 443**: VLESS и SSH over TLS делят с NaïveProxy домен, порт и сертификат Let's Encrypt за сайтом-приманкой.
- **Безопасная эксплуатация**: обновления с проверкой контрольных сумм и откатом, проверка конфигураций до развёртывания, полное резервное копирование и восстановление сервера.
- **Диагностика**: панель в реальном времени, проверки состояния, сроки действия сертификатов и журналы служб.

## Протоколы

| Протокол | Как передаётся | Порт | Когда использовать |
|---|---|---|---|
| **VLESS over WebSocket** (через NaïveProxy) | WebSocket + TLS, настоящий сертификат | 443/tcp | Первый выбор, где работает HTTPS |
| **VLESS over WebSocket** (через Cloudflare) | WebSocket через CDN Cloudflare | 2053/tcp | Когда IP сервера заблокирован |
| **NaïveProxy** | HTTP/2 + TLS с сайтом-приманкой | 443/tcp | Где проходит TLS до вашего домена |
| **SSH over TLS** (StunTLS) | SSH внутри TLS / WebSocket | 443/tcp | Альтернатива на том же порту |
| **SSH** | Обычный SSH | 22/tcp | Где открыт порт 22 |
| **SOCKS5** | Обычный SOCKS5 без шифрования | 1080/tcp | Только открытые сети |
| **DNSTT** / **NoizDNS** | Зашифрованный DNS-туннель | 53/udp | Ограниченные сети, где проходит DNS |
| **VayDNS** | DNS-туннель с настраиваемым форматом | 53/udp | Как DNSTT, с большим числом настроек |
| **Slipstream** | QUIC поверх DNS | 53/udp | Экспериментальный |
| **Soar** | Многопутевой DNS с поиском резолверов | 53/udp | Неравномерно фильтруемые резолверы |
| **VLESS + REALITY** (Xray) | TCP, имитирует настоящий TLS-сайт | любой TCP | Где проходит TLS к известным сайтам |
| **VLESS + XHTTP** (Xray) | HTTP-запросы поверх TLS | любой TCP | За CDN или где заблокирован WebSocket |
| **Hysteria 2** (Xray) | QUIC | любой UDP | Сети с потерями, где проходит UDP |
| **WireGuard** (Xray) | UDP | любой UDP | Где WireGuard не заблокирован |

Пригодность зависит от сервера, оператора и сети; это рекомендации, а не гарантии. Подробнее — в разделе [Протоколы](docs/ru/protocols.md).

## Быстрый старт

На новом сервере с Ubuntu 20.04+ или Debian 10+, с правами root:

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

Установщик спрашивает, какие протоколы включить, настраивает их, создаёт первого пользователя и печатает его конфигурации. Затем:

```bash
sudo rovunet                 # interactive menu
sudo rovunet users add       # add a user
sudo rovunet users configs   # print every user's configurations
sudo rovunet diag            # check services, ports, keys and DNS
```

Требования: доступ root, домен, DNS которого вы контролируете, и открытые порты для включаемых протоколов (как минимум 53/udp, 80/tcp и 443/tcp). См. [Начало работы](docs/ru/getting-started.md).

## Документация

| | |
|---|---|
| [Начало работы](docs/ru/getting-started.md) | Требования, DNS-записи, установка, первый туннель |
| [Протоколы](docs/ru/protocols.md) | Все протоколы, порты, схема доменов, маршрутизация |
| [Пользователи и конфигурации](docs/ru/users-and-configs.md) | Пользователи, выдача, форматы экспорта, клиенты |
| [Ядра Xray](docs/ru/xray-cores.md) | VLESS + REALITY, VLESS + XHTTP, Hysteria 2, WireGuard |
| [Эксплуатация](docs/ru/operations.md) | Обновление, резервные копии, мониторинг, файлы и службы |
| [Справочник команд](docs/ru/commands.md) | Все команды и параметры |
| [Устранение неполадок](docs/ru/troubleshooting.md) | Симптомы, причины и решения |

> **Состояние:** ранний выпуск, активная разработка. Проверьте нужные протоколы в своей среде, прежде чем полагаться на них, и держите резервную копию (`sudo rovunet backup`). См. [состояние интеграции](docs/ROVUNET-STATUS.md).

## Поддержка

Rovunet — неоплачиваемая волонтёрская работа. Пожертвования приветствуются: **USDT в сети Tron (TRC20)** на адрес `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ` (отправляйте только USDT в сети TRC20). Вопросы и новости: [t.me/RovunetVPN](https://t.me/RovunetVPN).

## Лицензия

AGPL-3.0. Rovunet Panel основан на [SlipGate](https://github.com/anonvector/slipgate) автора Anonvector и официально с ним не связан; сведения об исходных и сторонних компонентах — в [NOTICE](NOTICE.md).
