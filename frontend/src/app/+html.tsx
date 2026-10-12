import { ScrollViewStyleReset } from 'expo-router/html';
import type { PropsWithChildren } from 'react';

// Dokumen HTML root untuk web (static rendering): meta dasar dan favicon.
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
