import { useEffect, useState } from 'react';
import { Text, TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import { validateInsertInput } from '@/lib/tickets';

type InsertTicketModalProps = {
  visible: boolean;
  /** Posisi awal: panjang antrian menunggu + 1 (paling belakang). */
  defaultPosition: number;
  saving?: boolean;
  errorMessage?: string;
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

  async function submit() {
    const position = Number(positionText);
    const problem = validateInsertInput(identifier, position);
    setValidationError(problem ?? undefined);
    if (problem) return;
    const ok = await onSubmit(identifier, position);
    if (ok) onClose();
  }

  const shown = validationError ?? errorMessage;

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
      {shown ? <Text className="text-[12px] font-normal text-live">{shown}</Text> : null}
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
