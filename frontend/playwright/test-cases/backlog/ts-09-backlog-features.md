# TS-09 — Fitur Backlog (belum atau baru sebagian selesai)

> **Tujuan**: mencatat ekspektasi dari `product-overview.md` yang **belum selesai** atau **baru sebagian**, supaya selisihnya tidak hilang.
>
> Test yang masih terbuka ada di `ts-09-backlog-features.spec.ts` dan berstatus **fixme**. Saat fitur selesai, ubah `test.fixme` menjadi `test`, lengkapi langkahnya, dan pindahkan barisnya ke `../selesai/ts-09-fitur-selesai.md`.
>
> Fitur yang sudah selesai, dihapus dari produk, atau tidak diperlukan tercatat di `../selesai/ts-09-fitur-selesai.md`.

## Ringkasan status

| TC | Fitur | Sumber | Status di kode | Diuji di |
|---|---|---|---|---|
| TC-09-15 | Paket dan pembayaran langganan | 4.18, US-21 | **Sebagian**: pemblokiran owner yang langganannya habis dan tambah 30 hari (MOCK, tanpa pembayaran) sudah ada. Pilihan paket (bulanan, tahunan, sekali bayar) belum ada | `ts-04-subscription.spec.ts` |

Nomor TC-09-14 tidak ada di daftar ini. Nomornya tidak diganti, supaya ID yang sudah dipakai tidak bergeser.

---

## Detail item yang belum selesai

### TC-09-15 — Paket dan pembayaran langganan (sisa: pilihan paket)
| | |
|---|---|
| **Expected Result** | Owner tanpa langganan melihat opsi paket (bulanan, tahunan, sekali bayar). Setelah pembayaran sukses, akses diperpanjang |
| **Sisa** | Pilihan paket. Halaman langganan saat ini hanya menawarkan "Tambah 30 Hari" (MOCK, tanpa pembayaran). Test ini sama dengan TC-04-04 di `ts-04-subscription.spec.ts` |

---

## Catatan penelusuran

- Status di atas diperiksa terhadap kode dan spec pada saat dokumen ini diperbarui. Jika ada perubahan kode, periksa ulang baris yang relevan.
- `ts-09-backlog-features.spec.ts` hanya berisi test untuk item yang belum atau baru sebagian selesai.
