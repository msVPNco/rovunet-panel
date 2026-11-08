<div dir="rtl">

# مستندات پنل رووُنت (Rovunet Panel)

[English](../en/README.md) · **فارسی** · [Русский](../ru/README.md) · [简体中文](../zh-CN/README.md)

پنل رووُنت یک سرور لینوکسی را به سرور دسترسی چندپروتکلی تبدیل می‌کند. هر پروتکل را به‌صورت سرویس systemd نصب و مدیریت می‌کند، کاربر می‌سازد و برای هر کاربر کانفیگ آمادهٔ وارد کردن در کلاینت چاپ می‌کند.

## فهرست

| # | صفحه | محتوا |
|---|---|---|
| ۱ | [شروع به کار](getting-started.md) | پیش‌نیازها، انتخاب سرور، رکوردهای DNS، نصب، اولین تونل |
| ۲ | [پروتکل‌ها](protocols.md) | همهٔ پروتکل‌ها، روش انتقال، پورت‌ها، چیدمان دامنه، مسیریابی |
| ۳ | [کاربران و کانفیگ‌های کلاینت](users-and-configs.md) | مدل کاربر، اشتراک کانفیگ، قالب‌های خروجی، اپ‌های کلاینت |
| ۴ | [هسته‌های Xray](xray-cores.md) | VLESS + REALITY، VLESS + XHTTP، Hysteria 2 و WireGuard |
| ۵ | [عملیات و نگهداری](operations.md) | به‌روزرسانی، بکاپ و بازیابی، پایش، WARP، فایل‌ها و سرویس‌ها، حذف |
| ۶ | [مرجع دستورها](commands.md) | همهٔ دستورها و گزینه‌هایشان |
| ۷ | [عیب‌یابی](troubleshooting.md) | نشانه‌ها، علت‌ها و راه‌حل‌ها |
| ۸ | [سهمیه و لینک اشتراک](limits-and-subscriptions.md) | کاربر آزاد یا سهمیه‌ای، لینک اشتراک، آدرس‌های اضافه |

## قراردادها

- دستورها روی سرور و با دسترسی root اجرا می‌شوند، پس با ⁦`sudo`⁩ نوشته شده‌اند.
- ⁦`example.com⁦`⁩ یعنی دامنهٔ شما و ⁦`⁩SERVER_IP`⁩ یعنی IP عمومی سرور.
- نام پروتکل‌ها همان نام رسمی پروژه‌هایشان است: VLESS، REALITY و XHTTP (پروژهٔ Project X)، Hysteria 2، WireGuard، NaïveProxy، SSH و SOCKS5. DNSTT، NoizDNS، VayDNS، Slipstream و Soar تونل‌های DNS هستند.
- در پنل، **تونل** یعنی یک نمونهٔ پیکربندی‌شده از یک پروتکل با یک تگ (نام)، و **هسته** یعنی یک سرور Xray که با ⁦`core-install`⁩ نصب شده است.

## پروژه

- سورس: [github.com/msVPNco/rovunet-panel](https://github.com/msVPNco/rovunet-panel) · [تغییرات](../../CHANGELOG.md) · [وضعیت یکپارچگی](../ROVUNET-STATUS.md)
- اپ اندروید: [github.com/msVPNco/rovunet-app](https://github.com/msVPNco/rovunet-app)
- جامعه: [t.me/RovunetVPN](https://t.me/RovunetVPN)
- بر پایهٔ [SlipGate](https://github.com/anonvector/slipgate) ساختهٔ Anonvector و بدون وابستگی رسمی به آن؛ [NOTICE](../../NOTICE.md) را ببینید. مجوز: AGPL-3.0.

</div>
