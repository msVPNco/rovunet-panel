نسخهٔ ۱.۲۱.۲ نصب و ارتقای معمول پنل را کامل می‌کند و آدرس‌های قبلی سابسکریپشن، کاربران و شناسهٔ پروفایل‌ها را نگه می‌دارد. هسته‌های مدیریت‌شدهٔ ناقص به‌صورت خودکار تکمیل می‌شوند. برای حساب نامحدود واجد شرایط و نصب کامل با یک آدرس، هر دو خروجی سابسکریپشن ۲۴ پروفایل دارند؛ نمایش و اتصال همهٔ آن‌ها به پشتیبانی اپ و شبکه هم وابسته است.

بعد از ارتقای سرور با `sudo rovunet update`، در اپ همان سابسکریپشن ذخیره‌شده را Update کنید. نیازی به ارسال لینک تازه یا پاک‌کردن اشتراک نیست. هستهٔ Hy2 قدیمیِ مدیریت‌شده یک بار با همان اطلاعات کاربر به Salamander منتقل می‌شود؛ انتخاب صریح حالت خاموش حفظ می‌شود. فرمت بدون اتلاف AWG و SS2022 به خواننده‌های قدیمی اجازه می‌دهد موارد پشتیبانی‌نشده را رد کنند و بقیهٔ اشتراک را به‌روز نگه دارند.

فایل‌های Linux برای AMD64 و ARM64، سورس‌ها، مجوزها، گزارش آزمون و گزارش فارسی یکپارچه‌سازی پیوست هستند. تمام ۳۴ فایل با کلید تولیدی محلی احراز شده‌اند؛ کلید خصوصی وارد گیت‌هاب نشده است. README و راهنماهای نصب و عملیات نیز به‌روز شده‌اند.

پیش‌نیازهای نسخهٔ بعدی در `ROVUNET-PANEL-1.21.2-INTEGRATION-FA.txt` و قرارداد `docs/contracts/panel-integration-v1.md` آمده‌اند: پشتیبانی کامل AWG v3 و زنجیرهٔ کلید SS2022، هماهنگی Hy2/TLS، ادغام اتمی سابسکریپشن با شناسه‌های ثابت، تشخیص امن موارد پشتیبانی‌نشده، و آزمون روی خروجی نهایی اپ‌ها. چهار محور پذیرش گستردهٔ امنیتی و عملیاتی همچنان باز هستند؛ این انتشار ادعای تکمیل آن‌ها یا اتصال هر پروتکل در همهٔ شبکه‌ها را ندارد. محدودیت حساب‌های زمان‌دار و حجمی حفظ شده است.

نصب‌های خصوصی `1.20.1-managed-preview.3` یک بار به بستهٔ امضاشدهٔ `rovunet-preview-to-v1.21.2-linux-amd64.tar.gz` و دستورهای همراه آن نیاز دارند، چون باینری قدیمی خودش به‌روزرسان عمومی را متوقف می‌کند. پس از انتقال، مسیر معمول Update برقرار است.

---

Rovunet Panel v1.21.2 completes missing managed cores during normal installation/update while preserving saved subscription URLs, account credentials and item identities. A fully eligible unlimited account on a complete single-address setup receives 24 profiles in both feeds. Apps display and connect their supported subset; timed and traffic-limited account eligibility remains restricted.

Run `sudo rovunet update`, then refresh the same saved subscription in the app. Hy2 has a one-time Salamander migration with explicit opt-out preserved. Lossless standard AWG/SS2022 URIs let older readers skip unsupported items safely. README and operation guides are updated.

Both Linux architectures, matching source/license archives, acceptance evidence and the Persian integration handoff are attached. All 34 assets are authenticated with the production signing identity; the private key stays in the Windows-local vault. Source: `ac23d1a3ed07009567e004415606da7fed8779d3`.

This is the owner-authorized current functional release. Future app-dependent work follows the version-independent integration contract after the app foundations are ready. Four broader security/operational qualification gates remain open; candidate runtime binaries have not been promoted. See the attached handoff and acceptance record for the exact test scope and remaining prerequisites.
