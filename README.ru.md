# Rovunet Panel

[English](README.md) · [فارسی](README.fa.md) · [简体中文](README.zh-CN.md) · Русский

**Открытая панель для самостоятельного развёртывания многопротокольного доступа на совместимом Linux-сервере.** Объединяет установку служб, создание пользователей, экспорт конфигураций, systemd, диагностику и резервные копии. Несколько путей уменьшают зависимость от одного протокола, но не гарантируют доступ в любой сети.

Управляемые пути включают VLESS WebSocket, NaiveProxy, прямой SSH, SSH over TLS/STunTLS, SOCKS5, DNS-семейства DNSTT, NoizDNS, VayDNS, Slipstream и Soar. Поддержка клиента шире автоустановки: REALITY/XHTTP/Hysteria2/WireGuard через core-plan могут требовать ручного развёртывания. [Состояние интеграции](docs/ROVUNET-STATUS.md).

## Установка

На новом Ubuntu 20.04+ или Debian 10+ с правами root:

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

Нужны собственный сервер, домен с управляемым DNS и открытые порты выбранных служб. Установщик спрашивает протоколы, настраивает их, создаёт пользователя и печатает конфигурации. Для существующего SlipGate сначала прочитайте руководство миграции и сделайте копию; не экспериментируйте без возможности восстановления сервера.

```bash
sudo slipgate
sudo slipgate users add
sudo slipgate users configs
sudo slipgate diag
sudo slipgate backup
sudo slipgate export-clients --user USER --format rovunet
```

## Клиенты и диагностика

[Rovunet Android](https://github.com/msVPNco/rovunet-android) импортирует rovunet://, сохраняет метаданные коллекции, подключает выбранный путь напрямую и отдельно проверяет HTTPS через туннель; DNS-этап теста требует согласия. Другие совместимые клиенты используют стандартные vless://. [Публичные выпуски](https://github.com/msVPNco/rovunet-android/releases) · [Состояние и подпись](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md). Приватный пакет не обновляет публичный на месте, неподписанная заготовка не является установщиком.

Успех SOCKS CONNECT не доказывает передачу данных: проверяйте аутентификацию, REP, DNS и полезные данные отдельно. Работа базового Soar не гарантирует разрешённые SSH-адрес/порт; проверьте реальные политики сервера. DNS-ошибка не доказывает цензуру. Планируйте порты Caddy/REALITY TCP и HTTP/3/Hysteria2 UDP без конфликта.

## Документация и поддержка

[Полное руководство](docs/GUIDE.md) · [Пользователю](docs/USER_GUIDE.md) · [Изменения](CHANGELOG.md) · [Telegram](https://t.me/RovunetVPN). До применения проверьте нужные протоколы и выполните slipgate backup; нагрузку, ресурсы и восстановление проверяйте в своей среде.

Добровольная помощь разработке: **USDT, только Tron/TRC20**, `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ`. Пожертвование не требуется для использования.

## Лицензия

AGPL-3.0, [NOTICE](NOTICE.md). На основе [SlipGate](https://github.com/anonvector/slipgate) автора Anonvector; независимый проект без официальной связи. Команда slipgate сохранена для совместимости.
