<div dir="rtl">

# مرجع دستورها

[مستندات](README.md) › مرجع دستورها

همهٔ دستورها را با root اجرا کنید (⁦`sudo rovunet …⁦`⁩). گزینهٔ لازمی که داده نشود به‌صورت تعاملی پرسیده می‌شود؛ ⁦`⁩rovunet`⁩ بدون آرگومان منو را باز می‌کند.

## سیستم

| دستور | توضیح |
|---|---|
| ⁦`rovunet`⁩ | منوی تعاملی |
| ⁦`rovunet install [--bin-dir DIR]`⁩ | نصب پیش‌نیازها و راه‌اندازی پروتکل‌های انتخاب‌شده |
| ⁦`rovunet update`⁩ | آپدیت به آخرین release و ساختن دوبارهٔ سرویس‌ها |
| ⁦`rovunet restart`⁩ | راه‌اندازی دوبارهٔ روتر DNS، پروکسی SOCKS5 و همهٔ تونل‌ها |
| ⁦`rovunet stats`⁩ | داشبورد زنده |
| ⁦`rovunet diag`⁩ | عیب‌یابی |
| ⁦`rovunet mtu VALUE`⁩ | تنظیم MTU همهٔ تونل‌های DNSTT، NoizDNS و VayDNS |
| ⁦`rovunet backup [FILE \| -]`⁩ | بکاپ کانفیگ، کلیدها، کاربران و هسته‌های Xray |
| ⁦`rovunet restore [FILE \| -] [--yes]⁦`⁩ | بازیابی بکاپ؛ ⁦`⁩-`⁩ از ورودی استاندارد می‌خواند |
| ⁦`rovunet uninstall`⁩ | حذف هرچه پنل نصب کرده |
| ⁦`rovunet --version`⁩ | نمایش نسخه |

## کاربران

| دستور | توضیح |
|---|---|
| ⁦`rovunet users`⁩ | منوی کاربران |
| ⁦`rovunet users add`⁩ | افزودن کاربر (می‌پرسد آزاد یا سهمیه‌ای، بعد روز و گیگابایت) |
| ⁦`rovunet users renew`⁩ | برگرداندن کاربر متوقف‌شده (منقضی یا بیش از سهمیه)، مصرف از صفر |
| ⁦`rovunet expire-users`⁩ | متوقف کردن کاربران منقضی یا بیش از سهمیه و به‌روز کردن لینک‌های اشتراک (با تایمر هر ۵ دقیقه) |
| ⁦`rovunet users bulk_add --count N --prefix P`⁩ | افزودن تا ۵۰۰ کاربر با رمز تصادفی |
| ⁦`rovunet users remove`⁩ | حذف کاربر |
| ⁦`rovunet users list`⁩ | کاربران و کانفیگ‌هایشان |
| ⁦`rovunet users configs`⁩ | همهٔ کانفیگ‌های همهٔ کاربران |
| ⁦`rovunet export-clients --user U --format rovunet [--annotated]`⁩ | لینک‌های یک کاربر برای اسکریپت |

## تونل‌ها

| دستور | توضیح |
|---|---|
| ⁦`rovunet tunnel add`⁩ | افزودن تونل (گزینه‌ها پایین‌تر) |
| ⁦`rovunet tunnel edit --tag T`⁩ | تغییر تونل (گزینه‌ها پایین‌تر) |
| ⁦`rovunet tunnel remove T⁦`⁩ / ⁦`⁩--all`⁩ | حذف یک تونل یا همه |
| ⁦`rovunet tunnel start T⁦`⁩ / ⁦`⁩stop T`⁩ | اجرا یا توقف تونل |
| ⁦`rovunet tunnel status [T]`⁩ | همهٔ تونل‌ها و هسته‌ها، یا جزئیات یک تونل |
| ⁦`rovunet tunnel share T [--user U]`⁩ | چاپ کانفیگ یک تونل |
| ⁦`rovunet tunnel logs T`⁩ | لاگ سرویس یک تونل |

### گزینه‌های ⁦`tunnel add`⁩

| گزینه | برای | معنا |
|---|---|---|
| ⁦`--transport⁦`⁩ | همه | ⁦`⁩dnstt⁦`⁩، ⁦`⁩vaydns⁦`⁩، ⁦`⁩slipstream⁦`⁩، ⁦`⁩soar⁦`⁩، ⁦`⁩naive⁦`⁩، ⁦`⁩stuntls⁦`⁩، ⁦`⁩vless⁦`⁩، ⁦`⁩direct-ssh⁦`⁩، ⁦`⁩direct-socks5⁦`⁩، ⁦`⁩external`⁩ |
| ⁦`--backend⁦`⁩ | تونل‌های DNS، NaïveProxy | ⁦`⁩socks⁦`⁩، ⁦`⁩ssh⁦`⁩، ⁦`⁩both⁦`⁩؛ برای Soar: ⁦`⁩direct⁦`⁩ یا ⁦`⁩ssh`⁩ |
| ⁦`--tag`⁩ | همه | نام تونل |
| ⁦`--domain`⁩ | هرکدام که دامنه دارد | دامنه یا زیردامنهٔ تونل |
| ⁦`--private-key⁦`⁩، ⁦`⁩--public-key`⁩ | DNSTT، VayDNS، Soar | وارد کردن کلید موجود |
| ⁦`--record-type⁦`⁩، ⁦`⁩--idle-timeout⁦`⁩، ⁦`⁩--keep-alive⁦`⁩، ⁦`⁩--clientid-size⁦`⁩، ⁦`⁩--queue-size`⁩ | VayDNS | تنظیم قالب انتقال |
| ⁦`--allow-ports⁦`⁩ | Soar با direct | پورت‌های مقصد مجاز، مثل ⁦`⁩80,443⁦`⁩ یا ⁦`⁩any`⁩ |
| ⁦`--email⁦`⁩، ⁦`⁩--decoy-url`⁩ | NaïveProxy | ایمیل Let's Encrypt، سایت پوششی |
| ⁦`--cdn⁦`⁩، ⁦`⁩--tls⁦`⁩، ⁦`⁩--listen-port⁦`⁩، ⁦`⁩--cdn-address⁦`⁩، ⁦`⁩--block-ports`⁩ | VLESS | حالت Cloudflare، TLS، پورت شنود، IP تمیز CDN، پورت‌های مقصد مسدود |
| ⁦`--port`⁩ | External | پورت UDP محلی مقصد |

### گزینه‌های ⁦`tunnel edit`⁩

| گزینه | معنا |
|---|---|
| ⁦`--new-tag`⁩ | تغییر نام تونل |
| ⁦`--mtu`⁩ | MTU تونل DNS |
| ⁦`--front NAIVE_TAG \| -`⁩ | بردن VLESS یا SSH over TLS پشت پورت ۴۴۳ِ NaïveProxy، یا برگرداندن آن |
| ⁦`--cdn-address IP \| -`⁩ | IP تمیز Cloudflare برای کانفیگ‌های بعدی VLESS |
| گزینه‌های تنظیم VayDNS | مثل ⁦`tunnel add`⁩ |

## هسته‌های Xray

| دستور | توضیح |
|---|---|
| ⁦`rovunet core-install --name N --protocol P --host H [options]`⁩ | نصب یک هسته برای همهٔ کاربران؛ [هسته‌های Xray](xray-cores.md#installing-a-core) را ببینید |
| ⁦`rovunet core-list`⁩ | هسته‌های نصب‌شده |
| ⁦`rovunet core-remove N`⁩ | حذف یک هسته |
| ⁦`rovunet core-plan --protocol P --host H --output DIR [options]`⁩ | فقط نوشتن فایل‌های یک هسته |

## مسیریابی DNS

| دستور | توضیح |
|---|---|
| ⁦`rovunet router status`⁩ | حالت مسیریابی و مسیرها |
| ⁦`rovunet router mode`⁩ | جابه‌جایی بین حالت single و multi |
| ⁦`rovunet router switch`⁩ | تغییر تونل فعال در حالت single |

## پیکربندی

| دستور | توضیح |
|---|---|
| ⁦`rovunet config export⁦`⁩ | چاپ ⁦`⁩config.json`⁩ بدون کلیدها |
| ⁦`rovunet config import PATH⁦`⁩ | جایگزینی ⁦`⁩config.json`⁩ (بدون کلیدها) |
| ⁦`rovunet config address IP \| auto`⁩ | آدرس عمومی برای کانفیگ‌های SSH، SOCKS5 و SSH over TLS؛ بعد از ویرگول می‌توان آدرس‌های دیگر همین سرور را اضافه کرد، مثلاً ⁦`auto,198.51.100.9`⁩ |

## داخلی

برای سرویس‌های systemd: ⁦`rovunet dnsrouter serve⁦`⁩، ⁦`⁩rovunet socks serve⁦`⁩، ⁦`⁩rovunet stuntls serve⁦`⁩، ⁦`⁩rovunet vless serve⁦`⁩، ⁦`⁩rovunet post-update`⁩.

بعدی: [عیب‌یابی](troubleshooting.md)

</div>
