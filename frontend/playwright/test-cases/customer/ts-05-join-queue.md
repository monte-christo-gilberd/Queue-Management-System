# TS-05 — Bergabung ke Antrian (Customer)

> **Test Environment**
> - Web: `http://localhost:8081` · API: `http://localhost:8080`
> - Antrian: dibuat owner berlangganan lewat API (`QMS_OWNER_EMAIL` / `QMS_OWNER_PASSWORD`)
> - Customer: akun baru
> - Sumber behaviour: `product-overview.md` — US-14, 4.2 (Join Queue via QR Code), edge case 4.2

> **Catatan implementasi**: join hanya lewat popup "Masuk ke Antrian" yang memindai QR dengan kamera dan langsung memanggil `POST /api/v1/join`. Halaman `/join?t=` hanya menampilkan petunjuk. Kamera tidak bisa dijalankan di tes, jadi join disiapkan lewat API dengan token dari `GET /queues/:id/qr`.

---

### TC-05-01 — Tautan join di address bar tidak membuat tiket
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.2: tiket hanya dibuat lewat pemindaian QR, bukan dengan membuka tautan |
| **Steps** | 1. Login sebagai customer 2. Buka `/join?t=<token>` langsung di address bar |
| **Expected Result** | Halaman menampilkan petunjuk "Pindai QR di aplikasi" dan tidak ada tombol **Gabung Antrian**. Customer belum punya tiket (status antrian membalas 404) |

### TC-05-02 — Tautan join tanpa login meminta login
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14 AC1: "they are prompted to log in before registering" · 4.2 langkah 4 |
| **Steps** | 1. Tanpa login, buka `/join?t=<token>` |
| **Expected Result** | Pengguna diarahkan ke halaman login (field "Email atau username" tampil) |

### TC-05-03 — Bergabung memberi nomor di akhir antrian
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14 AC2: "added to the end of the queue, or as the first number if the queue is empty"; 4.2 langkah 5–6 |
| **Steps** | 1. Antrian kosong 2. Customer membuka `/join?t=<token>` 3. Klik **Gabung Antrian** |
| **Expected Result** | Layar menampilkan "Nomor Anda" berisi **A-001** dan "Orang di depan Anda" bernilai 0 |

### TC-05-04 — Tautan yang sama tidak membuat nomor baru
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.2 edge case: "Customer scans the same QR code again after already joining → shows their existing number" |
| **Steps** | 1. Customer sudah bergabung (A-001) 2. Buka `/join?t=<token>` lagi |
| **Expected Result** | Nomor yang tampil tetap A-001. Tombol **Gabung Antrian** tidak tampil. Tidak ada tiket kedua |

### TC-05-05 — Antrian tertutup tidak menerima pendaftaran
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.2 edge case: "Customer scans the QR code while the queue is closed" |
| **Steps** | 1. Owner menutup antrian 2. Customer baru membuka `/join?t=<token>` |
| **Expected Result** | Customer yang belum bergabung tidak bisa melihat detail antrian. Saat menekan **Gabung Antrian**, pesan "Antrian sudah ditutup" tampil dan tidak ada nomor yang dibuat. *Catatan: selisih dengan 4.2 ("tampil saat scan") karena backend hanya memberi detail antrian ke owner dan pemegang tiket.* |

### TC-05-06 — Token tidak valid
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.2 (token diubah) |
| **Steps** | 1. Login 2. Buka `/join?t=bukan-token` |
| **Expected Result** | Pesan QR tidak valid tampil. Tidak ada tombol gabung |

### TC-05-07 — Tanpa token
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Login 2. Buka `/join` |
| **Expected Result** | Tidak ada tiket yang dibuat |

### TC-05-08 — QR statis yang sudah diganti
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.2 (ganti kode QR) |
| **Steps** | 1. Owner menyalin tautan lama 2. Owner mengganti kode QR 3. Customer membuka tautan lama |
| **Expected Result** | Pesan QR tidak valid tampil |

### TC-05-09 — Popup Masuk ke Antrian
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Login 2. Klik **Masuk Ke Antrian** |
| **Expected Result** | Petunjuk memakai kamera HP tampil, beserta form "tempel tautan antrian" dan tombol **Masuk** |

### TC-05-10 — Join dengan menempel tautan
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Login sebagai customer 2. Klik **Masuk Ke Antrian** 3. Tempel tautan `/join?t=...` dari owner 4. Klik **Masuk** |
| **Expected Result** | Customer mendapat tiket dan diarahkan ke halaman tiket, tanpa scan QR |

### TC-05-11 — Tautan bukan tautan antrian
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Buka popup 2. Tempel tautan lain (mis. `https://example.com/lainnya`) 3. Klik **Masuk** |
| **Expected Result** | Pesan "Tautan ini bukan tautan antrian." tampil, tidak ada tiket dibuat |
