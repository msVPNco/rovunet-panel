<div dir="rtl">

# هسته‌های Xray

[مستندات](README.md) › هسته‌های Xray

هستهٔ Xray یک سرور Xray جداست که پنل کنار تونل‌ها نصب و مدیریت می‌کند. چهار پروتکلی را می‌دهد که سرورهای داخلی پنل ندارند.

| پروتکل | انتقال | گواهی | مناسب برای |
|---|---|---|---|
| **VLESS + REALITY** (XTLS Vision) | TCP | لازم نیست: دست‌دهی شبیه یک سایت واقعی است (⁦`--target`⁩) | شبکه‌هایی که TLS به سایت‌های شناخته‌شده از آن‌ها رد می‌شود |
| **VLESS + XHTTP** | TCP، درخواست‌های HTTP روی TLS | لازم، معتبر برای ⁦`--sni`⁩ | پشت CDN، یا جایی که WebSocket بسته است |
| **Hysteria 2** | UDP (QUIC) | لازم، معتبر برای ⁦`--sni`⁩ | شبکه‌های پرنوسان یا ناپایدار که UDP از آن‌ها رد می‌شود |
| **WireGuard** | UDP | لازم نیست | شبکه‌هایی که WireGuard را مسدود نمی‌کنند |

<a id="installing-a-core"></a>
## نصب یک هسته

</div>

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

<div dir="rtl">

| گزینه | معنا |
|---|---|
| ⁦`--name`⁩ | نام هسته: ۱ تا ۳۱ نویسه از ⁦`a-z`⁩، ⁦`0-9`⁩ و ⁦`-`⁩ |
| ⁦`--protocol`⁩ | ⁦`reality`⁩، ⁦`xhttp`⁩، ⁦`hysteria2`⁩ یا ⁦`wireguard`⁩ |
| ⁦`--host`⁩ | نام یا IP عمومی که کلاینت به آن وصل می‌شود |
| ⁦`--sni`⁩ | نام گواهی، یا نام سروری که REALITY نشان می‌دهد (پیش‌فرض: ⁦`--host`⁩) |
| ⁦`--target`⁩ | فقط REALITY: سایت TLS واقعی که تقلید می‌شود، به شکل ⁦`host:port`⁩ |
| ⁦`--port`⁩ | پورت شنود (پیش‌فرض 8443)؛ باید آزاد باشد |
| ⁦`--devices`⁩ | تعداد اطلاعات ورود جدا برای کلاینت‌ها، ۱ تا ۱۰۰۰ |
| ⁦`--cert`⁩، ⁦`--key`⁩ | XHTTP و Hysteria 2: مسیر گواهی و کلید خصوصی |

## نصب چه می‌کند

۱. آزاد بودن پورت را روی پروتکل درست بررسی می‌کند (TCP برای REALITY و XHTTP، UDP برای Hysteria 2 و WireGuard).  
۲. نسخهٔ پین‌شدهٔ Xray را که همراه همین release منتشر شده نصب می‌کند و آن را با SHA256SUMS همان release تأیید می‌کند. اگر ⁦`/usr/local/bin/xray`⁩ از قبل باشد، دست نمی‌خورد.  
۳. کانفیگ سرور را می‌نویسد و پیش از فعال شدن، Xray خودش آن را آزمایش می‌کند (⁦`xray run -test`⁩).  
۴. سرویس ⁦`rovunet-core-<name>`⁩ را می‌سازد و اجرا می‌کند و پورت را در فایروال باز می‌کند.  
۵. برای هر دستگاه یک فایل کلاینت در ⁦`/etc/rovunet/core/<name>/clients/`⁩ می‌نویسد.

اگر هر مرحله‌ای شکست بخورد، هرچه ساخته شده بود پاک می‌شود.

## مدیریت هسته‌ها

</div>

```bash
sudo rovunet core-list            # name, protocol, port
sudo rovunet tunnel status        # cores appear with their service state
sudo rovunet core-remove r1       # stop and delete
```

<div dir="rtl">

⁦`core-plan`⁩ همین گزینه‌ها را به‌علاوهٔ ⁦`--output <پوشهٔ جدید>`⁩ می‌گیرد و فقط فایل‌ها را برای استقرار دستی می‌نویسد.

## فایل‌های کلاینت

هر دستگاه اطلاعات ورود جدا می‌گیرد: یک لینک ⁦`vless://`⁩ یا ⁦`hysteria2://`⁩، یا یک فایل ⁦`.conf`⁩ وایرگارد. این‌ها در اپ Rovunet و در کلاینت‌های استاندارد مبتنی بر Xray و WireGuard وارد می‌شوند. یک فایل WireGuard را هرگز هم‌زمان روی دو دستگاه استفاده نکنید.

## بکاپ

هسته‌ها در ⁦`rovunet backup`⁩ ذخیره و با ⁦`restore`⁩ دوباره راه‌اندازی می‌شوند. گواهی‌هایی که با ⁦`--cert`⁩/⁦`--key`⁩ داده‌اید بیرون از پنل‌اند و باید روی سرور جدید در همان مسیر باشند، وگرنه پنل گزارش می‌دهد آن هسته اجرا نشد.

## محدودیت‌ها

- پروفایل‌های هسته به‌صورت پروفایل‌های جدا وارد می‌شوند، نه داخل مجموعهٔ کاربر.
- هسته‌ها مستقل از کاربران پنل‌اند: حذف کاربر، اطلاعات ورود هسته را باطل نمی‌کند؛ هسته را حذف یا دوباره نصب کنید.

بعدی: [عملیات و نگهداری](operations.md)

</div>
