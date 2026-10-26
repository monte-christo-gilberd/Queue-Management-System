# TS-09 — Fitur Backlog (belum diimplementasikan)

> **Tujuan**: mencatat ekspektasi dari `product-overview.md` untuk fitur yang belum ada, supaya tidak hilang. Setiap test di `ts-09-backlog-features.spec.ts` berstatus **fixme** dan tidak dijalankan sampai fitur dikerjakan. Saat fitur selesai, ubah `test.fixme` menjadi `test` dan lengkapi langkahnya.

| TC | Fitur | Sumber | Status di kode |
|---|---|---|---|
| TC-09-01 | Papan tampilan publik | 4.5, US-7 (Scope #5, P1) | Belum ada |
| TC-09-02 | Skip nomor | 4.7, US-8 skip (Scope #7, P2) | Belum ada endpoint |
| TC-09-03 | Notifikasi customer berikutnya | 4.8, US-9 (Scope #8, P2) | Belum ada |
| TC-09-04 | Customer membatalkan nomornya | 4.9, US-16 (Scope #9, P2) | Belum ada |
| TC-09-05 | Owner menghapus customer dari antrian | 4.10, US-11 (Scope #10, P2) | Belum ada |
| TC-09-06 | Counter per counter | 4.11, US-4, US-19 (Scope #11, P2) | Belum ada |
| TC-09-07 | Menyisipkan customer di posisi tertentu | 4.12, US-10 (Scope #12, P3) | Belum ada |
| TC-09-08 | QR dinamis dan batas harian | 4.13, US-5 (Scope #13, P3) | Belum ada |
| TC-09-09 | Pengembalian otomatis setelah skip | 4.14, US-17 (Scope #14, P3) | Belum ada |
| TC-09-10 | Riwayat dan ekspor CSV/Excel | 4.15, US-13 (Scope #15, P4) | Belum ada |
| TC-09-11 | Catatan per nomor | 4.16, US-12 (Scope #16, P4) | Belum ada |
| TC-09-12 | Login dengan Google | 4.17, US-20 (Scope #17, P4) | Belum ada (OAuth) |
| TC-09-13 | Batas pendaftaran per antrian | 4.1, 4.2 edge case | Belum ada (`registration_limit` tidak ditegakkan) |
| TC-09-15 | Paket dan pembayaran langganan | 4.18, US-21 (Scope #18, P1) | Hanya pemblokiran; halaman harga belum ada |
| TC-09-16 | Scan QR dengan kamera | 4.2 langkah 3 | Placeholder; masuk lewat tautan/input manual |

---

### TC-09-01 — Papan tampilan publik
| | |
|---|---|
| **Expected Result** | Owner membuka tampilan publik dari antrian. Papan menampilkan nama antrian, nomor yang sedang dilayani, N nomor berikutnya, dan M nomor yang dilewati. Papan diperbarui otomatis. Bila N atau M = 0, bagian itu disembunyikan. Bila koneksi putus, papan menampilkan status "menyambung ulang" |

### TC-09-02 — Skip nomor
| | |
|---|---|
| **Expected Result** | Owner menekan **SKIP**. Nomor itu ditandai dilewati, antrian lanjut ke nomor berikutnya. Jika tidak ada nomor lagi, muncul pesan antrian kosong |

### TC-09-03 — Notifikasi customer berikutnya
| | |
|---|---|
| **Expected Result** | Saat customer menjadi berikutnya, ia otomatis menerima notifikasi push. Owner bisa menekan "Notify again" untuk mengirim ulang, dengan pembatasan frekuensi |

### TC-09-04 — Customer membatalkan nomornya
| | |
|---|---|
| **Expected Result** | Di halaman status, customer menekan "Cancel my number" lalu konfirmasi. Nomor berstatus dibatalkan dan keluar dari urutan. Membatalkan dua kali tidak menimbulkan error |

### TC-09-05 — Owner menghapus customer dari antrian
| | |
|---|---|
| **Expected Result** | Owner memilih nomor lalu "Remove". Nomor ditandai dilewati (tidak dihapus dari catatan). Menghapus nomor yang sudah dibatalkan atau sudah dilewati tidak menimbulkan efek |

### TC-09-06 — Counter per counter
| | |
|---|---|
| **Expected Result** | Owner membuat beberapa counter, dan dua counter bisa memanggil nomor bersamaan tanpa menggandakan nomor yang sama |

### TC-09-07 — Menyisipkan customer di posisi tertentu
| | |
|---|---|
| **Expected Result** | Owner menyisipkan customer di posisi yang dipilih. Customer itu menempati posisi tersebut di urutan |

### TC-09-08 — QR dinamis dan batas harian
| | |
|---|---|
| **Expected Result** | Setiap customer mendapat kode berbeda. Setelah batas harian tercapai, kode tidak bisa dipakai lagi untuk bergabung di hari itu |

### TC-09-09 — Pengembalian otomatis setelah skip
| | |
|---|---|
| **Expected Result** | Customer yang dilewati dimundurkan sejumlah posisi yang dikonfigurasi, lalu dipanggil lagi nanti |

### TC-09-10 — Riwayat dan ekspor
| | |
|---|---|
| **Expected Result** | Owner melihat semua antrian lama dan dapat mengekspor catatan satu antrian sebagai CSV atau Excel |

### TC-09-11 — Catatan per nomor
| | |
|---|---|
| **Expected Result** | Owner menambah satu catatan ke nomor, bisa diedit langsung, dan catatan itu tampil di riwayat |

### TC-09-12 — Login dengan Google
| | |
|---|---|
| **Expected Result** | Tombol "Login with Google" membuat akun yang langsung bisa dipakai |

### TC-09-13 — Batas pendaftaran per antrian
| | |
|---|---|
| **Expected Result** | Owner menetapkan batas. Setelah tercapai, pendaftaran baru ditolak dengan pesan "antrian penuh". Pembatalan membebaskan satu slot |

### TC-09-15 — Paket dan pembayaran langganan
| | |
|---|---|
| **Expected Result** | Owner tanpa langganan yang mencoba membuat antrian melihat opsi paket (bulanan, tahunan, sekali bayar). Setelah pembayaran sukses, akses diperpanjang |

### TC-09-16 — Scan QR dengan kamera
| | |
|---|---|
| **Expected Result** | Customer memindai QR dengan kamera ponsel dan langsung diarahkan ke halaman join |
