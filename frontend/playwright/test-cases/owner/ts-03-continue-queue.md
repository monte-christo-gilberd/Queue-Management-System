# TS-03 — Melanjutkan Antrian (Next)

> **Test Environment**
> - Web: `http://localhost:8081` · API: `http://localhost:8080`
> - Akun owner berlangganan: `QMS_OWNER_EMAIL` / `QMS_OWNER_PASSWORD`
> - Customer uji: akun baru yang bergabung lewat API
> - Sumber behaviour: `product-overview.md` — US-8 (next), 4.3 (Continue Queue)

---

### TC-03-01 — Next pada antrian kosong
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.3 edge case: "Owner clicks Next when the queue is empty" |
| **Steps** | 1. Buat antrian baru tanpa customer 2. Di Dashboard, klik **NEXT** |
| **Expected Result** | Pesan "Tidak ada yang menunggu di antrian ini" tampil. Tidak ada nomor yang berubah menjadi "sedang dilayani" |

### TC-03-02 — Next memajukan nomor yang dilayani
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-8 (Given customer finished, when next clicked, then current number changes) · 4.3 langkah 3 |
| **Steps** | 1. Buat antrian dengan dua customer (A-001, A-002) 2. Di Dashboard, klik **NEXT** |
| **Expected Result** | Notifikasi "Nomor A-001 sedang dilayani" tampil. A-002 menjadi nomor berikutnya |

### TC-03-03 — Klik ganda tidak memajukan dua nomor
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.3 edge case: "Owner double-clicks Next quickly" → tombol dinonaktifkan sebentar setelah klik |
| **Steps** | 1. Antrian dengan tiga customer 2. Klik **NEXT** dua kali dengan cepat |
| **Expected Result** | Hanya A-001 yang dilayani. A-002 belum menjadi "sedang dilayani" |

### TC-03-04 — Next berurutan mengikuti urutan bergabung
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14: customer ditambahkan di akhir antrian |
| **Steps** | 1. Antrian dengan tiga customer 2. Klik **NEXT** tiga kali (menunggu setiap kali selesai) |
| **Expected Result** | Nomor yang dilayani berurutan A-001, A-002, A-003 |
