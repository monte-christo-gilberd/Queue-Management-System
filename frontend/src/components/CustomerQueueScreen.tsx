import { PushToggle } from '@/components/PushToggle';
import { Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { cn } from '@/utils/cn';

export type CustomerTicketStatus = 'waiting' | 'completed' | 'cancelled' | 'skipped';

export type CustomerQueueScreenProps = {
  queueName: string;
  /** Deskripsi antrian dari owner (opsional). */
  queueDescription?: string | null;
  /** Nomor tiket saya yang sudah diformat (mis. "A-042"); null bila belum bergabung. */
  ticketNumber: string | null;
  ticketStatus: CustomerTicketStatus | null;
  /** Nomor saya sedang dilayani (tiket completed yang masih nomor sekarang). */
  servingNow?: boolean;
  /** Nomor yang sedang dilayani, sudah diformat; null bila belum ada. */
  currentNumber: string | null;
  /** Nama counter yang sedang memanggil (null bila counter default disembunyikan). */
  counterName?: string | null;
  /** Jumlah orang di depan saya; null bila tidak sedang menunggu. */
  waitingAhead: number | null;
  closed: boolean;
  loading?: boolean;
  errorMessage?: string;
  /** Tombol "Gabung Antrian" hanya tampil bila belum bergabung dan antrian masih buka. */
  canJoin: boolean;
  joining?: boolean;
  onJoin?: () => void;
  /** Tombol kembali ke halaman utama. Tidak tampil bila tidak diberikan. */
  onBack?: () => void;
};

const STATUS_LABEL: Record<CustomerTicketStatus, string> = {
  waiting: 'Menunggu',
  completed: 'Sudah dilayani',
  skipped: 'Dilewati',
  cancelled: 'Dibatalkan',
};

/** Layar customer (presentational). Hanya menampilkan data yang diterima lewat props. */
export function CustomerQueueScreen({
  queueName,
  queueDescription,
  ticketNumber,
  ticketStatus,
  servingNow = false,
  currentNumber,
  counterName,
  waitingAhead,
  closed,
  loading = false,
  errorMessage,
  canJoin,
  joining = false,
  onJoin,
  onBack,
}: CustomerQueueScreenProps) {
  return (
    <View className="flex-1 items-center bg-background px-5 py-10">
      <View className="w-full max-w-[420px] gap-5">
        <Text className="text-center text-[13px] font-semibold text-muted">Antrian</Text>
        <Text className="text-center text-[24px] font-black text-foreground">
          {queueName || (loading ? 'Memuat...' : 'Antrian')}
        </Text>
        {queueDescription ? (
          <Text className="text-center text-[14px] text-muted">{queueDescription}</Text>
        ) : null}

        {errorMessage ? (
          <Text className="text-center text-[13px] text-live">{errorMessage}</Text>
        ) : null}

        {closed ? (
          <View className="rounded-card border border-line p-5">
            <Text className="text-center text-[15px] font-extrabold text-foreground">Antrian ditutup</Text>
            <Text className="mt-1 text-center text-[12px] text-muted">
              Antrian ini sudah tidak menerima pendaftaran baru.
            </Text>
          </View>
        ) : null}

        {ticketNumber ? (
          <View className="gap-4 rounded-card bg-primary p-5">
            <Text className="text-center text-[12px] font-semibold text-white">Nomor Anda</Text>
            <Text className="text-center text-[56px] font-black text-white">{ticketNumber}</Text>
            <Text className="text-center text-[13px] font-bold text-white">
              {servingNow ? 'Sedang dilayani' : ticketStatus ? STATUS_LABEL[ticketStatus] : ''}
            </Text>
          </View>
        ) : null}

        {ticketNumber ? (
          <View className="gap-3 rounded-card border border-line p-4">
            <Row label="Sedang dilayani" value={currentNumber ?? '-'} />
            {counterName ? <Row label="Counter" value={counterName} /> : null}
            <Row
              label="Orang di depan Anda"
              value={waitingAhead === null ? '-' : String(waitingAhead)}
            />
          </View>
        ) : null}

        {canJoin ? (
          <AppButton label="Gabung Antrian" variant="dark" loading={joining} onPress={onJoin} />
        ) : null}

        {ticketNumber && !closed && (ticketStatus === 'waiting' || servingNow) ? <PushToggle /> : null}
        {onBack ? <AppButton label="Kembali ke Halaman Utama" variant="outline" onPress={onBack} /> : null}
      </View>
    </View>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <View className="flex-row items-center justify-between">
      <Text className="text-[13px] font-normal text-muted">{label}</Text>
      <Text className={cn('text-[15px] font-extrabold text-foreground')}>{value}</Text>
    </View>
  );
}
