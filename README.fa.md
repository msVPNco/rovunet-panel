<div dir="rtl">

<p align="center">
  <img src="assets/rovunet-logo.png" alt="Rovunet" width="180">
</p>

<h1 align="center">پنل رووُنت (Rovunet Panel)</h1>

<p align="center">
  سرور خودمیزبان و چندپروتکلی برای عبور از فیلترینگ.<br>
  یک نصب، همهٔ پروتکل‌ها، کانفیگ آماده برای هر کاربر.
</p>

<p align="center">
  <a href="README.md">English</a> · <b>فارسی</b> ·
  <a href="https://t.me/RovunetVPN">تلگرام</a> ·
  <a href="https://github.com/msVPNco/rovunet-android">اپ اندروید</a>
</p>

---

پنل رووُنت سمت سرورِ اپ اندرویدی [Rovunet](https://github.com/msVPNco/rovunet-android) را روی هر سرور لینوکسی راه‌اندازی و مدیریت می‌کند: تونل‌های DNS برای سخت‌ترین شبکه‌ها، پروکسی‌های HTTPS که مثل یک سایت معمولی دیده می‌شوند، VLESS، SSH روی TLS و SOCKS5. پنل کاربر می‌سازد، همهٔ سرویس‌ها را با systemd روشن نگه می‌دارد و در پایان نصب، کانفیگ‌های هر کاربر را نمایش می‌دهد.

> **وضعیت: نسخهٔ اولیه.** روی سروری اجرا کنید که در صورت نیاز بتوانید از نو بسازید و از تنظیمات نسخهٔ پشتیبان بگیرید (`slipgate backup`).

## نصب سریع

روی یک سرور تازهٔ Ubuntu 20.04 به بالا یا Debian 10 به بالا، با دسترسی root:

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

نصب‌کننده می‌پرسد کدام پروتکل‌ها فعال شوند، آن‌ها را راه‌اندازی می‌کند، اولین کاربر را می‌سازد و همهٔ کانفیگ‌های او را نمایش می‌دهد. بعد از آن:

```bash
sudo slipgate                 # منوی تعاملی
sudo slipgate users add       # افزودن کاربر
sudo slipgate users configs   # نمایش دوبارهٔ کانفیگ همهٔ کاربران
sudo slipgate diag            # بررسی سرویس‌ها، پورت‌ها، کلیدها و DNS
```

اگر قبلاً SlipGate نصب کرده‌اید، [راهنمای جابه‌جایی سرور](docs/GUIDE.md#quick-start) را ببینید؛ تنظیمات، تونل‌ها و کاربران حفظ می‌شوند.

## پروتکل‌ها

| پروتکل | روش انتقال | پورت | کاربرد |
|---|---|---|---|
| **VLESS پشت NaiveProxy** | WebSocket پشت Caddy با گواهی واقعی | 443/tcp | انتخاب اول: سریع و شبیه یک سایت HTTPS معمولی |
| **NaiveProxy** | پروکسی HTTP/2 با سایت پوششی | 443/tcp | وقتی اتصال TLS به دامنه برقرار است |
| **SSH روی TLS (StunTLS)** | SSH داخل TLS/WebSocket، پشت 443 | 443/tcp | جایگزین VLESS روی همان پورت |
| **SSH مستقیم** | SSH ساده | 22/tcp | سریع، در شبکه‌هایی که پورت 22 باز است |
| **VLESS پشت Cloudflare** | WebSocket از طریق CDN کلادفلر | 2053/tcp | وقتی IP سرور مسدود شده؛ کندتر |
| **DNSTT / NoizDNS** | تونل رمزنگاری‌شده روی DNS | 53/udp | وقتی فقط DNS باز است؛ کند ولی مقاوم |
| **VayDNS** | تونل DNS با قالب قابل تنظیم | 53/udp | مثل DNSTT با تنظیمات بیشتر |
| **Slipstream** | QUIC روی DNS | 53/udp | آزمایشی |
| **Soar** | DNS چندمسیره با کشف resolver | 53/udp | وقتی resolverها به‌طور ناهمسان فیلتر شده‌اند |
| **SOCKS5** | SOCKS5 ساده | 1080/tcp | فقط برای شبکه‌های باز (رمزنگاری ندارد) |

این یادداشت‌ها از آزمایش‌های میدانی در ایران در اواخر ۲۰۲۶ است. شرایط شبکه تغییر می‌کند، به همین دلیل اپ رووُنت مسیرها را خودش آزمایش می‌کند و مسیری را که کار می‌کند انتخاب می‌کند. برای پروتکل‌های Xray (یعنی REALITY، XHTTP، Hysteria2 و WireGuard) فعلاً می‌توانید با `slipgate core-plan` کانفیگ بسازید و دستی راه‌اندازی کنید؛ پشتیبانی کامل در حال آماده شدن است.

## اپ‌های کلاینت

- **[رووُنت برای اندروید](https://github.com/msVPNco/rovunet-android)** (پیشنهادی): لینک‌های `rovunet://` را وارد می‌کند، مسیرهای هر کاربر را در یک مجموعه کنار هم می‌گذارد و خودش آن‌ها را آزمایش می‌کند. [دانلود آخرین نسخهٔ آزمایشی](https://github.com/msVPNco/rovunet-android/releases).
- **کلاینت‌های استاندارد** مثل v2rayNG، Hiddify یا NekoBox: از لینک‌های `vless://` استفاده کنید که پنل کنار هر کانفیگ VLESS نمایش می‌دهد.

```bash
sudo slipgate export-clients --user USER --format rovunet
```

## پیش‌نیازها

- سرور لینوکسی (Ubuntu 20.04 به بالا، Debian 10 به بالا یا مشابه) با دسترسی root
- یک دامنه که DNS آن دست شماست و به سرور اشاره می‌کند (بخش [Domain Layout](docs/GUIDE.md#domain-layout) را ببینید)
- پورت‌های باز برای پروتکل‌هایی که فعال می‌کنید: حداقل 53/udp، 80/tcp و 443/tcp

## مستندات

- [راهنمای کامل](docs/GUIDE.md): نصب، پروتکل‌ها، چیدمان دامنه و همهٔ دستورها (انگلیسی)
- [راهنمای کاربر](docs/USER_GUIDE.md) (انگلیسی و فارسی)
- [راهنمای فارسی VLESS](docs/VLESS_FA.md)
- [یادداشت‌های خروجی](ROVUNET-EXPORT.txt) و [یادداشت‌های Xray](VEYRA-MANAGER.md)
- [فهرست تغییرات](CHANGELOG.md)

## حمایت

رووُنت کاری داوطلبانه و بدون درآمد است. اگر به کارتان آمده، می‌توانید حمایت مالی کنید:

- **USDT (شبکهٔ Tron، استاندارد TRC20):** `TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ`

به این آدرس فقط USDT روی شبکهٔ Tron (TRC20) بفرستید.

پرسش و اخبار: [t.me/RovunetVPN](https://t.me/RovunetVPN)

## مجوز

مجوز AGPL-3.0. پنل رووُنت بر پایهٔ [SlipGate](https://github.com/anonvector/slipgate) ساختهٔ Anonvector است و وابستگی رسمی به آن ندارد؛ فایل [NOTICE](NOTICE.md) را ببینید. برای سازگاری، نام دستور همچنان `slipgate` است.

</div>
