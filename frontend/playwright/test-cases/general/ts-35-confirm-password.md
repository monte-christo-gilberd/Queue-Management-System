# TS-35 — Pesan konfirmasi password (Queue Management System)

> **Test Environment**
> - Web: `http://localhost:8080` (lewat nginx) · API: `http://localhost:8080`
> - Browser: Chromium, viewport 1280 × 800
> - Spec: `playwright/unit-test/ts-35-confirm-password.spec.ts`

> **Aturan**
> Pesan "Password tidak sama" tampil sebagai teks di bawah field konfirmasi (bukan toast). Pesan hanya tampil setelah field
> konfirmasi ditinggalkan (blur) atau tombol kirim ditekan, dan hilang sendiri begitu isinya sama dengan password. Selama
> pengguna masih mengetik di field konfirmasi, pesan tidak boleh muncul.

> **Test Data**
> Tidak ada akun yang dibuat. Semua penolakan terjadi di sisi klien sebelum permintaan ke server. Halaman reset memakai token sembarang
> (form tetap tampil sampai password dikirim).

---

### TC-35-01 — Daftar: mengetik tidak memunculkan pesan
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Buka `/register` 2. Isi PASSWORD 3. Ketik di KONFIRMASI PASSWORD karakter demi karakter, mulai dari sebagian lalu sampai sama persis |
| **Expected Result** | Pesan "Password tidak sama" tidak muncul sama sekali selama fokus masih di field konfirmasi |

### TC-35-02 — Daftar: pesan muncul setelah field ditinggalkan dan hilang saat isinya sama
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Isi PASSWORD 2. Isi konfirmasi yang berbeda 3. Pindahkan fokus dari field konfirmasi 4. Ubah konfirmasi menjadi sama dengan password |
| **Expected Result** | Pesan belum tampil sebelum langkah 3, tampil setelah langkah 3, dan hilang setelah langkah 4 |

### TC-35-03 — Daftar: mengubah password mengevaluasi ulang kesamaannya
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Isi PASSWORD dan konfirmasi yang sama, lalu tinggalkan field konfirmasi 2. Ubah PASSWORD menjadi nilai lain 3. Samakan lagi konfirmasi |
| **Expected Result** | Tidak ada pesan di langkah 1, pesan tampil di langkah 2 (field konfirmasi sudah pernah ditinggalkan), dan hilang di langkah 3 |

### TC-35-04 — Daftar: konfirmasi kosong tidak memunculkan pesan
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Isi PASSWORD 2. Fokus ke KONFIRMASI PASSWORD tanpa mengetik, lalu tinggalkan |
| **Expected Result** | Pesan tidak muncul (field kosong ditangani validasi saat kirim) |

### TC-35-05 — Daftar: tombol Daftar dengan konfirmasi berbeda
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Isi USERNAME, EMAIL, PASSWORD, dan konfirmasi yang berbeda 2. Klik **Daftar** |
| **Expected Result** | Teks inline "Password tidak sama" tampil dan toast "Password dan konfirmasi password harus sama" tampil. Pengguna tetap di `/register` |

### TC-35-06 — Reset password: perilaku sama dengan halaman Daftar
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Buka `/reset-password?token=token-sembarang` 2. Isi PASSWORD BARU 3. Ketik sebagian di konfirmasi 4. Tinggalkan field 5. Samakan isi 6. Isi konfirmasi berbeda lalu klik **Simpan Password** |
| **Expected Result** | Tidak ada pesan saat mengetik, pesan tampil setelah field ditinggalkan, hilang saat isi sama, dan setelah Simpan Password tampil pesan inline serta toast galat kirim. Form tetap tampil |
