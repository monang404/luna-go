# Security Policy

## Supported Versions

Hanya rilis terbaru dari `luna-go` yang akan menerima patch keamanan. Disarankan bagi pengguna untuk selalu menjalankan versi terbaru.

| Version | Supported          |
| ------- | ------------------ |
| >= 0.1.x| :white_check_mark: |
| < 0.1.0 | :x:                |

## Reporting a Vulnerability

Karena `luna-go` mengeksekusi shell command dan menulis file secara dinamis atas nama pengguna, kami menganggap serius kerentanan yang berpotensi *membypass* sistem **Permission Gate**. 

Area spesifik yang dianggap sebagai kerentanan tingkat tinggi:
- *Path Traversal*: AI Agent dapat mengakses file di luar *Project Root* atau *Additional Dirs* tanpa *approval*.
- *Privilege Escalation / Permission Bypass*: Eksekusi shell tanpa interupsi *permission* yang semestinya, pada mode `default` atau mode di mana izin diwajibkan.

**Tolong jangan laporkan masalah kerentanan secara publik lewat GitHub Issues.**
Sebagai gantinya, kirim laporan Anda secara privat lewat email (atau mekanisme pelaporan privat GitHub jika diaktifkan untuk repository ini). 

Kami akan berupaya menanggapi dalam waktu 48 jam dan menyediakan *patch* sesegera mungkin.
