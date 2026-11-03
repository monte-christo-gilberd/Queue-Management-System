import { Pressable, Text, View } from 'react-native';

import type { MyTicket } from '@/lib/tickets';
import { formatTicketNumber } from '@/utils/format';

const STATUS_LABEL: Record<MyTicket['status'], string> = {
  waiting: 'Menunggu',
  currently_served: 'Sedang dilayani',
  completed: 'Sudah dilayani',
  skipped: 'Dilewati',
  cancelled: 'Dibatalkan',
  deleted: 'Dihapus',
};

export type MyTicketsListProps = {
  tickets: MyTicket[];
  onOpen: (queueId: number) => void;
};

/** Satu kartu tiket saya (presentational). Ketuk untuk membuka status antrian. */
export function MyTicketCard({ ticket, onOpen }: { ticket: MyTicket; onOpen: (queueId: number) => void }) {
  return (
    <Pressable
      onPress={() => onOpen(ticket.queue_id)}
      className="flex-row items-center justify-between rounded-card border border-line p-3.5">
      <View className="gap-0.5">
        <Text className="text-[15px] font-extrabold text-foreground">{ticket.queue_name}</Text>
        <Text className="text-[12px] text-muted">
          {STATUS_LABEL[ticket.status]}
          {ticket.queue_status === 'closed' ? ' · antrian ditutup' : ''}
        </Text>
      </View>
      <Text className="text-[18px] font-black text-primary">
        {formatTicketNumber(ticket.ticket_number, ticket.ticket_prefix)}
      </Text>
    </Pressable>
  );
}

/** Daftar tiket saya (presentational). Ketuk untuk membuka status antrian. */
export function MyTicketsList({ tickets, onOpen }: MyTicketsListProps) {
  if (tickets.length === 0) return null;

  return (
    <View className="gap-3">
      {tickets.map((ticket) => (
        <MyTicketCard key={ticket.id} ticket={ticket} onOpen={onOpen} />
      ))}
    </View>
  );
}
