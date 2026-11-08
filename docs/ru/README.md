# Документация Rovunet Panel

[English](../en/README.md) · [فارسی](../fa/README.md) · **Русский** · [简体中文](../zh-CN/README.md)

Rovunet Panel превращает Linux-сервер в многопротокольный сервер доступа. Панель устанавливает каждый протокол как службу systemd и управляет им, создаёт пользователей и печатает для каждого из них готовые к импорту клиентские конфигурации.

## Содержание

| # | Страница | О чём |
|---|---|---|
| 1 | [Начало работы](getting-started.md) | Требования, выбор сервера, DNS-записи, установка, первый туннель |
| 2 | [Протоколы](protocols.md) | Все протоколы, способ передачи, порты, схема доменов, маршрутизация |
| 3 | [Пользователи и клиентские конфигурации](users-and-configs.md) | Модель пользователей, выдача конфигураций, форматы экспорта, клиенты |
| 4 | [Ядра Xray](xray-cores.md) | VLESS + REALITY, VLESS + XHTTP, Hysteria 2 и WireGuard |
| 5 | [Эксплуатация](operations.md) | Обновление, резервное копирование и восстановление, мониторинг, WARP, файлы и службы, удаление |
| 6 | [Справочник команд](commands.md) | Все команды и их параметры |
| 7 | [Устранение неполадок](troubleshooting.md) | Симптомы, причины и решения |
| 8 | [Лимиты и ссылки подписки](limits-and-subscriptions.md) | Безлимитные и ограниченные пользователи, ссылки подписки, дополнительные адреса |

## Соглашения

- Команды выполняются на сервере от root, поэтому показаны с `sudo`.
- `example.com` — ваш домен, `SERVER_IP` — публичный IP-адрес сервера.
- Названия протоколов соответствуют исходным проектам: VLESS, REALITY и XHTTP (Project X), Hysteria 2, WireGuard, NaïveProxy, SSH и SOCKS5. DNSTT, NoizDNS, VayDNS, Slipstream и Soar — DNS-туннели.
- В панели **туннель** — один настроенный экземпляр протокола с тегом (именем), а **ядро** — один сервер Xray, установленный командой `core-install`.

## Проект

- Исходный код: [github.com/msVPNco/rovunet-panel](https://github.com/msVPNco/rovunet-panel) · [Журнал изменений](../../CHANGELOG.md) · [Состояние интеграции](../ROVUNET-STATUS.md)
- Приложение для Android: [github.com/msVPNco/rovunet-app](https://github.com/msVPNco/rovunet-app)
- Сообщество: [t.me/RovunetVPN](https://t.me/RovunetVPN)
- Основано на [SlipGate](https://github.com/anonvector/slipgate) автора Anonvector, официально не связано с ним; см. [NOTICE](../../NOTICE.md). Лицензия: AGPL-3.0.
