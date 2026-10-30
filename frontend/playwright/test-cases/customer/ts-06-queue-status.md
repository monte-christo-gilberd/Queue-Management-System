# TS-06 — Status dan Posisi Antrian (Customer)

> **Test Environment**
> - Web: `http://localhost:8081` · API: `http://localhost:8080`
> - Antrian dan owner: seperti TS-05
> - Sumber behaviour: `product-overview.md` — US-15, 4.4 (Customer View: Queue Status & Position)

---

### TC-06-01 — Status menampilkan nomor, nomor dilayani, dan jumlah di depan
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-15 AC1–2; 4.4 langkah 2 |
| **Steps** | 1. Antrian dengan tiga customer (A-001, A-002, A-003) 2. Customer ketiga membuka halamannya |
| **Expected Result** | Menampilkan nomor **A-003**, "Sedang dilayani" bernilai "-" (belum ada), dan "Orang di depan Anda" bernilai **2** |

### TC-06-02 — Setelah dilayani, status berubah menjadi "Sudah dilayani"
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.4 langkah 2 (posisi diperbarui saat antrean bergerak) |
| **Steps** | 1. Owner memanggil NEXT satu kali 2. Customer pertama membuka halamannya |
| **Expected Result** | Status tiket menampilkan "Sudah dilayani", dan "Sedang dilayani" menampilkan A-001 |

### TC-06-03 — Status tetap benar setelah tautan dibuka ulang di perangkat lain
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.4 edge case: "Customer logs in on a different device … still shows their number correctly" |
| **Steps** | 1. Customer sudah bergabung (A-001) 2. Buka konteks browser baru (sesi bersih) 3. Login dengan akun yang sama 4. Buka `/join?t=<token>` |
| **Expected Result** | Nomor A-001 tampil. Tidak ada tombol **Gabung Antrian** |

### TC-06-04 — Status tetap tampil setelah browser ditutup dan dibuka lagi
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.4 edge case: "Customer closes their browser and reopens the same link later" |
| **Steps** | 1. Customer sudah bergabung 2. Tutup halaman lalu buka tautan yang sama lagi (sesi tetap tersimpan) |
| **Expected Result** | Status tampil dengan nomor yang sama, tanpa perlu bergabung ulang |
