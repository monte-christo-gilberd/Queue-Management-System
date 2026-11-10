/**
 * Sumber tunggal token desain (warna, radius, breakpoint).
 * Dipakai oleh `src/constants/app-theme.ts` (prop non-style) DAN `tailwind.config.js` (NativeWind).
 * Ditulis sebagai CommonJS karena tailwind.config.js tidak bisa mengimpor TypeScript.
 */

/**
 * Warna TETAP — tidak berubah antar tema: merek, status, dan elemen halaman auth (login/register/lupa
 * password yang selalu gelap) serta chrome gelap (header, tombol hitam).
 */
const fixedColors = {
  dark: '#1C1A1B', // chrome gelap (header, tombol hitam, latar auth) — BUKAN warna teks utama
  white: '#FFFFFF',
  black: '#000000',
  primary: '#00B2FF',
  primarySoft: '#62B5FF',
  accent: '#0A00B4',
  live: '#FF0000',
  badgeDone: '#DDE83A',
  // dipakai halaman auth & tombol Google (tidak ikut tema)
  textMuted: '#555555',
  border: '#B9B9B9',
  placeholder: '#B3B3B3',
  overlay: 'rgba(0,0,0,0.25)',
  scrim: 'rgba(0,0,0,0.2)',
  modalBackdrop: 'rgba(0,0,0,0.3)',
  panelActive: 'rgba(255,255,255,0.45)',
  // papan display gelap + indikator "live" di QueueLiveScreen (sengaja tidak ikut tema)
  board: '#14181D',
  boardLine: '#2A3037',
  boardMuted: '#8B9199',
  boardDim: '#5E656D',
  liveGreen: '#4ADE80',
};

/**
 * Warna BERTEMA — dipakai halaman selain auth lewat variabel CSS (`bg-background`, `text-foreground`, ...).
 * `light` aktif sekarang. `dark` sudah disiapkan untuk nanti; aktifkan dengan ENABLE_DARK_MODE
 * di tailwind.config.js setelah mekanisme toggle (state tema) dibuat.
 */
const themedColors = {
  background: { light: '#E5E7EB', dark: '#121417' }, // latar halaman
  surface: { light: '#F3F3F3', dark: '#2E343D' }, // kartu / input terisi (dark: beda jelas dari latar)
  foreground: { light: '#1C1A1B', dark: '#F3F4F6' }, // teks utama
  muted: { light: '#555555', dark: '#9CA3AF' }, // teks sekunder
  line: { light: '#B9B9B9', dark: '#565E6A' }, // border kartu / garis timeline
  divider: { light: '#D1D5DB', dark: '#3A414B' }, // garis pemisah (tab bar)
  disabled: { light: '#C4C4C4', dark: '#737B87' }, // elemen nonaktif / pudar
  avatar: { light: '#C9B6B3', dark: '#8F7B77' }, // ikon kosong & latar avatar (dark: terbaca di atas kartu)
  panel: { light: '#94AEFF', dark: '#2B3A6B' }, // latar collapsible
};

/**
 * Radius sudut dalam piksel. Nama kunci didaftarkan ke tailwind-merge di utils/cn.ts agar `rounded-pill`
 * menimpa `rounded-button`, dan dipasang ke tailwind.config.js sebagai `rounded-<nama>`.
 */
const radius = {
  /** Sudut kolom input teks. */
  input: 12,
  /** Sudut tombol. */
  button: 12,
  /** Sudut kartu. */
  card: 16,
  /** Bentuk kapsul (pill), nilai besar agar sudut menjadi setengah lingkaran. */
  pill: 999,
};

/** Breakpoint layar lebar (PC/tablet) — sama dengan `Layout.wideBreakpoint`. */
const wideBreakpoint = 1024;
/** Lebar maksimal konten di layar lebar (piksel), sama dengan `Layout.desktopMaxWidth`. */
const desktopMaxWidth = 1200;

/** Semua warna versi terang (datar) — untuk prop non-style (warna ikon, placeholderTextColor, spinner).
 *  Kunci ditulis eksplisit agar TypeScript mengenali setiap nama. */
const colors = {
  ...fixedColors,
  background: themedColors.background.light,
  surface: themedColors.surface.light,
  foreground: themedColors.foreground.light,
  muted: themedColors.muted.light,
  line: themedColors.line.light,
  divider: themedColors.divider.light,
  disabled: themedColors.disabled.light,
  avatar: themedColors.avatar.light,
  panel: themedColors.panel.light,
};

/** Peta `--color-<nama>` → nilai untuk satu mode. */
function cssVars(mode) {
  return Object.fromEntries(
    Object.entries(themedColors).map(([k, v]) => [`--color-${k}`, v[mode]]),
  );
}

module.exports = {
  fixedColors,
  themedColors,
  colors,
  cssVars,
  radius,
  wideBreakpoint,
  desktopMaxWidth,
};
