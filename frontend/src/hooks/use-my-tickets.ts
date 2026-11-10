/**
 * use-my-tickets: daftar tiket milik akun ini di semua antrian, untuk daftar "Antrian Saya".
 *
 * Dipakai oleh : src/app/(owner)/antrian.tsx, src/app/(owner)/dashboard.tsx.
 * Bergantung   : lib/tickets (listMyTickets, tipe MyTicket).
 * Alur         : muat sekali saat dipasang. `reload` dipanggil pemanggil setelah tiket berubah.
 * Catatan      : Daftar ini tambahan, bukan inti halaman, sehingga galat diabaikan dan daftar dikosongkan.
 */

import { useCallback, useEffect, useState } from 'react';

import { listMyTickets, type MyTicket } from '@/lib/tickets';

/**
 * Tiket saya di semua antrian. Gagal diam-diam (daftar ini tambahan, bukan inti halaman).
 *
 * Tanpa argumen. Mengembalikan objek dengan field:
 * - `tickets`: daftar tiket milik akun ini. Kosong bila belum ada atau gagal dimuat.
 * - `reload()`: memuat ulang daftar. Galat dijadikan daftar kosong.
 */
export function useMyTickets() {
  const [tickets, setTickets] = useState<MyTicket[]>([]);

  // `reload` dipakai pemanggil setelah aksi, sedangkan effect di bawah hanya memuat saat dipasang.
  const reload = useCallback(async () => {
    try {
      setTickets(await listMyTickets());
    } catch {
      setTickets([]);
    }
  }, []);

  useEffect(() => {
    let active = true;
    void listMyTickets().then(
      (data) => {
        if (active) setTickets(data);
      },
      () => {
        if (active) setTickets([]);
      },
    );
    return () => {
      active = false;
    };
  }, []);

  return { tickets, reload };
}
