/**
 * gen-icons.mjs: membuat semua ikon aplikasi (PNG) dari satu sumber, logo/logo.svg.
 *
 * Jalankan dari folder frontend/:  node scripts/gen-icons.mjs
 * Lalu build ulang APK (ikon ikut di dalam APK, tidak bisa diganti lewat EAS Update).
 *
 * Kenapa skrip: Android/Expo hanya menerima PNG, dan tiap jenis ikon punya aturan sendiri. Merender ulang dari SVG
 * menjaga semua ikon tetap sama dengan logo bila logo diubah. Render memakai Chromium milik Playwright (sudah jadi
 * devDependency untuk tes e2e), jadi tidak perlu ImageMagick/rsvg/sharp.
 *
 * Berkas yang dihasilkan (assets/images/):
 *   icon.png                     1024  logo utuh (plat gelap membulat + lingkaran). Ikon utama di app.json.
 *   android-icon-background.png  1024  latar polos warna plat (#11151D) untuk ikon adaptif.
 *   android-icon-foreground.png  1024  hanya lingkaran, transparan, diperkecil ke zona aman ikon adaptif. Launcher
 *                                      memotong ikon adaptif menjadi lingkaran/squircle/dll., jadi plat membulat dari
 *                                      SVG tidak boleh ikut (akan terpotong tidak rapi), dan isi harus berada di
 *                                      lingkaran tengah 66/108 kanvas.
 *   android-icon-monochrome.png  1024  siluet putih lingkaran (ikon bertema Android 13+; hanya alfa yang dipakai).
 *   notification-icon.png          96  siluet putih lingkaran untuk ikon kecil notifikasi. Android mewarnai ikon ini
 *                                      sendiri; ikon berwarna akan tampil sebagai kotak putih.
 *   splash-icon.png              1024  lingkaran berwarna, transparan, untuk splash screen (latar #11151D diatur di
 *                                      app.json). Plat tidak ikut karena warnanya sama dengan latar splash. Android 12+
 *                                      memotong gambar splash menjadi lingkaran, jadi isinya juga dijaga di zona aman.
 */
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { chromium } from '@playwright/test';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const out = (name) => path.join(root, 'assets/images', name);

// Sumber: logo/logo.svg. Warna plat dan lingkaran dibaca dari berkasnya supaya tidak ada nilai ganda di sini.
const logo = readFileSync(path.join(root, 'logo/logo.svg'), 'utf8');
const plate = /<rect[^>]*fill="(#[0-9A-Fa-f]{6})"/.exec(logo)?.[1] ?? '#11151D';
// Semua <circle> di logo (tiga lingkaran antrean dan cincin putih), tanpa plat <rect>.
const circles = (logo.match(/<circle[^>]*\/>/g) ?? []).join('\n');
if (!circles) throw new Error('logo.svg tidak berisi <circle>; skrip perlu disesuaikan dengan logo baru');

// Lingkaran di logo dalam koordinat viewBox 0..200: pusat dan jari-jari (ditambah separuh tebal garis cincin, 2.5).
const shapes = [...circles.matchAll(/cx="([\d.]+)"\s+cy="([\d.]+)"\s+r="([\d.]+)"/g)].map((m) => {
  const [cx, cy, r] = m.slice(1).map(Number);
  return { cx, cy, r: r + 1.25 };
});

// Titik tengah susunan lingkaran = tengah kotak pembatasnya.
const center = (() => {
  const x0 = Math.min(...shapes.map((c) => c.cx - c.r));
  const x1 = Math.max(...shapes.map((c) => c.cx + c.r));
  const y0 = Math.min(...shapes.map((c) => c.cy - c.r));
  const y1 = Math.max(...shapes.map((c) => c.cy + c.r));
  return { x: (x0 + x1) / 2, y: (y0 + y1) / 2, half: Math.max(x1 - x0, y1 - y0) / 2 };
})();

// Jarak terjauh dari titik tengah ke tepi lingkaran mana pun. Lingkaran tersusun diagonal, jadi ujungnya keluar
// dari lingkaran yang sebesar kotak pembatas; masker ikon adaptif berbentuk lingkaran harus memakai jarak ini.
const reach = Math.max(...shapes.map((c) => Math.hypot(c.cx - center.x, c.cy - center.y) + c.r));

/**
 * viewBoxInCircle: viewBox persegi sehingga seluruh lingkaran masuk ke lingkaran di tengah kanvas berdiameter
 * `diameter` bagian lebar kanvas (mis. 0.58 untuk zona aman ikon adaptif 66/108 = 0.61, dengan sedikit jarak).
 */
function viewBoxInCircle(diameter) {
  const side = (2 * reach) / diameter;
  return `${center.x - side / 2} ${center.y - side / 2} ${side} ${side}`;
}

/** viewBoxSquare: viewBox persegi sehingga kotak pembatas lingkaran mengisi `fill` bagian kanvas (ikon notifikasi). */
function viewBoxSquare(fill) {
  const side = (2 * center.half) / fill;
  return `${center.x - side / 2} ${center.y - side / 2} ${side} ${side}`;
}

/** white: ubah semua warna isi dan garis menjadi putih (opacity dipertahankan) untuk ikon siluet. */
const white = (svg) => svg.replace(/(fill|stroke)="#[0-9A-Fa-f]{3,6}"/g, '$1="#FFFFFF"');

// Daftar ikon: nama berkas, ukuran piksel, dan isi SVG.
const icons = [
  // Logo utuh apa adanya.
  { name: 'icon.png', size: 1024, svg: logo },
  // Latar polos ikon adaptif.
  {
    name: 'android-icon-background.png',
    size: 1024,
    svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1" fill="${plate}"/></svg>`,
  },
  // Lapisan depan ikon adaptif: seluruh lingkaran di dalam lingkaran berdiameter 58% kanvas, sedikit di bawah zona
  // aman 61% (66/108), supaya tidak terpotong masker launcher bentuk apa pun.
  {
    name: 'android-icon-foreground.png',
    size: 1024,
    svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBoxInCircle(0.58)}">${circles}</svg>`,
  },
  // Ikon bertema: geometri sama dengan foreground, warna putih.
  {
    name: 'android-icon-monochrome.png',
    size: 1024,
    svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBoxInCircle(0.58)}">${white(circles)}</svg>`,
  },
  // Ikon notifikasi: siluet putih, hampir penuh (ikon ini kecil, 24dp).
  {
    name: 'notification-icon.png',
    size: 96,
    svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBoxSquare(0.9)}">${white(circles)}</svg>`,
  },
  // Splash screen: lingkaran berwarna di dalam lingkaran berdiameter 62% kanvas (zona aman ikon splash Android 12+:
  // isi harus muat di lingkaran 2/3 kanvas). Latar transparan; warna latar splash diatur di app.json.
  {
    name: 'splash-icon.png',
    size: 1024,
    svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBoxInCircle(0.62)}">${circles}</svg>`,
  },
];

const browser = await chromium.launch();
try {
  const page = await browser.newPage();
  for (const { name, size, svg } of icons) {
    // SVG dipaksa ke ukuran target supaya hasil tajam (bukan PNG kecil yang diperbesar).
    const sized = svg.replace(/<svg\b([^>]*?)(\swidth="[^"]*")?(\sheight="[^"]*")?/, `<svg$1 width="${size}" height="${size}"`);
    await page.setViewportSize({ width: size, height: size });
    await page.setContent(
      `<html><body style="margin:0;background:transparent">${sized}</body></html>`,
    );
    // omitBackground: latar halaman transparan, jadi sudut membulat dan area kosong tetap transparan.
    const png = await page.locator('svg').first().screenshot({ omitBackground: true });
    writeFileSync(out(name), png);
    console.log(`ditulis assets/images/${name} (${size}x${size})`);
  }
} finally {
  await browser.close();
}
