<div dir="rtl">

# شروع به کار

[مستندات](README.md) › شروع به کار

## ۱. پیش‌نیازها

| مورد | نیاز |
|---|---|
| سیستم‌عامل | لینوکس با systemd؛ Ubuntu 24.04 LTS آزمایش شده است. توزیع‌های دیگر به بررسی سازگاری runtime و ABI نیاز دارند |
| دسترسی | root (یا sudo) از طریق SSH |
| دامنه | دامنه‌ای که رکوردهای DNS آن را در اختیار دارید؛ برای تونل‌های DNS، NaïveProxy، VLESS و هسته‌های Xray مبتنی بر TLS لازم است |
| سخت‌افزار | ۱ هستهٔ CPU، ۱ گیگابایت RAM و ۲۰ گیگابایت دیسک برای یک گروه کوچک کافی است؛ پهنای باند مهم‌تر از CPU است |
| پورت‌ها | بسته به پروتکل‌هایی که فعال می‌کنید؛ جدول زیر |

| پورت | استفاده |
|---|---|
| ⁦53/udp⁩ و ⁦53/tcp⁩ | تونل‌های DNS (DNSTT، NoizDNS، VayDNS، Slipstream، Soar) |
| ⁦80/tcp⁩ و ⁦443/tcp⁩ | NaïveProxy و هرچه پشت آن است (VLESS، SSH over TLS) |
| ⁦22/tcp⁩ | SSH |
| ⁦1080/tcp⁩ | SOCKS5 (فقط شبکه‌های آزاد) |
| ⁦2053/tcp⁩ (یا 2083، 2087، 2096، 8443) | VLESS پشت Cloudflare |
| هر پورت آزاد TCP یا UDP | هسته‌های Xray |

## ۲. انتخاب سرور

- **ورودی UDP/53 باید آزاد باشد** تا تونل‌های DNS کار کنند. بعضی ارائه‌دهنده‌ها آن را می‌بندند یا محدود می‌کنند؛ پیش از خرید، سیاست فایروال و شبکهٔ ارائه‌دهنده را بررسی کنید.
- منطقه‌ای نزدیک به کاربران انتخاب کنید که شبکهٔ آن‌ها هنوز به آن دسترسی داشته باشد.
- ارائه‌دهندهٔ بدون محدودیت پورت و با پهنای باند کافی بهتر است.

## ۳. رکوردهای DNS

رکوردهای پروتکل‌هایی را که می‌خواهید **پیش از نصب** بسازید. همهٔ رکوردها **DNS only** (ابر خاکستری در Cloudflare) باشند: Caddy برای گرفتن گواهی Let's Encrypt باید مستقیم در دسترس باشد و تونل‌های DNS به DNS خام روی سرور نیاز دارند.

</div>

```text
A   ns.example.com   → SERVER_IP          DNS tunnels: the name server
NS  t.example.com    → ns.example.com     DNSTT / NoizDNS, SOCKS5 backend
NS  ts.example.com   → ns.example.com     DNSTT / NoizDNS, SSH backend
NS  v.example.com    → ns.example.com     VayDNS, SOCKS5 backend
NS  vs.example.com   → ns.example.com     VayDNS, SSH backend
NS  s.example.com    → ns.example.com     Slipstream, SOCKS5 backend
NS  ss.example.com   → ns.example.com     Slipstream, SSH backend
NS  r.example.com    → ns.example.com     Soar, direct backend
NS  rs.example.com   → ns.example.com     Soar, SSH backend
A   example.com      → SERVER_IP          NaïveProxy, and VLESS / SSH over TLS fronted by it
```

<div dir="rtl">

نمونه‌های حداقلی:

- **فقط NaïveProxy:** ⁦`A example.com → SERVER_IP`⁩.
- **فقط DNSTT:** ⁦`A ns.example.com → SERVER_IP⁦`⁩ و ⁦`⁩NS t.example.com → ns.example.com`⁩.

رکورد ⁦`NS⁦`⁩ یک زیردامنه را به سرور شما واگذار می‌کند: پرسش‌های ⁦`⁩t.example.com⁦`⁩ به ⁦`⁩ns.example.com`⁩، یعنی سرور شما، می‌رسند و تونل روی پورت ۵۳ پاسخ می‌دهد. اعمال تغییرات از چند دقیقه تا چند ساعت طول می‌کشد. بررسی:

</div>

```bash
dig +short A ns.example.com
dig +short NS t.example.com
```

<div dir="rtl">

تنها استثنا VLESS پشت Cloudflare است که رکورد **Proxied** می‌خواهد؛ [پروتکل‌ها › VLESS روی WebSocket](protocols.md#vless-over-websocket) را ببینید.

## ۴. نصب

### آنلاین (پیشنهادی)

</div>

```bash
curl -fsSL https://raw.githubusercontent.com/msVPNco/rovunet-panel/v1.21.2/install.sh -o rovunet-install.sh
python3 -c 'import hashlib,pathlib; assert hashlib.sha256(pathlib.Path("rovunet-install.sh").read_bytes()).hexdigest()=="a8aaa38d979049fa5e42c9f8f444e519d28625d1d67872ddaeff2d0dfc6b3498", "Installer checksum mismatch"'
sudo bash rovunet-install.sh
```

<div dir="rtl">

نصب‌کننده می‌پرسد کدام پروتکل‌ها فعال شوند، باینری‌های لازم را دانلود می‌کند، کلید می‌سازد، سرویس‌های systemd و قوانین فایروال را ایجاد می‌کند، اولین کاربر را می‌سازد و همهٔ کانفیگ‌های او را چاپ می‌کند.

اجرای دوبارهٔ نصب روی سرور پیکربندی‌شده، تونل‌های موجود را نگه می‌دارد، فقط پروتکل‌های تازه را اضافه می‌کند و دوباره همهٔ کانفیگ‌ها را چاپ می‌کند. برای تونل دوم از نوعی که دارید، ⁦`sudo rovunet tunnel add`⁩ را به کار ببرید.

اگر NaïveProxy و VLESS با هم انتخاب شوند، VLESS (و SSH over TLS اگر انتخاب شده باشد) از همان ابتدا پشت NaïveProxy روی پورت ۴۴۳ قرار می‌گیرند.

### آفلاین (وقتی سرور به گیت‌هاب دسترسی ندارد)

پنل را از [آخرین release](https://github.com/msVPNco/rovunet-panel/releases) همین پروژه و باینری‌های ترنسپورت را از [upstream](https://github.com/anonvector/slipgate/releases) دانلود کنید و به سرور ببرید:

</div>

```bash
# On your computer
mkdir rovunet-bundle && cd rovunet-bundle
curl -LO https://github.com/msVPNco/rovunet-panel/releases/latest/download/rovunet-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/dnstt-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/slipstream-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/vaydns-server-linux-amd64
curl -LO https://github.com/anonvector/slipgate/releases/latest/download/caddy-naive-linux-amd64
scp * root@SERVER_IP:/tmp/rovunet/

# On the server
chmod +x /tmp/rovunet/*
sudo cp /tmp/rovunet/rovunet-linux-amd64 /usr/local/bin/rovunet
sudo rovunet install --bin-dir /tmp/rovunet
```

<div dir="rtl">

روی سرورهای ARM به‌جای ⁦`amd64⁦`⁩ بنویسید ⁦`⁩arm64`⁩.

### از سورس

</div>

```bash
git clone https://github.com/msVPNco/rovunet-panel.git
cd rovunet-panel
make build
sudo ./rovunet install
```

<div dir="rtl">

### جابه‌جایی از SlipGate یا نسخهٔ قدیمی پنل

سروری که با نام قبلی پنل (⁦`slipgate⁦`⁩ و ⁦`⁩/etc/slipgate`⁩) نصب شده، در جای خود مهاجرت می‌کند؛ [عملیات › به‌روزرسانی](operations.md#updating) را ببینید.

## ۵. منوی تعاملی و خط فرمان

- ⁦`sudo rovunet`⁩ بدون آرگومان منوی تعاملی را باز می‌کند و برای راه‌اندازی اول بهترین است.
- هر گزینهٔ منو یک دستور با گزینه‌های خط فرمان هم هست، برای اسکریپت‌نویسی؛ [مرجع دستورها](commands.md) را ببینید. گزینهٔ لازمی که داده نشود، به‌صورت تعاملی پرسیده می‌شود.

## ۶. اولین تونل و اولین کاربر

</div>

```bash
sudo rovunet tunnel add --transport dnstt --backend socks --tag mytunnel --domain t.example.com
sudo rovunet users add
sudo rovunet users configs
sudo rovunet tunnel status
```

<div dir="rtl">

⁦`tunnel add⁦`⁩ کلیدهای تونل را می‌سازد، سرویس ⁦`⁩rovunet-mytunnel⁦`⁩ را ایجاد می‌کند (همین حالا و هنگام بوت اجرا می‌شود) و دامنه را در روتر DNS ثبت می‌کند. ⁦`⁩users configs⁦`⁩ همهٔ کانفیگ‌های همهٔ کاربران را چاپ می‌کند. اگر تونلی ⁦`⁩active⁦`⁩ نبود، ⁦`⁩sudo rovunet diag`⁩ را اجرا کنید.

بعدی: [پروتکل‌ها](protocols.md)

</div>
