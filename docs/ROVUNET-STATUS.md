# وضعیت یکپارچگی پنل و اپ

[انتشارهای عمومی اپ](https://github.com/msVPNco/rovunet-android/releases) · [وضعیت دقیق اپ](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md) · [راهنمای ارتقا](https://github.com/msVPNco/rovunet-android/blob/main/docs/UPGRADE.md)

سورس اپ برای آلفا۱۴/کد ۲۵ آماده می‌شود؛ انتشار عمومی قبلی آلفا۱۱ است. این صفحه انتشار APK امضاشدهٔ تازه را ادعا نمی‌کند. بسته‌های خصوصی Rovunet Test مستقل‌اند و آپدیت مستقیم نسخهٔ عمومی نیستند. امضای تولیدی و مسیر مهاجرت باید قبل از انتشار تعیین شوند.

## قرارداد خروجی و اتصال

قالب rovunet:// فرادادهٔ مجموعه را حمل می‌کند؛ لینک استاندارد VLESS برای کلاینت‌های سازگار هم ارائه می‌شود. اپ مسیر انتخاب‌شده را بدون تست اجباری وصل می‌کند؛ تست HTTPS جداگانه از داخل تونل اجرا و پیش از ورود تست به جایگزین DNS رضایت گرفته می‌شود. انتخاب‌های ذخیره‌شده حفظ می‌شوند؛ Soar و Soar+SSH جدید در ویرایشگر اپ UDP + TCP را پیش‌فرض دارند، ولی مقدار صریح خروجی پنل یا کانفیگ موجود بی‌صدا تغییر نمی‌کند.

پشتیبانی کلاینت از یک مسیر به معنی استقرار خودکار آن توسط پنل نیست. core-plan برای تولید برنامه/کانفیگ استقرار دستی است و مدیریت خودکار همهٔ مسیرهای هسته را ثابت نمی‌کند.

## عیب‌یابی مشترک

برای SOCKS باید greeting/auth، کد CONNECT، DNS و عبور داده جدا بررسی شوند؛ موفقیت اتصال TCP یا شکست هر دو DNS، علت را قطعی نمی‌کند. برای Soar+SSH، سالم بودن تونل پایه کافی نیست: مقصد و پورت SSH باید در تنظیم واقعی سرور مجاز باشند. مقایسهٔ auto/on باید همان build، شبکه، سرور و resolver را ثبت کند؛ سیاست پیکربندی، شاهد موفقیت مسیر نیست. اطلاعات حساس را از لاگ تفصیلی حذف کنید و فقط گزارش امن را عمومی به اشتراک بگذارید.

## کارهای پیش از استقرار گسترده

- endpoint اشتراک HTTPS و شناسهٔ پایدار سرور/کاربر مستقل از IP و دامنه.
- اتصال خروجی core-plan به مجموعهٔ همان کاربر.
- تخصیص پورت بدون برخورد Caddy/REALITY روی TCP و HTTP/3/Hysteria2 روی UDP.
- اجرای قرارداد واقعی Go exporter/Kotlin importer در CI و ثبت نتیجهٔ همان کامیت.
- بررسی چندکاربره، منابع، مجوز مقصدهای سرویس‌ها و بازیابی پس از خطا.

موفقیت یک اتصال، کارکرد برای صدها کاربر یا همهٔ شبکه‌ها را اثبات نمی‌کند. بازخورد نسخهٔ خصوصی، شاهد انتشار عمومی یا تأیید مسیر مهاجرت نیست.

پشتیبانی: https://t.me/RovunetVPN

حمایت اختیاری از توسعه: USDT فقط روی Tron/TRC20، آدرس TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ. استفاده از برنامه به پرداخت وابسته نیست.

## English

Android source is preparing alpha14/code 25; alpha11 is the prior public release. Private candidates are separate from public updates. Read the app's current status and migration notes. Native exports retain collection metadata; explicit connection does not require a ping, and HTTPS tests are separate. Client protocol support does not imply panel-managed deployment. SOCKS/Soar field diagnosis, stable subscription identity, core-plan integration, port allocation and multi-user resource/recovery checks remain necessary.
