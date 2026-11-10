/**
 * QueueTimeline: kartu gelap berisi timeline nomor antrian dan tombol Skip, Next, serta catatan untuk owner.
 *
 * Dipakai oleh : app/(owner)/dashboard.tsx, components/QueueDetail.tsx, app/(owner)/queues/[queueId]/index.tsx
 * Bergantung   : utils/cn
 * Peran        : owner
 * Catatan      : komponen presentasi; aksi Skip dan Next dijalankan oleh pemanggil, dan nomor sudah diformat sebelum masuk.
 */
import { Pressable, Text, View } from "react-native";

import { cn } from "@/utils/cn";

/** Satu baris timeline nomor antrian. */
export type TimelineItem = {
  /** Nomor tiket sudah diformat (mis. "A-041"). */
  number: string;
  /** Keadaan nomor: done (selesai), current (sedang dilayani, disorot), waiting (menunggu), skipped (dilewati), atau cancelled (dibatalkan). */
  status: "done" | "current" | "waiting" | "skipped" | "cancelled";
};

/** Props kartu timeline antrian. */
export type QueueTimelineProps = {
  /** Judul kartu, mis. nama antrian. */
  title: string;
  /** Urutan nomor dari atas ke bawah; setiap item menjadi satu baris timeline. */
  items: TimelineItem[];
  /** Dipanggil saat tombol Skip ditekan. Tombol nonaktif bila tidak diberikan. */
  onSkip?: () => void;
  /** Dipanggil saat tombol Next ditekan. Tombol nonaktif bila tidak diberikan. */
  onNext?: () => void;
  /** Buka popup catatan untuk antrian ini. */
  onNote?: () => void;
  /** Ketuk bagian kartu (judul dan timeline) untuk membuka detail antrian. */
  onOpen?: () => void;
  /** true saat aksi Next berjalan; label tombol berubah menjadi "..." dan tombol nonaktif. Default false. */
  nextLoading?: boolean;
  /** true saat aksi Skip berjalan; label tombol berubah menjadi "..." dan tombol nonaktif. Default false. */
  skipLoading?: boolean;
};

const STATUS_TEXT: Record<TimelineItem["status"], string> = {
  done: "selesai",
  current: "sedang dilayani",
  waiting: "menunggu",
  skipped: "dilewati",
  cancelled: "dibatalkan",
};

/** Daftar timeline tiket (presentational). Dipakai di kartu dashboard dan detail antrian. */
export function TimelineList({ items }: { items: TimelineItem[] }) {
  return (
    <View>
      {items.map((item, index) => {
        // Garis penghubung antar-item tidak digambar setelah item terakhir.
        const last = index === items.length - 1;
        return (
          <View key={`${item.number}-${index}`} className="flex-row">
            <View className="w-6 items-center">
              <View
                className={cn(
                  "h-4 w-4 items-center justify-center rounded-full border-2",
                  item.status === "current" ? "border-primary bg-primary" : "border-[#6b6b6b]",
                )}
              >
                {item.status !== "waiting" && item.status !== "current" ? (
                  <View className="h-1.5 w-1.5 rounded-full bg-[#6b6b6b]" />
                ) : null}
              </View>
              {last ? null : <View className="w-px flex-1 bg-[#3a3a3a]" />}
            </View>
            <View className={cn("flex-1 gap-0.5", last ? "pb-0" : "pb-4")}>
              <Text
                className={cn(
                  "text-[15px] font-bold",
                  item.status === "current" ? "text-white" : "text-[#8a8a8a]",
                )}
              >
                {item.number}
              </Text>
              <Text
                className={cn(
                  "text-[12px]",
                  item.status === "current" ? "text-primary" : "text-[#6b6b6b]",
                )}
              >
                {STATUS_TEXT[item.status]}
              </Text>
            </View>
          </View>
        );
      })}
    </View>
  );
}

/** Baris "Append Note" bergaya kartu gelap (presentational). */
export function AppendNoteRow({ onNote }: { onNote?: () => void }) {
  return (
    <Pressable
      onPress={onNote}
      disabled={!onNote}
      className="flex-row items-center justify-between rounded-[14px] bg-[#1c1c1c] px-4 py-3.5 dark:border dark:border-line active:opacity-80"
    >
      <View className="gap-0.5">
        <Text className="text-[14px] font-bold text-white">Append Note</Text>
        <Text className="text-[12px] text-[#6b6b6b]">Catatan untuk nomor saat ini</Text>
      </View>
      <Text className="text-[22px] font-bold text-primary">+</Text>
    </Pressable>
  );
}

/** Kartu antrian gelap dengan daftar timeline dan tombol Skip/Next (presentational). */
export function QueueTimeline({
  title,
  items,
  onSkip,
  onNext,
  onNote,
  onOpen,
  nextLoading = false,
  skipLoading = false,
}: QueueTimelineProps) {
  // Tombol Skip dan Next nonaktif bila callback tidak ada atau aksinya sedang berjalan, sehingga tidak bisa ditekan dua kali.
  return (
    <View className="w-full gap-5 rounded-[20px] bg-dark p-5 dark:border dark:border-line">
      <Pressable onPress={onOpen} disabled={!onOpen} className="gap-5 active:opacity-80">
        <Text className="text-[16px] font-bold text-white">{title}</Text>
        <TimelineList items={items} />
      </Pressable>

      <AppendNoteRow onNote={onNote} />

      <View className="flex-row gap-3">
        <Pressable
          onPress={onSkip}
          disabled={!onSkip || skipLoading}
          className="h-12 flex-1 items-center justify-center rounded-[10px] border border-[#3a3a3a] active:opacity-80"
        >
          <Text className="text-[15px] font-bold text-white">{skipLoading ? "..." : "Skip"}</Text>
        </Pressable>
        <Pressable
          onPress={onNext}
          disabled={!onNext || nextLoading}
          className="h-12 flex-1 items-center justify-center rounded-[10px] bg-primary active:opacity-80"
        >
          <Text className="text-[15px] font-bold text-white">{nextLoading ? "..." : "Next"}</Text>
        </Pressable>
      </View>
    </View>
  );
}
