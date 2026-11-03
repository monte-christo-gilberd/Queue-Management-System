import { useEffect, useState } from 'react';
import { Text, TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';

type AppendNoteModalProps = {
  visible: boolean;
  onCancel: () => void;
  /** Dipanggil dengan isi catatan; penutupan popup diatur pemanggil setelah simpan berhasil. */
  onSubmit: (note: string) => void;
  /** Catatan yang sudah ada untuk nomor ini; mengisi TextInput setiap popup dibuka. */
  initialNote?: string;
  /** True bila nomor ini sudah punya catatan: tombol berbunyi "Simpan Catatan". */
  hasNote?: boolean;
  loading?: boolean;
  saving?: boolean;
  errorMessage?: string;
  placeholder?: string;
};

export function AppendNoteModal({
  visible,
  onCancel,
  onSubmit,
  initialNote = '',
  hasNote = false,
  loading = false,
  saving = false,
  errorMessage,
  placeholder = 'Tulis catatan untuk nomor ini...',
}: AppendNoteModalProps) {
  // State tampilan murni: isi TextInput sebelum di-submit. Diisi ulang setiap kali popup dibuka.
  const [note, setNote] = useState(initialNote);

  useEffect(() => {
    if (visible) setNote(initialNote);
  }, [visible, initialNote]);

  return (
    <ModalCard visible={visible} onRequestClose={onCancel}>
      <Text className="text-[24px] font-extrabold text-foreground">Append Note</Text>
      <TextInput
        value={note}
        onChangeText={setNote}
        editable={!loading}
        placeholder={placeholder}
        placeholderTextColor={AppColors.placeholder}
        multiline
        textAlignVertical="top"
        className="h-[120px] rounded-card bg-surface p-3.5 text-[14px] text-foreground"
      />
      {errorMessage ? <Text className="text-[12px] font-normal text-live">{errorMessage}</Text> : null}
      <View className="flex-row gap-3">
        <AppButton label="Cancel" variant="outline" className="flex-1" onPress={onCancel} />
        <AppButton
          label={loading ? 'Memuat...' : saving ? 'Menyimpan...' : hasNote ? 'Simpan Catatan' : 'Append Note'}
          variant="accent"
          className="flex-1"
          disabled={loading || saving}
          onPress={() => onSubmit(note)}
        />
      </View>
    </ModalCard>
  );
}
