import { test } from '@playwright/test';

/**
 * Fitur yang BELUM atau BARU SEBAGIAN ada. Setiap tes ditandai fixme sampai sisanya dikerjakan.
 * Dokumennya: test-cases/backlog/ts-09-backlog-features.md. Fitur yang sudah selesai ada di
 * test-cases/selesai/ts-09-fitur-selesai.md, dan test-nya ada di spec tematiknya.
 *
 * Saat sebuah item selesai: ubah fixme menjadi test biasa, lengkapi langkahnya, lalu pindahkan
 * barisnya dari dokumen backlog ke dokumen selesai.
 */
const OPEN: Array<[title: string, source: string, sisa: string]> = [
  ['TC-09-15 Paket dan pembayaran langganan: pilihan paket', '4.18, US-21', 'hanya tambah 30 hari (MOCK, tanpa pembayaran), pilihan paket belum ada'],
];

test.describe('TS-09 Fitur yang belum selesai', () => {
  for (const [title, source, sisa] of OPEN) {
    test(title, () => {
      test.fixme(true, `Belum selesai — ${sisa} (sumber: product-overview ${source})`);
    });
  }
});
