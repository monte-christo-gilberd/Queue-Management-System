import { Image } from 'expo-image';
import { useState } from 'react';
import { Platform, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import { copyToClipboard } from '@/lib/web-origin';
import type { QueueQR } from '@/types/api';

export type QueueQRCardProps = {
  qr: QueueQR | null;
  loading?: boolean;
  error?: string;
  toggling?: boolean;
  replacing?: boolean;
  /** Detik sisa token dinamis; null bila token statis (atau belum ada token). */
  countdown?: number | null;
  onToggle?: (type: 'static' | 'dynamic') => void;
  onReplace?: () => void;
  onRetry?: () => void;
};

function formatCountdown(seconds: number | null): string {
  if (seconds == null) return '';
  const safe = Math.max(0, seconds);
  const mm = Math.floor(safe / 60);
  const ss = safe % 60;
  return `${String(mm).padStart(2, '0')}:${String(ss).padStart(2, '0')}`;
}

/**
 * Kartu QR owner: menampilkan QR dari `image_url` (dirender qr-service), toggle
 * statis/dinamis, hitung mundur token dinamis, salin tautan, unduh PNG, dan
 * penggantian kode statis.
 */
export function QueueQRCard({
  qr,
  loading = false,
  error,
  toggling = false,
  replacing = false,
  countdown = null,
  onToggle,
  onReplace,
  onRetry,
}: QueueQRCardProps) {
  const [copied, setCopied] = useState(false);
  const [replaceOpen, setReplaceOpen] = useState(false);

  const imageUrl = qr?.image_url ?? null;
  const joinUrl = qr?.join_url ?? null;
  const isDynamic = qr?.type === 'dynamic';

  async function handleCopy() {
    if (!joinUrl) return;
    setCopied(await copyToClipboard(joinUrl));
  }

  // Unduh hanya di web (anchor download). Native bisa memakai share sheet nanti.
  function handleDownload() {
    if (!imageUrl || Platform.OS !== 'web' || typeof document === 'undefined') return;
    const link = document.createElement('a');
    link.href = imageUrl;
    link.download = `qr-antrian-${qr?.queue_id ?? 'qms'}.png`;
    link.rel = 'noopener';
    document.body.appendChild(link);
    link.click();
    link.remove();
  }

  if (loading) {
    return (
      <View className="gap-3 rounded-card border border-line p-3.5">
        <Text className="text-[14px] font-bold text-foreground">QR Antrian</Text>
        <Text className="text-[12px] text-muted">Memuat QR...</Text>
      </View>
    );
  }

  if (error) {
    return (
      <View className="gap-3 rounded-card border border-line p-3.5">
        <Text className="text-[14px] font-bold text-foreground">QR Antrian</Text>
        <Text className="text-[12px] text-live">
          Layanan QR sedang tidak tersedia, coba lagi
        </Text>
        {onRetry ? <AppButton label="COBA LAGI" variant="dark" onPress={onRetry} /> : null}
      </View>
    );
  }

  if (!qr) {
    return (
      <View className="gap-3 rounded-card border border-line p-3.5">
        <Text className="text-[14px] font-bold text-foreground">QR Antrian</Text>
        <Text className="text-[12px] text-muted">QR belum tersedia untuk antrian ini.</Text>
      </View>
    );
  }

  const countdownText = formatCountdown(countdown);
  const expired = countdown === 0;

  return (
    <View className="gap-3 rounded-card border border-line p-3.5">
      <View className="flex-row items-center justify-between">
        <Text className="text-[14px] font-bold text-foreground">QR Antrian</Text>
        <View className="flex-row items-center gap-2">
          <Text className="text-[11px] font-bold uppercase tracking-wide text-muted">
            {isDynamic ? 'Dinamis' : 'Statis'}
          </Text>
          {isDynamic && countdownText ? (
            <Text className="text-[11px] font-bold text-live" style={{ fontFamily: 'monospace' }}>
              {expired ? 'kedaluwarsa' : countdownText}
            </Text>
          ) : null}
        </View>
      </View>

      <View className="items-center">
        {imageUrl ? (
          <Image
            source={{ uri: imageUrl }}
            style={{ width: 220, height: 220 }}
            contentFit="contain"
            accessibilityLabel={`QR join antrian ${qr.queue_id}`}
          />
        ) : (
          <Text className="text-[12px] text-muted">Gambar QR tidak tersedia.</Text>
        )}
      </View>

      <Text className="text-[12px] text-muted">
        {isDynamic
          ? 'Token dinamis diperbarui otomatis saat sisa waktu tinggal 20 detik.'
          : 'Token statis berlaku sampai Anda mengganti kode.'}
      </Text>

      <View className="flex-row gap-2">
        {joinUrl ? (
          <AppButton
            label={copied ? 'TAUTAN TERSALIN' : 'SALIN TAUTAN'}
            variant="outline"
            className="flex-1"
            onPress={() => void handleCopy()}
          />
        ) : null}
        {imageUrl && Platform.OS === 'web' ? (
          <AppButton label="UNDUH PNG" variant="outline" className="flex-1" onPress={handleDownload} />
        ) : null}
      </View>

      <AppButton
        label={isDynamic ? 'AKTIFKAN MODE STATIS' : 'AKTIFKAN MODE DINAMIS'}
        variant="soft"
        loading={toggling}
        disabled={toggling || replacing}
        onPress={() => onToggle?.(isDynamic ? 'static' : 'dynamic')}
      />

      {qr.type === 'static' ? (
        <AppButton
          label="GANTI KODE"
          variant="dark"
          disabled={replacing || toggling}
          onPress={() => setReplaceOpen(true)}
        />
      ) : null}

      <ModalCard visible={replaceOpen} onRequestClose={() => setReplaceOpen(false)}>
        <Text className="text-[20px] font-extrabold text-foreground">Ganti kode QR?</Text>
        <Text className="text-[13px] text-muted">
          Kode lama langsung tidak berlaku untuk bergabung. Cetak atau bagikan QR yang baru.
        </Text>
        <AppButton
          label="Ya, Ganti Kode"
          variant="dark"
          loading={replacing}
          onPress={() => {
            onReplace?.();
            setReplaceOpen(false);
          }}
        />
        <AppButton label="Batal" variant="outline" onPress={() => setReplaceOpen(false)} />
      </ModalCard>
    </View>
  );
}
