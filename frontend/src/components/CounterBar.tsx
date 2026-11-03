import { useState } from 'react';
import { Pressable, ScrollView, Text, TextInput, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { AppButton } from '@/components/ui/AppButton';
import { ModalCard } from '@/components/ui/ModalCard';
import type { useCounterSession } from '@/hooks/use-counter-session';
import { MAX_COUNTERS, MAX_COUNTER_NAME_LENGTH, validateCounterName, type Counter } from '@/lib/counters';
import type { Ticket } from '@/types/api';
import { formatTicketNumber } from '@/utils/format';

type CounterSession = ReturnType<typeof useCounterSession>;

type CounterBarProps = {
  session: CounterSession;
  /** Papan antrian: nomor yang dilayani diambil dari sini supaya selalu terbaru. */
  tickets: Ticket[];
  prefix?: string | null;
};

/**
 * Tab counter untuk satu antrian: pindah counter, buat counter (tab baru), ganti nama (tekan lama),
 * dan tutup counter non-default (tombol ×). Dengan satu counter saja, hanya tombol "+" yang tampil.
 */
export function CounterBar({ session, tickets, prefix }: CounterBarProps) {
  const [renaming, setRenaming] = useState<Counter | null>(null);
  const [removing, setRemoving] = useState<Counter | null>(null);
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  // Nomor yang sedang dilayani tiap counter, dari papan.
  const servingNumber = new Map<number, number>();
  for (const t of tickets) {
    if (t.status === 'currently_served' && t.called_counter_id) servingNumber.set(t.called_counter_id, t.ticket_number);
  }
  const atLimit = session.counters.length >= MAX_COUNTERS;
  const multi = session.counters.length > 1;

  function openRename(counter: Counter) {
    setError(undefined);
    setName(counter.name);
    setRenaming(counter);
  }

  function openRemove(counter: Counter) {
    setError(undefined);
    setRemoving(counter);
  }

  async function submitRename() {
    if (!renaming) return;
    const problem = validateCounterName(name, false);
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    const message = await session.rename(renaming.id, name.trim());
    setBusy(false);
    if (message) {
      setError(message);
    } else {
      setRenaming(null);
    }
  }

  async function confirmRemove() {
    if (!removing) return;
    setBusy(true);
    const message = await session.remove(removing.id);
    setBusy(false);
    if (message) {
      setError(message);
    } else {
      setRemoving(null);
    }
  }

  const servingOf = (counter: Counter) => {
    const number = servingNumber.get(counter.id);
    return number === undefined ? null : formatTicketNumber(number, prefix);
  };

  return (
    <View className="gap-2">
      <View className="flex-row items-center gap-2">
        {multi ? (
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerClassName="gap-2 pr-2">
            {session.counters.map((counter) => {
              const active = counter.id === session.counterId;
              const number = servingOf(counter);
              return (
                <View
                  key={counter.id}
                  className={`flex-row items-center gap-1 rounded-pill border px-3 py-1.5 ${
                    active ? 'border-primary bg-primarySoft' : 'border-line bg-surface'
                  }`}>
                  <Pressable
                    accessibilityRole="tab"
                    accessibilityState={{ selected: active }}
                    onPress={() => session.select(counter.id)}
                    onLongPress={() => openRename(counter)}
                    delayLongPress={400}>
                    <Text className={`text-[13px] font-bold ${active ? 'text-white' : 'text-foreground'}`}>
                      {counter.name} · {number ?? 'kosong'}
                    </Text>
                  </Pressable>
                  {counter.is_default ? null : (
                    <Pressable
                      accessibilityRole="button"
                      accessibilityLabel={`Tutup ${counter.name}`}
                      onPress={() => openRemove(counter)}
                      hitSlop={8}>
                      <Text className="px-1 text-[14px] font-bold text-foreground">×</Text>
                    </Pressable>
                  )}
                </View>
              );
            })}
          </ScrollView>
        ) : null}
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Tambah counter"
          accessibilityState={{ disabled: atLimit }}
          disabled={atLimit || busy}
          onPress={() => void session.add()}
          className={`h-9 items-center justify-center rounded-pill px-3.5 ${atLimit ? 'bg-line' : 'bg-dark'}`}>
          <Text className="text-[13px] font-bold text-white">
            {atLimit ? `Maks ${MAX_COUNTERS} counter` : '+ Tambah counter'}
          </Text>
        </Pressable>
      </View>

      {session.notice ? <Text className="text-[12px] font-normal text-foreground">{session.notice}</Text> : null}
      {session.fellBack && session.current ? (
        <Text className="text-[12px] font-normal text-foreground">
          Counter yang dipilih sudah dihapus; perangkat ini kembali ke {session.current.name}.
        </Text>
      ) : null}
      {session.error ? <Text className="text-[12px] font-normal text-live">{session.error}</Text> : null}

      <ModalCard visible={renaming !== null} onRequestClose={() => setRenaming(null)}>
        <Text className="text-[20px] font-extrabold text-foreground">Ganti nama counter</Text>
        <TextInput
          value={name}
          onChangeText={setName}
          editable={!busy}
          maxLength={MAX_COUNTER_NAME_LENGTH}
          placeholder="Nama counter"
          placeholderTextColor={AppColors.placeholder}
          className="h-[48px] rounded-card bg-surface px-3.5 text-[14px] text-foreground"
        />
        {error ? <Text className="text-[12px] font-normal text-live">{error}</Text> : null}
        <View className="flex-row gap-3">
          <AppButton label="Batal" variant="outline" className="flex-1" onPress={() => setRenaming(null)} />
          <AppButton
            label={busy ? 'Menyimpan...' : 'Simpan'}
            variant="accent"
            className="flex-1"
            disabled={busy}
            onPress={() => void submitRename()}
          />
        </View>
      </ModalCard>

      <ModalCard visible={removing !== null} onRequestClose={() => setRemoving(null)}>
        <Text className="text-[20px] font-extrabold text-foreground">Tutup {removing?.name}?</Text>
        <Text className="text-[14px] text-foreground">
          {removing && servingOf(removing)
            ? `${servingOf(removing)} sedang dilayani counter ini. Nomor itu akan kembali ke urutan terdepan.`
            : 'Counter ini kosong dan akan dihapus.'}
        </Text>
        {error ? <Text className="text-[12px] font-normal text-live">{error}</Text> : null}
        <View className="flex-row gap-3">
          <AppButton label="Batal" variant="outline" className="flex-1" onPress={() => setRemoving(null)} />
          <AppButton
            label={busy ? 'Menutup...' : 'Tutup counter'}
            variant="accent"
            className="flex-1"
            disabled={busy}
            onPress={() => void confirmRemove()}
          />
        </View>
      </ModalCard>
    </View>
  );
}
