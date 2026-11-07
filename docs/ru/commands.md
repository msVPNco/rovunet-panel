# Справочник команд

[Документация](README.md) › Справочник команд

Выполняйте все команды от root (`sudo rovunet …`). Недостающий обязательный параметр запрашивается интерактивно; `rovunet` без аргументов открывает меню.

## Система

| Команда | Описание |
|---|---|
| `rovunet` | Интерактивное меню |
| `rovunet install [--bin-dir DIR]` | Установить зависимости и настроить выбранные протоколы |
| `rovunet update` | Обновиться до последнего выпуска и пересоздать службы |
| `rovunet restart` | Перезапустить DNS-маршрутизатор, SOCKS5-прокси и все туннели |
| `rovunet stats` | Панель в реальном времени |
| `rovunet diag` | Диагностика |
| `rovunet mtu VALUE` | Задать MTU всем туннелям DNSTT, NoizDNS и VayDNS |
| `rovunet backup [FILE \| -]` | Резервная копия конфигурации, ключей, пользователей и ядер Xray |
| `rovunet restore [FILE \| -] [--yes]` | Восстановить копию; `-` читает стандартный ввод |
| `rovunet uninstall` | Удалить всё, что установила панель |
| `rovunet --version` | Показать версию |

## Пользователи

| Команда | Описание |
|---|---|
| `rovunet users` | Меню пользователей |
| `rovunet users add` | Добавить пользователя (спрашивает безлимит или лимит, затем дни и ГБ) |
| `rovunet users renew` | Вернуть остановленного (истёк срок или квота) пользователя, расход обнуляется |
| `rovunet expire-users` | Остановить пользователей с истёкшим сроком или квотой и обновить ссылки подписки (по таймеру каждые 5 минут) |
| `rovunet users bulk_add --count N --prefix P` | Добавить до 500 пользователей со случайными паролями |
| `rovunet users remove` | Удалить пользователя |
| `rovunet users list` | Пользователи и их конфигурации |
| `rovunet users configs` | Все конфигурации всех пользователей |
| `rovunet export-clients --user U --format rovunet [--annotated]` | Ссылки одного пользователя для скриптов |

## Туннели

| Команда | Описание |
|---|---|
| `rovunet tunnel add` | Добавить туннель (параметры ниже) |
| `rovunet tunnel edit --tag T` | Изменить туннель (параметры ниже) |
| `rovunet tunnel remove T` / `--all` | Удалить один туннель или все |
| `rovunet tunnel start T` / `stop T` | Запустить или остановить туннель |
| `rovunet tunnel status [T]` | Все туннели и ядра или подробности одного туннеля |
| `rovunet tunnel share T [--user U]` | Показать конфигурацию туннеля |
| `rovunet tunnel logs T` | Журнал службы туннеля |

### Параметры `tunnel add`

| Параметр | Для | Значение |
|---|---|---|
| `--transport` | всех | `dnstt`, `vaydns`, `slipstream`, `soar`, `naive`, `stuntls`, `vless`, `direct-ssh`, `direct-socks5`, `external` |
| `--backend` | DNS-туннели, NaïveProxy | `socks`, `ssh`, `both`; для Soar: `direct` или `ssh` |
| `--tag` | всех | Имя туннеля |
| `--domain` | всех с доменом | Домен или поддомен туннеля |
| `--private-key`, `--public-key` | DNSTT, VayDNS, Soar | Импорт существующих ключей |
| `--record-type`, `--idle-timeout`, `--keep-alive`, `--clientid-size`, `--queue-size` | VayDNS | Настройка формата передачи |
| `--allow-ports` | Soar direct | Разрешённые порты назначения, например `80,443` или `any` |
| `--email`, `--decoy-url` | NaïveProxy | E-mail для Let's Encrypt, сайт-приманка |
| `--cdn`, `--tls`, `--listen-port`, `--cdn-address`, `--block-ports` | VLESS | Режим Cloudflare, TLS, порт прослушивания, чистый IP CDN, закрытые порты назначения |
| `--port` | External | Локальный UDP-порт для перенаправления |

### Параметры `tunnel edit`

| Параметр | Значение |
|---|---|
| `--new-tag` | Переименовать туннель |
| `--mtu` | MTU DNS-туннеля |
| `--front NAIVE_TAG \| -` | Пустить VLESS или SSH over TLS через порт 443 NaïveProxy или отменить |
| `--cdn-address IP \| -` | Чистый IP Cloudflare для будущих конфигураций VLESS |
| Параметры настройки VayDNS | Как для `tunnel add` |

## Ядра Xray

| Команда | Описание |
|---|---|
| `rovunet core-install --name N --protocol P --host H [options]` | Установить ядро для всех пользователей; см. [Ядра Xray](xray-cores.md#installing-a-core) |
| `rovunet core-list` | Установленные ядра |
| `rovunet core-remove N` | Удалить ядро |
| `rovunet core-plan --protocol P --host H --output DIR [options]` | Только записать файлы ядра |

## DNS-маршрутизация

| Команда | Описание |
|---|---|
| `rovunet router status` | Режим и маршруты |
| `rovunet router mode` | Переключить режим single/multi |
| `rovunet router switch` | Сменить активный туннель в режиме single |

## Конфигурация

| Команда | Описание |
|---|---|
| `rovunet config export` | Показать `config.json` без ключей |
| `rovunet config import PATH` | Заменить `config.json` (без ключей) |
| `rovunet config address IP \| auto` | Публичный адрес для конфигураций SSH, SOCKS5 и SSH over TLS; после запятых можно добавить другие адреса этого сервера, например `auto,198.51.100.9` |

## Внутренние

Используются службами systemd: `rovunet dnsrouter serve`, `rovunet socks serve`, `rovunet stuntls serve`, `rovunet vless serve`, `rovunet post-update`.

Далее: [Устранение неполадок](troubleshooting.md)
