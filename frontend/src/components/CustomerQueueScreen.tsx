/**
 * CustomerQueueScreen: layar status antrian untuk customer (nomor saya, antrian di depan, bergabung, dan batal).
 *
 * Dipakai oleh : app/ticket/[queueId].tsx, app/(user)/dashboard.tsx, components/ui/MyTicketModal.tsx
 * Bergantung   : components/PushToggle, ui/AppButton, ui/ModalCard, ui/Toast, utils/cn
 * Peran        : customer
 * Catatan      : presentasi murni; semua data dan aksi datang dari props, sedangkan pemanggilan API ada di pemanggil.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { useState } from 'react';
import { PushToggle } from '@/components/PushToggle';
import { Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import { cn } from '@/utils/cn';

/** Status tiket dari sudut pandang customer; nilainya sama dengan status tiket dari API. */
export type CustomerTicketStatus = 'waiting' | 'currently_served' | 'completed' | 'cancelled' | 'skipped' | 'deleted';

/** Props layar status antrian customer. */
export type CustomerQueueScreenProps = {
  /** Nama antrian. Bila kosong, judul menampilkan "Memuat..." (saat loading) atau "Antrian". */
  queueName: string;
  /** Deskripsi antrian dari owner (opsional). */
  queueDescription?: string | null;
  /** Nomor tiket saya yang sudah diformat (mis. "A-042"); null bila belum bergabung. */
  ticketNumber: string | null;
  /** Status tiket saya; null bila belum bergabung. Menentukan label di kartu nomor dan kapan notifikasi push tampil. */
  ticketStatus: CustomerTicketStatus | null;
  /** Nomor saya sedang dilayani (tiket berstatus currently_served). */
  servingNow?: boolean;
  /** Nomor yang sedang dilayani, sudah diformat; null bila belum ada. */
  currentNumber: string | null;
  /** Nama counter yang sedang memanggil (null bila counter default disembunyikan). */
  counterName?: string | null;
  /** Jumlah orang di depan saya; null bila tidak sedang menunggu. */
  waitingAhead: number | null;
  /** true bila antrian sudah ditutup: banner "Antrian ditutup" tampil dan notifikasi push disembunyikan. */
  closed: boolean;
  /** true saat data antrian sedang dimuat. Default false. */
  loading?: boolean;
  /** Galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
  errorMessage?: string;
  /** Tombol "Gabung Antrian" hanya tampil bila belum bergabung dan antrian masih buka. */
  canJoin: boolean;
  /** true saat permintaan bergabung berjalan; tombol "Gabung Antrian" berstatus loading. Default false. */
  joining?: boolean;
  /** Dipanggil saat "Gabung Antrian" ditekan; opsional. */
  onJoin?: () => void;
  /** Tombol "Batalkan nomor saya" tampil bila true (tiket waiting atau currently_served di antrian aktif). */
  canCancel?: boolean;
  /** true saat pembatalan berjalan; tombol konfirmasi berstatus loading. Default false. */
  cancelling?: boolean;
  /** Dipanggil hanya setelah customer mengonfirmasi pembatalan di modal. Opsional. */
  onCancel?: () => void;
  /** Tombol kembali ke halaman utama. Tidak tampil bila tidak diberikan. */
  onBack?: () => void;
};

/** Label Indonesia untuk setiap status tiket customer. */
export const STATUS_LABEL: Record<CustomerTicketStatus, string> = {
  waiting: 'Menunggu',
  currently_served: 'Sedang dilayani',
  completed: 'Sudah dilayani',
  skipped: 'Dilewati',
  cancelled: 'Dibatalkan',
  deleted: 'Dihapus',
};

/** Layar status antrian customer (presentational). Hanya menampilkan data yang diterima lewat props. */
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
  canCancel = false,
  cancelling = false,
  onCancel,
  onBack,
}: CustomerQueueScreenProps) {
  useErrorToast(errorMessage);
  const [cancelConfirmOpen, setCancelConfirmOpen] = useState(false);

  // Notifikasi push hanya ditawarkan bila tiket masih menunggu atau sedang dilayani di antrian yang belum ditutup.
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

        {canCancel && onCancel ? (
          <AppButton
            label="Batalkan nomor saya"
            variant="outline"
            loading={cancelling}
            onPress={() => setCancelConfirmOpen(true)}
          />
        ) : null}

        {ticketNumber && !closed && (ticketStatus === 'waiting' || servingNow) ? <PushToggle /> : null}
        {onBack ? <AppButton label="Kembali ke Halaman Utama" variant="outline" onPress={onBack} /> : null}
      </View>

      <ModalCard visible={cancelConfirmOpen} onRequestClose={() => setCancelConfirmOpen(false)}>
        <Text className="text-[20px] font-extrabold text-foreground">Batalkan nomor ini?</Text>
        <Text className="text-[13px] text-muted">
          Nomor Anda keluar dari antrian. Untuk kembali, Anda harus bergabung lagi dan mendapat nomor baru.
        </Text>
        <AppButton
          label="Ya, Batalkan"
          variant="dark"
          loading={cancelling}
          onPress={() => {
            onCancel?.();
            setCancelConfirmOpen(false);
          }}
        />
        <AppButton label="Tidak" variant="outline" onPress={() => setCancelConfirmOpen(false)} />
      </ModalCard>
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
