# TS-02 — Mengelola Antrian (Owner)

> **Test Environment**
> - Web: `http://localhost:8081` · API: `http://localhost:8080`
> - Akun owner berlangganan aktif: `QMS_OWNER_EMAIL` / `QMS_OWNER_PASSWORD` (dibuat manual di DB; tes di-skip bila kosong)
> - Sumber behaviour: `product-overview.md` — US-1, US-2, 4.1 (Create, View, and Manage a Queue)

---

### TC-02-01 — Membuat antrian baru
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-1, 4.1 langkah 1–2 |
| **Steps** | 1. Login sebagai owner berlangganan 2. Klik **Buat Antrian Baru** 3. Isi nama antrian 4. Klik **Buat Antrian** |
| **Expected Result** | Antrian dibuat dan langsung terbuka di halaman Detail Antrian, dengan nama yang sama |

### TC-02-02 — Nama antrian kosong ditolak
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.1 edge case: "Owner submits New Queue with an empty name" |
| **Steps** | 1. Klik **Buat Antrian Baru** 2. Biarkan nama kosong 3. Klik **Buat Antrian** |
| **Expected Result** | Pesan validasi tampil ("Nama antrian wajib diisi"). Antrian tidak dibuat |

### TC-02-03 — Mengubah nama dan deskripsi
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-2, 4.1 langkah 4 |
| **Steps** | 1. Buka detail antrian 2. Ubah nama dan deskripsi 3. Klik **SIMPAN** 4. Muat ulang halaman |
| **Expected Result** | Setelah muat ulang, nama baru tetap tampil |

### TC-02-04 — Menutup antrian setelah konfirmasi
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-2 (remove queue), 4.1 langkah 5 — implementasi memakai "tutup" (status `closed`), bukan hapus |
| **Steps** | 1. Buka detail antrian aktif 2. Klik **TUTUP ANTRIAN** 3. Pada popup, klik **Ya, Tutup Antrian** |
| **Expected Result** | Antrian berstatus tertutup: muncul "Antrian ini sudah ditutup." dan tombol **TUTUP ANTRIAN** hilang |

### TC-02-05 — Batal menutup antrian
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Klik **TUTUP ANTRIAN** 2. Klik **Batal** |
| **Expected Result** | Popup tertutup. Antrian tetap aktif (tombol **TUTUP ANTRIAN** masih ada) |

### TC-02-06 — Filter "Aktif" hanya menampilkan antrian aktif
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.1 "View" |
| **Steps** | 1. Siapkan satu antrian aktif dan satu antrian tertutup 2. Buka **Daftar Antrian** 3. Pilih filter **Aktif** |
| **Expected Result** | Antrian tertutup tidak tampil. Antrian aktif tampil |

### TC-02-07 — Tautan join tersedia di detail antrian
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.2 langkah 1 ("Generate QR Code" → tautan ke halaman join) |
| **Steps** | 1. Buka detail antrian aktif |
| **Expected Result** | Setelah klik **LIHAT QR**, popup menampilkan QR dengan tombol **SALIN TAUTAN** dan **UNDUH PNG** |

### TC-02-07b — Salin tautan menaruh tautan join di clipboard
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Buka detail antrian aktif 2. Klik **LIHAT QR** 3. Klik **SALIN TAUTAN** |
| **Expected Result** | Label berubah menjadi **TAUTAN TERSALIN** lalu kembali ke **SALIN TAUTAN** setelah beberapa detik. Di konteks aman (localhost atau HTTPS), isi clipboard adalah tautan `/join?t=v1.…` |

### TC-02-07c — Salin gagal: ada pesan dan tautan bisa disalin manual
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Matikan Clipboard API dan buat `execCommand` gagal 2. Buka popup QR 3. Klik **SALIN TAUTAN** |
| **Expected Result** | Pesan "Tidak bisa menyalin otomatis. Pilih tautan di bawah lalu salin manual." tampil, kolom "Tautan join" berisi tautan `/join?t=v1.…`, dan label **TAUTAN TERSALIN** tidak muncul |

### TC-02-08 — Daftar antrian bisa dibuka dari sidebar/menu
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Dari menu profil, pilih **Daftar Antrian** (atau buka `/queues`) |
| **Expected Result** | Halaman "Semua Antrian" tampil |
