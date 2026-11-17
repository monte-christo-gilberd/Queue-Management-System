# TS-01 — Authentication (Queue Management System)

> **Test Environment**
> - Web: `http://localhost:8081` (Expo web) · API: `http://localhost:8080`
> - Browser: Chromium, viewport 1280 × 800
> - Sumber behaviour: `Capstone-Project/documentation/docs/product-overview.md` (US-14, 4.2, 4.17 untuk login)

> **Test Data**
> | Akun | Dibuat oleh | Keterangan |
> |---|---|---|
> | Akun baru | tes (`registerAccount`) | Email `<username>@qms-test.local`, password `Password123!` |

---

### TC-01-01 — Login dengan email
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14 (login sebelum bergabung), 4.2 langkah 4 |
| **Steps** | 1. Buka halaman login <br> 2. Isi "Email atau username" dengan email akun <br> 3. Isi password <br> 4. Klik **Login** |
| **Expected Result** | Pengguna masuk ke Dashboard owner. Layar "Belum ada antrian yang aktif" tampil untuk akun baru |

### TC-01-02 — Login dengan username
| | |
|---|---|
| **Priority** | P0 |
| **Steps** | 1. Buka halaman login <br> 2. Isi "Email atau username" dengan username akun <br> 3. Isi password <br> 4. Klik **Login** |
| **Expected Result** | Sama seperti TC-01-01: pengguna masuk ke Dashboard |

### TC-01-03 — Password salah
| | |
|---|---|
| **Priority** | P0 |
| **Steps** | 1. Login dengan email akun dan password yang salah |
| **Expected Result** | Pesan "Email/username atau password salah" tampil. Pengguna tetap di halaman login. Pesan tidak membedakan mana yang salah |

### TC-01-04 — Email/username tidak terdaftar
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Login dengan identifier `tidak-ada-<acak>` dan password apa pun |
| **Expected Result** | Pesan yang sama dengan TC-01-03 ("Email/username atau password salah") |

### TC-01-05 — Login dengan isian kosong
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Kosongkan kedua field 2. Klik **Login** |
| **Expected Result** | Pesan "Email/username dan password wajib diisi". Tidak ada permintaan login yang berhasil |

### TC-01-06 — Register akun baru
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14: pelanggan yang belum punya akun harus bisa membuat akun |
| **Steps** | 1. Buka halaman register 2. Isi username, email, password, dan konfirmasi password 3. Klik **Daftar** |
| **Expected Result** | Pengguna langsung masuk ke Dashboard tanpa login ulang |

### TC-01-07 — Register dengan email yang sudah terdaftar
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Daftarkan email yang sudah ada |
| **Expected Result** | Pesan "Email sudah terdaftar". Pengguna tetap di halaman register |

### TC-01-08 — Register dengan konfirmasi password berbeda
| | |
|---|---|
| **Priority** | P2 |
| **Steps** | 1. Isi password dan konfirmasi yang berbeda 2. Pindahkan fokus dari field konfirmasi 3. Klik **Daftar** |
| **Expected Result** | Pesan "Password tidak sama" belum tampil selama pengguna masih mengetik di field konfirmasi, lalu tampil di bawah field setelah fokus pindah. Pendaftaran ditolak dan tidak ada akun dibuat. Rincian perilaku pesan ada di TS-35 |

### TC-01-09 — Halaman terlindungi mengarahkan ke login, lalu kembali
| | |
|---|---|
| **Priority** | P0 |
| **Sumber** | US-14: setelah login, pengguna kembali ke halaman yang dituju |
| **Steps** | 1. Tanpa login, buka `/queues` 2. Login dengan akun baru |
| **Expected Result** | Setelah diarahkan ke login, URL berisi `next=/queues`. Setelah login, pengguna berada di `/queues` |

### TC-01-10 — Sesi bertahan setelah halaman di-refresh
| | |
|---|---|
| **Priority** | P1 |
| **Steps** | 1. Login 2. Refresh halaman Dashboard |
| **Expected Result** | Pengguna tetap di Dashboard, tidak diarahkan ke login |

### TC-01-11 — Logout menghapus sesi
| | |
|---|---|
| **Priority** | P0 |
| **Steps** | 1. Login 2. Buka menu profil di header 3. Pilih **Keluar** 4. Buka `/dashboard` langsung |
| **Expected Result** | Setelah keluar, pengguna diarahkan ke login. Membuka halaman terlindungi juga diarahkan ke login |

### TC-01-12 — Parameter `next` tidak bisa dipakai untuk pindah ke situs lain
| | |
|---|---|
| **Priority** | P1 |
| **Sumber** | Keamanan (open redirect) |
| **Steps** | 1. Buka `/login?next=https://evil.example` 2. Login |
| **Expected Result** | Setelah login, pengguna tetap di aplikasi (halaman Dashboard), bukan di situs lain |
