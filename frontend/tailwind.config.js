const plugin = require('tailwindcss/plugin');
const {
  fixedColors,
  themedColors,
  cssVars,
  radius,
  wideBreakpoint,
  desktopMaxWidth,
} = require('./src/constants/tokens');

// Mode gelap aktif: nilai `.dark` dikirim. Toggle ada di src/hooks/use-theme-mode.ts (NativeWind colorScheme).
const ENABLE_DARK_MODE = true;

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
