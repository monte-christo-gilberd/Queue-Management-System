# TS-04 — Langganan Owner

> **Test Environment**
> - Web: `http://localhost:8081` · API: `http://localhost:8080`
> - Akun tanpa langganan: akun baru (tidak punya baris `owners`)
> - Akun dengan langganan kedaluwarsa (opsional): `QMS_EXPIRED_OWNER_EMAIL` / `QMS_EXPIRED_OWNER_PASSWORD`
> - Sumber behaviour: `product-overview.md` — US-21, 4.18 (langganan owner)

---

### TC-04-01 — Owner tanpa langganan tidak bisa membuat antrian
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-21 AC1: "Given an owner account with no active subscription, when they try to create a queue, then they're blocked" |
| **Steps** | 1. Login sebagai akun baru (tanpa langganan) 2. Klik **Buat Antrian Baru** 3. Isi nama dan klik **Buat Antrian** |
| **Expected Result** | Pesan "Langganan Anda tidak aktif…" tampil. Antrian tidak dibuat |

### TC-04-02 — Owner tanpa langganan tetap bisa membuka daftar antrian
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | Pembaruan change log 2026-10-02: langganan hanya membatasi aksi yang membuat/mengubah data |
| **Steps** | 1. Login sebagai akun tanpa langganan 2. Buka `/queues` |
| **Expected Result** | Halaman "Semua Antrian" tampil tanpa pesan langganan |

### TC-04-03 — Antrian yang sudah ada tetap berjalan setelah langganan kedaluwarsa
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-21 AC3: "existing queues they already created keep running until closed" |
| **Steps** | 1. Login sebagai owner berlangganan kedaluwarsa yang sudah punya antrian aktif 2. Buka Dashboard |
| **Expected Result** | Antrian aktif tetap tampil dan tombol **NEXT** bisa dipakai. Hanya pembuatan antrian baru yang diblokir |

