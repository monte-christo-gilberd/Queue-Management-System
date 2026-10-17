import { useCallback, useEffect, useState } from 'react';

import { listMyTickets, type MyTicket } from '@/lib/tickets';

/** Tiket saya di semua antrian. Gagal diam-diam (daftar ini tambahan, bukan inti halaman). */
export function useMyTickets() {
  const [tickets, setTickets] = useState<MyTicket[]>([]);

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
