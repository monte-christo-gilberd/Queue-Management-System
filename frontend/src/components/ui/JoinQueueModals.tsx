import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { Pressable, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { ModalCard } from '@/components/ui/ModalCard';
import { parseQueueIdFromLink } from '@/lib/join-link';

type JoinQueueModalsProps = {
  visible: boolean;
  onClose: () => void;
  /** Dipanggil saat link dikirim lewat popup input manual. Sementara belum dipakai: submit hanya menutup popup. */
  onSubmitLink?: (link: string) => void;
};

// Pola 13x13 untuk placeholder QR: 3 penanda sudut (5x5) + data semu.
const GRID = 13;
const FINDER = 5;

function isFilled(r: number, c: number): boolean {
  const finders: [number, number][] = [
    [0, 0],
    [0, GRID - FINDER],
    [GRID - FINDER, 0],
  ];
  for (const [fr, fc] of finders) {
    if (r >= fr - 1 && r <= fr + FINDER && c >= fc - 1 && c <= fc + FINDER) {
      const rr = r - fr;
      const cc = c - fc;
      if (rr < 0 || cc < 0 || rr >= FINDER || cc >= FINDER) return false; // jarak kosong di sekeliling
      const ring = rr === 0 || cc === 0 || rr === FINDER - 1 || cc === FINDER - 1;
      const center = rr === 2 && cc === 2;
      return ring || center;
    }
  }
  return (r * 7 + c * 3 + r * c) % 3 === 0;
}

/** Placeholder QR code (bukan QR sungguhan) — hanya untuk menampilkan perkiraan tampilan kamera. */
function QrPlaceholder() {
  return (
    <View className="rounded-lg bg-white p-3">
      {Array.from({ length: GRID }, (_, r) => (
        <View key={r} className="flex-row">
          {Array.from({ length: GRID }, (_, c) => (
            <View key={c} className={isFilled(r, c) ? 'h-2.5 w-2.5 bg-dark' : 'h-2.5 w-2.5 bg-white'} />
          ))}
        </View>
      ))}
    </View>
  );
}

/**
 * Popup "Masuk ke Antrian": langkah 1 = tampilan kamera (placeholder QR), langkah 2 = input link manual.
 * Pindah ke langkah 2 menghapus tampilan kamera dari pohon komponen (kamera mati) dan menampilkan input link.
 */
export function JoinQueueModals({ visible, onClose, onSubmitLink }: JoinQueueModalsProps) {
  // State tampilan murni: langkah popup + isi input link sebelum di-submit.
  const [step, setStep] = useState<'scan' | 'link'>('scan');
  const [link, setLink] = useState('');
  const [linkError, setLinkError] = useState<string>();

  // Setiap popup dibuka ulang, mulai dari tampilan kamera dengan input kosong.
  useEffect(() => {
    if (visible) {
      setStep('scan');
      setLink('');
      setLinkError(undefined);
    }
  }, [visible]);

  return (
    <ModalCard visible={visible} onRequestClose={onClose}>
      {step === 'scan' ? (
        <>
          <Text className="text-[24px] font-extrabold text-foreground">Scan QR Antrian</Text>

          {/* TODO: implementasi kamera (mis. expo-camera) menggantikan placeholder ini; aktif hanya di langkah "scan". */}
          <View className="h-64 w-full items-center justify-center overflow-hidden rounded-card bg-dark">
            <QrPlaceholder />
            {/* Bingkai pemindai: 4 sudut. */}
            <View className="absolute left-5 top-5 h-7 w-7 rounded-tl-lg border-l-[3px] border-t-[3px] border-white" />
            <View className="absolute right-5 top-5 h-7 w-7 rounded-tr-lg border-r-[3px] border-t-[3px] border-white" />
            <View className="absolute bottom-5 left-5 h-7 w-7 rounded-bl-lg border-b-[3px] border-l-[3px] border-white" />
            <View className="absolute bottom-5 right-5 h-7 w-7 rounded-br-lg border-b-[3px] border-r-[3px] border-white" />
          </View>
          <Text className="text-center text-[12px] text-muted">
            Arahkan kamera ke QR code antrian
          </Text>

          <AppButton label="Masukkan Link Manual" variant="dark" onPress={() => setStep('link')} />
          <Pressable onPress={onClose} className="items-center py-1" accessibilityRole="button">
            <Text className="text-[13px] font-bold" style={{ color: AppColors.textMuted }}>
              Tutup
            </Text>
          </Pressable>
        </>
      ) : (
        <>
          <Text className="text-[24px] font-extrabold text-foreground">Masukkan Link</Text>
          <FormField
            label="Link Antrian"
            placeholder="https://..."
            value={link}
            onChangeText={(value) => {
              setLink(value);
              setLinkError(undefined);
            }}
            autoCapitalize="none"
            keyboardType="url"
          />
          {linkError ? <Text className="text-[13px] text-live">{linkError}</Text> : null}
          <AppButton
            label="Gabung Antrian"
            variant="dark"
            onPress={() => {
              const queueId = parseQueueIdFromLink(link);
              if (queueId === null) {
                setLinkError('Tautan tidak valid. Tempel tautan antrian yang Anda terima.');
                return;
              }
              onSubmitLink?.(link);
              onClose();
              router.push(`/join/${queueId}` as '/');
            }}
          />
          <AppButton label="Batal" variant="outline" onPress={onClose} />
        </>
      )}
    </ModalCard>
  );
}
