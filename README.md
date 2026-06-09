# CF-Scanner GUI ⚡

[🇬🇧 English](#english) | [🇮🇷 فارسی](#فارسی)

---

<a id="english"></a>
## 🇬🇧 English

A modern, fast, and feature-rich graphical user interface for scanning Cloudflare IPs, testing download speeds, and evaluating real-world proxy delays using Xray core. Built entirely in Go and Fyne.

### ✨ Features
- **Dynamic IP Scanning**: Automatically fetch the latest Cloudflare IPv4 subnets and scan them for alive endpoints.
- **Latency & Speed Testing**: Test TCP ping latency and real HTTP download speeds.
- **Xray Core Integration**: Evaluate actual connection delays using `Xray-core` directly from the UI.
- **Bilingual Interface**: Seamlessly switch between English and Persian (RTL) interfaces dynamically.
- **Modern UI**: Clean, responsive, and beautiful design with both light and dark themes.
- **Cross-Platform**: Designed to work effortlessly on Windows and Linux.

### 🛠 Usage
1. Open the application.
2. Select your desired IP subnets or enter custom IPs.
3. Click "Start Test" to begin TCP and HTTP latency scans.
4. For Xray-specific tests, navigate to the **Xray** tab, paste your config, and evaluate real-world delay.
5. Export the best performing configs directly to your clipboard or file!

---

<a id="فارسی"></a>
## 🇮🇷 فارسی

یک رابط کاربری گرافیکی مدرن، سریع و پرامکانات برای اسکن آی‌پی‌های کلودفلر، تست سرعت دانلود و ارزیابی تاخیر واقعی با استفاده از هسته Xray. این برنامه کاملاً با زبان Go و فریم‌ورک Fyne ساخته شده است.

### ✨ ویژگی‌ها
- **اسکن پویای آی‌پی‌ها**: دریافت خودکار جدیدترین ساب‌نت‌های کلودفلر و اسکن آن‌ها.
- **تست پینگ و سرعت**: تست پینگ با پروتکل TCP و ارزیابی سرعت واقعی دانلود با HTTP.
- **پشتیبانی از Xray**: ارزیابی پینگ واقعی (Real Delay) کانفیگ‌ها به کمک `Xray-core` از داخل خود برنامه.
- **رابط کاربری دوزبانه**: قابلیت تغییر لحظه‌ای زبان برنامه بین انگلیسی و فارسی (با چیدمان راست‌چین).
- **طراحی مدرن**: رابط کاربری تمیز و واکنش‌گرا (Responsive) با پشتیبانی از حالت‌های تاریک و روشن (Dark/Light Mode).
- **چند پلتفرمی**: سازگاری کامل با سیستم‌عامل‌های ویندوز و لینوکس.

### 🛠 راهنمای استفاده
۱. برنامه را باز کنید.
۲. ساب‌نت‌های آی‌پی مورد نظرتان را انتخاب کرده یا آی‌پی‌های دلخواه خود را وارد کنید.
۳. روی دکمه "شروع تست" کلیک کنید تا اسکن پینگ و سرعت آغاز شود.
۴. برای تست دقیق‌تر، به تب **Xray** بروید، کانفیگ خود را قرار دهید و تاخیر واقعی را ارزیابی کنید.
۵. بهترین کانفیگ‌های خروجی را مستقیماً کپی کرده یا در یک فایل ذخیره کنید!

## 📜 License
This project is licensed under the GPL-3.0 License.
