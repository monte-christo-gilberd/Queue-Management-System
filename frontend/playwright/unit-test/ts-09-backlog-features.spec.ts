import { test } from '@playwright/test';

/**
 * Fitur backlog: ekspektasi dicatat sesuai product-overview.md, dan setiap tes ditandai fixme
 * sampai fitur dikerjakan. Lihat test-cases/queue-management/backlog/ts-09-backlog-features.md.
 */
const BACKLOG: Array<[title: string, source: string]> = [
  // Catatan status: TC-09-02 backlog (Skip = tandai nomor sekarang `skipped` lalu panggil berikutnya); TC-09-06 hanya counter default (CRUD di Tier 2);
  // TC-09-08 QR dinamis sudah ada (batas harian belum); TC-09-16 sudah ada tetapi kamera tidak bisa diotomasi.
  ['TC-09-01 Papan tampilan publik', '4.5, US-7'],
  ['TC-09-02 Skip nomor', '4.7, US-8 skip'],
  ['TC-09-03 Notifikasi customer berikutnya', '4.8, US-9'],
  ['TC-09-04 Customer membatalkan nomornya', '4.9, US-16'],
  ['TC-09-05 Owner menghapus customer dari antrian', '4.10, US-11'],
  ['TC-09-06 Counter per counter', '4.11, US-4, US-19'],
  ['TC-09-07 Menyisipkan customer di posisi tertentu', '4.12, US-10'],
  ['TC-09-08 QR dinamis dan batas harian', '4.13, US-5'],
  ['TC-09-09 Pengembalian otomatis setelah skip', '4.14, US-17'],
  ['TC-09-10 Riwayat dan ekspor', '4.15, US-13'],
  ['TC-09-11 Catatan per nomor', '4.16, US-12'],
  ['TC-09-12 Login dengan Google', '4.17, US-20'],
  ['TC-09-13 Batas pendaftaran per antrian', '4.1, 4.2'],
  ['TC-09-15 Paket dan pembayaran langganan', '4.18, US-21'],
  ['TC-09-16 Scan QR dengan kamera', '4.2 langkah 3'],
];

test.describe('TS-09 Fitur backlog', () => {
  for (const [title, source] of BACKLOG) {
    test(title, () => {
      test.fixme(true, `Backlog — belum diimplementasikan (sumber: product-overview ${source})`);
    });
  }
});
