# وضعیت یکپارچگی پنل و اپ

[انتشارهای عمومی اپ](https://github.com/msVPNco/rovunet-android/releases) · [وضعیت دقیق اپ](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md) · [راهنمای ارتقا](https://github.com/msVPNco/rovunet-android/blob/main/docs/UPGRADE.md)

نسخهٔ فعلی اپ را فقط از [وضعیت اپ](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md) بخوانید؛ این سند نسخه را تکرار نمی‌کند تا دوباره کهنه نشود. نسخه‌های عمومی از آلفا۱۴ به بعد یک هویت تولیدی ثابت دارند (مالک پروژه تعویض امضای قدیمی را پیش از استقرار کاربران تأیید کرد) و روی هم بدون حذف برنامه نصب می‌شوند. فایل نصب را از انتشار گیت‌هاب بگیرید. نصب‌های آلفا۱۱ با امضای قدیمی ارتقای مستقیم ندارند؛ نسخه‌های عمومی بعدی همین کلید را حفظ می‌کنند. بسته‌های خصوصی Rovunet Test مستقل‌اند. [هویت و سیاست بازیابی امضا](https://github.com/msVPNco/rovunet-android/blob/main/docs/SIGNING-IDENTITY.md).

## قرارداد خروجی و اتصال

قالب rovunet:// فرادادهٔ مجموعه را حمل می‌کند؛ لینک استاندارد VLESS برای کلاینت‌های سازگار هم ارائه می‌شود. اپ مسیر انتخاب‌شده را بدون تست اجباری وصل می‌کند؛ تست HTTPS جداگانه از داخل تونل اجرا و پیش از ورود تست به جایگزین DNS رضایت گرفته می‌شود. انتخاب‌های ذخیره‌شده حفظ می‌شوند؛ Soar و Soar+SSH جدید در ویرایشگر اپ UDP + TCP را پیش‌فرض دارند، ولی مقدار صریح خروجی پنل یا کانفیگ موجود بی‌صدا تغییر نمی‌کند.

هسته‌های Xray (REALITY، XHTTP، Hysteria2 و WireGuard) با `rovunet core-install` نصب و مدیریت می‌شوند؛ `core-plan` همچنان فقط فایل برای استقرار دستی می‌سازد. CI اپ، خروجی واقعی هر چهار پروتکل پنل و خروجی کامل کاربرها (Soar، VLESS و رمزهای v33) را با importer واقعی اپ آزمایش می‌کند.

## عیب‌یابی مشترک

برای SOCKS باید greeting/auth، کد CONNECT، DNS و عبور داده جدا بررسی شوند؛ موفقیت اتصال TCP یا شکست هر دو DNS، علت را قطعی نمی‌کند. برای Soar+SSH، سالم بودن تونل پایه کافی نیست: مقصد و پورت SSH باید در تنظیم واقعی سرور مجاز باشند. مقایسهٔ auto/on باید همان build، شبکه، سرور و resolver را ثبت کند؛ سیاست پیکربندی، شاهد موفقیت مسیر نیست. اطلاعات حساس را از لاگ تفصیلی حذف کنید و فقط گزارش امن را عمومی به اشتراک بگذارید.

## کارهای پیش از استقرار گسترده

- (انجام شد در پنل ۱.۱۳) endpoint اشتراک HTTPS برای هر کاربر؛ باقی مانده: شناسهٔ پایدار سرور/کاربر مستقل از IP و دامنه.
- اتصال کانفیگ‌های هستهٔ Xray به مجموعهٔ همان کاربر (فعلاً هر دستگاه جدا و بدون گروه وارد اپ می‌شود).
- آزمایش هسته‌های Xray و مهاجرت از نام قبلی روی سرور و دستگاه واقعی.
- بررسی چندکاربره، منابع، مجوز مقصدهای سرویس‌ها و بازیابی پس از خطا.

موفقیت یک اتصال، کارکرد برای صدها کاربر یا همهٔ شبکه‌ها را اثبات نمی‌کند. بازخورد نسخهٔ خصوصی، شاهد انتشار عمومی یا تأیید مسیر مهاجرت نیست.

پشتیبانی: https://t.me/RovunetVPN

حمایت اختیاری از توسعه: USDT فقط روی Tron/TRC20، آدرس TG3Mhou3pdq9KaFgJSHjDiDx2ahhJuHvbJ. استفاده از برنامه به پرداخت وابسته نیست.

## English

For the current app version, read the [app status](https://github.com/msVPNco/rovunet-android/blob/main/docs/STATUS.md); this document does not repeat it so it cannot drift again. Public releases from alpha14 onward share one managed production signer (explicitly authorized before user deployment) and install over each other without uninstalling. It is a fresh install relative to the historical alpha11 signer; subsequent direct APKs must reuse the pinned identity. Private candidates remain separate from public updates. Read the app's current status and migration notes. Native exports retain collection metadata; explicit connection does not require a ping, and HTTPS tests are separate. Client protocol support does not imply panel-managed deployment. Xray cores are installed and managed with `rovunet core-install`, and the app's CI imports the panel's real exports (all four core protocols, Soar, VLESS, v33 credentials). SOCKS/Soar field diagnosis, a stable server/user identity independent of IP and domain (the per-user subscription link has existed since panel v1.13), grouping core profiles into the user's collection, real-server and real-device testing, and multi-user resource/recovery checks remain necessary.
