<div dir="rtl">

# عیب‌یابی

[مستندات](README.md) › عیب‌یابی

با ⁦`sudo rovunet diag⁦`⁩ شروع کنید، بعد لاگ سرویس تونل موردنظر را ببینید (⁦`⁩sudo rovunet tunnel logs <tag>`⁩).

## سرور

| نشانه | علت محتمل | راه‌حل |
|---|---|---|
| تونل ⁦`active⁦`⁩ نیست | مشکل پورت، کلید یا کانفیگ | ⁦`⁩sudo rovunet diag⁦`⁩، بعد ⁦`⁩sudo rovunet tunnel logs <tag>`⁩ |
| پورت ۵۳ اشغال است | stub listener سرویس systemd-resolved | پنل آن را خاموش می‌کند؛ اگر برگشت: ⁦`sudo systemctl disable --now systemd-resolved⁦`⁩ و در ⁦`⁩/etc/resolv.conf⁦`⁩ بنویسید ⁦`⁩nameserver 1.1.1.1`⁩ |
| ⁦`dig`⁩ به سرور timeout می‌شود | ارائه‌دهنده ورودی UDP/53 را بسته | ارائه‌دهنده را عوض کنید، یا از پروتکل‌های روی ۴۴۳ استفاده کنید |
| Let's Encrypt شکست می‌خورد (NaïveProxy) | رکورد A هنوز اعمال نشده، یا پورت ۸۰ بسته است | صبر کنید، پورت ۸۰ را باز کنید، بعد ⁦`sudo rovunet restart`⁩ |
| از چند تونل DNS فقط یکی کار می‌کند | حالت مسیریابی single | ⁦`sudo rovunet router status⁦`⁩؛ با ⁦`⁩router mode`⁩ عوضش کنید |
| در ⁦`tunnel add`⁩ یک پورت TCP رد می‌شود | برنامهٔ دیگری روی آن گوش می‌دهد | پورت دیگری بگیرید یا آن برنامه را متوقف کنید |
| گواهی در ⁦`tunnel status`⁩ علامت خورده | منقضی، رو به انقضا یا خراب | تمدید یا جایگزینش کنید، بعد تونل را دوباره اجرا کنید |
| بعد از آپدیت، سرویسی هنوز نام قدیمی دارد | مهاجرت نیمه‌کاره مانده | ⁦`sudo rovunet post-update`⁩ را دوباره اجرا کنید؛ تکرارش بی‌خطر است |

## کلاینت

| نشانه | علت محتمل | راه‌حل |
|---|---|---|
| وصل می‌شود ولی ترافیک رد نمی‌شود | مسیر شبکه یا کلاینت | از خود سرور تست کنید: ⁦`curl -x socks5h://USER:PASS@SERVER_IP:1080 https://api.ipify.org`⁩. اگر آن‌جا کار کرد، سرور سالم است |
| ⁦`Connection was reset⁦`⁩ | فیلترینگ اتصال روی آن پورت را قطع می‌کند | با ⁦`⁩tcpdump -ni any tcp port 8443`⁩ ببینید بسته‌ای می‌رسد یا نه؛ اگر نه، تونل را پشت NaïveProxy ببرید |
| کاربر وارد نمی‌شود | اطلاعات ورود یا بک‌اند اشتباه | ⁦`sudo rovunet users list`⁩؛ بک‌اند تونل را بررسی کنید |
| VLESS وصل می‌شود ولی سایتی باز نمی‌شود | کاربر حذف شده یا رمزش عوض شده | کانفیگ تازه بدهید |
| «WebSocket upgrade failed» | مسیر داخل لینک با سرور یکی نیست | کانفیگ تازه بدهید |
| خطای ۵۲۱ یا ۵۲۲ کلودفلر | کلودفلر به سرور نمی‌رسد | پورت را در فایروال ارائه‌دهنده باز کنید |
| خطای ۵۲۵ یا ۵۲۶ کلودفلر | حالت SSL اشتباه | SSL/TLS را روی **Full** بگذارید |
| خطای v2rayNG روی VLESS | Mux یا XUDP روشن است | Mux را خاموش کنید؛ فقط UDP ساده پشتیبانی می‌شود |
| تونل DNS کند است | MTU برای مسیر بزرگ است | ⁦`sudo rovunet mtu 1200`⁩ (برای لینک خیلی ضعیف 1100) |
| کلیدی باید عوض می‌شد | کانفیگ‌های قبلی کلید قدیمی دارند | ⁦`sudo rovunet tunnel edit --tag <tag>`⁩، بعد کانفیگ تازه بدهید |

## لاگ‌ها

</div>

```bash
journalctl -u rovunet-<tag> -n 200 --no-pager
journalctl -u rovunet-dnsrouter -n 200 --no-pager
journalctl -u rovunet-socks5 -n 200 --no-pager
journalctl -u rovunet-core-<name> -n 200 --no-pager
```

<div dir="rtl">

پرسش‌ها: [t.me/RovunetVPN](https://t.me/RovunetVPN)

</div>
