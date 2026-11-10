/**
 * display: logika murni untuk layar display publik (kartu counter, ukuran huruf, status sambungan).
 *
 * Dipakai oleh : src/hooks/use-display-screen.ts (isConnected, changedCounterIds, SSE_FRESH_MS),
 *                src/components/DisplayScreen.tsx (buildCards, layoutColumns, numberFontSize, formatClock).
 * Bergantung   : lib/display.ts (tipe DisplaySnapshot dan DisplayCounter).
 * Alur         : snapshot dan event SSE -> use-display-screen.ts -> utils ini -> DisplayScreen.
 * Catatan      : Tidak ada pemanggilan API di sini. Semua fungsi murni sehingga bisa diuji di Node.
 */

import type { DisplayCounter, DisplaySnapshot } from '@/lib/display';

/** Satu kartu di layar. `title` null = jangan tampilkan nama counter (antrian dengan satu counter). */
export type DisplayCard = {
  /** Kunci React untuk kartu; id counter dalam bentuk string. */
  key: string;
  /** ID counter di database. */
  id: number;
  /** Nama counter; null bila antrian hanya punya satu counter sehingga nama disembunyikan. */
  title: string | null;
  /** Nomor yang sedang dilayani, sudah diformat (mis. "A-007"); null bila counter tidak melayani. */
  number: string | null;
};

/** Satu kartu per counter, urutan dari server (counter default dulu). Fungsi murni. */
export function buildCards(snapshot: Pick<DisplaySnapshot, 'counters' | 'multi_counter'>): DisplayCard[] {
  return snapshot.counters.map((c: DisplayCounter) => ({
    key: String(c.id),
    id: c.id,
    title: snapshot.multi_counter ? c.name : null,
    number: c.serving,
  }));
}

/**
 * Id counter yang sekarang melayani nomor yang BERBEDA dari snapshot sebelumnya (untuk sorotan sesaat).
 * Snapshot pertama tidak menyorot apa pun. Counter baru yang langsung melayani ikut disorot.
 */
export function changedCounterIds(prev: DisplaySnapshot | null, next: DisplaySnapshot): number[] {
  if (!prev) return [];
  const before = new Map(prev.counters.map((c) => [c.id, c.serving]));
  return next.counters.filter((c) => c.serving !== null && before.get(c.id) !== c.serving).map((c) => c.id);
}

// Jumlah kolom menurut jumlah kartu (indeks = jumlah kartu, 0..10).
const LANDSCAPE_COLUMNS = [1, 1, 2, 3, 2, 3, 3, 4, 4, 3, 5];

/** Jumlah kolom grid kartu. Layar tegak memakai paling banyak dua kolom. Fungsi murni. */
export function layoutColumns(count: number, landscape: boolean): number {
  const n = Math.min(Math.max(count, 0), 10);
  if (landscape) return LANDSCAPE_COLUMNS[n];
  return n <= 3 ? 1 : 2;
}

/**
 * Ukuran huruf nomor besar supaya muat di kartu. `areaWidth` dan `areaHeight` = ruang untuk seluruh grid kartu.
 * Label seperti "A-007" lebarnya sekitar 0,62 em per karakter. Fungsi murni.
 */
export function numberFontSize(opts: {
  count: number;
  areaWidth: number;
  areaHeight: number;
  landscape: boolean;
  labelLength?: number;
}): number {
  const count = Math.max(opts.count, 1);
  const columns = layoutColumns(count, opts.landscape);
  const rows = Math.ceil(count / columns);
  const gap = 16;
  const cardWidth = (opts.areaWidth - gap * (columns - 1)) / columns - 32; // minus padding kartu
  const cardHeight = (opts.areaHeight - gap * (rows - 1)) / rows - 96; // minus ruang judul dan label
  // Ukuran diambil dari batas lebar atau tinggi, mana yang lebih kecil, lalu dijepit ke 28-240 piksel.
  // Tinggi memakai 80% agar ada ruang napas di dalam kartu.
  const byWidth = cardWidth / ((opts.labelLength ?? 5) * 0.62);
  const byHeight = cardHeight * 0.8;
  return Math.round(Math.min(Math.max(Math.min(byWidth, byHeight), 28), 240));
}

/** Detak SSE (heartbeat 25 detik) dianggap hidup selama bytenya baru: 25 detik ditambah margin 5 detik. */
export const SSE_FRESH_MS = 30_000;
/** Pengambilan snapshot yang berhasil dianggap segar selama ini (polling cadangan tiap 3 detik). */
export const FETCH_FRESH_MS = 10_000;

/**
 * Layar dianggap tersambung bila stream SSE masih berdetak ATAU snapshot baru berhasil diambil. Bila keduanya
 * basi, layar menampilkan "Menyambung ulang..." alih-alih angka yang bisa sudah salah. Fungsi murni.
 */
export function isConnected(now: number, lastByteAt: number | null, lastFetchOkAt: number | null): boolean {
  const sse = lastByteAt !== null && now - lastByteAt < SSE_FRESH_MS;
  const fetched = lastFetchOkAt !== null && now - lastFetchOkAt < FETCH_FRESH_MS;
  return sse || fetched;
}

/** Jam "17:12" untuk sudut layar. Fungsi murni. */
export function formatClock(date: Date): string {
  const two = (n: number) => String(n).padStart(2, '0');
  return `${two(date.getHours())}:${two(date.getMinutes())}`;
}
