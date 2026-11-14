# Changelog

Changes in Rovunet Panel. Each release's notes come from its section here.

## v2.0.1

- Complete setup targets30 unrestricted profiles,29 time-only profiles and six shared-budget finite paths. Add TLS Vision, XHTTP REALITY, AnyTLS, TUIC and separate Mieru TCP/UDP.
- Order main protocols first and all ten DNS paths last; keep protocol display labels, stable profile IDs and saved subscription URLs.
- Restore the original status1 unlimited shape for SQLite-bound accounts (null limit/used, not_metered). Keep recorded consumption intact and expose its lower bound through opt-in status2.
- Use2.0.1 for the first public version-two release so private2.0.0 builds receive an actual versioned update.
- Share native finite quotas in durable SQLite; preserve known consumption, periods and credentials. Format2 encrypted checkpoints support same-identity forward reconciliation, without stale database/config rollback or a fresh-target recovery bypass.
- Keep committed SQLite policy/metadata reads independent of the sole writer through a bounded read-only WAL pool. Slow writes no longer consume the policy read's deadline merely while waiting for that same connection; reservations still require a serialized, validated durable commit.
- Restart the running built-in SOCKS5 worker after an authenticated panel swap; credential reload alone cannot load new code. Preserve stopped workers and their enable state.
- Authenticate and validate runtime upgrades with durable resume and retained old hashes. Refresh installed DNS/TLS transports and use exact reviewed source/dependency pins.
- Close held compatibility-core streams on revocation; retain per-account finite cutoff and the healthy user's session. Fix service and resolver permissions under strict umask and retain administrator resolver choices.
- Add certificate health/recovery and public Slipstream certificate companions. Refresh the runtime corresponding sources, native security backports and component notices.
- See [server acceptance and explicit qualification boundaries](docs/releases/v2.0.1.md). Artifact publication requires final signed exact-byte acceptance; broader physical-client/off-site/fresh-target qualification is separately tracked.

## v1.21.2

The owner requests this functional release now. App version observations are historical fixtures; future app-dependent work follows the version-independent integration handoff.

- Normal install/update completes missing managed cores; eligible unlimited accounts receive 24 subscription profiles on Android and current Windows through their existing subscription URLs.
- Current Windows feeds now include AmneziaWG and Shadowsocks 2022. Both feeds use lossless standard AWG/SS2022 URIs so older Windows/macOS readers preserve supported items and skip unsupported ones; no subscription/token/account reissue. The other 22 native items and native CLI output are retained.
- Fresh Hy2 uses Salamander. An old account-managed Hy2 is migrated once with its endpoint/auth retained; explicit plain mode and stopped states survive later updates. Update the same saved subscription after upgrading the server.
- Resume missing setup and pending compatibility migration even when the installed version already matches Latest. Keep private recovery and prior valid feeds on errors.
- Fix stale Xray API sockets after power loss, private-file ownership during restore, subscription token/path boundaries and stale account/config updates. Preserve restricted quota eligibility and known held-stream limits.
- Authenticated Ed25519 releases/bootstrap and recoverable binary updates; publisher key stays in the owner's Windows-local DPAPI vault. CI stages an unsigned draft for local verification/signing.
- Preserve the reviewed preview promotion path and all published assets. Broad runtime/source/native ABI, off-site custody, physical-device and full provider/recovery qualification remain open; candidate runtime pins are not promoted in this functional release.


## Unreleased — private engineering review (2026-11-12)

- Stage and authenticate downloaded executables before replacement; exact runtime pins, independent production-key requirement, bootstrap/update recovery journals, atomic files and preserved stopped-service choices. Production roots remain unprovisioned and runtime/recovery/client gates stay open; no new public release.
- Validate subscription tokens before file paths, reject stale config writes, preserve strict v1 and old links during normal refresh, add independent link rotation and retain the last valid feed on oversized/failed address expansion.
- Default operator backups to authenticated age encryption; keep legacy decode and explicit plaintext export; report partial restore with a nonzero exit. Protect administrator SSH accounts, narrow owned forwarding rules and target explicit password-change sessions.
- Bound DNS workers, queue, pending maps, backend sockets, TCP response memory and aggregate UDP rates; prove a narrow DNS systemd restriction with start/restart payload tests. Add versioned out-of-band capability/security/privacy/provenance documentation and isolated CI acceptance checks.
- فارسی: این نسخه خصوصیِ بررسی است؛ ساخت و آزمایش محلی انجام می‌شود، انتشار عمومی/سرور زنده تغییر نمی‌کند. کلید اعتماد تولیدی، بررسی موتورهای قدیمی، بازیابی کامل VM و پذیرش کلاینت‌های متناظر هنوز شرط باز انتشارند. لینک‌ها و نام‌های تأییدشده حفظ شده و محدودیت دستگاهِ لغوشده فعال نشده است.
- Русский: частная проверка; публикация закрыта до принятия корня доверия, runtime, восстановления VM и клиентской совместимости.
- 简体中文：私人审查版本；生产信任根、运行时审查、虚拟机恢复与匹配客户端验收仍是发布条件。

## v1.21.1

### English

- Make normal installation and Quick Wizard use the complete preset: both DNS/Soar backends and all nine protocol cores. Updating an existing 20-profile setup adds AmneziaWG, SS2022, Trojan TLS and VLESS Encryption automatically; interactive updates of incomplete base setups request missing domain/DNS settings.
- Keep existing users, passwords, tokens, profile IDs, core files and stopped-core choices. Validate a private backup before creating missing cores, select free ports without killing listeners, and resume partial setup without duplicate cores or regenerated keys. Report actual Android/Windows profile counts and explicit pending prerequisites; `rovunet complete-setup` retries setup without rerunning all transport updates.
- Put automatically added SS2022/Trojan/VLESS Encryption cores on a separately verified pinned Xray executable. Preserve the legacy Xray file and retain runtime selection through account changes, start/stop and backup restore.
- Recognize legacy display tags `xhttp`, `hysteria2`, `soar-direct` and the four protocol-name tags. Only display names change. Reinstalling with existing users no longer offers an implicit credential reset or replaces the proxy credential list with a single user.
- The full supported setup remains **24 Android / 22 Windows profiles** for an active account without a traffic quota and one public address. Valid DNS/certificates, free ports and a usable TUN device remain prerequisites; cloud firewall rules are external. Traffic-limited accounts retain their existing restrictions. Established-stream/device-limit limitations from v1.21.0 remain unchanged.

### فارسی

- نصب معمولی و Quick Wizard به‌صورت پیش‌فرض چیدمان کامل را می‌سازند. آپدیتِ سرور ۲۰تایی چهار پروتکل جدید را خودکار اضافه می‌کند؛ برای تنظیمات پایهٔ ناقص، دامنه‌ها و DNS بخش‌های غایب در همان مسیر تعاملی پرسیده می‌شوند.
- کاربران، رمزها، کلیدها، شناسه‌ها و لینک‌های قبلی حفظ می‌شوند. قبل از ساخت هسته‌های غایب بکاپ خصوصی معتبر گرفته می‌شود؛ پورت اشغال‌شده آزادسازی اجباری نمی‌شود و اجرای مجدد کار نیمه‌تمام را بدون هستهٔ تکراری ادامه می‌دهد. تعداد واقعی کانفیگ و علت ناتمام‌ماندن نمایش داده می‌شود؛ دستور بازیابی `rovunet complete-setup` است.
- موتور سه پروتکل جدید Xray جدا از موتور قبلی نصب می‌شود. نام‌های قدیمی xhttp، hysteria2 و soar-direct نیز به نام‌های نمایشی مصوب نگاشت می‌شوند. نصب مجدد با کاربران موجود، اعتبارنامهٔ قبلی را بازنویسی نمی‌کند.
- خروجی چیدمان کامل برای حساب فعالِ بدون سهمیهٔ حجمی و یک آدرس عمومی Android=24 و Windows=22 است؛ DNS، گواهی معتبر، TUN و پورت آزاد لازم‌اند. محدودیت‌های حساب حجمی، جریان‌های باز و سقف دستگاه مطابق نسخهٔ قبل باقی‌اند.

### Русский

- Полная установка и Quick Wizard используют общий полный набор; обновление сервера с 20 профилями автоматически добавляет четыре новых протокола. Частичная настройка запрашивает недостающие домены/DNS интерактивно.
- Сохраняются пользователи, ключи, ID и подписки; перед добавлением создаётся проверенная приватная копия, выбираются свободные порты, повторный запуск не создаёт дубликатов. `rovunet complete-setup` продолжает незавершённую настройку.
- Новые Xray-ядра используют отдельный закреплённый runtime. Исправлены старые отображаемые имена. Полная конфигурация: Android 24 / Windows 22 при выполненных требованиях; ограничения аккаунтов с лимитом трафика сохранены.

### 简体中文

- 常规安装及 Quick Wizard 默认使用完整配置；已有 20 个配置的服务器更新后自动补齐四种新协议。基础配置不完整时，交互式流程询问缺失的域名/DNS。
- 保留用户、密钥、配置 ID 和订阅地址；新增前验证私有备份，选择空闲端口，重复执行不会生成重复内核。`rovunet complete-setup` 可继续未完成的配置。
- 新 Xray 协议使用独立的固定版本运行时，并修复旧显示名称。在满足前提的完整配置下提供 Android 24 个 / Windows 22 个；保留流量配额和其他既有限制。

## v1.21.0

### English

- Add account-managed **AmneziaWG, Shadowsocks 2022, Trojan TLS and VLESS Encryption**: installation, targeted account updates, persistent start/stop, port changes, removal, backups and certificate tracking. Release assets include pinned runtimes, their source archives, licenses and checksums for Linux amd64/arm64.
- Keep existing subscription URLs and item IDs through normal updates. Refresh feeds after core/tunnel lifecycle changes; serialize writers and preserve the last valid feed on generation errors. Android uses `.json`; Windows uses `.windows.json`. The tested full setup yields **24 Android / 22 Windows configurations**, depending on installed services and account eligibility. Released Windows excludes AmneziaWG and SS2022.
- Apply readable names to the 24 reviewed configurations. Only the native display-name field changes; endpoint, credentials, embedded core profile, collection identity, subscription token and item order stay unchanged.
- Use one account-eligibility/export policy across CLI and subscriptions, retain Soar in the final installation output, and export credentials only after provisioning is confirmed.
- Persist Xray source counters atomically, retain known usage after a reset/crash and report incomplete samples as partial. Backups include external TLS files, applied credentials and usage history. Firewall ownership preserves shared/preexisting rules.
- **Limits:** all four added protocols are excluded from traffic-limited accounts. Removing credentials in the pinned Xray rejects new sessions but leaves established streams alive, including after expiry. Direct Soar has no separate account authentication. Exact simultaneous-device limiting is not implemented. SS2022 and Trojan without Flow have upstream deprecation warnings.
- The owner accepted the staged server upgrade and renamed subscription output. Local tests include actual Xray payloads, two-peer AmneziaWG network namespaces, rollback/recovery, display-only comparisons, full tests, vet and race. This does not establish whole-host reboot, long-run stability, battery or independent SS2022 UDP behavior.
- Existing stable installations update with `sudo rovunet update`. The accepted private `preview.3` has a separate, checksummed **preview-to-v1.21.0 amd64 upgrade package** that preserves the running cores and links; it checks every existing profile before accepting the binary replacement. Do not rerun the fresh installer on that preview.

### فارسی

- چهار گزینهٔ **AmneziaWG، Shadowsocks 2022، Trojan TLS و VLESS Encryption** وارد مدیریت حساب، نصب، شروع/توقف، تغییر پورت، حذف، پشتیبان و پیگیری گواهی شدند. فایل‌های انتشار هر دو معماری، منابع موتورهای مبنا، مجوز و checksum دارند.
- لینک و شناسهٔ سابسکریپشن قبلی در آپدیت معمولی حفظ می‌شود. نام نمایشی ۲۴ کانفیگ خواناتر شده؛ کلید، رمز، مقصد و ترتیب عوض نمی‌شوند. چیدمان کامل آزموده‌شده Android=24 و Windows=22 است؛ خروجی هر حساب به سرویس‌ها و محدودیت‌هایش بستگی دارد. Windows فعلی AmneziaWG و SS2022 را نمی‌گیرد.
- شمارندهٔ پایدار مصرف، حفظ حد پایین مصرف در خطا، نوسازی اشتراک پس از تغییرات، پشتیبان TLS و حفظ قواعد فایروال مشترک تکمیل شدند.
- هر چهار گزینهٔ تازه از حساب حجمی کنار گذاشته می‌شوند. سه گزینهٔ تازهٔ Xray اتصال جدید کاربر منقضی/حذف‌شده را رد می‌کنند، ولی جریان قبلی ممکن است ادامه یابد. سقف دقیق دستگاه پیاده نشده و Soar مستقیم کلید مشترک دارد. هشدارهای deprecated بودن SS2022 و Trojan بدون Flow حفظ شده‌اند.
- نصب آزمایشی و نام‌ها به تأیید مالک رسیده‌اند. آزمون‌های محلی و شواهد شبکه، اثبات ریبوت کامل میزبان، پایداری طولانی، باتری یا UDP مستقل SS2022 نیستند. برای preview.3 بستهٔ ارتقای جداگانهٔ amd64 ارائه شده؛ نصب اولیه را روی این سرور دوباره اجرا نکنید.

### Русский

- Добавлены управляемые аккаунтами AmneziaWG, Shadowsocks 2022, Trojan TLS и VLESS Encryption, операции жизненного цикла, резервирование TLS/учётных данных и закреплённые runtime-ресурсы с исходниками, лицензиями и контрольными суммами.
- Ссылки подписок и ID сохраняются; новые отображаемые имена не меняют параметры подключения. Проверенная полная конфигурация: Android 24 / Windows 22; Windows исключает AmneziaWG и SS2022. Исправлены единая политика экспорта, обновление подписок и сохранение счётчиков после ошибок.
- Все четыре новых протокола исключены для аккаунтов с лимитом трафика. Новые протоколы Xray запрещают новые сеансы после удаления, но открытые потоки могут продолжаться. Точного лимита устройств нет. Для принятой preview.3 предусмотрен отдельный пакет обновления amd64 без переустановки ядер.

### 简体中文

- 新增账户托管的 AmneziaWG、Shadowsocks 2022、Trojan TLS 和 VLESS Encryption，支持生命周期操作、TLS/凭据备份，并提供固定版本运行时、源码、许可证和校验和。
- 保留订阅地址与配置 ID；新的显示名称不改变连接参数。完整测试配置为 Android 24 / Windows 22，Windows 不接收 AmneziaWG 和 SS2022。统一导出策略、订阅刷新及故障后计数持久化得到修复。
- 四种新增协议均排除流量受限账户。新增 Xray 协议在删除后拒绝新会话，但现有流可能继续；尚无精确设备数限制。已验收的 preview.3 提供独立 amd64 升级包，无需重新安装内核。

## v1.20.1

- **An update that is cut short no longer leaves a tunnel without its program.** `rovunet update` moves each transport binary (`dnstt-server`, `slipstream-server`, `vaydns-server`, `caddy-naive`) to `.bak`, downloads the new one and puts it in place. If it stopped in between, the binary was gone and the service restarted every 5 seconds failing with status 203/EXEC; this happened on a real server on the v1.20.0 update, while GitHub was answering some downloads with HTTP 504. The next update now puts the old binary back when the binary is missing and a `.bak` exists, and removes the half-finished download. A binary that is in place is never changed, and a backup is never removed. The cause of the interruption itself is not known.
- If a server is already in that state, the same fix by hand: `mv /usr/local/bin/<name>.bak /usr/local/bin/<name>`, `rm -f /usr/local/bin/<name>.tmp`, then restart the service.

## v1.20.0

- **A new install now sets up two REALITY cores.** Next to `reality` (9443, 2096 or 8880/tcp) the installer adds `reality2` on 2087, 2053 or 8443/tcp (the first free), disguised as a different site. The two sites are never from the same company (two Microsoft sites would be blocked together). Every user gets a configuration on each, with the same counting and the same limits. If one site or port is blocked on a user's network, the other may still connect; on a real server `reality2` (Apple, 2087) connected on a fibre line and on a mobile network. Only a new install does this: a server that already has a REALITY core is left as it is, and `reality2` can be added by hand with `rovunet core-install --name reality2 --protocol reality ...` (see [Xray cores](docs/en/xray-cores.md)).
- **The links of a second core have their own name.** `Rovunet-reality2-<user>` instead of a second `Rovunet-reality-<user>`, so other apps do not show two links with one name. Cores named in any other way, and the first REALITY core, keep their link names.
- Found while testing: after `core-install`, the subscription links did not include the new core until the next limits check (up to 5 minutes) or `rovunet expire-users`. The docs now say so.

## v1.19.1

- **Adding a traffic-limited user no longer says "SSH + SOCKS".** Such a user has neither; the message now says "VLESS, REALITY and XHTTP only". A user with only an expiry still gets SSH and SOCKS.
- The docs now say that after an update `metering_status` is `unavailable` until the first limits check, about 5 minutes later.

## v1.19.0

- **Each user now has an account status file for the app**, next to the subscription: the same link with `.json` replaced by `.status.json`. It says whether the account is `active`, `expired` or `limit_reached`, the expiry, the traffic limit and the traffic used (in bytes, both directions), how complete and how fresh that count is (`metering_status` is `partial` when a core did not answer or a counter file could not be read; `sampled_at` is the last read in which every core answered, so a failed read never makes an old count look fresh), and a `revision` that goes up when anything but the times changes (it belongs to the URL: a password change gives a new link that starts again at 1). It grants nothing: the server enforces the limits on its own. The subscription file is unchanged, because the app rejects keys it does not know. See [Limits and subscription links](docs/en/limits-and-subscriptions.md#account-status-file) for every key.
- **Stable identifiers.** The panel gets a random `server_id`, and every user an `account_id` and a `period_id`. They do not depend on the server's IP or name. `users renew` keeps the account and starts a new period, so the app never adds two periods together; `users passwd` keeps both. Existing servers and users get theirs on the first run after the update.
- **A stopped user keeps the status file** (with the reason) while the subscription goes, so the app can say why the configs stopped working. A user removed for good has neither; a 404 is not "expired".
- The subscription and status files are served with `Cache-Control: no-store`.
- The limits check now warns when a counter file cannot be read (the counts may be low); before, such a file was skipped without a word.
- v1.18.0 has now been checked on a real server: a user renewed with a traffic limit was refused on SSH and NaïveProxy, was left out of SOCKS5, Hysteria 2 and WireGuard, and kept REALITY and XHTTP; REALITY traffic counted within 1% of what the app reported.

## v1.18.0

- **A traffic-limited user can now use only the paths whose traffic is counted, and the server enforces it.** Before, "limited users get only VLESS, REALITY and XHTTP" was only what the panel printed: the server still accepted such a user on SSH, NaïveProxy, SOCKS5, Hysteria 2 and WireGuard, which are not counted. So a configuration a user saved while unlimited kept working after `users renew` gave them a quota (their credentials are derived from the same password), and their traffic never reached the quota. Now one rule (`MeteredOnly`) decides, everywhere: a limited user has no SSH account, no `basic_auth` line in NaïveProxy, no SOCKS5 login, and is left out of Hysteria 2 and WireGuard cores. It applies when a user is added, renewed, restored from a backup or after a password change, when a new core is made, and on `rovunet update`.
- **What happens on update.** Users who are already traffic-limited lose their SSH account (open sessions end) and their Hysteria 2 and WireGuard credentials, and the cores that held them restart once. Their VLESS, REALITY and XHTTP configs, record, limit and subscription link are untouched.
- **A core nobody may use is stopped, not left with an empty list.** It keeps its keys and settings and starts again when an unlimited user exists (this includes removing the last user of a core).
- **A removal that could not be applied is never reported as done.** If Xray rejects the new configuration or will not restart, it is tried twice, 15 seconds apart. If it still fails, the core is stopped (running the old configuration would keep the user's access) and the failure is shown in `rovunet users list`. The 5-minute limits timer retries it. An addition that fails still keeps the previous configuration running. The cost: when this happens, other users of that core are disconnected too.
- **Retiring a limited user no longer fails because they have no SSH account**, and removing a user no longer deletes a same-named account that Rovunet did not create.
- **Not covered** (they cannot be tied to one user): Soar without a login, DNS tunnels with no SSH account behind them, and cores made with the old `--devices` option. The NaïveProxy service is kept running if its new Caddyfile is rejected (it also serves the subscription links and VLESS); the rejection is shown as a warning and retried every 5 minutes.
- Tested with unit tests that replace systemd, Xray and the account commands; nothing here has been run against a real server yet. See `docs/testing/quota-access.md` for the checks to run on a throwaway server.

## v1.17.1

- **Removing the extra addresses now refreshes the subscription links at once.** In v1.17.0 only adding them did, so after `rovunet config address auto` the links kept the extra addresses' configs until the next 5-minute run.

## v1.17.0

- **Extra addresses for the same server, for resistance to blocking.** `sudo rovunet config address` now takes several addresses separated by commas: the first is the main one (or `auto`), the rest are other IPs or hostnames of this same server, for example `auto,198.51.100.9,198.51.100.10`. Each user's subscription link then carries every address-based config (VLESS, REALITY, XHTTP, Hysteria2, WireGuard, StunTLS and so on) once per address, so when one address is blocked the app can use the next. Domain-based configs (NaïveProxy, DNS tunnels) are not repeated. Every extra address must reach this server with the same ports open (an IP added to its network interface, or a hostname pointing at it); this does not change where the server listens, and it does not add another location.

## v1.16.0

- **Backups now include each user's traffic used so far, and restore brings it back.** Before, restoring onto a new server (or after a reinstall) started every traffic-limited user's counter from zero, giving them their whole quota again. Restore adds only what the server is missing, so restoring twice, or onto a server that already counted traffic, never counts it twice. Older backups restore as before (their users start from zero, as they did).

## v1.15.2

- **A traffic-limited user's subscription is refreshed every 5 minutes.** Their feed includes a REALITY or XHTTP config only while that core's statistics answer. Right after an update the cores are still restarting, so the feed written at that moment could miss them and stay that way until the next user change. The limits timer now rewrites the feeds on each run (only files whose content changed are touched), so the missing configs appear within 5 minutes. This was seen on a renewed user whose configs did not grow after the update.

## v1.15.1

- **`rovunet diag` no longer lists the limits timer as an orphaned service** (v1.14.0 added `rovunet-user-expiry` and `diag` did not know it).
- **`diag` checks each REALITY and XHTTP core's traffic statistics** and says whether they answer, since traffic limits on those cores depend on it. A core whose statistics do not answer is a warning, not a failure: limited users are simply not given that core.

## v1.15.0

- **Traffic limits now cover REALITY and XHTTP too.** Those cores expose Xray's statistics on a loopback-only port (a statistics API, a counter per user named by the panel username, and a routing rule; WireGuard and Hysteria 2 have none and stay unmetered). The limits timer reads each core's counters every 5 minutes and adds them to the user's total together with VLESS and SOCKS5. A traffic-limited user is now given the VLESS, REALITY and XHTTP configs, and a core is given only if its statistics answer (checked when the configs are printed); otherwise the user keeps just VLESS.
- **`rovunet update` restarts each REALITY and XHTTP core once** to apply the new configuration. Xray clears its counters when a core restarts, so the panel reads them just before it restarts a core for a user change, and a restart outside the panel loses up to 5 minutes of counting.
- This relies on the `xray api statsquery` command and the classic statistics configuration. They were written from the documented form and a CI test of the generated config, not run against your Xray yet: if a limited user is not given REALITY/XHTTP configs, or `users list` shows no growing usage for it, the statistics are not answering and the output of `xray api statsquery -s 127.0.0.1:10085 -pattern "user>>>"` is what is needed.

## v1.14.0

### Traffic limits, and stopped users can be renewed

- **The GB limit is now enforced, for VLESS (WebSocket) traffic.** The panel's own VLESS and SOCKS5 servers count each user's bytes (both directions, saved every 15 seconds in `/var/lib/rovunet/usage`). A timer checks every 5 minutes (it was hourly), so a little more than the limit can pass. 1 GB is 1024³ bytes.
- **A user with a traffic limit is given only the VLESS config**, in `users configs`, the install summary and the subscription feed, and no shared Soar config. Traffic over SSH, NaïveProxy, the DNS tunnels and the Xray cores is not counted, and the user is not handed their credentials, so the limit is real rather than an easy-to-bypass number. The panel says so when you set one, and warns if there is no VLESS tunnel yet. Counting the Xray cores is not done.
- **Expired or used-up users are stopped, not lost.** They are removed from every service exactly as before (open sessions end), but a record keeps the password and the subscription link. `users --action renew` brings the user back with new limits counted from now and **the same password and link**, so nothing has to be re-sent. `users list` shows stopped users and each limited user's usage; `users remove <name>` on a stopped user forgets the record.
- Only the VLESS and SOCKS5 servers count: VLESS needs `rovunet update` (the servers restart once to start counting).

## v1.13.1

- **The subscription link now appears everywhere configs are shown.** v1.13.0 printed it only for a single user's configs (`users` > list > a user, and after adding a user). The "Show all configs" action and the install summary, which print `[soar] <user>, no login needed` and the rest, left it out. They now print each user's link, and a user created before links existed gets theirs on the spot. No new user is needed.

## v1.13.0

### Subscription link for every user

- **Each user gets one link that delivers all of their configs to the app**, whether the user is unlimited or limited. `users configs` prints it under the configs: `https://<naive-domain>/sub/<token>.json`. Paste it into the app's Subscriptions menu and the app imports (and later refreshes) every config in one step.
- **It is served on port 443 by the NaïveProxy Caddy**, with the domain's real certificate, as plain static files: no new service, no new port, no application behind it. The app requires HTTPS, so port 22 (SSH) or any plain-HTTP port cannot carry it. Without a NaïveProxy tunnel there is no link.
- **The file holds exactly what `users configs` prints** (rovunet:// configs), in the one JSON shape the app accepts, so the two cannot disagree. It is rewritten whenever users or tunnels change and on `rovunet update`.
- **The link is an unguessable 128-bit name, like a share link:** anyone who has it can read that user's configs, so treat it like the password. Changing the user's password makes a new link and retires the old one; removing or expiring the user deletes the file. The directory can be entered but not listed, so a local SSH account cannot enumerate the files.
- A NaïveProxy Caddy restarts once on update to start serving `/sub/`.

## v1.12.1

- **REALITY picks its disguise at random.** When no `--target` is given, `core-install` and the installer choose a random site from a built-in list of 16 (Microsoft, Amazon and its image servers, Apple, Cloudflare, Google download and font hosts, NVIDIA, Samsung, Intel, Oracle, Mozilla). Each candidate is first checked from your server for TLS 1.3 and HTTP/2, which REALITY needs, and only working ones are drawn. The server name follows the chosen site. If none can be reached the installer falls back to `www.microsoft.com`; `core-install` stops and asks for `--target`. Cores installed earlier keep their target.
- The list is a judgement about sites that are rarely blocked, not a guarantee. It is `RealityTargets` in `internal/coreplan/targets.go`.

## v1.12.0

### Users can expire

- **Adding a user asks "Unlimited or Limited?"** Unlimited is the default. For Limited you are asked for days and then for GB, and a blank answer to either means unlimited. For scripts, `rovunet users --action add --username bob --days 30 --gb 50` skips the questions; `bulk_add` takes the same two options.
- **An expired user is removed automatically.** An hourly timer (`rovunet-user-expiry.timer`, set up when the first user with an expiry is added, or by `rovunet update`) runs `rovunet expire-users`, which removes the user exactly like `users remove`, so open sessions end too. The user is deleted, not suspended.
- **`users list` shows each user's limits.** Changing a password keeps the user's expiry and limit.
- **The GB limit is recorded but not enforced yet.** The panel says so when you set one. Enforcement, and a subscription link, are the next releases.

## v1.11.3

- **`rovunet diag` names the process on every listening port**, including VLESS (2053), the DNS router (53) and SOCKS5 (1080), which v1.11.2 had left out. Names are no longer cut at 15 characters (`slipstream-serv` is now `slipstream-server`).
- **A TLS-SSH tunnel carried by NaïveProxy now says so** (`none of its own (carried by <tag>)`) instead of showing no port line.

## v1.11.2

- **`rovunet diag` checks the NaïveProxy and TLS-SSH ports.** Until now a Caddy that was running but held no port still showed green. A TLS-SSH tunnel carried by NaïveProxy has no port of its own and is skipped.
- **Listening ports name the process that holds them**, e.g. `listening (xray)`, so a port taken by the wrong program stands out. This is a local check only: it does not show whether the port is reachable from the internet.

## v1.11.1

- **`rovunet diag` now checks the Xray cores.** For each installed core (REALITY, XHTTP, Hysteria 2, WireGuard) it reports whether the service is active and enabled at boot, whether the port is listening, how many days its TLS certificate has left, and whether the renewal timer is on. A certificate with under 7 days left is a failure.
- **Fixed false "Orphaned Services" warnings:** `diag` listed every Xray core and the certificate check as orphaned, because it only knew the tunnels.

## v1.11.0

- **Plain SOCKS5 is no longer in the default output.** The direct SOCKS5 server is unencrypted and was blocked on the mobile network we measured, so `users configs`, the install summary and `export-clients` leave it out and say how many they hid. List it with `ROVUNET_ALL_CONFIGS=1` (menus) or `export-clients --all`. Nothing is removed: the server keeps running and existing tunnels are unchanged. NaïveProxy's SOCKS variant is not affected.

## v1.10.2

- **NaïveProxy now gives both configs everywhere.** One NaïveProxy tunnel serves SOCKS and SSH clients, and the install summary printed both, but `users configs` and `export-clients` printed only the SOCKS one. They now print `<name>-socks` and `<name>-ssh`.

## v1.10.1

- **Removing a user now ends that user's open connections.** Until now only new logins were refused: a VLESS (WebSocket) or SOCKS5 connection that was already open kept working after `users remove`, or after a password change, until the app disconnected. Both servers now close the sessions of a removed or re-keyed user at once and leave other users' sessions alone. SSH, NaïveProxy and the Xray cores already dropped them.

## v1.10.0

### Fastest first, and every protocol the app supports from one install

- **Configurations are listed from the fastest to the slowest:** VLESS over WebSocket, XHTTP, REALITY, NaïveProxy, SSH over TLS, SSH, then Hysteria 2, WireGuard and SOCKS5, and the DNS tunnels last. `users configs`, the install summary and `export-clients` follow it, with Xray cores slotted in among the tunnels.
- **The installer sets up VLESS + XHTTP and Hysteria 2** with the certificate Caddy obtained for the NaïveProxy domain, next to REALITY and WireGuard. REALITY and XHTTP are marked as recommended; Hysteria 2, WireGuard and SOCKS5 are marked as blocked on some networks, which a test on a filtered mobile network confirmed.
- **A renewed TLS certificate is served without any action.** A timer runs `rovunet core-certs` twice a day and restarts XHTTP and Hysteria 2 cores whose certificate file changed. `rovunet update` sets the timer up for cores installed earlier.
- Docs in four languages: a "Choosing" section with the measured result, and the order of the configurations.

## v1.9.3

- The WireGuard file printed by `users configs` no longer has indented lines. The official WireGuard app does not recognise an indented `[Peer]` section and rejected the file with "Missing attribute in Peer's PublicKey".

## v1.9.2

- `users configs` and the install summary also print, under each Xray core profile, the plain link or the WireGuard file for other apps, so a core can be tested with the official WireGuard app or any VLESS client.

## v1.9.1

### Core installs no longer accept pasted or placeholder addresses

- `core-install` turns a hostname pasted as a Markdown link, such as `[www.example.com](https://www.example.com)` (chat apps and browsers paste these), back into the plain name, and cores already stored that way are repaired the next time the panel updates them.
- It rejects a host, server name or REALITY target that is not an IP address or a domain name, such as the placeholder `SERVER_IP`, and names the offending value. Before, such a core installed without complaint and its clients pointed at an address that does not exist.

## v1.9.0

### Xray cores for every user

- **One install delivers every protocol.** The installer offers VLESS + REALITY and WireGuard next to the other protocols and sets them up after the first user, on the first free default port.
- **Cores serve the panel's users.** Every user gets a configuration on every Xray core. `users add`, `remove` and `passwd` keep the cores in step: Xray validates each change, a rejected change keeps the previous configuration, and a changed password revokes the old configuration. Adding a user never changes an existing user's configuration.
- **One group per user in the app.** `users configs`, the install summary and `export-clients` print each user's core profiles as native `rovunet://` profiles in the user's collection, next to their tunnels. `export-clients --format standard` prints the plain `vless://` and `hysteria2://` links.
- `core-install` serves the panel's users by default; `--devices` keeps the per-device mode. Cores installed by v1.8 keep working as they are.
- Backups carry the cores' new state, so restored cores keep their keys and every user's configuration.

## v1.8.2

### Fixes

- **Services failing after updating from v1.7.0.** The old updater starts the new version from the old command's path, so the SOCKS5 proxy, Soar, SSH over TLS and VLESS services were regenerated to run `/usr/local/bin/slipgate`, which the migration had removed, and failed with `203/EXEC`. Services now always run the installed `rovunet`, and the next update repairs any unit still pointing at the old command.
- `tunnel status` showed every non-running state as `unknown`, which hid those failures. It now shows systemd's own state (`activating`, `failed`, `inactive`), and direct SSH and SOCKS5 tunnels show the state of sshd and the shared SOCKS5 proxy.

## v1.8.1

### Fixes

- **Soar configs join each user's group in the app.** The install summary, `tunnel add` and `tunnel share` printed the login-free Soar config once without a user, so the app put it in a group of its own. It is now printed per user, like `users configs`.
- **Updating from v1.7.0 is safer.** If a step after the migration fails, the update no longer makes the old updater rebuild old-named services on paths that were moved; the config directory is moved before users and groups are renamed, and a failed move restarts the stopped services.
- The migration makes `config.json` owner-only (0600) right away; v1.7.0 wrote it readable by all local users.

## v1.8.0

The panel is now **`rovunet`** everywhere. Updating from v1.7.0 migrates the server automatically.

### Migration

- Command, binary, systemd services, system users and groups, `/etc/rovunet`, sshd_config blocks, firewall and resolver files all use the Rovunet name.
- The first run of the new version moves an existing server over in place: services are stopped, users and groups are renamed with the same UIDs/GIDs, the config directory is moved, stored paths are rewritten without touching any password, services come back under their new names, sshd_config is changed only after `sshd -t` accepts it, and the command becomes `/usr/local/bin/rovunet`.
- Old backups still restore. Issued client configs keep working: the VLESS key derivation and the `slipnet://` format are unchanged.

### New

- **Xray cores:** `rovunet core-install`, `core-remove` and `core-list` install REALITY, XHTTP, Hysteria2 or WireGuard as managed services: the port is checked first, the pinned Xray build is verified against this release's SHA256SUMS, Xray tests the config before it is deployed, and the TCP or UDP port is opened. Cores appear in `tunnel status` and are included in backups.
- `tunnel status` shows each certificate's names, issuer type and expiry, and flags expired, soon-to-expire and unreadable ones.
- `export-clients --annotated` labels each link with its tunnel.

### Security

- An external SOCKS5 proxy with no users now refuses connections and drops live sessions, instead of becoming an open proxy.
- An oversized EDNS0 size in a DNS query can no longer crash the DNS router.
- `config.json` is written owner-only (0600), including files saved by older versions.
- `rovunet update` verifies the download against SHA256SUMS, installs atomically and rolls back if the install fails.

### Fixes

- Firewall rules survive reboot without replaying a full iptables dump: only the panel's own ports are inserted, so Docker, fail2ban and other tools' rules are left alone.
- Editing a DNSTT, Slipstream, VayDNS or Soar tunnel restarts its service instead of leaving the old settings running.
- Overlapping tunnel domains always route to the most specific one.
- Exported configs keep passwords that contain `|` intact (v33).
- A new StunTLS tunnel's port is opened in the firewall, and a TCP port already in use is refused before the tunnel is saved.
- Renaming a Slipstream tunnel keeps its certificate paths valid.
- Removing the default tunnel clears the default route.
- Freeing a UDP port no longer looks at TCP listeners.
- Uninstall removes SSH tunnel users and their group.

## v1.7.0

First release as **Rovunet Panel**.

### Security

- **NaiveProxy no longer accepts the built-in `slipgate:slipgate` login.** A Naive tunnel created without its own login used to fall back to that well-known login in Caddy, so anyone could use the proxy. Now only the tunnel's own login (if set) and panel users can log in; with neither, Caddy gets a random login and stays closed. A Naive config without any login is refused with a hint to run `slipgate users add`.

### Changes

- Caddy serves HTTP/1.1 and HTTP/2 only and no longer opens UDP 443 for HTTP/3, so Hysteria2 or another UDP service can use that port. Naive clients use HTTP/2 and are unaffected.
- Rebranded as **Rovunet Panel** (based on SlipGate, not affiliated): logo, README, guides under `docs/`, NOTICE.md. The `slipgate` command, service names and `/etc/slipgate` are unchanged.
- `rovunet://` v32 exports, `export-clients` and `core-plan` (from the Rovunet 0.3.0 alpha work).
- A release is published only when `VERSION` in the Makefile changes on main (or by hand, or from a `v*` tag); other pushes to main never release.
- New README in English and Persian, Rovunet banner in the installer and the menu.
- Install and `slipgate update` now use this repository's releases (`msVPNco/rovunet-panel`).

## v1.6.8-vless.9

### Fixes

- **Running `slipgate install` again** on a configured server keeps the tunnels that are there instead of adding clashing copies (e.g. a second StunTLS on the same port), and the summary prints the configs of **every** tunnel, not only the new ones.
- No more "slipgate-socks5.service is not active, cannot reload" during install: a stopped SOCKS5 service is restarted instead of reloaded.

## v1.6.8-vless.8

### Highlights

- **VLESS and StunTLS on port 443 through NaiveProxy.** `slipgate tunnel edit --tag <tunnel> --front naive` serves them behind NaiveProxy's Caddy with its real Let's Encrypt certificate and decoy site: no Cloudflare hop, no self-signed certificate, and links without `allowInsecure` (which Xray clients drop from 2026-08). Install does this automatically when NaiveProxy is selected. Caddyfile changes are checked with `caddy-naive validate` before they are applied.
- **NaiveProxy accepts every Rovunet Panel user**, not only the first one; user changes update Caddy live.
- **StunTLS**: WebSocket pings are answered to the client instead of corrupting the SSH stream.
- **Direct SOCKS5** listens publicly again after user changes.
- **Soar**: direct (no-login) configs appear in `slipgate users configs`.
- **VLESS links** keep Rovunet's SNI fragmentation off on import, like the `slipnet://` configs.
- Docs: rewritten README fork section, a "which protocol where" guide from field tests, and the full Persian guide in `docs/VLESS_FA.md`.

### Upgrade

```bash
sudo slipgate update
sudo slipgate tunnel edit --tag vless --front naive     # optional, recommended
sudo slipgate tunnel edit --tag stuntls --front naive   # if StunTLS's own port is blocked
sudo slipgate users configs
```

Includes v1.6.8-vless.5 to vless.7.

## v1.6.8-vless.4 and earlier

- Built-in VLESS over WebSocket (behind Cloudflare or direct), per-user UUIDs, stable links from a server-wide secret, VLESS in the install wizard.
- Fork release and self-update infrastructure; firewall rules inserted ahead of REJECT; `/run/sshd` created before `sshd -t`.
