import { Text } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';

export type ConfirmModalProps = {
  visible: boolean;
  title: string;
  message: string;
  /** Teks tombol konfirmasi, mis. "Ya, Lanjut". */
  confirmLabel: string;
  cancelLabel?: string;
  loading?: boolean;
  onConfirm: () => void;
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
