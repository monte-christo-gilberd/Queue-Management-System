# TS-08 — Otorisasi dan Keamanan Akses

> **Test Environment**
> - Web: `http://localhost:8081` · API: `http://localhost:8080`
> - Akun: owner berlangganan (`QMS_OWNER_*`), customer, dan akun tanpa langganan
> - Sumber behaviour: `product-overview.md` — 4.6 (hanya aksi yang diizinkan), 4.4 (status milik akun sendiri), 4.1 (antrian milik owner)

---

### TC-08-01 — Orang yang tidak punya tiket tidak bisa melihat status antrian
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.4: status hanya untuk pemilik tiket |
| **Steps** | 1. Customer A bergabung 2. Customer B (tanpa tiket) memanggil status antrian lewat API |
| **Expected Result** | Respons 404 (tidak membocorkan keberadaan antrian). Halaman join untuk B tidak menampilkan nomor A |

### TC-08-02 — Customer tidak bisa memanggil NEXT
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.6 / US-18: hanya owner yang bisa melanjutkan antrian |
| **Steps** | 1. Customer memanggil `POST …/next` lewat API 2. Buka halaman status customer |
| **Expected Result** | API menolak dengan 404. Halaman customer tidak menampilkan tombol **NEXT** |

### TC-08-03 — Owner lain tidak bisa membuka detail antrian
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.1: antrian dikelola oleh owner-nya |
| **Steps** | 1. Akun tanpa antrian (bukan pemilik) membuka `/queues/<id antrian owner lain>` |
| **Expected Result** | Pesan "Antrian tidak ditemukan" tampil. Nama dan izin antrian tidak tampil |

### TC-08-04 — Token tidak valid dikembalikan ke login
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | Keamanan sesi (sesi habis → login ulang) |
| **Steps** | 1. Simpan token palsu sebagai sesi 2. Buka `/dashboard` |
| **Expected Result** | Pengguna diarahkan ke halaman login |

### TC-08-05 — API antrian menolak permintaan tanpa token
| | |
|---|---|
| **Priority** | P0 |
| **Steps** | 1. Panggil `GET /api/v1/queues` tanpa header Authorization |
| **Expected Result** | Respons 401 |

### TC-08-06 — Nama antrian yang berisi HTML ditolak
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | Keamanan input (validator `no_html_tags` di backend) |
| **Steps** | 1. Owner berlangganan membuka **Buat Antrian Baru** 2. Isi nama `<b>x</b>` 3. Klik **Buat Antrian** |
| **Expected Result** | Pesan "Data tidak valid, periksa kembali isian Anda" tampil. Antrian tidak dibuat |

### TC-08-07 — Akun tanpa langganan tidak bisa mengubah antrian
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | US-21 / 4.18 (langganan membatasi aksi yang mengubah data) |
| **Steps** | 1. Akun tanpa langganan memanggil `PUT /api/v1/queues/<id>` untuk antrian yang bukan miliknya |
| **Expected Result** | Permintaan ditolak (404 atau 402). Nama antrian tidak berubah |
