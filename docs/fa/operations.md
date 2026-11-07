<div dir="rtl">

# عملیات و نگهداری

[مستندات](README.md) › عملیات و نگهداری

<a id="updating"></a>
## به‌روزرسانی

</div>

```bash
sudo rovunet update
```

<div dir="rtl">

آپدیت آخرین release را دانلود می‌کند، آن را با SHA256SUMS همان release تأیید می‌کند، باینری را به‌صورت اتمیک جایگزین می‌کند (و اگر نصب خطا بدهد به نسخهٔ قبل برمی‌گردد)، باینری‌های ترنسپورت را تازه می‌کند و سرویس‌ها را دوباره می‌سازد. تونل‌ها، کاربران و کلیدها دست نمی‌خورند و لینک‌های فعلی کلاینت‌ها کار می‌کنند.

### مهاجرت از نام قبلی

سروری که با نام قبلی پنل (دستور ⁦`slipgate⁦`⁩ و ⁦`⁩/etc/slipgate`⁩) نصب شده، یک بار با دستور قدیمی آپدیت می‌شود و خودکار مهاجرت می‌کند:

</div>

```bash
sudo slipgate backup
sudo slipgate update
sudo rovunet tunnel status
```

<div dir="rtl">

مهاجرت سرویس‌های قدیمی را متوقف می‌کند، ⁦`/etc/slipgate⁦`⁩ را به ⁦`⁩/etc/rovunet⁦`⁩ منتقل می‌کند، مسیرهای ذخیره‌شده را بدون دست زدن به هیچ رمزی بازنویسی می‌کند، کاربران و گروه‌های سیستمی را با همان UID و GID تغییرنام می‌دهد، سرویس‌ها را به نام ⁦`⁩rovunet-*⁦`⁩ دوباره می‌سازد، تنظیمات sshd را فقط بعد از تأیید ⁦`⁩sshd -t⁦`⁩ عوض می‌کند، دسترسی ⁦`⁩config.json⁦`⁩ را فقط برای مالک می‌گذارد و دستور را در ⁦`⁩/usr/local/bin/rovunet`⁩ نصب می‌کند. سروری که SlipGate اصلی را اجرا می‌کند هم همین‌طور جابه‌جا می‌شود:

</div>

```bash
curl -fsSL -o /tmp/rovunet https://github.com/msVPNco/rovunet-panel/releases/latest/download/rovunet-linux-amd64
sudo install -m 755 /tmp/rovunet /usr/local/bin/rovunet
sudo rovunet post-update
```

<div dir="rtl">

## بکاپ و بازیابی

بکاپ هرچه برای اجرای همین تنظیمات روی سرور دیگری لازم است در خود دارد: کانفیگ، کلیدهای خصوصی و گواهی‌های همهٔ تونل‌ها، همهٔ کاربران با رمزشان و هسته‌های Xray. با بازیابی روی سرور دیگر، همان تونل‌ها با همان کلیدها برمی‌گردند و کلاینت‌ها بدون لینک تازه کار می‌کنند.

</div>

```bash
sudo rovunet backup                      # asks: save to a file, or print text to copy
sudo rovunet backup /root/rovunet-backup.txt
sudo rovunet restore /root/rovunet-backup.txt
ssh root@old rovunet backup - | ssh root@new rovunet restore - --yes   # server to server
```

<div dir="rtl">

بکاپ یک بلوک متن بین ⁦`-----BEGIN ROVUNET BACKUP-----⁦`⁩ و ⁦`⁩-----END ROVUNET BACKUP-----`⁩ است. همهٔ کلیدهای خصوصی و رمزها در آن است: خصوصی نگهش دارید. بکاپ‌های ساخته‌شده با نام قبلی هم بازیابی می‌شوند. مصرف تاکنونی هر کاربر هم در بکاپ است، پس بازیابی سهمیهٔ کاربران محدود را دوباره کامل نمی‌کند.

بازیابی روی سرور جدید:

۱. پنل را نصب کنید؛ وقتی ویزارد ترنسپورت‌ها را پرسید، Ctrl-C بزنید، چون بازیابی آن‌ها را می‌سازد.  
۲. ⁦`sudo rovunet restore`⁩ را اجرا کنید و متن را بچسبانید، یا مسیر فایل را بدهید.  
۳. رکوردهای DNS را به IP جدید ببرید؛ بازیابی رکوردهایی را که باید عوض شوند فهرست می‌کند (⁦`ns.<domain>`⁩ و دامنه‌های NaïveProxy).

بازیابی باینری‌های ترنسپورت را نصب می‌کند، کلیدها و کانفیگ را می‌نویسد، کاربران را می‌سازد، پورت‌های فایروال را باز می‌کند، اگر WARP روشن بوده دوباره راه‌اندازی‌اش می‌کند، همهٔ سرویس‌ها را اجرا می‌کند و هسته‌های Xray را دوباره بالا می‌آورد. هرچه از قبل روی سرور باشد جایگزین می‌شود، ولی پیش از آن در ⁦`/root/rovunet-pre-restore-<date>.txt`⁩ ذخیره می‌شود.

⁦`rovunet config export⁦`⁩ فقط ⁦`⁩config.json⁦`⁩ را بدون کلیدها چاپ می‌کند: برای جابه‌جایی سرور از ⁦`⁩backup`⁩ استفاده کنید.

## پایش

| دستور | نمایش |
|---|---|
| ⁦`sudo rovunet stats`⁩ | داشبورد زنده: CPU، RAM، ترافیک، اتصال‌ها به تفکیک پروتکل، وضعیت تونل‌ها |
| ⁦`sudo rovunet tunnel status`⁩ | همهٔ تونل‌ها و هسته‌های Xray با وضعیت سرویس |
| ⁦`sudo rovunet tunnel status <tag>`⁩ | جزئیات یک تونل: کلیدها، پورت، MTU، نام، نوع صادرکننده و انقضای گواهی |
| ⁦`sudo rovunet diag`⁩ | بررسی سلامت: سرویس‌ها، پورت‌ها، کلیدها، resolve شدن DNS، ماندگاری بعد از بوت |
| ⁦`sudo rovunet tunnel logs <tag>`⁩ | لاگ سرویس یک تونل |

جزئیات گواهی، گواهی منقضی، رو به انقضا (کمتر از ۱۴ روز) و خراب را علامت می‌زند.

## خروجی WARP (اختیاری)

Cloudflare WARP می‌تواند ترافیک خروجی سرور را ببرد؛ برای وقتی که سایت‌های موردنیاز کاربران IP سرور را رد می‌کنند، یا به‌عنوان یک لایهٔ اضافه. از منوی تعاملی روشنش کنید. کمی تأخیر و وابستگی به Cloudflare اضافه می‌کند. Soar با بک‌اند direct، VLESS و هسته‌های Xray از WARP رد نمی‌شوند.

## فایروال

پنل پورت‌های تونل‌ها و هسته‌ها را با UFW، firewalld یا iptables، هرکدام که فعال باشد، باز می‌کند. با iptables، پورت‌هایی را که باز کرده در ⁦`/etc/iptables/rovunet-ports`⁩ ثبت می‌کند و هنگام بوت فقط همان‌ها را درج می‌کند و به قوانین ابزارهای دیگر (Docker، fail2ban) دست نمی‌زند.

## فایل‌ها و سرویس‌ها

| مسیر | محتوا |
|---|---|
| ⁦`/etc/rovunet/config.json`⁩ | کانفیگ اصلی: تونل‌ها، کاربران، مسیریابی (فقط مالک) |
| ⁦`/etc/rovunet/tunnels/<tag>/`⁩ | کلیدها، گواهی‌ها و کانفیگ یک تونل |
| ⁦`/etc/rovunet/vless.secret`⁩ | کلید مخفی سرور که UUID و مسیر VLESS از آن ساخته می‌شود |
| ⁦`/etc/rovunet/core/<name>/⁦`⁩ | یک هستهٔ Xray: ⁦`⁩server.json⁦`⁩، ⁦`⁩meta⁦`⁩ و ⁦`⁩clients/`⁩ هر دستگاه |
| ⁦`/usr/local/bin/rovunet`⁩ | پنل، همراه سرورهای داخلی SOCKS5، Soar، SSH over TLS و VLESS |
| ⁦`/usr/local/bin/dnstt-server⁦`⁩، ⁦`⁩slipstream-server⁦`⁩، ⁦`⁩vaydns-server⁦`⁩، ⁦`⁩caddy-naive`⁩ | باینری‌های ترنسپورت |
| ⁦`/usr/local/bin/xray⁦`⁩ | هستهٔ پین‌شدهٔ Xray برای ⁦`⁩core-install`⁩ |
| ⁦`/etc/iptables/rovunet-ports`⁩ | پورت‌هایی که پنل در iptables باز کرده |

| یونیت systemd | نقش |
|---|---|
| ⁦`rovunet-dnsrouter`⁩ | روتر DNS روی پورت ۵۳ |
| ⁦`rovunet-socks5`⁩ | پروکسی SOCKS5 داخلی |
| ⁦`rovunet-<tag>`⁩ | یکی برای هر تونل |
| ⁦`rovunet-core-<name>`⁩ | یکی برای هر هستهٔ Xray |
| ⁦`rovunet-iptables`⁩ | بازگرداندن پورت‌های فایروال پنل هنگام بوت |

</div>

```bash
journalctl -u rovunet-mytunnel -n 200 --no-pager
```

<div dir="rtl">

## حذف

</div>

```bash
sudo rovunet uninstall
```

<div dir="rtl">

همهٔ سرویس‌ها، کانفیگ‌ها، باینری‌ها، کاربر سیستمی و کاربران SSH تونل را حذف می‌کند. رکوردهای DNS دست نمی‌خورند؛ اگر لازم است، آن‌ها را در پنل ارائه‌دهندهٔ DNS پاک کنید.

بعدی: [مرجع دستورها](commands.md)

</div>
