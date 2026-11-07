<div dir="rtl">

<p align="center"><img src="assets/rovunet-logo.png" alt="Rovunet" width="140"></p>

<h1 align="center">پنل رووُنت (Rovunet Panel)</h1>

<p align="center"><b>سرور خودمیزبان و چندپروتکلی برای عبور از فیلترینگ</b><br>یک نصب، همهٔ پروتکل‌های تحت مدیریت، و برای هر کاربر کانفیگ آماده و لینک اشتراک.</p>

<p align="center">
  <a href="https://github.com/msVPNco/rovunet-panel/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/msVPNco/rovunet-panel?sort=date&display_name=tag&label=release&color=0E7C86"></a>
  <img alt="Platform" src="https://img.shields.io/badge/platform-Linux-14304D">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-6D4AFF"></a>
  <a href="https://t.me/RovunetVPN"><img alt="Telegram" src="https://img.shields.io/badge/Telegram-RovunetVPN-2F6FEB?logo=telegram&logoColor=white"></a>
</p>

<p align="center"><a href="README.md">English</a> · <b>فارسی</b> · <a href="README.ru.md">Русский</a> · <a href="README.zh-CN.md">简体中文</a></p>

<p align="center">📘 <a href="docs/guides/Rovunet-Panel-Guide-FA.pdf">راهنمای جامع (PDF)</a> · <a href="docs/fa/README.md">مستندات</a> · <a href="https://github.com/msVPNco/rovunet-android">اپ اندروید</a> · <a href="CHANGELOG.md">تغییرات</a></p>

---

پنل رووُنت ابزاری متن‌باز است که یک سرور لینوکسی را به سرور دسترسی چندپروتکلی تبدیل می‌کند. هر پروتکل را به‌صورت سرویس systemd نصب و مدیریت می‌کند، کاربر می‌سازد و کانفیگ کلاینت‌ها را خروجی می‌دهد، تا هر کاربر چند مسیر اتصال مستقل داشته باشد و به یک پروتکل یا یک وضعیت شبکه وابسته نماند.

## ویژگی‌ها

- **۱۴ پروتکل** در چهار دسته‌ی پروکسی، SSH، تونل DNS و هسته‌ی Xray، با یک ابزار خط فرمان و منوی تعاملی.
- **یک لاگین برای هر کاربر** روی همه‌ی تونل‌ها، و شناسه‌ی VLESS جدا برای هر کاربر که ظرف چند ثانیه قابل لغو است.
- **کانفیگ آماده‌ی وارد کردن**: لینک بومی ⁦`rovunet://⁦`⁩ برای [اپ Rovunet](https://github.com/msVPNco/rovunet-android)، و ⁦`⁩vless://⁦`⁩، ⁦`⁩hysteria2://`⁩ و فایل WireGuard استاندارد برای کلاینت‌های دیگر.
- **سرو روی پورت ۴۴۳**: VLESS و SSH over TLS دامنه، پورت و گواهی Let's Encrypt NaïveProxy را پشت یک سایت پوششی مشترک دارند.
- **عملیات امن**: آپدیت تأییدشده با checksum و برگشت خودکار، آزمایش کانفیگ پیش از استقرار، و بکاپ و بازیابی کامل سرور.
- **عیب‌یابی**: داشبورد زنده، بررسی سلامت، تاریخ انقضای گواهی‌ها و لاگ هر سرویس.
- **سهمیه و لینک اشتراک**: کاربر آزاد یا سهمیه‌ای (روز و/یا گیگابایت، با بررسی هر ۵ دقیقه) و برای هر کاربر یک لینک اشتراک که همهٔ کانفیگ‌هایش را برای منوی Subscriptions اپ می‌آورد.
- **چند آدرس برای یک سرور**: آی‌پی‌های دیگر سرور را اضافه کنید تا لینک‌ها هر کانفیگ وابسته به آدرس را برای هر آدرس بیاورند و مسدود شدن یک آی‌پی کاربران را قطع نکند.
- **هدف خودکار REALITY**: سایت پوششی از بانکی از سایت‌های پرترافیک و کم‌فیلتر انتخاب می‌شود.

## پروتکل‌ها

| پروتکل | روش انتقال | پورت | کاربرد |
|---|---|---|---|
| **VLESS over WebSocket**، پشت NaïveProxy | WebSocket + TLS با گواهی واقعی | ⁦443/tcp⁩ | انتخاب اول هرجا HTTPS کار می‌کند |
| **VLESS over WebSocket**، پشت Cloudflare | WebSocket از طریق CDN کلادفلر | ⁦2053/tcp⁩ | وقتی IP سرور مسدود است |
| **NaïveProxy** | HTTP/2 + TLS با سایت پوششی | ⁦443/tcp⁩ | جایی که TLS به دامنهٔ شما برقرار است |
| **SSH over TLS** (StunTLS) | SSH داخل TLS / WebSocket | ⁦443/tcp⁩ | جایگزین روی همان پورت |
| **SSH** | SSH ساده | ⁦22/tcp⁩ | جایی که پورت ۲۲ باز است |
| **SOCKS5** | SOCKS5 ساده، بدون رمزنگاری | ⁦1080/tcp⁩ | فقط شبکه‌های آزاد |
| **DNSTT** / **NoizDNS** | تونل DNS رمزنگاری‌شده | ⁦53/udp⁩ | شبکه‌های محدودی که DNS از آن‌ها رد می‌شود |
| **VayDNS** | تونل DNS با قالب قابل تنظیم | ⁦53/udp⁩ | مثل DNSTT با تنظیمات بیشتر |
| **Slipstream** | QUIC روی DNS | ⁦53/udp⁩ | آزمایشی |
| **Soar** | DNS چندمسیره با کشف resolver | ⁦53/udp⁩ | resolverهایی که ناهمسان فیلتر شده‌اند |
| **VLESS + REALITY** (Xray) | TCP، شبیه یک سایت TLS واقعی | هر TCP | جایی که TLS به سایت‌های شناخته‌شده رد می‌شود |
| **VLESS + XHTTP** (Xray) | درخواست‌های HTTP روی TLS | هر TCP | پشت CDN، یا جایی که WebSocket بسته است |
| **Hysteria 2** (Xray) | QUIC | هر UDP | شبکه‌های پرنوسان که UDP از آن‌ها رد می‌شود |
| **WireGuard** (Xray) | UDP | هر UDP | جایی که WireGuard مسدود نیست |

مناسب بودن هر پروتکل به سرور، اپراتور و شبکه بستگی دارد؛ این جدول راهنماست، نه تضمین. جزئیات در [پروتکل‌ها](docs/fa/protocols.md).

## شروع سریع

روی یک سرور تازهٔ ⁦Ubuntu 20.04+⁩ یا ⁦Debian 10+⁩ و با دسترسی root:

</div>

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/main/install.sh | sudo bash
```

<div dir="rtl">

نصب‌کننده می‌پرسد کدام پروتکل‌ها فعال شوند، آن‌ها را راه‌اندازی می‌کند، اولین کاربر را می‌سازد و کانفیگ‌های او را چاپ می‌کند. بعد:

</div>

```bash
sudo rovunet                 # interactive menu
sudo rovunet users add       # add a user
sudo rovunet users configs   # print every user's configurations
sudo rovunet diag            # check services, ports, keys and DNS
```

<div dir="rtl">

پیش‌نیازها: دسترسی root، دامنه‌ای که DNS آن را در اختیار دارید، و پورت‌های باز برای پروتکل‌هایی که فعال می‌کنید (دست‌کم ⁦53/udp⁩، ⁦80/tcp⁩ و ⁦443/tcp⁩). [شروع به کار](docs/fa/getting-started.md) را ببینید.

## مستندات

| | |
|---|---|
| 📘 [راهنمای جامع (PDF)](docs/guides/Rovunet-Panel-Guide-FA.pdf) | راهنمای مصور: معماری، انتخاب پروتکل، روند نصب، کاربران و سهمیه، نگهداری. به [English](docs/guides/Rovunet-Panel-Guide-EN.pdf) · [Русский](docs/guides/Rovunet-Panel-Guide-RU.pdf) · [简体中文](docs/guides/Rovunet-Panel-Guide-ZH.pdf) هم هست |
| [شروع به کار](docs/fa/getting-started.md) | پیش‌نیازها، رکوردهای DNS، نصب، اولین تونل |
| [پروتکل‌ها](docs/fa/protocols.md) | همهٔ پروتکل‌ها، پورت‌ها، چیدمان دامنه، مسیریابی |
| [کاربران و کانفیگ‌ها](docs/fa/users-and-configs.md) | کاربران، اشتراک، قالب‌های خروجی، اپ‌های کلاینت |
| [سهمیه و اشتراک](docs/fa/limits-and-subscriptions.md) | محدودیت حجم و زمان، لینک اشتراک، آدرس‌های اضافه |
| [هسته‌های Xray](docs/fa/xray-cores.md) | VLESS + REALITY، VLESS + XHTTP، Hysteria 2، WireGuard |
| [عملیات و نگهداری](docs/fa/operations.md) | به‌روزرسانی، بکاپ و بازیابی، پایش، فایل‌ها و سرویس‌ها |
| [مرجع دستورها](docs/fa/commands.md) | همهٔ دستورها و گزینه‌ها |
| [عیب‌یابی](docs/fa/troubleshooting.md) | نشانه‌ها، علت‌ها و راه‌حل‌ها |

> **وضعیت:** انتشار اولیه و در حال توسعه. پیش از تکیه بر پروتکل‌ها، آن‌ها را در محیط خودتان آزمایش کنید و بکاپ داشته باشید (⁦`sudo rovunet backup`⁩). [وضعیت یکپارچگی](docs/ROVUNET-STATUS.md) را ببینید.

## حمایت

رووُنت کاری داوطلبانه و بدون درآمد است. حمایت مالی با سپاس پذیرفته می‌شود: **USDT روی شبکهٔ Tron (TRC20)** به آدرس ⁦`TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ`⁩ (فقط USDT روی TRC20 بفرستید). پرسش‌ها و اخبار: [t.me/RovunetVPN](https://t.me/RovunetVPN).

## مجوز

AGPL-3.0. پنل رووُنت بر پایهٔ [SlipGate](https://github.com/anonvector/slipgate) ساختهٔ Anonvector است و وابستگی رسمی به آن ندارد؛ انتساب‌های upstream و اجزای ثالث در [NOTICE](NOTICE.md) آمده است.

</div>
