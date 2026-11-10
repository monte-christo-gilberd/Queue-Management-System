/**
 * InsertTicketModal: popup owner untuk menyisipkan customer yang sudah punya akun ke posisi tertentu di antrian.
 *
 * Dipakai oleh : components/QueueDetail.tsx
 * Bergantung   : components/ui/Toast (useErrorToast), lib/tickets (validateInsertInput),
 *                components/ui/AppButton, components/ui/ModalCard
 * Alur         : isian -> validasi lokal -> onSubmit (server) -> popup menutup bila berhasil
 * Catatan      : Validasi lokal dijalankan dulu; server tidak dipanggil bila isian belum lolos.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { useEffect, useState } from 'react';
import { Text, TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import { validateInsertInput } from '@/lib/tickets';

type InsertTicketModalProps = {
  /** Popup tampil bila true. Isian di-reset setiap kali berubah menjadi true. */
  visible: boolean;
  /** Posisi awal: panjang antrian menunggu + 1 (paling belakang). */
  defaultPosition: number;
  /** Bila true, isian dikunci dan tombol berbunyi "Menyisipkan...". Default false. */
  saving?: boolean;
  /** Galat dari server (opsional); tampil sebagai toast. */
  errorMessage?: string;
  /** Dipanggil saat popup diminta menutup (Batal, latar, atau back perangkat, atau setelah sisip berhasil). */
  onClose: () => void;
  /** Mengembalikan true bila sisip berhasil; popup lalu menutup diri. */
  onSubmit: (identifier: string, position: number) => Promise<boolean>;
};

/** Popup sisip customer (US-10): isian email atau username, dan posisi tujuan. */
export function InsertTicketModal({
  visible,
  defaultPosition,
  saving = false,
  errorMessage,
  onClose,
  onSubmit,
}: InsertTicketModalProps) {
  const [identifier, setIdentifier] = useState('');
  const [positionText, setPositionText] = useState(String(defaultPosition));
  const [validationError, setValidationError] = useState<string>();

  // Setiap popup dibuka, isian kembali ke nilai awal.
  useEffect(() => {
    if (visible) {
      setIdentifier('');
      setPositionText(String(defaultPosition));
      setValidationError(undefined);
    }
  }, [visible, defaultPosition]);

  // Posisi dibaca dari teks; pesan galat validasi berasal dari validateInsertInput.
  async function submit() {
    const position = Number(positionText);
    const problem = validateInsertInput(identifier, position);
    setValidationError(problem ?? undefined);
    if (problem) return;
    const ok = await onSubmit(identifier, position);
    if (ok) onClose();
  }

  // Galat validasi lokal didahulukan atas galat dari server.
  const shown = validationError ?? errorMessage;
  useErrorToast(shown);

  return (
    <ModalCard visible={visible} onRequestClose={onClose}>
      <Text className="text-[20px] font-extrabold text-foreground">Sisip customer</Text>
      <Text className="text-[13px] text-muted">
        Customer harus sudah punya akun. Masukkan email atau username, lalu posisi tujuannya (1 = berikutnya).
      </Text>
      <Text className="text-[13px] font-semibold text-foreground">Email atau username</Text>
      <TextInput
        value={identifier}
        onChangeText={setIdentifier}
        editable={!saving}
        autoCapitalize="none"
        placeholder="contoh: budi@mail.com"
        placeholderTextColor={AppColors.placeholder}
        className="rounded-card bg-surface p-3.5 text-[14px] text-foreground"
      />
      <Text className="text-[13px] font-semibold text-foreground">Posisi</Text>
      <TextInput
        value={positionText}
        onChangeText={setPositionText}
        editable={!saving}
        keyboardType="number-pad"
        placeholder="contoh: 1"
        placeholderTextColor={AppColors.placeholder}
        className="rounded-card bg-surface p-3.5 text-[14px] text-foreground"
      />
      <View className="flex-row gap-3">
        <AppButton label="Batal" variant="outline" className="flex-1" onPress={onClose} />
        <AppButton
          label={saving ? 'Menyisipkan...' : 'Sisip'}
          variant="accent"
          className="flex-1"
          disabled={saving}
          onPress={() => void submit()}
        />
      </View>
    </ModalCard>
  );
}
