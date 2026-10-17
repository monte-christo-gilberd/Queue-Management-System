# TS-05 — Bergabung ke Antrian (Customer)

> **Test Environment**
> - Web: `http://localhost:8081` · API: `http://localhost:8080`
> - Antrian: dibuat owner berlangganan lewat API (`QMS_OWNER_EMAIL` / `QMS_OWNER_PASSWORD`)
> - Customer: akun baru
> - Sumber behaviour: `product-overview.md` — US-14, 4.2 (Join Queue via QR Code), edge case 4.2

> **Catatan implementasi**: pemindaian QR belum ada. Customer membuka **tautan** `…/join/<id>` (sama dengan yang dihasilkan QR), atau memasukkannya lewat popup "Masukkan Link Manual".

---

### TC-05-01 — Tautan join meminta login
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14 AC1: "they are prompted to log in before registering" · 4.2 langkah 4 |
| **Steps** | 1. Tanpa login, buka `/join/<id antrian>` 2. Login dengan akun customer |
| **Expected Result** | Sebelum login, pengguna diarahkan ke halaman login. Setelah login, pengguna kembali ke `/join/<id>` dan melihat tombol **Gabung Antrian** |

### TC-05-02 — Daftar lewat tautan join kembali ke halaman join
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14 AC3: pelanggan tanpa akun tidak perlu kehilangan tautan |
| **Steps** | 1. Buka `/join/<id>` tanpa login 2. Klik **Daftar** 3. Isi form dan klik **Daftar** |
| **Expected Result** | Setelah daftar, pengguna kembali ke `/join/<id>` |

### TC-05-03 — Bergabung memberi nomor di akhir antrian
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14 AC2: "added to the end of the queue, or as the first number if the queue is empty"; 4.2 langkah 5–6 |
| **Steps** | 1. Antrian kosong 2. Customer membuka `/join/<id>` 3. Klik **Gabung Antrian** |
| **Expected Result** | Layar menampilkan "Nomor Anda" berisi **A-001** dan "Orang di depan Anda" bernilai 0 |

### TC-05-04 — Tautan yang sama tidak membuat nomor baru
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.2 edge case: "Customer scans the same QR code again after already joining → shows their existing number" |
| **Steps** | 1. Customer sudah bergabung (A-001) 2. Buka `/join/<id>` lagi |
| **Expected Result** | Nomor yang tampil tetap A-001. Tombol **Gabung Antrian** tidak tampil. Tidak ada tiket kedua |

### TC-05-05 — Antrian tertutup tidak menerima pendaftaran
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.2 edge case: "Customer scans the QR code while the queue is closed" |
| **Steps** | 1. Owner menutup antrian 2. Customer baru membuka `/join/<id>` |
| **Expected Result** | Customer yang belum bergabung tidak bisa melihat detail antrian. Saat menekan **Gabung Antrian**, pesan "Antrian sudah ditutup" tampil dan tidak ada nomor yang dibuat. *Catatan: selisih dengan 4.2 ("tampil saat scan") karena backend hanya memberi detail antrian ke owner dan pemegang tiket.* |

### TC-05-06 — Tautan tidak valid
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.2 (tautan yang tidak mengarah ke antrian) |
| **Steps** | 1. Login 2. Buka `/join/abc` |
| **Expected Result** | Pesan "Antrian tidak ditemukan" tampil. Tidak ada tombol gabung |

### TC-05-07 — Antrian tidak ada saat mencoba bergabung
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Login 2. Buka `/join/999999999` 3. Klik **Gabung Antrian** |
| **Expected Result** | Pesan "Antrian tidak ditemukan" tampil setelah klik |

### TC-05-08 — Masukkan tautan secara manual (popup)
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.2 alternatif: customer tidak bisa memindai, tetapi punya tautan |
| **Steps** | 1. Login sebagai customer di Dashboard 2. Klik **Masuk Ke Antrian** 3. Klik **Masukkan Link Manual** 4. Tempel `http://…/join/<id>` 5. Klik **Gabung Antrian** |
| **Expected Result** | Pengguna diarahkan ke `/join/<id>` |

### TC-05-09 — Input link manual yang salah
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Pada popup link manual, isi `bukan-tautan` 2. Klik **Gabung Antrian** |
| **Expected Result** | Pesan "Tautan tidak valid…" tampil. Pengguna tetap di popup |
