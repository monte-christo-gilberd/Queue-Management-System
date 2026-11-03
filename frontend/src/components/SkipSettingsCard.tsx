import { useErrorToast } from '@/components/ui/Toast';
import { useState } from 'react';
import { Pressable, Text, View } from 'react-native';

import { AppButton } from '@/components/ui/AppButton';
import { FormField } from '@/components/ui/FormField';
import { validateSkipSettings } from '@/lib/skip-settings';
import type { SkipFallback, SkipSettings } from '@/types/api';
import { cn } from '@/utils/cn';

const OPTIONS: { value: SkipFallback; label: string; hint: string }[] = [
  { value: 'remove', label: 'Dihapus', hint: 'Nomor langsung ditandai dilewati' },
  { value: 'back', label: 'Mundur ke paling belakang', hint: 'Nomor kembali menunggu di urutan terakhir' },
  { value: 'after_n', label: 'Mundur N nomor', hint: 'Nomor kembali menunggu setelah N nomor di depannya' },
];

type Props = {
  settings: SkipSettings | null;
  disabled?: boolean;
  saving?: boolean;
  saved?: boolean;
  error?: string;
  onSave: (value: SkipSettings) => void;
};

export function SkipSettingsCard({ settings, disabled = false, saving = false, saved = false, error, onSave }: Props) {
  const [fallback, setFallback] = useState<SkipFallback>(settings?.fallback ?? 'remove');
  const [n, setN] = useState(settings?.n ? String(settings.n) : '1');
  const [max, setMax] = useState(settings ? String(settings.max_returns) : '1');
  const [localError, setLocalError] = useState<string>();
  useErrorToast(localError ?? error);
  const [synced, setSynced] = useState(settings !== null);

  // Isi form sekali saat pengaturan tersimpan selesai dimuat.
  if (settings && !synced) {
    setSynced(true);
    setFallback(settings.fallback);
    setN(settings.n ? String(settings.n) : '1');
    setMax(String(settings.max_returns));
  }

  function submit() {
    const result = validateSkipSettings({ fallback, n, maxReturns: max });
    if (!result.ok) {
      setLocalError(result.error);
      return;
    }
    setLocalError(undefined);
    onSave(result.value);
  }

  return (
    <View className="gap-3 rounded-card border border-line p-3.5" testID="skip-settings">
      <Text className="text-[12px] font-semibold text-muted">Saat nomor dilewati</Text>
      {OPTIONS.map((o) => (
        <Pressable
          key={o.value}
          accessibilityRole="radio"
          accessibilityState={{ checked: fallback === o.value, disabled }}
          disabled={disabled}
          onPress={() => setFallback(o.value)}
          className="flex-row items-center gap-3"
        >
          <View
            className={cn(
              'h-5 w-5 items-center justify-center rounded-full border-2',
              fallback === o.value ? 'border-foreground' : 'border-line',
            )}
          >
            {fallback === o.value ? <View className="h-2.5 w-2.5 rounded-full bg-foreground" /> : null}
          </View>
          <View className="flex-1">
            <Text className="text-[14px] font-bold text-foreground">{o.label}</Text>
            <Text className="text-[12px] text-muted">{o.hint}</Text>
          </View>
        </Pressable>
      ))}

      {fallback === 'after_n' ? (
        <FormField label="Mundur berapa nomor (N)" value={n} onChangeText={setN} keyboardType="number-pad" editable={!disabled} />
      ) : null}
      {fallback !== 'remove' ? (
        <FormField
          label="Batas dikembalikan per nomor"
          value={max}
          onChangeText={setMax}
          keyboardType="number-pad"
          editable={!disabled}
        />
      ) : null}

      {saved && !localError && !error ? <Text className="text-[12px] text-muted">Pengaturan tersimpan</Text> : null}
      <AppButton label="SIMPAN PENGATURAN" variant="dark" loading={saving} onPress={submit} disabled={disabled} />
    </View>
  );
}
