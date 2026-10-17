import { router } from 'expo-router';
import { useEffect, useRef } from 'react';
import { Animated, Easing, Platform, StyleSheet, Text, View } from 'react-native';

import { AppColors } from '@/constants/app-theme';
import { Fonts } from '@/constants/theme';
import { cn } from '@/utils/cn';
import { useIsWide } from '@/hooks/use-is-wide';
import ResponsiveScreenLayout, {
  type OwnerMenuItem,
} from '@/components/layout/ResponsiveScreenLayout';
import { AppButton } from '@/components/ui/AppButton';
import { Icon } from '@/components/ui/Icon';

/** Tiket versi tampilan: nomor sudah diformat + nama customer. */
/** Nama customer belum tersedia dari backend; tanpa nama hanya nomor yang ditampilkan. */
type LiveTicket = { number: string; name?: string };

export type QueueLiveScreenProps = {
  queueName?: string;
  counterName?: string;
  previousTicket?: LiveTicket | null;
  currentTicket?: LiveTicket | null;
  nextTicket?: LiveTicket | null;
  onSkip?: () => void;
  onNext?: () => void;
  /** Navigasi menu profil untuk ResponsiveScreenLayout (diteruskan dari route). */
  onMenuItemPress?: (item: OwnerMenuItem) => void;
};

// Pengecualian yang disengaja: papan display gelap + titik hijau status "live" (token board*, liveGreen).

type LayoutProps = {
  counterName: string;
  previousTicket: LiveTicket | null;
  currentTicket: LiveTicket | null;
  nextTicket: LiveTicket | null;
  onSkip: () => void;
  onNext: () => void;
};

export default function QueueLiveScreen({
  queueName = 'Antrian Loket 1',
  counterName = 'Counter 2',
  previousTicket = { number: 'A-041', name: 'Budi Santoso' },
  currentTicket = { number: 'A-042', name: 'Siti Rahma' },
  nextTicket = { number: 'A-043', name: 'Andi Wijaya' },
  onSkip = () => {},
  onNext = () => {},
  onMenuItemPress,
}: QueueLiveScreenProps) {
  const isWide = useIsWide();
  const shared: LayoutProps = {
    counterName,
    previousTicket,
    currentTicket,
    nextTicket,
    onSkip,
    onNext,
  };

  return (
    <ResponsiveScreenLayout
      title={queueName}
      onMenuItemPress={onMenuItemPress}
      activeItem="queues"
      onBack={() => router.back()}>
      {isWide ? <DesktopAirportBoardLayout {...shared} /> : <MobileTimelineLayout {...shared} />}
    </ResponsiveScreenLayout>
  );
}

/* ---------------------------- HP: timeline / stepper ---------------------------- */

function MobileTimelineLayout({
  counterName,
  previousTicket,
  currentTicket,
  nextTicket,
  onSkip,
  onNext,
}: LayoutProps) {
  return (
    <View className="gap-3 rounded-card border border-line p-4">
      <View className="flex-row items-center justify-between">
        <Text className="text-[15px] font-extrabold text-foreground">Antrean hari ini</Text>
        {/* TODO: backend belum mendukung ini (counter/loket) */}
        <View className="rounded-pill bg-surface px-3.5 py-1.5">
          <Text className="text-[12px] font-extrabold text-primary">{counterName}</Text>
        </View>
      </View>

      <View>
        {/* Sebelumnya: titik pudar + centang */}
        <TimelineRow
          first
          dot={
            <View className="h-4 w-4 items-center justify-center rounded-full bg-disabled">
              <Icon name="check" size={9} color={AppColors.white} />
            </View>
          }
          ticket={previousTicket}
          status="selesai"
          muted
        />
        {/* Sekarang: titik besar menyala dengan ring/glow */}
        <TimelineRow
          dot={
            <View className="h-7 w-7 items-center justify-center rounded-full bg-primary">
              <View className="h-6 w-6 items-center justify-center rounded-full bg-background">
                <View className="h-3.5 w-3.5 rounded-full bg-primary" />
              </View>
            </View>
          }
          ticket={currentTicket}
          status="sedang dilayani"
          current
        />
        {/* Berikutnya: hanya outline */}
        <TimelineRow last dot={<View className="h-4 w-4 rounded-full border-2 border-line bg-background" />} ticket={nextTicket} status="menunggu" />
      </View>

      <View className="flex-row gap-3">
        <AppButton label="Skip" variant="outline" className="flex-1" onPress={onSkip} />
        <AppButton label="Next" variant="primary" className="flex-1" onPress={onNext} />
      </View>
    </View>
  );
}

function TimelineRow({
  dot,
  ticket,
  status,
  first = false,
  last = false,
  muted = false,
  current = false,
}: {
  dot: React.ReactNode;
  ticket: LiveTicket | null;
  status: string;
  first?: boolean;
  last?: boolean;
  muted?: boolean;
  current?: boolean;
}) {
  return (
    <View className="flex-row">
      {/* Kolom titik: potongan garis atas + titik + potongan garis bawah → garis menyambung antar baris. */}
      <View className="w-9 items-center">
        <View className={cn('min-h-3 w-0.5 flex-1', first ? 'bg-transparent' : 'bg-line')} />
        {dot}
        <View className={cn('min-h-3 w-0.5 flex-1', last ? 'bg-transparent' : 'bg-line')} />
      </View>
      <View className={cn('flex-1 justify-center gap-0.5', current ? 'py-3' : 'py-2.5')}>
        <Text className={cn(
            current ? 'text-[18px] font-extrabold' : 'text-[13px] font-semibold',
            muted ? 'text-placeholder' : 'text-foreground',
          )}>
          {ticket ? (ticket.name ? `${ticket.number} · ${ticket.name}` : ticket.number) : '-'}
        </Text>
        <Text className={cn(
          current ? 'font-bold text-primary' : 'font-normal text-muted',
          'text-[11px]',
        )}>
          {ticket ? status : 'belum ada antrian'}
        </Text>
      </View>
    </View>
  );
}

/* ------------------------- PC: papan display bandara ------------------------- */

function DesktopAirportBoardLayout({
  counterName,
  previousTicket,
  currentTicket,
  nextTicket,
  onSkip,
  onNext,
}: LayoutProps) {
  return (
    <View className="items-center gap-5">
      <View className="w-full max-w-[680px] gap-7 rounded-[20px] bg-board px-12 py-10">
        <View className="flex-row items-start justify-between">
          {/* TODO: backend belum mendukung ini (counter/loket) */}
          <Text className="text-[18px] tracking-[1px] text-boardMuted" style={{ fontFamily: Fonts?.mono }}>{counterName.toLowerCase()}</Text>
          <View className="flex-row items-center gap-2">
            <LiveDot />
            <Text className="text-[16px] text-liveGreen" style={{ fontFamily: Fonts?.mono }}>live</Text>
          </View>
        </View>

        <View className="h-px bg-boardLine" />

        <View className="items-center gap-2.5 py-2">
          <Text
            className="text-[96px] font-normal leading-[108px] tracking-[2px] text-white"
            style={{ fontFamily: Fonts?.mono }}>{currentTicket?.number ?? '---'}</Text>
          <Text className="text-[22px] text-boardMuted" style={{ fontFamily: Fonts?.mono }}>{currentTicket ? (currentTicket.name ?? 'Sedang dilayani') : 'Belum ada antrian'}</Text>
        </View>

        <View className="h-px bg-boardLine" />

        <View className="flex-row items-start justify-between">
          <BoardSide label="sebelumnya" ticket={previousTicket} />
          <BoardSide label="berikutnya" ticket={nextTicket} alignRight />
        </View>
      </View>

      <View className="w-full max-w-[480px] flex-row gap-3">
        <AppButton label="Skip" variant="outline" className="flex-1" onPress={onSkip} />
        <AppButton label="Next" variant="primary" className="flex-1" onPress={onNext} />
      </View>
    </View>
  );
}

function BoardSide({
  label,
  ticket,
  alignRight = false,
}: {
  label: string;
  ticket: LiveTicket | null;
  alignRight?: boolean;
}) {
  return (
    <View className={cn('gap-1', alignRight ? 'items-end' : 'items-stretch')}>
      <Text className="text-[18px] tracking-[1px] text-boardDim" style={{ fontFamily: Fonts?.mono }}>{label}</Text>
      <Text className="text-[22px] text-boardMuted" style={{ fontFamily: Fonts?.mono }}>{ticket?.number ?? '---'}</Text>
    </View>
  );
}

/** Titik hijau berdenyut (khusus makna status "live"). */
function LiveDot() {
  const pulse = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const loop = Animated.loop(
      Animated.timing(pulse, {
        toValue: 1,
        duration: 1400,
        easing: Easing.out(Easing.ease),
        useNativeDriver: Platform.OS !== 'web',
      }),
    );
    loop.start();
    return () => loop.stop();
  }, [pulse]);

  return (
    <View className="h-3.5 w-3.5 items-center justify-center">
      <Animated.View
        style={[
          styles.livePulse,
          {
            opacity: pulse.interpolate({ inputRange: [0, 1], outputRange: [0.6, 0] }),
            transform: [{ scale: pulse.interpolate({ inputRange: [0, 1], outputRange: [1, 2.6] }) }],
          },
        ]}
      />
      <View className="h-2.5 w-2.5 rounded-full bg-liveGreen" />
    </View>
  );
}

const styles = StyleSheet.create({
  // Animated.View bukan komponen NativeWind: tetap memakai style.
  livePulse: {
    position: 'absolute',
    width: 10,
    height: 10,
    borderRadius: 5,
    backgroundColor: AppColors.liveGreen,
  },
});
