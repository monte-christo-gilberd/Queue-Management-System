/**
 * ConfirmModal: popup konfirmasi dua tombol (konfirmasi dan batal) di atas ModalCard.
 *
 * Dipakai oleh : components/Profile.tsx, components/DisplaySettingsCard.tsx, hooks/use-next-skip-confirm.tsx
 * Bergantung   : components/ui/AppButton, components/ui/ModalCard
 * Catatan      : Popup tidak menutup dirinya sendiri; induk yang mengubah visible setelah onConfirm atau onCancel.
 */
import { Text } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';

/** Props popup konfirmasi; perilakunya dijelaskan di ConfirmModal. */
export type ConfirmModalProps = {
  /** Popup tampil bila true. */
  visible: boolean;
  /** Judul tebal di atas pesan. */
  title: string;
  /** Penjelasan konsekuensi aksi, di bawah judul. */
  message: string;
  /** Teks tombol konfirmasi, mis. "Ya, Lanjut". */
  confirmLabel: string;
  /** Teks tombol batal. Default "Batal". */
  cancelLabel?: string;
  /** Bila true, tombol konfirmasi menampilkan spinner dan terkunci. Default false. */
  loading?: boolean;
  /** Dipanggil saat tombol konfirmasi ditekan. */
  onConfirm: () => void;
  /** Dipanggil saat tombol batal ditekan, latar gelap ditap, atau tombol back perangkat ditekan. */
  onCancel: () => void;
};

/**
 * Popup konfirmasi bersama untuk aksi yang tidak bisa di-undo (Next, Skip, hapus nomor).
 * Tidak ada `autoFocus` pada tombol konfirmasi, supaya klik ganda tidak mengonfirmasi tanpa sengaja.
 */
export function ConfirmModal({
  visible,
  title,
  message,
  confirmLabel,
  cancelLabel = 'Batal',
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  return (
    <ModalCard visible={visible} onRequestClose={onCancel}>
      <Text className="text-[20px] font-extrabold text-foreground">{title}</Text>
      <Text className="text-[13px] text-muted">{message}</Text>
      <AppButton label={confirmLabel} variant="dark" loading={loading} onPress={onConfirm} />
      <AppButton label={cancelLabel} variant="outline" onPress={onCancel} />
    </ModalCard>
  );
}
