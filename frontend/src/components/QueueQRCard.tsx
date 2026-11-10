/**
 * QueueQRCard: kartu QR join untuk owner, dengan mode statis atau dinamis (token berhitung mundur).
 *
 * Dipakai oleh : app/(owner)/queues/[queueId]/index.tsx
 * Bergantung   : ui/Toast, ui/AppButton, ui/Icon, ui/ModalCard, ui/QrCode, hooks/use-theme-mode, lib/qr-matrix (renderQrPngDataUrl), lib/web-origin (copyToClipboard), types/api (QueueQR)
 * Peran        : owner
 * Catatan      : QR digambar dari `join_url`; token dinamis diganti otomatis setelah kedaluwarsa, dan token statis berlaku sampai kode diganti.
 */
import { useErrorToast, useToast } from '@/components/ui/Toast';
import { useEffect, useState } from 'react';
import { Platform, Pressable, Text, TextInput, View, useWindowDimensions } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { Icon } from '@/components/ui/Icon';
import { ModalCard } from '@/components/ui/ModalCard';
import { QrCode } from '@/components/ui/QrCode';
import { useThemeColor } from '@/hooks/use-theme-mode';
import { renderQrPngDataUrl } from '@/lib/qr-matrix';
import { copyToClipboard } from '@/lib/web-origin';
import type { QueueQR } from '@/types/api';

/** Props kartu QR antrian owner. */
export type QueueQRCardProps = {
  /** Data QR antrian (termasuk `join_url` dan tipe statis/dinamis); null bila belum ada atau belum dimuat. */
  qr: QueueQR | null;
  /** true saat QR sedang dimuat; kartu menampilkan teks "Memuat QR...". Default false. */
  loading?: boolean;
  /** Galat memuat QR; kartu diganti dengan tombol coba lagi bila `onRetry` ada. */
  error?: string;
  /** true saat pergantian mode statis/dinamis berjalan; tombol mode berstatus loading. Default false. */
  toggling?: boolean;
  /** true saat penggantian kode statis berjalan; tombol konfirmasi berstatus loading. Default false. */
  replacing?: boolean;
  /** Detik sisa token dinamis; null bila token statis (atau belum ada token). */
  countdown?: number | null;
  /** Dipanggil dengan tipe tujuan saat tombol MODE STATIS atau MODE DINAMIS ditekan. */
  onToggle?: (type: 'static' | 'dynamic') => void;
  /** Dipanggil setelah pengguna mengonfirmasi ganti kode statis. */
  onReplace?: () => void;
  /** Muat ulang QR dinamis dari server (token dikeluarkan ulang, tanpa konfirmasi). */
  onRefresh?: () => Promise<void>;
  /** Dipanggil saat tombol "Coba Lagi" pada kondisi galat ditekan. Opsional. */
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
 * Kartu QR owner: hanya tombol buka popup dan toggle statis/dinamis. QR (digambar dari `join_url`), hitung mundur token dinamis, salin tautan, unduh PNG, dan
 * penggantian kode statis ada di popup.
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
  onRefresh,
  onRetry,
}: QueueQRCardProps) {
  useErrorToast(error);
  const { width } = useWindowDimensions();
  const [copied, setCopied] = useState(false);
  // Salin otomatis gagal: tautan ditampilkan di kolom yang bisa dipilih agar tetap bisa disalin manual.
  const [copyFailed, setCopyFailed] = useState(false);
  const toast = useToast();
  const foreground = useThemeColor('foreground');
  const [qrOpen, setQrOpen] = useState(false);
  const [confirmReplace, setConfirmReplace] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const joinUrl = qr?.join_url ?? null;
  const isDynamic = qr?.type === 'dynamic';

  // Label "TERSALIN" hanya sebentar; tanpa ini tombol terus mengaku sudah menyalin, juga setelah token dinamis berganti.
  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(t);
  }, [copied]);

  async function handleCopy() {
    if (!joinUrl) return;
    const ok = await copyToClipboard(joinUrl);
    setCopied(ok);
    setCopyFailed(!ok);
    if (!ok) toast.show('Tidak bisa menyalin otomatis. Pilih tautan di bawah lalu salin manual.');
  }

  // Unduh hanya di web: PNG digambar dari tautan lewat canvas. Native bisa memakai share sheet nanti.
  function handleDownload() {
    if (!joinUrl || Platform.OS !== 'web' || typeof document === 'undefined') return;
    const dataUrl = renderQrPngDataUrl(joinUrl);
    if (!dataUrl) return;
    const link = document.createElement('a');
    link.href = dataUrl;
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

  // Galat memuat QR menggantikan isi kartu dengan tombol coba lagi.
  if (error) {
    return (
      <View className="gap-3 rounded-card border border-line p-3.5">
        <Text className="text-[14px] font-bold text-foreground">QR Antrian</Text>
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

  // Permintaan muat ulang tidak dijalankan ganda selama permintaan sebelumnya belum selesai.
  async function handleRefresh() {
    if (!onRefresh || refreshing) return;
    setRefreshing(true);
    try {
      await onRefresh();
    } finally {
      setRefreshing(false);
    }
  }

  function closeQr() {
    setQrOpen(false);
    setConfirmReplace(false);
  }

  // Di popup, tombol di pojok kanan atas memuat ulang QR pada mode dinamis (tanpa konfirmasi) dan meminta konfirmasi ganti kode pada mode statis.
  return (
    <View className="gap-3 rounded-card border border-line p-3.5">
      <View className="flex-row items-center justify-between">
        <Text className="text-[14px] font-bold text-foreground">QR Antrian</Text>
        <Text className="text-[11px] font-bold uppercase tracking-wide text-muted">
          {isDynamic ? 'Dinamis' : 'Statis'}
        </Text>
      </View>

      <View className="flex-row gap-2">
        <AppButton label="LIHAT QR" variant="dark" className="flex-1" onPress={() => setQrOpen(true)} />
        <AppButton
          label={isDynamic ? 'MODE STATIS' : 'MODE DINAMIS'}
          variant="soft"
          className="flex-1"
          loading={toggling}
          disabled={toggling || replacing}
          onPress={() => onToggle?.(isDynamic ? 'static' : 'dynamic')}
        />
      </View>

      <ModalCard visible={qrOpen} onRequestClose={closeQr}>
        {confirmReplace ? (
          <>
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
                setConfirmReplace(false);
              }}
            />
            <AppButton label="Batal" variant="outline" onPress={() => setConfirmReplace(false)} />
          </>
        ) : (
          <>
            <View className="flex-row items-center justify-between">
              <Text className="text-[20px] font-extrabold text-foreground">QR Antrian</Text>
              <View className="flex-row items-center gap-2">
                <Text className="text-[11px] font-bold uppercase tracking-wide text-muted">
                  {isDynamic ? 'Dinamis' : 'Statis'}
                </Text>
                <Pressable
                  onPress={() => (isDynamic ? void handleRefresh() : setConfirmReplace(true))}
                  disabled={replacing || toggling || refreshing}
                  accessibilityRole="button"
                  accessibilityLabel={isDynamic ? 'Muat ulang QR' : 'Ganti kode'}
                  hitSlop={8}
                  className="h-7 w-7 items-center justify-center rounded-full border border-line"
                  style={{ opacity: replacing || toggling || refreshing ? 0.5 : 1 }}
                >
                  <Icon name="refresh" size={16} color={foreground} />
                </Pressable>
                {isDynamic && countdownText ? (
                  <Text className="text-[11px] font-bold text-live" style={{ fontFamily: 'monospace' }}>
                    {expired ? 'kedaluwarsa' : countdownText}
                  </Text>
                ) : null}
              </View>
            </View>

            <View className="items-center">
              {joinUrl ? (
                <QrCode value={joinUrl} size={Math.min(400, width - 80)} />
              ) : (
                <Text className="text-[12px] text-muted">QR tidak tersedia.</Text>
              )}
            </View>

            <Text className="text-[12px] text-muted">
              {isDynamic
                ? 'Token dinamis diganti otomatis setelah kedaluwarsa.'
                : 'Token statis berlaku sampai Anda mengganti kode.'}
            </Text>

            {joinUrl && copyFailed ? (
              <TextInput
                accessibilityLabel="Tautan join"
                value={joinUrl}
                editable={false}
                selectTextOnFocus
                className="rounded-card border border-line px-3 py-2 text-[12px] text-foreground"
                // editable=false tidak bisa difokus di web; userSelect membuat teksnya tetap bisa diblok.
                style={{ userSelect: 'all' } as object}
              />
            ) : null}

            <View className="flex-row gap-2">
              {joinUrl ? (
                <AppButton
                  label={copied ? 'TAUTAN TERSALIN' : 'SALIN TAUTAN'}
                  variant="outline"
                  className="flex-1"
                  onPress={() => void handleCopy()}
                />
              ) : null}
              {joinUrl && Platform.OS === 'web' ? (
                <AppButton label="UNDUH PNG" variant="outline" className="flex-1" onPress={handleDownload} />
              ) : null}
            </View>

            <AppButton label="Tutup" variant="outline" onPress={closeQr} />
          </>
        )}
      </ModalCard>
    </View>
  );
}
