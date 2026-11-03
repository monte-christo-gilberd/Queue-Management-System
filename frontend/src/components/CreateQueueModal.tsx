import { useErrorToast } from '@/components/ui/Toast';
import { useState } from 'react';
import { Text } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { ModalCard } from '@/components/ui/ModalCard';

export type CreateQueueModalProps = {
  visible: boolean;
  onClose: () => void;
  /** Dipanggil dengan isian form. Validasi dan pemanggilan API ada di route. */
  onSubmit: (name: string, description: string, ticketPrefix: string) => void;
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
  useErrorToast(errorMessage);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [prefix, setPrefix] = useState('');

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
      <FormField
        label="Awalan Nomor"
        placeholder="Opsional, 1-3 huruf/angka (default A)"
        value={prefix}
        onChangeText={(v) => setPrefix(v.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 3))}
        autoCapitalize="characters"
      />

      <AppButton
        label="Buat Antrian"
        variant="dark"
        loading={loading}
        onPress={() => onSubmit(name, description, prefix)}
      />
      <AppButton label="Batal" variant="outline" onPress={onClose} />
    </>
  );
}
