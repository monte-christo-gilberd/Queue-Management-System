# TS-07 — Pembaruan Real-time

> **Test Environment**
> - Dua sesi browser: owner (dashboard) dan customer (halaman join)
> - Sumber behaviour: `product-overview.md` — 4.3 (Next), 4.4 (status diperbarui tanpa refresh), 4.5 edge case "reconnecting"

---

### TC-07-01 — Dashboard owner menerima customer baru tanpa refresh
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-6 (melihat customer yang terdaftar) · 4.4 langkah 3 |
| **Steps** | 1. Owner membuka Dashboard 2. Customer bergabung dari sesi lain |
| **Expected Result** | Nomor customer (A-001) muncul di papan Dashboard tanpa memuat ulang halaman |

### TC-07-02 — Status customer diperbarui saat owner menekan NEXT
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | 4.4 langkah 3: "Page auto-refreshes as the queue advances" |
| **Steps** | 1. Dua customer, A-001 dan A-002 2. Customer A-002 membuka halaman status 3. Owner menekan NEXT |
| **Expected Result** | Tanpa refresh, halaman A-002 menampilkan "Sedang dilayani" A-001 dan "Orang di depan Anda" 0 |

### TC-07-03 — Customer melihat antrian ditutup tanpa refresh
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.2 edge case (antrian tertutup) — pembaruan langsung |
| **Steps** | 1. Customer membuka halaman status 2. Owner menutup antrian |
| **Expected Result** | Tanpa refresh, halaman customer menampilkan "Antrian ditutup" |

### TC-07-04 — Koneksi terputus lalu pulih: status disinkronkan ulang
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | 4.5 edge case: "Display screen loses its network connection → reconnecting state rather than stale numbers" |
| **Steps** | 1. Customer A-002 membuka halaman status 2. Hentikan stream event (simulasi putus) 3. Owner menekan NEXT 4. Pulihkan koneksi |
| **Expected Result** | Setelah koneksi pulih, halaman menampilkan "Sedang dilayani" A-001 (tidak tertinggal di data lama) |
