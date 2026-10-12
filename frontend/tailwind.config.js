const plugin = require('tailwindcss/plugin');
const {
  fixedColors,
  themedColors,
  cssVars,
  radius,
  wideBreakpoint,
  desktopMaxWidth,
} = require('./src/constants/tokens');

// Ubah ke true setelah toggle tema (state) diimplementasikan. Selama false, hanya nilai terang yang dikirim.
// Catatan: di native, NativeWind memakai `colorScheme.set()` — sesuaikan cara pemilihan mode saat mengaktifkan.
const ENABLE_DARK_MODE = false;

/** @type {import('tailwindcss').Config} */
module.exports = {
  // Semua kode ada di ./src (route di src/app, komponen di src/components).
  content: ['./src/**/*.{js,jsx,ts,tsx}'],
  presets: [require('nativewind/preset')],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ...fixedColors,
        // Warna bertema merujuk variabel CSS: bg-background, text-foreground, border-line, ...
        ...Object.fromEntries(Object.keys(themedColors).map((k) => [k, `var(--color-${k})`])),
      },
      borderRadius: {
        input: `${radius.input}px`,
        button: `${radius.button}px`,
        card: `${radius.card}px`,
        pill: `${radius.pill}px`,
      },
      maxWidth: { desktop: `${desktopMaxWidth}px` },
      // Pengganti useIsWide(): `wide:flex-row` aktif di lebar >= 1024.
      screens: { wide: `${wideBreakpoint}px` },
    },
  },
  plugins: [
    plugin(({ addBase }) => {
      addBase({
        ':root': cssVars('light'),
        ...(ENABLE_DARK_MODE ? { '.dark': cssVars('dark') } : {}),
      });
    }),
  ],
};
