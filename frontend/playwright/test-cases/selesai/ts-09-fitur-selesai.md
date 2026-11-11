# TS-09 — Fitur yang Sudah Selesai (dipindah dari backlog)

> **Tujuan**: mencatat fitur dari `product-overview.md` yang sudah ada di kode dan sudah diuji. Fitur ini sudah keluar dari backlog. Test-nya ada di spec tematik yang tercantum di kolom *Diuji di*.
>
> Tidak ada lagi item backlog.

## Fitur selesai

| TC | Fitur | Sumber | Status | Diuji di |
|---|---|---|---|---|
| TC-09-01 | Papan tampilan publik | 4.5, US-7 | Layar publik `/display/[token]` menampilkan nomor dilayani, berikutnya, dan dilewati | `ts-32-public-display.spec.ts` |
| TC-09-02 | Skip nomor | 4.7, US-8 | Selesai | `ts-11-skip.spec.ts` (TC-09-02a sampai e) |
| TC-09-04 | Customer membatalkan nomornya | 4.9, US-16 | Selesai | `ts-16-customer-cancel.spec.ts` |
| TC-09-05 | Owner menghapus customer dari antrian | 4.10, US-11 | Selesai | `ts-17-owner-remove.spec.ts` |
| TC-09-06 | Counter per counter | 4.11, US-4, US-19 | Beberapa counter per antrian, tambah dan hapus counter, pemanggilan paralel | `ts-19-counters.spec.ts` |
| TC-09-07 | Menyisipkan customer di posisi tertentu | 4.12, US-10 | Selesai | `ts-18-sisip.spec.ts` |
| TC-09-10 | Riwayat dan ekspor | 4.15, US-13 | Riwayat owner, riwayat customer, dan ekspor | `ts-23-history.spec.ts`, `ts-24-export.spec.ts`, `ts-26-my-history.spec.ts` |
| TC-09-11 | Catatan per nomor | 4.16, US-12 | Selesai | `ts-15-notes.spec.ts`, `ts-25-notes-list.spec.ts` |
| TC-09-12 | Login dengan Google | 4.17, US-20 | Selesai di `auth-service` dan frontend: email baru membuat akun terverifikasi tanpa password; email yang sudah terdaftar dengan password wajib konfirmasi password sebelum ditautkan (tidak pernah otomatis). OAuth Google asli tidak bisa diotomasi, jadi popup Google dan dua endpoint disetubkan, ditambah dua tes ke backend sungguhan | `ts-34-google-login.spec.ts` (TC-34-01 sampai 13). Verifikasi ID token di server: tes Go `auth-service` dan skrip end-to-end dengan Google palsu. Google asli: manual (lihat `documentation/docs/setup-google-login.md`) |
| TC-09-16 | Scan QR dengan kamera | 4.2 langkah 3 | Ada, tetapi kamera tidak bisa diotomasi di Playwright | Verifikasi manual di ponsel lewat HTTPS. Jalur API di `ts-05` dan `ts-10` |

Catatan:
- TC-09-02 dan TC-09-06 sebelumnya tercatat sebagai backlog atau Tier 2. Kode dan test sudah membuktikan keduanya ada.
- TC-09-12 butuh client ID Google untuk berjalan: tanpa `EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID` di bundle, tombol tersembunyi dan tes tampilan di `ts-34` di-skip (tes kontrak API di-skip bila `GOOGLE_CLIENT_IDS` kosong). Aplikasi harus dibuka lewat `http://localhost` atau https, bukan alamat IP.
- TC-09-16 selesai di sisi aplikasi, tetapi pemindaian dengan kamera hanya bisa diverifikasi manual.

## Dihapus dari produk atau tidak diperlukan (bukan backlog)

Fitur berikut tidak lagi dikerjakan. TC-09-08 dan TC-09-13 dibatalkan lewat keputusan produk 31 Okt 2026 (lihat `product-overview.md`, riwayat keputusan). Kolom yang masih ada di skema tidak dipakai kode dan tidak diuji. TC-09-03 dan TC-09-09 diputuskan tidak diperlukan sebagai fitur.

| TC | Fitur | Sumber | Keterangan |
|---|---|---|---|
| TC-09-08 | Batas harian QR | 4.13, US-5 | Dihapus dari produk. QR dinamis (TTL 120 detik) tetap ada dan diuji di `ts-05` |
| TC-09-13 | Batas pendaftaran per antrian (`registration_limit`) | 4.1, 4.2 | Dihapus dari produk |
| TC-09-03 | Tombol *Notify again* dan pembatasan frekuensinya | 4.8, US-9 | Tidak diperlukan. Notifikasi "Tinggal 1 nomor lagi" ke customer berikutnya sudah ada dan diuji lewat email di `ts-30` |
| TC-09-09 | Pengembalian otomatis setelah skip tanpa aksi owner | 4.14, US-17 | Tidak diperlukan. Pengembalian lewat aksi owner (mode `back` dan `after_n`, batas `max_returns`) sudah ada dan diuji di `ts-11` |
