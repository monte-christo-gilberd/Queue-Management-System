/**
 * CreateQueueModal: popup form untuk membuat antrian baru (nama, deskripsi, awalan nomor).
 *
 * Dipakai oleh : app/(owner)/dashboard.tsx
 * Bergantung   : ui/Toast (useErrorToast), ui/AppButton, ui/FormField, ui/ModalCard
 * Peran        : owner
 * Catatan      : awalan nomor dinormalisasi saat diketik; validasi nama dan pembuatan antrian dilakukan di route.
 */
import { useErrorToast } from '@/components/ui/Toast';
import { useState } from 'react';
import { Text } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { ModalCard } from '@/components/ui/ModalCard';

/** Props untuk popup Buat Antrian Baru. */
export type CreateQueueModalProps = {
  /** true menampilkan popup beserta form; false menyembunyikannya dan form dibongkar. */
  visible: boolean;
  /** Dipanggil saat popup diminta tutup, lewat tombol Batal atau tombol dismiss modal. */
  onClose: () => void;
  /** Dipanggil dengan isian form. Validasi dan pemanggilan API ada di route. */
  onSubmit: (name: string, description: string, ticketPrefix: string) => void;
  /** true saat pembuatan antrian berjalan; tombol Buat Antrian menampilkan status loading. Default false. */
  loading?: boolean;
  /** Galat dari route, ditampilkan sebagai toast. Opsional; kosong berarti tidak ada toast. */
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

  // Awalan dipaksa huruf besar, hanya huruf dan angka A-Z/0-9, dan dibatasi 3 karakter saat diketik.
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
