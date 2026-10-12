import { useState } from 'react';
import { Text, TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';

type AppendNoteModalProps = {
  visible: boolean;
  onCancel: () => void;
  onSubmit: (note: string) => void;
  placeholder?: string;
};

export function AppendNoteModal({
  visible,
  onCancel,
  onSubmit,
  placeholder = 'Tulis catatan antrian di sini...',
}: AppendNoteModalProps) {
  // State tampilan murni: isi TextInput sebelum di-submit.
  const [note, setNote] = useState('');

  return (
    <ModalCard visible={visible} onRequestClose={onCancel}>
      <Text className="text-[24px] font-extrabold text-foreground">Append Note</Text>
      <TextInput
        value={note}
        onChangeText={setNote}
        placeholder={placeholder}
        placeholderTextColor={AppColors.placeholder}
        multiline
        textAlignVertical="top"
        className="h-[120px] rounded-card bg-surface p-3.5 text-[14px] text-foreground"
      />
      <View className="flex-row gap-3">
        <AppButton
          label="Cancel"
          variant="outline"
          className="flex-1"
          onPress={() => {
            setNote('');
            onCancel();
          }}
        />
        <AppButton
          label="Append Note"
          variant="accent"
          className="flex-1"
          onPress={() => {
            onSubmit(note);
            setNote('');
          }}
        />
      </View>
    </ModalCard>
  );
}
