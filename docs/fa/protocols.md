<div dir="rtl">

# پروتکل‌ها

[مستندات](README.md) › پروتکل‌ها

## نمای کلی

| پروتکل | خانواده | روش انتقال | پورت پیش‌فرض | گواهی | ترنسپورت در پنل |
|---|---|---|---|---|---|
| [VLESS روی WebSocket](#vless-over-websocket) | پروکسی | WebSocket + TLS | ⁦443/tcp⁩ پشت Naïve، یا ⁦2053/tcp⁩ | Let's Encrypt پشت Naïve، یا خودامضا | ⁦`vless`⁩ |
| [NaïveProxy](#naiveproxy) | پروکسی | HTTP/2 + TLS | ⁦443/tcp⁩ | Let's Encrypt | ⁦`naive`⁩ |
| [SSH over TLS](#ssh-over-tls) (StunTLS) | SSH | TLS / WebSocket | ⁦443/tcp⁩ پشت Naïve، یا ⁦8443/tcp⁩ | خودامضا؛ پشت Naïve: Let's Encrypt | ⁦`stuntls`⁩ |
| [SSH](#ssh-and-socks5) | SSH | TCP | ⁦22/tcp⁩ | — | ⁦`direct-ssh`⁩ |
| [SOCKS5](#ssh-and-socks5) | پروکسی | TCP بدون رمزنگاری | ⁦1080/tcp⁩ | — | ⁦`direct-socks5`⁩ |
| [DNSTT](#dns-tunnels) / NoizDNS | تونل DNS | DNS، Curve25519 | ⁦53/udp⁩ + ⁦53/tcp⁩ | — | ⁦`dnstt`⁩ |
| [VayDNS](#dns-tunnels) | تونل DNS | DNS، KCP + Curve25519 | ⁦53/udp⁩ + ⁦53/tcp⁩ | — | ⁦`vaydns`⁩ |
| [Slipstream](#dns-tunnels) | تونل DNS | QUIC روی DNS | ⁦53/udp⁩ + ⁦53/tcp⁩ | خودامضا | ⁦`slipstream`⁩ |
| [Soar](#dns-tunnels) | تونل DNS | DNS چندمسیره، X25519 + Ed25519 | ⁦53/udp⁩ + ⁦53/tcp⁩ | — | ⁦`soar`⁩ |
| [VLESS + REALITY](xray-cores.md) | هستهٔ Xray | TCP، REALITY + XTLS Vision | هر پورت آزاد TCP | لازم نیست (شبیه یک سایت واقعی) | ⁦`core-install`⁩ |
| [VLESS + XHTTP](xray-cores.md) | هستهٔ Xray | درخواست‌های HTTP + TLS | هر پورت آزاد TCP | لازم | ⁦`core-install`⁩ |
| [Hysteria 2](xray-cores.md) | هستهٔ Xray | QUIC | هر پورت آزاد UDP | لازم | ⁦`core-install`⁩ |
| [WireGuard](xray-cores.md) | هستهٔ Xray | UDP | هر پورت آزاد UDP | — | ⁦`core-install`⁩ |
| [مسیر DNS خارجی](#external-dns-route) | سفارشی | DNS به یک پورت محلی | ⁦53/udp⁩ | — | ⁦`external`⁩ |

## انتخاب پروتکل

نتیجهٔ تست میدانی روی یک سرور از داخل ایران، پاییز ۱۴۰۵. نقطهٔ شروع است، نه تضمین؛ شبکه‌ها متفاوت‌اند و تغییر می‌کنند.

| وضعیت | اول امتحان کنید |
|---|---|
| HTTPS کار می‌کند | VLESS پشت NaïveProxy روی ۴۴۳: سریع، پایدار و شبیه یک سایت معمولی |
| پورت ۲۲ باز است | SSH: سریع |
| IP سرور مسدود است | VLESS پشت Cloudflare: کار می‌کند، به‌طور محسوس کندتر |
| TLS به سایت‌های شناخته‌شده رد می‌شود | VLESS + REALITY |
| شبکهٔ پرنوسان که UDP از آن رد می‌شود | Hysteria 2 |
| فقط DNS رد می‌شود | VayDNS یا DNSTT: کند، ولی کار می‌کنند |
| SSH over TLS روی پورت خودش reset می‌شود | آن را پشت NaïveProxy روی ۴۴۳ ببرید |
| فقط شبکه‌های آزاد | SOCKS5: رمزنگاری ندارد و شبکه‌های فیلترشده قطعش می‌کنند |

## بک‌اندها

تونل‌های DNS و NaïveProxy ترافیک رمزگشایی‌شده را به یک بک‌اند می‌دهند:

| بک‌اند | رفتار |
|---|---|
| ⁦`socks`⁩ | پروکسی SOCKS5 داخلی پنل؛ ساده و سریع |
| ⁦`ssh`⁩ | OpenSSH خود سرور ترافیک را می‌رساند، پس هر کاربر لاگین SSH می‌خواهد |
| ⁦`both⁦`⁩ | دو تونل می‌سازد، ⁦`⁩<tag>-socks⁦`⁩ و ⁦`⁩<tag>-ssh⁦`⁩؛ زیردامنهٔ نسخهٔ SSH یک ⁦`⁩s`⁩ اضافه می‌گیرد |
| ⁦`direct`⁩ | فقط Soar: سرور خودش به مقصد هر جریان وصل می‌شود |

<a id="vless-over-websocket"></a>
## VLESS روی WebSocket

پنل خودش VLESS روی WebSocket را سرو می‌کند. هر کاربر یک UUID می‌گیرد که از نام کاربری، رمز و یک کلید مخفی سرور (⁦`/etc/rovunet/vless.secret⁦`⁩) ساخته می‌شود: حذف کاربر یا تغییر رمز آن را ظرف چند ثانیه باطل می‌کند و کانفیگ اشتراکی هرگز رمز را در خود ندارد. جریان‌ها فقط به آدرس‌های عمومی می‌رسند و پورت ۲۵ به‌طور پیش‌فرض بسته است (⁦`⁩--block-ports`⁩). TCP و UDP ساده پشتیبانی می‌شوند، Mux و XUDP نه.

سه راه رسیدن به آن:

| حالت | روش | کاربرد |
|---|---|---|
| **پشت NaïveProxy** (پیشنهادی) | Caddy آن را روی ۴۴۳ با گواهی Let's Encrypt خودش می‌رساند؛ بقیهٔ مسیرها سایت پوششی را می‌بینند | پیش‌فرض وقتی NaïveProxy نصب است |
| **پشت Cloudflare** | کلاینت به IP کلودفلر وصل می‌شود و IP سرور پنهان می‌ماند | وقتی IP سرور مسدود است |
| **مستقیم** | پورت خودش با گواهی خودامضا | آزمایش |

</div>

```bash
# Fronted by NaïveProxy
sudo rovunet tunnel add --transport vless --tag myvless --domain example.com --cdn no --listen-port 2053
sudo rovunet tunnel edit --tag myvless --front myproxy      # "-" undoes it

# Behind Cloudflare
sudo rovunet tunnel add --transport vless --tag myvless-cf --domain cdn.example.com --cdn yes --listen-port 2053
sudo rovunet tunnel edit --tag myvless-cf --cdn-address 104.16.0.1   # clean Cloudflare IP, "-" resets
```

<div dir="rtl">

تنظیمات Cloudflare برای نام Proxied:

۱. DNS: ⁦`A cdn.example.com → SERVER_IP`⁩ با حالت **Proxied** (ابر نارنجی).  
۲. SSL/TLS › Overview: **Full**. حالت Flexible کار نمی‌کند و Full (strict) گواهی خودامضای سرور را نمی‌پذیرد.  
۳. Network: گزینهٔ **WebSockets** روشن باشد.  
۴. پورت: Cloudflare فقط روی 443، 2053، 2083، 2087، 2096 و 8443 پروکسی HTTPS می‌کند؛ وقتی NaïveProxy روی ۴۴۳ است، 2053 را بگیرید.

کانفیگ‌ها برای هر کاربر جدا هستند: ⁦`sudo rovunet tunnel share myvless --user alice⁦`⁩ یک لینک ⁦`⁩rovunet://⁦`⁩ برای اپ Rovunet و یک لینک استاندارد ⁦`⁩vless://⁦`⁩ برای v2rayNG، Hiddify، NekoBox و کلاینت‌های مشابه چاپ می‌کند. لینک‌های پشت Naïve به ⁦`⁩allowInsecure`⁩ نیازی ندارند؛ کلاینت‌های Xray از اوت ۲۰۲۶ این گزینه را نمی‌پذیرند. فرگمنت SNI در هر دو نوع لینک خاموش است و در اپ قابل روشن کردن است.

<a id="naiveproxy"></a>
## NaïveProxy

Caddy با پلاگین forwardproxy. Caddy گواهی Let's Encrypt را می‌گیرد و تمدید می‌کند (پورت ۸۰ و ۴۴۳ باید باز باشند) و به هر درخواستی که پروکسی احرازشده نباشد، یک سایت پوششی نشان می‌دهد. همهٔ کاربران پنل می‌توانند وارد شوند؛ ⁦`users add⁦`⁩، ⁦`⁩remove⁦`⁩ و ⁦`⁩passwd`⁩ در لحظه Caddy را به‌روز می‌کنند. تونل Naïve بدون هیچ لاگینی یک لاگین تصادفی می‌گیرد و بسته می‌ماند.

</div>

```bash
sudo rovunet tunnel add --transport naive --backend socks --tag myproxy \
  --domain example.com --email admin@example.com --decoy-url https://www.wikipedia.org
```

<div dir="rtl">

### سرو روی ۴۴۳ از پشت NaïveProxy

Caddy NaïveProxy می‌تواند VLESS و SSH over TLS را روی همان دامنه، پورت و گواهی ببرد. Caddy هر درخواست را مسیریابی می‌کند: مسیر مخفی WebSocket به VLESS یا SSH over TLS، درخواست پروکسی احرازشده به NaïveProxy و بقیه به سایت پوششی. هر تغییر Caddyfile پیش از اعمال با ⁦`caddy-naive validate`⁩ بررسی می‌شود؛ اگر رد شود، فایل قبلی سر جایش می‌ماند.

</div>

```bash
sudo rovunet tunnel edit --tag myvless --front naive
sudo rovunet tunnel edit --tag mytls --front naive
sudo rovunet users configs        # hand out the new configurations
```

<div dir="rtl">

<a id="ssh-over-tls"></a>
## SSH over TLS (StunTLS)

SSH داخل TLS و WebSocket که خود پنل سرو می‌کند. حالت‌های WebSocket، HTTP CONNECT، TLS خام و payload را خودکار تشخیص می‌دهد. روی پورت خودش (8443) گواهی خودامضا دارد و دامنه نمی‌خواهد؛ پشت NaïveProxy، کلاینت آن را به‌صورت SSH روی WebSocket با دامنهٔ Naïve، پورت ۴۴۳ و گواهی واقعی می‌بیند. اگر اپ روی پورت خودش خطای ⁦`Connection was reset`⁩ داد، شبکه آن پورت را بسته است: آن را پشت Naïve ببرید.

</div>

```bash
sudo rovunet tunnel add --transport stuntls --tag mytls
sudo rovunet tunnel edit --tag mytls --front myproxy
```

<div dir="rtl">

<a id="ssh-and-socks5"></a>
## SSH و SOCKS5

</div>

```bash
sudo rovunet tunnel add --transport direct-ssh --tag myssh
sudo rovunet tunnel add --transport direct-socks5 --tag mysocks
```

<div dir="rtl">

SSH از OpenSSH سرور روی پورت ۲۲ استفاده می‌کند؛ کاربران تونل لاگین با رمز و بدون shell دارند (فقط forwarding). SOCKS5 روی 1080 گوش می‌دهد و رمزنگاری ندارد. پروکسی SOCKS5 بدون کاربر اتصال‌ها را رد می‌کند و پروکسی باز نمی‌شود.

<a id="dns-tunnels"></a>
## تونل‌های DNS

| تونل | توضیح |
|---|---|
| **DNSTT** | تونل DNS با رمزنگاری Curve25519. همین سرور به کلاینت‌های **NoizDNS** هم پاسخ می‌دهد که مقاومت در برابر DPI را اضافه می‌کنند (کدگذاری base36/hex، حذف پیشوند CDN) |
| **VayDNS** | تونل DNS مبتنی بر KCP با Curve25519 و قالب قابل تنظیم: نوع رکورد، idle timeout، keep-alive، اندازهٔ client ID، اندازهٔ صف |
| **Slipstream** | QUIC روی DNS با احراز هویت گواهی. آزمایشی |
| **Soar** | تونل DNS چندمسیره ([Lantern](https://github.com/getlantern/soar)) با کشف resolver، داخل پنل. بک‌اند **direct**: بدون لاگین، سرور به مقصدهای عمومی روی پورت‌های مجاز (پیش‌فرض ⁦`80,443`⁩) وصل می‌شود و با کلاینت‌های استاندارد Soar سازگار است. بک‌اند **SSH**: فقط به سرور SSH می‌رسد، پس دسترسی با کاربران SSH است |

</div>

```bash
sudo rovunet tunnel add --transport dnstt  --backend socks  --tag mydnstt  --domain t.example.com
sudo rovunet tunnel add --transport vaydns --backend both   --tag myvaydns --domain v.example.com \
  --record-type txt --idle-timeout 10s --keep-alive 2s --clientid-size 2 --queue-size 512
sudo rovunet tunnel add --transport slipstream --backend ssh --tag myslip --domain s.example.com
sudo rovunet tunnel add --transport soar --backend direct --tag mysoar --domain r.example.com --allow-ports 80,443
sudo rovunet tunnel add --transport soar --backend ssh    --tag mysoar-ssh --domain rs.example.com
```

<div dir="rtl">

DNSTT و VayDNS کلید موجود را با ⁦`--private-key⁦`⁩ (و ⁦`⁩--public-key⁦`⁩ اختیاری) می‌پذیرند؛ Soar یک seed از نوع Ed25519 به base64 را با ⁦`⁩--private-key⁦`⁩ می‌پذیرد. ⁦`⁩sudo rovunet mtu 1200`⁩ مقدار MTU همهٔ تونل‌های DNSTT، NoizDNS و VayDNS را یک‌جا تنظیم می‌کند (پیش‌فرض 1232).

<a id="external-dns-route"></a>
## مسیر DNS خارجی

پرسش‌های DNS یک دامنه را به یک پورت UDP محلی می‌فرستد، برای پروتکلی که خودتان اجرا می‌کنید. پنل برایش سرویسی مدیریت نمی‌کند.

</div>

```bash
sudo rovunet tunnel add --transport external --tag my-proto --domain j.example.com --port 5301
```

<div dir="rtl">

## چیدمان دامنه

| تونل | دامنه | بک‌اند |
|---|---|---|
| dnstt-socks / dnstt-ssh | ⁦`t.example.com⁦`⁩ / ⁦`⁩ts.example.com`⁩ | SOCKS5 / SSH |
| vaydns-socks / vaydns-ssh | ⁦`v.example.com⁦`⁩ / ⁦`⁩vs.example.com`⁩ | SOCKS5 / SSH |
| slipstream-socks / slipstream-ssh | ⁦`s.example.com⁦`⁩ / ⁦`⁩ss.example.com`⁩ | SOCKS5 / SSH |
| soar-direct / soar-ssh | ⁦`r.example.com⁦`⁩ / ⁦`⁩rs.example.com`⁩ | direct / SSH |
| naive-socks / naive-ssh | ⁦`example.com`⁩ (مشترک) | SOCKS5 / SSH |
| vless، stuntls | ⁦`example.com`⁩ از طریق NaïveProxy | UUID هر کاربر / SSH |

هر تونل DNS زیردامنهٔ خودش را می‌خواهد؛ DNSTT و NoizDNS یکی را مشترک دارند. تونل‌های NaïveProxy دامنه را مشترک دارند چون HTTPS هستند، و VLESS و SSH over TLS روی آن سوارند، پس رکورد جدا نمی‌خواهند.

## حالت‌های مسیریابی

| حالت | رفتار |
|---|---|
| **single** | یک تونل DNS فعال است و روتر DNS روی پورت ۵۳ به آن می‌فرستد |
| **multi** | همهٔ تونل‌های DNS روی پورت‌های محلی اجرا می‌شوند و روتر پرسش‌ها را بر اساس دامنه، همیشه به دقیق‌ترین دامنهٔ منطبق، می‌فرستد. با ساختن تونل DNS دوم خودکار فعال می‌شود |

روتر روی UDP و TCP پورت ۵۳ پاسخ می‌دهد. پرسش‌های TCP (RFC 7766، به‌صورت pipelined) روی UDP به تونل می‌روند و روی همان اتصال پاسخ داده می‌شوند، پس همهٔ تونل‌های DNS برای resolverها و کلاینت‌هایی که TCP/53 به کار می‌برند هم کار می‌کنند.

## معماری

</div>

```text
                                 Rovunet client
            ┌──────────────────────────┼─────────────────────────────┐
     DNS :53 udp+tcp             HTTPS :443 tcp              Cloudflare :2053 (optional)
            │                          │                             │
┌───────────┼──────────────────────────┼─────────────────────────────┼──────────┐
│ server    ▼                          ▼                             │          │
│  ┌───────────────────┐   ┌─────────────────────────────────┐      │          │
│  │    DNS router     │   │ Caddy (NaïveProxy) :443         │      │          │
│  │ dispatch by domain│   │ Let's Encrypt + decoy website   │      │          │
│  └──┬────┬────┬───┬──┘   └────┬──────────────┬───────────┬─┘      │          │
│     ▼    ▼    ▼   ▼           ▼              ▼           ▼        ▼          │
│  DNSTT VayDNS Slip Soar   forward proxy  SSH over TLS   VLESS over WebSocket │
│     └────┴────┴───┘           │              │           │                   │
│              ▼                ▼              ▼           │                   │
│   ┌───────────────────────────────────────────────┐      │   Xray cores      │
│   │  Backend: SOCKS5 :1080 · SSH forwarding :22  │      │   (own ports)     │
│   └───────────────────────┬───────────────────────┘      │        │          │
│                     WARP (optional)                      │        │          │
│                           └─────────────┬────────────────┴────────┘          │
│                                      Internet                                │
└───────────────────────────────────────────────────────────────────────────────┘
```

<div dir="rtl">

Soar با بک‌اند direct، VLESS و هسته‌های Xray خودشان به مقصد وصل می‌شوند، پس ترافیکشان از لایهٔ بک‌اند و WARP رد نمی‌شود.

بعدی: [کاربران و کانفیگ‌های کلاینت](users-and-configs.md)

</div>
