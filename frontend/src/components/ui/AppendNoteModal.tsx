/**
 * AppendNoteModal: popup untuk menulis atau mengubah catatan pada satu nomor antrian.
 *
 * Dipakai oleh : app/(user)/dashboard.tsx, app/(user)/queues/[queueId]/index.tsx
 * Bergantung   : components/ui/ModalCard, components/ui/AppButton, components/ui/Toast (useErrorToast)
 * Alur         : isian -> onSubmit(note) -> induk menyimpan lalu menutup popup bila berhasil
 */
import { useErrorToast } from '@/components/ui/Toast';
import { useState } from 'react';
import { Text, TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';

type AppendNoteModalProps = {
  /** Popup tampil bila true. Isian diisi ulang setiap kali berubah menjadi true. */
  visible: boolean;
  /** Dipanggil saat Cancel atau latar ditekan; penutupan diatur induk. */
  onCancel: () => void;
  /** Dipanggil dengan isi catatan; penutupan popup diatur pemanggil setelah simpan berhasil. */
  onSubmit: (note: string) => void;
  /** Catatan yang sudah ada untuk nomor ini; mengisi TextInput setiap popup dibuka. Default kosong. */
  initialNote?: string;
  /** True bila nomor ini sudah punya catatan: tombol berbunyi "Simpan Catatan". Default false. */
  hasNote?: boolean;
  /** Bila true, isian dikunci dan tombol berbunyi "Memuat...". Default false. */
  loading?: boolean;
  /** Bila true, isian dan tombol dikunci dan tombol berbunyi "Menyimpan...". Default false. */
  saving?: boolean;
  /** Galat dari induk; tampil sebagai toast, bukan teks merah di dalam popup. */
  errorMessage?: string;
  /** Placeholder TextInput. Default "Tulis catatan untuk nomor ini...". */
  placeholder?: string;
};

/** Popup catatan nomor: TextInput multiline dengan tombol batal dan simpan. */
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
  useErrorToast(errorMessage);
  // State tampilan murni: isi TextInput sebelum di-submit. Diisi ulang setiap kali popup dibuka.
  const [note, setNote] = useState(initialNote);

  // Reset saat popup dibuka atau catatan awal berubah, lewat pola render-phase (bukan effect) agar tidak
  // memicu render berantai.
  const [prevVisible, setPrevVisible] = useState(visible);
  const [prevInitialNote, setPrevInitialNote] = useState(initialNote);
  if (visible !== prevVisible || initialNote !== prevInitialNote) {
    setPrevVisible(visible);
    setPrevInitialNote(initialNote);
    if (visible) setNote(initialNote);
  }

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
