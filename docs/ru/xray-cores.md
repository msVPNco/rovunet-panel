# Ядра Xray

[Документация](README.md) › Ядра Xray

Ядро Xray — отдельный сервер Xray, который панель устанавливает и обслуживает рядом с туннелями. Оно даёт четыре протокола, которых нет у встроенных серверов.

| Протокол | Транспорт | Сертификат | Лучше всего для |
|---|---|---|---|
| **VLESS + REALITY** (XTLS Vision) | TCP | Не нужен: рукопожатие имитирует настоящий сайт (`--target`) | Сетей, где проходит TLS к известным сайтам |
| **VLESS + XHTTP** | TCP, HTTP-запросы поверх TLS | Нужен, действительный для `--sni` | Работы за CDN или там, где заблокирован WebSocket |
| **Hysteria 2** | UDP (QUIC) | Нужен, действительный для `--sni` | Сетей с потерями и нестабильных сетей, где проходит UDP |
| **WireGuard** | UDP | Не нужен | Сетей, не блокирующих WireGuard |

<a id="installing-a-core"></a>
## Установка ядра

```bash
# VLESS + REALITY
sudo rovunet core-install --name r1 --protocol reality --host SERVER_IP \
  --target www.microsoft.com:443 --sni www.microsoft.com --port 8443 --devices 5

# VLESS + XHTTP
sudo rovunet core-install --name x1 --protocol xhttp --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 2096 --devices 5

# Hysteria 2
sudo rovunet core-install --name h1 --protocol hysteria2 --host vpn.example.com --sni vpn.example.com \
  --cert /path/fullchain.pem --key /path/privkey.pem --port 8443 --devices 5

# WireGuard
sudo rovunet core-install --name w1 --protocol wireguard --host SERVER_IP --port 51820 --devices 5
```

| Параметр | Значение |
|---|---|
| `--name` | Имя ядра: 1–31 символ, `a-z`, `0-9` и `-` |
| `--protocol` | `reality`, `xhttp`, `hysteria2` или `wireguard` |
| `--host` | Публичное имя или IP, к которому подключаются клиенты |
| `--sni` | Имя сертификата или имя сервера, которое предъявляет REALITY (по умолчанию `--host`) |
| `--target` | Только REALITY: настоящий TLS-сайт для имитации, в виде `host:port` |
| `--port` | Порт прослушивания (по умолчанию 8443); должен быть свободен |
| `--devices` | Количество отдельных учётных данных клиентов, 1–1000 |
| `--cert`, `--key` | XHTTP и Hysteria 2: пути к сертификату и закрытому ключу |

## Что делает установка

1. Проверяет, что порт свободен для нужного протокола (TCP для REALITY и XHTTP, UDP для Hysteria 2 и WireGuard).
2. Устанавливает закреплённую сборку Xray, опубликованную с этим выпуском, и сверяет её с SHA256SUMS выпуска. Уже установленный `/usr/local/bin/xray` сохраняется.
3. Записывает конфигурацию сервера, и Xray проверяет её (`xray run -test`) до запуска.
4. Создаёт и запускает службу `rovunet-core-<имя>` и открывает порт в брандмауэре.
5. Записывает по файлу клиента на устройство в `/etc/rovunet/core/<имя>/clients/`.

Если какой-либо шаг не удался, всё созданное удаляется.

## Управление ядрами

```bash
sudo rovunet core-list            # name, protocol, port
sudo rovunet tunnel status        # cores appear with their service state
sudo rovunet core-remove r1       # stop and delete
```

`core-plan` принимает те же параметры и `--output <новый каталог>` и только записывает файлы для ручного развёртывания.

## Файлы клиентов

Каждое устройство получает отдельные учётные данные: ссылку `vless://` или `hysteria2://` либо файл `.conf` WireGuard. Они импортируются в приложение Rovunet и в стандартные клиенты на основе Xray и WireGuard. Никогда не используйте один файл WireGuard на двух устройствах одновременно.

## Резервные копии

Ядра входят в `rovunet backup` и снова запускаются при `restore`. Сертификаты, указанные через `--cert`/`--key`, хранятся вне панели и должны существовать по тем же путям на новом сервере, иначе панель сообщит, что ядро не запущено.

## Ограничения

- Профили ядер импортируются как отдельные профили, а не в коллекцию пользователя.
- Ядра не зависят от пользователей панели: удаление пользователя не отзывает учётные данные ядра — удалите или переустановите ядро.

Далее: [Эксплуатация](operations.md)
