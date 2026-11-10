/**
 * MyTicketModal: popup ringkasan tiket milik customer di satu antrian.
 *
 * Dipakai oleh : app/(owner)/dashboard.tsx
 * Bergantung   : components/CustomerQueueScreen (STATUS_LABEL, tipe CustomerTicketStatus), components/ui/ModalCard,
 *                components/ui/AppButton, constants/theme (Fonts)
 * Catatan      : Data seluruhnya dari props; popup ini tidak mengambil data sendiri.
 */
import { Text, View } from 'react-native';

import { STATUS_LABEL, type CustomerTicketStatus } from '@/components/CustomerQueueScreen';
import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import { Fonts } from '@/constants/theme';

// Fonts?. melindungi bila Fonts tidak terdefinisi; MONO yang undefined berarti font default.
const MONO = Fonts?.mono;

/** Props popup tiket customer; perilakunya dijelaskan di MyTicketModal. */
export type MyTicketModalProps = {
  /** Popup tampil bila true. */
  visible: boolean;
  /** Dipanggil saat tombol Tutup, latar, atau back perangkat ditekan. */
  onClose: () => void;
  /** Nama antrian, tampil sebagai subjudul. */
  queueName: string;
  /** Nomor tiket saya yang sudah diformat (mis. "A-042"); null bila saya tidak punya tiket di antrian ini. */
  ticketNumber: string | null;
  /** Status tiket saya untuk label di bawah nomor; null bila tidak ada tiket. */
  ticketStatus: CustomerTicketStatus | null;
  /** Nomor yang sedang dilayani, sudah diformat; null bila belum ada. */
  currentNumber: string | null;
  /** Counter tempat nomor saya dilayani (null bila counter default disembunyikan). */
  counterName?: string | null;
  /** Jumlah orang di depan saya; null bila saya tidak sedang menunggu. */
  waitingAhead: number | null;
};

/** Baris label dan nilai di dalam popup. */
function Row({ label, value }: { label: string; value: string }) {
  return (
    <View className="flex-row items-center justify-between">
      <Text className="text-[13px] text-muted">{label}</Text>
      <Text className="text-[14px] font-bold text-foreground">{value}</Text>
    </View>
  );
}

/** Popup tiket customer: nomor, status, nomor yang sedang dilayani, dan jumlah orang di depan. */
export function MyTicketModal({
  visible,
  onClose,
  queueName,
  ticketNumber,
  ticketStatus,
  currentNumber,
  counterName,
  waitingAhead,
}: MyTicketModalProps) {
  // Baris "Di depan Anda" hanya tampil bila saya sedang menunggu (waitingAhead tidak null).
  // Baris "Counter" hanya tampil saat nomor saya sedang dilayani.
  return (
    <ModalCard visible={visible} onRequestClose={onClose}>
      <Text className="text-[20px] font-extrabold text-foreground">Tiket Anda</Text>
      <Text className="text-[13px] text-muted">{queueName}</Text>
      {ticketNumber ? (
        <>
          <View className="items-center gap-1 rounded-card bg-surface py-5">
            <Text className="text-[40px] font-extrabold text-foreground" style={{ fontFamily: MONO }}>
              {ticketNumber}
            </Text>
            <Text className="text-[14px] font-bold text-foreground">
              {ticketStatus ? STATUS_LABEL[ticketStatus] : ''}
            </Text>
          </View>
          <Row label="Sedang dilayani" value={currentNumber ?? '-'} />
          {waitingAhead !== null ? <Row label="Di depan Anda" value={`${waitingAhead} orang`} /> : null}
          {ticketStatus === 'currently_served' && counterName ? <Row label="Counter" value={counterName} /> : null}
        </>
      ) : (
        <Text className="text-[14px] text-foreground">Anda belum memiliki tiket di antrian ini.</Text>
      )}
      <AppButton label="Tutup" variant="outline" onPress={onClose} />
    </ModalCard>
  );
}
