/**
 * +html: shell HTML root untuk versi web (static rendering), pembungkus semua halaman.
 *
 * Rute        : tidak ada (berlaku untuk semua rute saat dirender ke HTML di web).
 * Akses       : publik.
 * Dipakai oleh: Expo Router saat membangun HTML statis untuk web.
 * Bergantung  : ScrollViewStyleReset dari expo-router/html.
 * Catatan     : Judul tab diatur di _layout.tsx lewat Head, bukan di sini. Ikon memakai salinan logo di public/.
 */
import { ScrollViewStyleReset } from 'expo-router/html';
import type { PropsWithChildren } from 'react';

/**
 * Dokumen HTML root untuk web: meta charset, viewport, dan favicon. Atribut lang="id" membuat dokumen berbahasa Indonesia.
 */
export default function Root({ children }: PropsWithChildren) {
  return (
    <html lang="id">
      <head>
        <meta charSet="utf-8" />
        <meta httpEquiv="X-UA-Compatible" content="IE=edge" />
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no" />
        {/* Judul tab ditetapkan di _layout.tsx lewat <Head> (agar tidak ditimpa navigasi klien). */}
        {/* Logo diambil dari frontend/logo/logo.svg (salinan di public/ agar disajikan di root). */}
        <link rel="icon" type="image/svg+xml" href="/logo.svg" />
        <ScrollViewStyleReset />
      </head>
      <body>{children}</body>
    </html>
  );
}
