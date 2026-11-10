/**
 * use-me: profil user yang sedang login, dan penanda apakah user adalah owner.
 *
 * Dipakai oleh : src/app/(owner)/dashboard.tsx, src/app/(owner)/_layout.tsx.
 * Bergantung   : lib/users (getMe, tipe Me).
 * Alur         : muat sekali saat hook dipasang. Tidak ada cache lintas pemasangan.
 * Catatan      : Galat tidak ditampilkan. `me` tetap null dan `loaded` menjadi true, sehingga pemanggil menganggap user bukan owner.
 */

import { useEffect, useState } from 'react';

import { getMe, type Me } from '@/lib/users';

/**
 * Profil user yang login. `loaded` false selama request berjalan; selama itu jangan tampilkan fitur owner.
 *
 * Tanpa argumen. Mengembalikan objek dengan field:
 * - `me`: profil user. null bila belum dimuat atau gagal dimuat.
 * - `isOwner`: true hanya bila profil memuat `is_owner` bernilai true.
 * - `loaded`: true setelah request selesai, berhasil atau gagal.
 */
export function useMe() {
  const [me, setMe] = useState<Me | null>(null);
  const [loaded, setLoaded] = useState(false);

  // Muat profil dengan then(). Guard `active` mencegah setState setelah unmount.
  useEffect(() => {
    let active = true;
    void getMe().then(
      (data) => {
        if (active) {
          setMe(data);
          setLoaded(true);
        }
      },
      () => {
        if (active) setLoaded(true);
      },
    );
    return () => {
      active = false;
    };
  }, []);

  return { me, isOwner: me?.is_owner === true, loaded };
}
