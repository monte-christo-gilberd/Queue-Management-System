/**
 * MyHistoryList: riwayat tiket milik customer, dengan popup detail per tiket.
 *
 * Dipakai oleh : app/(user)/riwayat.tsx
 * Bergantung   : ui/AppButton, ui/ModalCard, types/api (MyHistoryTicket), utils/format, utils/history
 * Peran        : customer
 * Catatan      : hanya baca; catatan owner tidak ditampilkan kepada customer.
 */
import { useState } from 'react';
import { Pressable, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import type { MyHistoryTicket } from '@/types/api';
import { formatDateTime, formatTicketNumber } from '@/utils/format';
import { customerOutcomeLabel, waitDuration } from '@/utils/history';

function Row({ label, value }: { label: string; value: string }) {
  return (
    <View className="flex-row items-center justify-between gap-3">
      <Text className="text-[13px] text-muted">{label}</Text>
      <Text className="flex-1 text-right text-[14px] font-bold text-foreground">{value}</Text>
    </View>
  );
}

/** Popup detail satu tiket di riwayat (baca saja). Tanpa catatan owner: catatan tidak pernah dilihat customer. */
function MyHistoryModal({ ticket, onClose }: { ticket: MyHistoryTicket | null; onClose: () => void }) {
  const waited = ticket ? waitDuration(ticket) : null;
  return (
    <ModalCard visible={ticket !== null} onRequestClose={onClose}>
      {ticket ? (
        <>
          <Text className="text-[20px] font-extrabold text-foreground">Detail Tiket</Text>
          <Text className="text-[13px] text-muted">{ticket.queue_name}</Text>
          <View className="items-center gap-1 rounded-card bg-surface py-5">
            <Text className="text-[36px] font-extrabold text-foreground">
              {formatTicketNumber(ticket.ticket_number, ticket.ticket_prefix)}
            </Text>
            <Text className="text-[14px] font-bold text-foreground">{customerOutcomeLabel(ticket.outcome)}</Text>
          </View>
          <Row label="Bergabung" value={formatDateTime(ticket.joined_at)} />
          {ticket.called_at ? <Row label="Dipanggil" value={formatDateTime(ticket.called_at)} /> : null}
          {waited ? <Row label="Lama menunggu" value={waited} /> : null}
          {ticket.completed_at ? <Row label="Selesai" value={formatDateTime(ticket.completed_at)} /> : null}
          {ticket.cancelled_at ? <Row label="Dibatalkan" value={formatDateTime(ticket.cancelled_at)} /> : null}
          {ticket.counter_name ? <Row label="Counter" value={ticket.counter_name} /> : null}
          {ticket.skip_count > 0 ? <Row label="Dilewati" value={`${ticket.skip_count}x`} /> : null}
          {ticket.queue_closed ? <Text className="text-[12px] text-muted">Antrian ini sudah ditutup.</Text> : null}
        </>
      ) : null}
      <AppButton label="Tutup" variant="outline" onPress={onClose} />
    </ModalCard>
  );
}

/** Props daftar riwayat tiket customer. */
export type MyHistoryListProps = {
  /** Tiket riwayat milik customer, satu kartu per tiket. Kosong berarti tidak ada yang dirender. */
  items: MyHistoryTicket[];
  /** true bila masih ada halaman berikutnya; tombol "Muat lagi" ditampilkan. Default false. */
  hasMore?: boolean;
  /** true saat halaman berikutnya sedang dimuat; tombol "Muat lagi" berstatus loading. Default false. */
  loadingMore?: boolean;
  /** Dipanggil saat tombol "Muat lagi" ditekan untuk mengambil halaman berikutnya. Opsional. */
  onLoadMore?: () => void;
};

/** Riwayat tiket saya, satu kartu per tiket (presentational). Ketuk kartu untuk melihat detail tiket. */
export function MyHistoryList({ items, hasMore = false, loadingMore = false, onLoadMore }: MyHistoryListProps) {
  const [selected, setSelected] = useState<MyHistoryTicket | null>(null);
  if (items.length === 0) return null;

  return (
    <>
      <View className="gap-3">
        {items.map((t) => (
          <Pressable
            key={t.ticket_id}
            accessibilityRole="button"
            accessibilityLabel={`Detail tiket ${formatTicketNumber(t.ticket_number, t.ticket_prefix)} ${t.queue_name}`}
            onPress={() => setSelected(t)}
            className="flex-row items-center justify-between rounded-card border border-line p-3.5">
            <View className="flex-1 gap-0.5">
              <Text className="text-[15px] font-extrabold text-foreground">{t.queue_name}</Text>
              <Text className="text-[12px] text-muted">
                {customerOutcomeLabel(t.outcome)} · {formatDateTime(t.joined_at)}
              </Text>
            </View>
            <Text className="text-[18px] font-black text-primary">
              {formatTicketNumber(t.ticket_number, t.ticket_prefix)}
            </Text>
          </Pressable>
        ))}
        {hasMore ? <AppButton label="Muat lagi" variant="outline" loading={loadingMore} onPress={onLoadMore} /> : null}
      </View>
      <MyHistoryModal ticket={selected} onClose={() => setSelected(null)} />
    </>
  );
}
