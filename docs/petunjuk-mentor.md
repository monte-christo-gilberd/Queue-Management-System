# Petunjuk uji untuk mentor: QMS (Queue Management System)

**Alamat:** https://qms.montechristo.my.id
**Syarat akses:** buka dari **WiFi kantor**. Dari jaringan lain (mis. data seluler) akses diblokir dengan sengaja.
**Akun:** diberikan terpisah oleh pembuat aplikasi (1 owner dan 4 customer). Jangan menyebarkannya.

QMS memungkinkan pemilik usaha (**owner**) membuat antrian digital, dan pelanggan (**customer**) bergabung dengan memindai QR lalu mendapat pemberitahuan saat gilirannya tiba.

## Persiapan

| Perangkat | Peran | Catatan |
|---|---|---|
| Laptop | **Owner** | Chrome, Edge, atau Firefox. |
| HP 1 sampai 4 | **Customer** | Chrome di Android paling aman. Gunakan browser biasa, **bukan** browser di dalam aplikasi (WhatsApp, Instagram, dan sejenisnya), karena kamera dan notifikasi sering diblokir. |

Satu browser hanya melayani satu akun. Untuk dua customer di satu HP, pakai jendela penyamaran atau browser berbeda.

## A. Owner: membuat antrian

1. Login dengan akun owner.
2. Pilih **Buat Antrian Baru**, isi **Nama Antrian**, lalu **Buat Antrian**.
3. Buka antrian itu dan pilih **LIHAT QR**. Tampilkan QR di layar laptop.
4. (Opsional) Di pengaturan antrian: ubah **Awalan Nomor** (mis. `B`), atau atur perilaku **Saat nomor dilewati**.

## B. Customer: bergabung

1. Login dengan akun customer di HP.
2. Pilih **Gabung Antrian** atau **Masuk Ke Antrian**, lalu **Pindai QR di aplikasi** dan izinkan kamera.
3. Arahkan kamera ke QR di layar owner. Nomor antrian muncul.
4. Ketuk tombol notifikasi lalu **Izinkan** bila diminta, supaya pemberitahuan giliran bisa diterima.

Penting: bergabung **hanya** bisa lewat pemindai di dalam aplikasi. Membuka tautan QR dengan kamera bawaan HP hanya menampilkan info antrian, bukan bergabung.

## C. Owner: menjalankan antrian

| Tombol | Fungsi | Yang diharapkan di customer |
|---|---|---|
| **NEXT** | Memanggil nomor berikutnya | Notifikasi "Giliran Anda"; nomor di belakangnya dapat "Tinggal 1 nomor lagi". |
| **PANGGIL ULANG** | Memanggil ulang nomor yang sedang dipanggil | Notifikasi panggilan ulang. |
| **SKIP** | Melewati nomor yang tidak hadir | Notifikasi "dilewati"; nomor kembali ke antrian atau dihapus sesuai pengaturan. |
| **Tutup** | Menutup antrian | Customer baru tidak bisa bergabung ("Antrian ditutup"). |

Layar customer ikut berubah tanpa dimuat ulang.

## D. Hal yang bisa dicoba

- Dua customer bergabung ke antrian yang sama, lalu owner menekan NEXT berulang.
- Customer yang sama memindai QR dua kali: tiketnya tetap satu, tidak dobel.
- Owner menutup antrian, lalu customer lain mencoba bergabung: ditolak.
- Owner mengganti QR (**Ganti kode QR?**): QR lama tidak berlaku lagi.

## Catatan perangkat

- **Notifikasi** (Web Push) butuh **HTTPS** dan browser yang mendukung. Di **iPhone/iPad**: buka di Safari, ketuk Bagikan, pilih **Tambahkan ke Layar Utama**, lalu buka dari ikon itu. Tanpa itu, notifikasi tidak muncul dan aplikasi menampilkan petunjuknya.
- Bila notifikasi tidak muncul tetapi layar tiket berubah sendiri, antrian tetap benar: itu hanya pemberitahuan di luar aplikasi.
- Kamera tidak mau menyala: periksa izin kamera di pengaturan situs, dan pastikan bukan browser dalam aplikasi.
- Aplikasi tidak terbuka dan hanya muncul halaman blokir: Anda tidak berada di WiFi kantor.
- Tampilan dibuat untuk ponsel. Di laptop tetap bisa dipakai.

## Batas yang disengaja

- Aplikasi ini versi demo di satu laptop. Bila internet laptop terputus, aplikasi tidak bisa dibuka sampai pulih (biasanya beberapa detik).
- Data demo bisa dikosongkan sewaktu-waktu oleh pembuat aplikasi. Akun tidak ikut terhapus.
- Notifikasi yang terlewat saat layanan notifikasi sedang mati tidak dikirim ulang. Antrian tetap berjalan.