# پنل Rovunet نسخهٔ ۲

پنل لینوکسی مدیریت حساب، اشتراک چندپروتکلی، به‌روزرسانی امضاشده و پشتیبان رمزگذاری‌شده.

[English](README.md) · [Русский](README.ru.md) · [简体中文](README.zh-CN.md) · [دانلود](https://github.com/msVPNco/rovunet-panel/releases) · [یادداشت نسخه](docs/releases/v2.0.0.md)

## نصب کامل و تعداد کانفیگ‌ها

نصب معمولی ۱۵ هستهٔ مدیریت‌شده و سرویس‌های پایهٔ HTTPS، SSH و DNS را راه‌اندازی می‌کند. حساب نامحدود **۳۰ کانفیگ**، حساب فقط زمان‌دار **۲۹ کانفیگ** و حساب حجمی، با زمان یا بدون زمان، **۶ مسیر دارای حسابداری قابل اعمال** دریافت می‌کند. این اعداد تعداد مسیرند، نه تعداد پروتکل مستقل. نصب ناقص، تداخل پورت یا مشکل گواهی باید صریح گزارش شود.

سرور تازهٔ Ubuntu 24.04، دسترسی root و systemd لازم است. فایل‌های amd64 و arm64 جدا هستند؛ آزمون شبیه‌سازی ARM با سخت‌افزار واقعی یکی نیست. دامنهٔ HTTPS باید به سرور اشاره کند و زیردامنه‌های DNS به شنوندهٔ authoritative سرور واگذار شوند. برای UDP و پروتکل‌های مستقیم، پروکسی ابری را خاموش و پورت لازم را در فایروال ابری باز کنید.

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v2.0.0/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="897108ce22f619bad23aecc6792283e4af01df0172b917b28e03082675fe76a6", "Installer checksum mismatch"'
sudo env ROVUNET_RELEASE_TAG=v2.0.0 bash rovunet-install.sh
```


نصاب، امضای Ed25519 بسته و هش هر باینری را بررسی می‌کند. ریشهٔ اعتماد را مستقل نگهداری کنید؛ هش همین صفحه به‌تنهایی احراز اصالت مستقل نیست. نصب کامل پیش‌فرض است؛ `rovunet install --custom` برای انتخاب بخشی از سرویس‌ها و `rovunet complete-setup` برای تکمیل است.

## ترتیب خروجی

**گروه اصلی و سریع ابتدا، مسیرهای کمکی SSH بعد از آن و هر ۱۰ مسیر DNS در انتها** قرار می‌گیرند. این ترتیب نمایشی است و ادعای نتیجهٔ تست سرعت اینترنت نیست. تغییر نام نمایشی، شناسه یا رمز اتصال را تغییر نمی‌دهد.


| Profile group | Default ports | Account policy |
|---|---|---|
| VLESS TLS Vision, REALITY 1/2, XHTTP REALITY, XHTTP TLS | 36444, 9443, 2087, 37444, 2096 TCP | Unlimited, timed, finite |
| VLESS WS TLS | 443 TCP, fronted by NaiveProxy | Unlimited, timed, finite |
| Hysteria 2 (Salamander), TUIC | 8443, 33444 UDP | Unlimited, timed |
| Mieru TCP / UDP | 34444 TCP / 35444 UDP | Unlimited, timed |
| AnyTLS | 32444 TCP | Unlimited, timed |
| Shadowsocks 2022, VLESS Encryption, Trojan TLS | 28444 TCP+UDP, 31444 TCP, 30444 TCP | Unlimited, timed; compatibility protocols |
| AmneziaWG, WireGuard | 51840, 51820 UDP | Unlimited, timed |
| NaiveProxy SOCKS5 / SSH; StunTLS SSH; direct SSH | 443 TCP; direct SSH 22 TCP | Unlimited, timed |
| Soar Direct / SSH | 53 UDP | Direct: unlimited only; SSH: unlimited, timed |
| DNSTT / NoizDNS SOCKS5 / SSH | 53 UDP | Unlimited, timed |
| VayDNS SOCKS5 / SSH; Slipstream SOCKS5 / SSH | 53 UDP | Unlimited, timed |


پشت‌صحنه‌های DNS روی آدرس محلی هستند. SOCKS5 به‌صورت پیش‌فرض فقط محلی است؛ شنوندهٔ عمومی همواره احراز هویت می‌خواهد. Soar Direct کلید مشترک دارد و برای حساب زمان‌دار یا حجمی منتشر نمی‌شود. SS2022 و Trojan بدون Flow در Xray فعلی deprecated هستند و صرفاً مسیر سازگاری اختیاری‌اند. برای قطع نشست باز کاربر لغوشده، هستهٔ مشترک سازگاری بازراه‌اندازی می‌شود؛ کاربران دیگر همان هسته ممکن است لحظه‌ای نیاز به اتصال مجدد داشته باشند.

## حجم، سهمیه و لینک‌های قبلی

حساب‌های جدید از بودجهٔ پایدار و مشترک SQLite استفاده می‌کنند. مسیرهای حجمی قبل از عبور داده سهمیه رزرو می‌کنند. نبودِ آمار تاریخی، مصرف صفر یا ماندهٔ قطعی محسوب نمی‌شود و پوشش ناقص باید در اپ مشخص باشد.

مبنای نمایش حجم ۱۰۲۴ است: **۲۵ گیگابایت = ۲۵٬۶۰۰ مگابایت = ۲۶٬۸۴۳٬۵۴۵٬۶۰۰ بایت**. اپ نباید سهمیهٔ ۲۵ گیگ را با تقسیم ده‌دهی ۲۶٫۸۴ گیگ نمایش بدهد.

به‌روزرسانی معمولی هویت حساب، مصرف ثبت‌شده، دوره، رمزها و نشانی اشتراک را حفظ می‌کند. بعد از ارتقای سرور، همان اشتراک قبلی را در اپ Update کنید. اندروید `.json` و ویندوز `.windows.json` دارد؛ فایل ویندوز جایگزین پشتیبانی واقعی پروتکل در اندروید نیست. سقف دقیق دستگاه هم‌زمان هنوز فعال نشده است. IPهای اضافی می‌توانند تعداد مسیرهای وابسته به آدرس را بیشتر کنند.

## مدیریت، گواهی و بازیابی

```bash
sudo rovunet                         # interactive menu
sudo rovunet users add               # account creation, time and traffic limits
sudo rovunet subscriptions-refresh   # regenerate saved URLs
sudo rovunet certificates --mode check
sudo rovunet certificates --mode recover --tag slipstream-socks
sudo rovunet diag
sudo rovunet backup --file /root/rovunet-backup.age --recipient age1YOUR_PUBLIC_RECIPIENT
```


گزینهٔ ۸ منو بستهٔ امضاشده را بررسی و runtimeها و تنظیمات مرتبط را با ثبت پیشرفت پایدار به‌روز می‌کند. قبل از نگهداری، پشتیبان رمزگذاری‌شده بگیرید. دیتابیس قدیمی را هنگام فعالیت نویسنده‌های مصرف جایگزین نکنید. سازگاری با تمام پیش‌نمایش‌های قدیمی ادعا نمی‌شود؛ معیار نسخهٔ ۲ نصب کامل تازه است.

برای ساخت هویت پشتیبان `rovunet backup-keygen --help` را ببینید؛ کلید خصوصی بازیابی را خارج از سرور نگهداری کنید. `backup-check` اعتبار پشتیبان را می‌سنجد و `restore` برای حساب SQLite، checkpoint قالب ۲ را فقط به‌صورت رو به جلو با همان نصب و مرجع فعلی تطبیق می‌دهد. دیتابیس گم‌شده یا تنظیمات تغییرکرده را با نسخهٔ قدیمی جایگزین نمی‌کند. این checkpoint آنلاین به‌تنهایی پشتیبان بازیابی سرور خالی نیست؛ برای آن، مسیر مستقل و احرازشدهٔ `recovery-checkpoint` لازم است. لینک اشتراک، پشتیبان و لاگ خام خصوصی‌اند.

Caddy گواهی عمومی HTTPS را خودکار تمدید می‌کند. منوی Certificates سلامت را بررسی و بازیابی گواهی منقضی را برای TLS front یا تگ Slipstream اجرا می‌کند. فایل `.trust.json` فقط گواهی عمومی Slipstream را از مبدأ اشتراک احرازشده می‌دهد. اپ باید اعتبارسنجی، نگهداری pin، حفظ آخرین pin معتبر در خطای دانلود و بازیابی صریح pin تغییریافته را پیاده کند؛ هیچ کلید خصوصی به اپ ارسال نمی‌شود.

## حدود آزمون و امنیت

منشأ runtimeها، نسخهٔ دقیق منابع و وابستگی‌ها، وصله‌ها، مجوزها و گزارش بررسی advisory همراه بسته‌اند. نبودِ یافتهٔ شناخته‌شده در snapshot پایگاه آسیب‌پذیری، تضمین امنیت مطلق نیست. مجوز پنل AGPL-3.0 است؛ runtimeها و وابستگی‌هایشان مجوز مستقل دارند، از جمله اجزای GPL در Mieru، sing-box و ترکیب Caddy/Naive.

آزمون کنترل‌شدهٔ انتقال دادهٔ WireGuard و TUIC، عبور آن‌ها در تمام اپراتورها را تضمین نمی‌کند. WireGuard استاندارد و AmneziaWG دو گزینهٔ جدا هستند. نتیجهٔ پنل به‌معنای تأیید همهٔ اپ‌ها، نگهداری فیزیکی کلید در محل مستقل یا همهٔ شبکه‌ها نیست. [قرارداد اپ‌ها](docs/contracts/panel-integration-v1.md) قابلیت‌ها را مشخص می‌کند؛ [دروازه‌های ارزیابی گسترده](docs/release-gates.json) جداگانه دنبال می‌شوند.
