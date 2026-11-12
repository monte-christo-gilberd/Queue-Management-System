/**
 * use-me: profil user yang sedang login, dan penanda apakah user adalah owner.
 *
 * Dipakai oleh : src/app/(owner)/dashboard.tsx, src/app/(owner)/_layout.tsx.
 * Bergantung   : lib/users (getMe, tipe Me).
 * Alur         : muat saat hook dipasang. Pemasangan yang bersamaan berbagi satu request (fetchMeShared); tidak ada cache setelah selesai.
 * Catatan      : Galat tidak ditampilkan. `me` tetap null dan `loaded` menjadi true, sehingga pemanggil menganggap user bukan owner.
 */

import { useEffect, useState } from 'react';

import { getMe, type Me } from '@/lib/users';

/**
 * Request GET /users/me yang sedang berjalan. Pemanggil yang mount bersamaan (mis. layout (owner) dan dashboard)
 * berbagi satu request. Dibersihkan begitu selesai, sehingga pemasangan berikutnya tetap mengambil data terbaru.
 */
let inflightMe: Promise<Me> | null = null;

function fetchMeShared(): Promise<Me> {
  if (!inflightMe) {
    inflightMe = getMe().finally(() => {
      inflightMe = null;
    });
  }
  return inflightMe;
}

/** Pemanggil useMe yang sedang terpasang; semuanya diberi profil baru saat refreshMe dipanggil. */
const listeners = new Set<(me: Me) => void>();

/**
 * refreshMe: ambil ulang profil lalu perbarui semua useMe yang terpasang (mis. menu owner di panel kanan berubah
 * langsung setelah user berhasil menjadi owner). Galat diabaikan; data lama tetap dipakai.
 */
export async function refreshMe(): Promise<void> {
  try {
    const data = await getMe();
    listeners.forEach((fn) => fn(data));
  } catch {
    // data lama tetap dipakai
  }
}

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
    void fetchMeShared().then(
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
    const listener = (data: Me) => {
      if (active) setMe(data);
    };
    listeners.add(listener);
    return () => {
      active = false;
      listeners.delete(listener);
    };
  }, []);

  return { me, isOwner: me?.is_owner === true, loaded };
}
