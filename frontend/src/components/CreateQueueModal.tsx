import { useState } from 'react';
import { Text } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { ModalCard } from '@/components/ui/ModalCard';

export type CreateQueueModalProps = {
  visible: boolean;
  onClose: () => void;
  /** Dipanggil dengan isian form. Validasi dan pemanggilan API ada di route. */
  onSubmit: (name: string, description: string) => void;
  loading?: boolean;
  errorMessage?: string;
};

/**
 * Popup "Buat Antrian Baru". Form di-mount hanya saat popup terbuka, jadi isiannya
 * otomatis kosong setiap kali dibuka tanpa perlu reset manual.
 */
export default function CreateQueueModal({
  visible,
  onClose,
  onSubmit,
  loading = false,
  errorMessage,
}: CreateQueueModalProps) {
  return (
    <ModalCard visible={visible} onRequestClose={onClose}>
      {visible ? (
        <CreateQueueForm onClose={onClose} onSubmit={onSubmit} loading={loading} errorMessage={errorMessage} />
      ) : null}
    </ModalCard>
  );
}

function CreateQueueForm({
  onClose,
  onSubmit,
  loading,
  errorMessage,
}: Omit<CreateQueueModalProps, 'visible'>) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  return (
    <>
      <Text className="text-[24px] font-extrabold text-foreground">Buat Antrian Baru</Text>
      <FormField
        label="Nama Antrian"
        placeholder="mis. Loket Pelayanan 1"
        value={name}
        onChangeText={setName}
      />
      <FormField
        label="Deskripsi"
        placeholder="Opsional"
        value={description}
        onChangeText={setDescription}
      />

      {errorMessage ? <Text className="text-[13px] text-live">{errorMessage}</Text> : null}

      <AppButton
        label="Buat Antrian"
        variant="dark"
        loading={loading}
        onPress={() => onSubmit(name, description)}
      />
      <AppButton label="Batal" variant="outline" onPress={onClose} />
    </>
  );
}
