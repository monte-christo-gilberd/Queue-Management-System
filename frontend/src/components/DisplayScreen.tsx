import { useEffect, useState } from 'react';
import { Text, View, useWindowDimensions } from 'react-native';

import { DisplayCounterCard } from '@/components/DisplayCounterCard';
import type { DisplayStatus } from '@/hooks/use-display-screen';
import type { DisplaySnapshot } from '@/lib/display';
import { buildCards, formatClock, layoutColumns, numberFontSize } from '@/utils/display';

export type DisplayScreenProps = {
  snapshot: DisplaySnapshot | null;
  status: DisplayStatus;
  /** false: stream dan pengambilan sama-sama basi. Angka tetap tampil, ditandai "Menyambung ulang...". */
  connected: boolean;
  /** Id counter yang nomornya baru berganti. */
  highlighted: number[];
};

const PADDING = 24;
const GAP = 16;
const HEADER_HEIGHT = 96;
const LIST_HEIGHT = 120;

/** Jam kecil di sudut: tanda layar masih hidup. Terpisah supaya detiknya tidak menggambar ulang seluruh layar. */
function Clock() {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 10_000);
    return () => clearInterval(t);
  }, []);
  return <Text className="text-[28px] font-bold text-placeholder">{formatClock(now)}</Text>;
}

function NumberList({ title, numbers }: { title: string; numbers: string[] }) {
  if (numbers.length === 0) return null;
  return (
    <View className="gap-2" style={{ minHeight: LIST_HEIGHT - 16 }}>
      <Text className="text-[20px] font-semibold text-placeholder">{title}</Text>
      <View className="flex-row flex-wrap" style={{ gap: 12 }}>
        {numbers.map((n) => (
          <View key={n} className="rounded-card border border-line px-4 py-2">
            <Text className="text-[32px] font-extrabold text-white">{n}</Text>
          </View>
        ))}
      </View>
    </View>
  );
}

/**
 * Layar publik antrian: satu kartu per counter (mengikuti counter yang ada, bukan angka tetap), lalu daftar nomor
 * berikutnya dan dilewati. Tanpa kontrol apa pun. Bagian dengan 0 nomor tidak digambar sama sekali.
 */
export function DisplayScreen({ snapshot, status, connected, highlighted }: DisplayScreenProps) {
  const { width, height } = useWindowDimensions();

  if (status === 'invalid') {
    return (
      <View className="flex-1 items-center justify-center bg-dark p-8">
        <Text accessibilityRole="alert" className="text-center text-[36px] font-black text-white">
          Tautan layar tidak berlaku
        </Text>
        <Text className="mt-3 text-center text-[18px] text-placeholder">
          Tautan ini salah, sudah diganti, atau layarnya dimatikan oleh pemilik antrian.
        </Text>
      </View>
    );
  }

  if (!snapshot) {
    return (
      <View className="flex-1 items-center justify-center bg-dark p-8">
        <Text className="text-center text-[28px] font-bold text-white">
          {status === 'error' ? 'Menyambung ulang...' : 'Memuat...'}
        </Text>
      </View>
    );
  }

  const closed = snapshot.status === 'closed';
  const cards = buildCards(snapshot);
  const landscape = width >= height;
  const columns = layoutColumns(cards.length, landscape);
  const hasLists = snapshot.upcoming.length > 0 || snapshot.skipped.length > 0;
  const areaWidth = width - PADDING * 2;
  const areaHeight = Math.max(height - PADDING * 2 - HEADER_HEIGHT - (hasLists ? LIST_HEIGHT * (snapshot.upcoming.length > 0 && snapshot.skipped.length > 0 ? 2 : 1) : 0), 160);
  const rows = Math.ceil(cards.length / columns);
  const cardWidth = Math.floor((areaWidth - GAP * (columns - 1)) / columns);
  const cardHeight = Math.floor((areaHeight - GAP * (rows - 1)) / rows);
  const fontSize = numberFontSize({ count: cards.length, areaWidth, areaHeight, landscape });

  return (
    <View className="flex-1 bg-dark" style={{ padding: PADDING, gap: GAP }}>
      <View className="flex-row items-start justify-between" style={{ minHeight: HEADER_HEIGHT - GAP }}>
        <View className="flex-1 pr-4">
          <Text className="text-[44px] font-black text-white" numberOfLines={1}>
            {snapshot.queue_name}
          </Text>
          {snapshot.queue_desc ? (
            <Text className="text-[20px] text-placeholder" numberOfLines={1}>
              {snapshot.queue_desc}
            </Text>
          ) : null}
        </View>
        <Clock />
      </View>

      {!connected ? (
        <View accessibilityRole="alert" className="rounded-card bg-live px-4 py-2">
          <Text className="text-center text-[22px] font-bold text-white">Menyambung ulang...</Text>
        </View>
      ) : null}
      {closed ? (
        <View accessibilityRole="alert" className="rounded-card border border-line px-4 py-2">
          <Text className="text-center text-[22px] font-bold text-white">Antrian ditutup</Text>
        </View>
      ) : null}

      <View className="flex-row flex-wrap" style={{ gap: GAP }}>
        {cards.map((card) => (
          <DisplayCounterCard
            key={card.key}
            title={card.title}
            number={closed ? null : card.number}
            width={cardWidth}
            minHeight={cardHeight}
            numberFontSize={fontSize}
            highlighted={highlighted.includes(card.id)}
          />
        ))}
      </View>

      <NumberList title="Berikutnya" numbers={snapshot.upcoming} />
      <NumberList title="Dilewati" numbers={snapshot.skipped} />
    </View>
  );
}
