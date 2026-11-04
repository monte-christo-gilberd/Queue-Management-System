import { useEffect, useState } from 'react';
import { Linking, Platform, Pressable, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { ConfirmModal } from '@/components/ui/ConfirmModal';
import { useErrorToast, useToast } from '@/components/ui/Toast';
import { DISPLAY_MAX_COUNT, type DisplayConfig } from '@/lib/display';
import { displayUrl } from '@/lib/display-url';
import { copyToClipboard } from '@/lib/web-origin';

export type DisplaySettingsCardProps = {
  config: DisplayConfig | null;
  loading?: boolean;
  /** Ada aksi yang sedang berjalan: semua kontrol dinonaktifkan. */
  busy?: boolean;
  error?: string;
  onEnable: () => void;
  onChangeCounts: (counts: { upcoming_count?: number; skipped_count?: number }) => void;
  onRotate: () => void;
  onDisable: () => void;
};

function Stepper({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: number;
  disabled: boolean;
  onChange: (value: number) => void;
}) {
  const btn = 'h-9 w-9 items-center justify-center rounded-full border border-line';
  return (
    <View className="flex-row items-center justify-between">
      <Text className="flex-1 text-[13px] font-semibold text-foreground">{label}</Text>
      <View className="flex-row items-center gap-3">
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={`Kurangi ${label}`}
          disabled={disabled || value <= 0}
          onPress={() => onChange(value - 1)}
          className={btn}
          style={{ opacity: disabled || value <= 0 ? 0.4 : 1 }}
        >
          <Text className="text-[18px] font-bold text-foreground">−</Text>
        </Pressable>
        <Text accessibilityLabel={`${label}: ${value}`} className="w-6 text-center text-[16px] font-extrabold text-foreground">
          {value}
        </Text>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={`Tambah ${label}`}
          disabled={disabled || value >= DISPLAY_MAX_COUNT}
          onPress={() => onChange(value + 1)}
          className={btn}
          style={{ opacity: disabled || value >= DISPLAY_MAX_COUNT ? 0.4 : 1 }}
        >
          <Text className="text-[18px] font-bold text-foreground">+</Text>
        </Pressable>
      </View>
    </View>
  );
}

/**
 * Kartu "Layar Publik" di detail antrian (owner): aktifkan, salin atau buka tautan, atur jumlah nomor berikutnya
 * dan dilewati (0 menyembunyikan bagiannya), ganti tautan, dan matikan. Mengganti atau mematikan meminta konfirmasi
 * karena layar yang sedang terpasang langsung berhenti bekerja.
 */
export function DisplaySettingsCard({
  config,
  loading = false,
  busy = false,
  error,
  onEnable,
  onChangeCounts,
  onRotate,
  onDisable,
}: DisplaySettingsCardProps) {
  useErrorToast(error);
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  const [confirm, setConfirm] = useState<'rotate' | 'disable' | null>(null);
  const url = config ? displayUrl(config.token) : null;

  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(t);
  }, [copied]);

  async function copy() {
    if (!url) return;
    const ok = await copyToClipboard(url);
    setCopied(ok);
    if (!ok) toast.show('Tidak bisa menyalin otomatis. Pilih tautan lalu salin manual.');
  }

  function openScreen() {
    if (!url) return;
    if (Platform.OS === 'web' && typeof window !== 'undefined') window.open(url, '_blank', 'noopener');
    else void Linking.openURL(url);
  }

  if (loading) {
    return (
      <View className="gap-3 rounded-card border border-line p-3.5">
        <Text className="text-[14px] font-bold text-foreground">Layar Publik</Text>
        <Text className="text-[12px] text-muted">Memuat...</Text>
      </View>
    );
  }

  return (
    <View className="gap-3 rounded-card border border-line p-3.5" testID="display-settings">
      <Text className="text-[14px] font-bold text-foreground">Layar Publik</Text>

      {!config ? (
        <>
          <Text className="text-[12px] text-muted">
            Tampilkan nomor yang sedang dilayani di monitor ruang tunggu, tanpa perlu login.
          </Text>
          <AppButton label="AKTIFKAN LAYAR" variant="dark" loading={busy} disabled={busy} onPress={onEnable} />
        </>
      ) : (
        <>
          <Text className="text-[12px] text-muted">Buka tautan ini di layar. Siapa pun yang memegangnya bisa melihat nomor antrian.</Text>
          {url ? (
            <Text selectable accessibilityLabel="Tautan layar" className="rounded-card border border-line px-3 py-2 text-[12px] text-foreground">
              {url}
            </Text>
          ) : null}
          <View className="flex-row gap-2">
            {url ? (
              <AppButton
                label={copied ? 'TAUTAN TERSALIN' : 'SALIN TAUTAN LAYAR'}
                variant="outline"
                className="flex-1"
                onPress={() => void copy()}
              />
            ) : null}
            {url ? <AppButton label="BUKA LAYAR" variant="dark" className="flex-1" onPress={openScreen} /> : null}
          </View>

          <Stepper
            label="nomor berikutnya"
            value={config.upcoming_count}
            disabled={busy}
            onChange={(v) => onChangeCounts({ upcoming_count: v })}
          />
          <Stepper
            label="nomor dilewati"
            value={config.skipped_count}
            disabled={busy}
            onChange={(v) => onChangeCounts({ skipped_count: v })}
          />

          <View className="flex-row gap-2">
            <AppButton label="GANTI TAUTAN" variant="outline" className="flex-1" disabled={busy} onPress={() => setConfirm('rotate')} />
            <AppButton label="MATIKAN LAYAR" variant="outline" className="flex-1" disabled={busy} onPress={() => setConfirm('disable')} />
          </View>
        </>
      )}

      <ConfirmModal
        visible={confirm === 'rotate'}
        title="Ganti tautan layar?"
        message="Layar yang memakai tautan lama langsung berhenti bekerja. Buka tautan yang baru di layar itu."
        confirmLabel="Ya, Ganti Tautan"
        onCancel={() => setConfirm(null)}
        onConfirm={() => {
          setConfirm(null);
          onRotate();
        }}
      />
      <ConfirmModal
        visible={confirm === 'disable'}
        title="Matikan layar publik?"
        message="Layar yang sedang terpasang berhenti menampilkan antrian. Anda bisa mengaktifkannya lagi nanti, dengan tautan baru."
        confirmLabel="Ya, Matikan"
        onCancel={() => setConfirm(null)}
        onConfirm={() => {
          setConfirm(null);
          onDisable();
        }}
      />
    </View>
  );
}
